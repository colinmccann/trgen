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

func (session *CTRDSession) runSession() {
	for _, tr := range session.Traceroutes {
		writeTracerouteMetadataToTerminal(tr)
		if session.OutputType == "terminal" {
			writeTracerouteHeadersToTerminal(tr)
		}
		trace(session, &tr)
	}

	writeSessionToOutput(session)
}

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
		// fmt.Printf("wb: %+v\n", string(wb))

		// start the clock for the RTT
		startTime := time.Now()

		// setting this so that the WriteTo and ReadFrom timeout on failure
		if err := icmpConn.SetDeadline(time.Now().Add(session.Timeout)); err != nil {
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
			tr.Hops[i-1] = CTRDHop{
				Num:      i,
				Ip:       "*",
				Hostname: "*",
			}
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
		// handle err
		hostname, _ := lookupHostnameForIP(ip)

		tr.Hops[i-1] = CTRDHop{
			Num:      i,
			Ip:       ip,
			Hostname: hostname,
			Latency:  latency,
		}

		writeHopToOutput(session, tr.Hops[i-1])

		// TODO - check this
		if icmpAnswer.Type == ipv4.ICMPTypeEchoReply {
			fmt.Println("\nTraceroute reached destination")
			break
		}
	}

	// TODO - remove zero values from the hops slice? The hops slice is maxHops long, with unassigned values at the end
	// tr.hops = tr.hops(:)
}
