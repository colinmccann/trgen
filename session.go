package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// runSession handles the everything btw in and out
func (session *CTRDSession) runSession() {
	for i, tr := range session.Traceroutes {
		writeTracerouteMetadataToTerminal(tr)
		if session.OutputType == "terminal" {
			writeTracerouteHeadersToTerminal(tr)
		}
		trace(session, &session.Traceroutes[i])
	}

	// cleanup(session) - or addMetadata?
	writeSessionToOutput(session)
}

/* TODO - something is up here, and I think it relates to a misunderstanding I have
   Why can't I pass a reference to tr, why does it need to be &session.Traceroutes[i]?
   - is the tr referenced somehow different from the tr in the session?
   Why do I need to trwrite tr[i-1] = Hop{}
*/
func trace(session *CTRDSession, tr *CTRDTraceroute) {
	// open up the listening address for returning ICMP packets
	// if we're going to do multiple TRs concurrently, we'll have to open multiple of these, right?
	icmpConn, err := icmp.ListenPacket("udp4", "0.0.0.0")
	// icmpConn, err := net.ListenPacket("ip6:ipv6-icmp", "::")
	// icmpConn.IPv6PacketConn().SetHopLimit(i)
	// ipv6_sock := ipv6.NewPacketConn(icmpConn)
	// defer ipv6_sock.Close()
	// ipv6_sock.SetHopLimit(i)
	if err != nil {
		log.Fatal(err)
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
		log.Fatal(err)
	}

	for i := 1; i <= session.MaxHops; i++ {
		// set the time to live
		icmpConn.IPv4PacketConn().SetTTL(i)

		// start the clock for the RTT
		startTime := time.Now()

		// setting this so that the WriteTo and ReadFrom timeout on failure
		if err := icmpConn.SetDeadline(time.Now().Add(time.Duration(session.Timeout))); err != nil {
			// TODO - clean up this error handling
			fmt.Fprintf(os.Stderr, "Could not set the read timeout on the ipv4 socket: %s\n", err)
			os.Exit(1)
		}

		// "62.141.54.25" - heisse.de
		// "142.1.217.155" - ixmaps.ca
		// "140.82.114.3" - github.com
		icmpConn.WriteTo(wb, &net.UDPAddr{IP: tr.DestinationIP, Zone: "en0"})

		readBuffer := make([]byte, 1500)

		n, peer, err := icmpConn.ReadFrom(readBuffer)
		// fmt.Printf("N: %v, Peer: %v", n, peer)
		// this is where it's dying. Is it something with the wrong interface? Or ICMP is rejected?
		// the ReadFrom never completes, n = 0, peer = nil
		// this doesn't exactly work. Once it hits something non-responsive, it just continues forever until hitting max hops
		if err != nil {
			// tr.Hops[i-1] = CTRDHop{
			// 	Num:      i,
			// 	Ip:       "*",
			// 	Hostname: "*",
			// }
			tr.Hops[i-1].Num = i
			tr.Hops[i-1].IP = "*"
			tr.Hops[i-1].Hostname = "*"
			writeHopToOutput(session, tr.Hops[i-1])
			continue
		}

		// split off the port, since it'll choke the DNS lookup
		// ip is a string for now, set to something stricter when we clean up the IP - hostname / port DNS conversions
		ip, _, err := net.SplitHostPort(peer.String())
		if err != nil {
			log.Fatal(err)
		}

		icmpAnswer, err := icmp.ParseMessage(1, readBuffer[:n])
		if err != nil {
			log.Fatal(err)
		}

		// finish line for the RTT
		// TODO - is this the right place to put this?
		latency := time.Since(startTime)
		// latency := msDuration(time.Since(startTime))
		// handle err
		hostname, _ := lookupHostnameForIP(ip)

		// tr.Hops[i-1] = CTRDHop{
		// 	Num:      i,
		// 	Ip:       ip,
		// 	Hostname: hostname,
		// 	Latency:  latency,
		// }
		tr.Hops[i-1].Num = i
		tr.Hops[i-1].Ip = ip
		tr.Hops[i-1].Hostname = hostname
		tr.Hops[i-1].Latency = msDuration(latency)

		writeHopToOutput(session, tr.Hops[i-1])

		// TODO - check this
		// end of the line for this traceroute
		if icmpAnswer.Type == ipv4.ICMPTypeEchoReply {
			tr.Length = i
			tr.Terminated = true
			tr.Hops = tr.Hops[:i]

			fmt.Println("\nTraceroute reached destination")
			break
		}
	}

}

// cleanup(*session CTRDSession) {
// 	// remove zero values from TRs
// 	for _, tr := range session.traceroutes {

// 	}
// 	// add tr metadata
// 	// - length
// 	// - terminated
// 	// - ?
// }
