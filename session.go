package main

import (
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func (session *CTRDSession) runSession() {
	for i, tr := range session.Traceroutes {
		printTracerouteMetadataToTerminal(tr)
		if session.OutputType == Terminal {
			printTracerouteHeadersToTerminal(tr)
		}
		trace(session, &session.Traceroutes[i])
	}
}

/* TODO - something is up here, and I think it relates to a misunderstanding I have
   Why can't I pass a reference to tr, why does it need to be &session.Traceroutes[i]?
   - is the tr referenced somehow different from the tr in the session?
   Why do I need to trwrite tr[i-1] = Hop{}
*/
func trace(session *CTRDSession, tr *CTRDTraceroute) {
	tr.StartedAt = time.Now().UTC()
	defer func() {
		tr.EndedAt = time.Now().UTC()
	}()
	// open up the listening address for returning ICMP packets
	// if we're going to do multiple TRs concurrently, we'll have to open multiple of these, right?
	icmpConn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	// icmpConn, err := net.ListenPacket("ip6:ipv6-icmp", "::")
	// icmpConn.IPv6PacketConn().SetHopLimit(i)
	// ipv6_sock := ipv6.NewPacketConn(icmpConn)
	// defer ipv6_sock.Close()
	// ipv6_sock.SetHopLimit(i)
	if err != nil {
		logError(err.Error())
	}
	defer icmpConn.Close()

	wm := icmp.Message{
		Type: ipv4.ICMPTypeEcho, Code: 0,
		Body: &icmp.Echo{
			ID: os.Getpid() & 0xffff, Seq: 1,
			Data: []byte("CTRD says hello!"),
		},
	}
	wb, err := wm.Marshal(nil)
	if err != nil {
		logError(err.Error())
	}

	for i := 1; i <= session.MaxHops; i++ {
		// set the time to live
		tr.Hops[i-1].Num = i
		icmpConn.IPv4PacketConn().SetTTL(i)

		// setting a deadline so that the WriteTo and ReadFrom timeout on failure
		if err := icmpConn.SetDeadline(time.Now().Add(time.Duration(session.Timeout))); err != nil {
			logError(err.Error())
			// TODO - is os.Exit this really justified here?
			os.Exit(1)
		}

		var (
			RTTs          []msDuration
			peer          net.Addr
			n             int
			readFromError error
		)
		readBuffer := make([]byte, 1500)
		// this can also be done in go routines? try it, see what would happen
		for j := 1; j <= session.NumAttempts; j++ {
			// start the clock for the RTT
			startTime := time.Now()

			// send the msg off to the hop target
			icmpConn.WriteTo(wb, &net.UDPAddr{IP: tr.DestinationIP, Zone: "en0"})

			// read the response
			n, peer, readFromError = icmpConn.ReadFrom(readBuffer)
			if readFromError != nil {
				// RTTs = append(RTTs, "*")
				continue
			}

			// finish line for the RTT
			RTTs = append(RTTs, msDuration(time.Since(startTime)))
		}

		// if the target doesn't respond
		if readFromError != nil {
			tr.Hops[i-1].IP = "*"
			tr.Hops[i-1].Hostname = "*"
			printHopToOutput(session, tr.Hops[i-1])
			continue
		}

		// split off the port, since it'll choke the DNS lookup
		// ip is a string for now, set to something stricter when we clean up the IP - hostname / port DNS conversions
		ip, _, err := net.SplitHostPort(peer.String())
		if err != nil {
			logError(err.Error())
		}
		hostname, err := lookupHostnameForIP(ip)
		if err != nil {
			logError(err.Error())
		}

		tr.Hops[i-1].IP = ip
		tr.Hops[i-1].Hostname = hostname
		tr.Hops[i-1].RTTs = RTTs

		printHopToOutput(session, tr.Hops[i-1])

		// how did the hop respond? This is used to decide if we're at the end of the set of hops
		icmpAnswer, err := icmp.ParseMessage(1, readBuffer[:n])
		if err != nil {
			logError(err.Error())
		}
		if icmpAnswer.Type == ipv4.ICMPTypeEchoReply {
			tr.Length = i
			tr.Terminated = true
			tr.Hops = tr.Hops[:i]

			logInfo("\nTraceroute reached destination")
			break
		}
	}

}
