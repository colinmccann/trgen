package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const (
	defaultMaxHops = 12
	defaultTimeout = 1 * time.Second // how long to wait for a response before going to next hop
	defaultInfile  = "data/traceroute_targets.txt"
	defaultOutfile = "data/results.txt"
)

// should these structs be interfaces?
// TODO - are these terms too generic? ctrdTraceroute, ctrdHop, etc - YES, fix this!
// outputType should a be restricted set of options
type CTRDSession struct {
	maxHops     int
	timeout     time.Duration
	outputType  string
	outputPath  string
	localIP     net.IP
	traceroutes []CTRDTraceroute
}

type CTRDTraceroute struct {
	originIP            net.IP
	destinationIP       net.IP
	destinationHostname string
	hops                []CTRDHop
}

type CTRDHop struct {
	num      int
	ip       string
	hostname string
	latency  time.Duration
}

func main() {
	/*********************** Misc / config ****************************/
	// TODO - create an init/setup func, put this in
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	/*********************** Flags ****************************/
	maxHops := flag.Int("m", defaultMaxHops, "Max hops in the traceroute (ie max ttl)")
	timeout := flag.Duration("t", defaultTimeout, "Timeout to wait for an answer in one hop")
	trTarget := flag.String("u", "", "Traceroute target (url or ip)")
	infile := flag.Bool("i", false, "Set to allow an input file")
	infilePath := flag.String("ipath", defaultInfile, "Specify path to traceroute targets input file")
	outfile := flag.Bool("o", false, "Set to allow an output file")
	outfilePath := flag.String("opath", defaultOutfile, "Specify path to results output file")
	flag.Parse()

	/*********************** Session ****************************/

	// what other vals go in here?
	// - session time
	session := CTRDSession{
		maxHops: *maxHops,
		timeout: *timeout,
		localIP: getLocalIP(),
	}

	/*********************** I/O ****************************/
	// TODO - this still feels weird, too global objecty. Is it not better to have a return here?
	handleInput(&session, *trTarget, *infile, *infilePath)
	handleOutput(&session, *outfile, *outfilePath)

	/****************** TRACEROUTE STARTS HERE *******************/

	for _, tr := range session.traceroutes {
		if session.outputType == "terminal" {
			writeTracerouteHeadersToOutput(tr)
		}
		trace(&session, &tr)
	}

	writeSessionToOutput(&session)

	/*********************** Cleanup and exit ****************************/
	os.Exit(0)
}

func trace(session *CTRDSession, tr *CTRDTraceroute) {

	/*********************** Networking component ****************************/

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

	for i := 1; i <= session.maxHops; i++ {
		// set the time to live
		icmpConn.IPv4PacketConn().SetTTL(i)

		// start the clock for the RTT
		startTime := time.Now()

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

		// "62.141.54.25" - heisse.de
		// "142.1.217.155" - ixmaps.ca
		// "140.82.114.3" - github.com
		icmpConn.WriteTo(wb, &net.UDPAddr{IP: tr.destinationIP, Zone: "en0"})

		readBuffer := make([]byte, 1500)

		if err := icmpConn.SetDeadline(time.Now().Add(session.timeout)); err != nil {
			// TODO - clean up this error handling
			fmt.Fprintf(os.Stderr, "Could not set the read timeout on the ipv4 socket: %s\n", err)
			os.Exit(1)
		}

		n, peer, err := icmpConn.ReadFrom(readBuffer)
		// this is where it's dying. Is it something with the wrong interface? Or ICMP is rejected?
		// the ReadFrom never completes, n = 0, peer = nil
		// this doesn't exactly work. Once it hits something non-responsive, it just continues forever until hitting max hops
		if err != nil {
			tr.hops[i-1] = CTRDHop{
				num:      i,
				ip:       "*",
				hostname: "*",
			}

			// hop.ip, hop.hostname, hop.latency = "*", "*"
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
		hostname, _ := lookupIpHostname(ip)

		tr.hops[i-1] = CTRDHop{
			num:      i,
			ip:       ip,
			hostname: hostname,
			latency:  latency,
		}

		if session.outputType == "terminal" {
			writeHopToOutput(tr.hops[i-1])
		}

		// TODO - check this
		if icmpAnswer.Type == ipv4.ICMPTypeEchoReply {
			fmt.Println("Traceroute complete")
			break
		}

		// TODO - remove zero values from the hops slice? The hops slice is maxHops long, with unassigned values at the end
		// tr.hops = tr.hops(:)
	}
}

// for a given IP or hostname string target, return the IP, hostname and err
// modelled off https://github.com/microsoft/ethr
func IPLookup(target string) (net.IP, string, error) {
	var ipAddr net.IP
	var ipStr string

	// if it's an IP
	ip := net.ParseIP(target)
	if ip != nil {
		ipAddr = ip
		ipStr = target
		return ipAddr, ipStr, nil
	}

	// if it's a hostname
	// TODO - this isn't clean enough. Try with various structures
	// remove the schema if exists
	u, _ := url.Parse(target)
	hostname := ""
	if u.Hostname() != "" {
		hostname = u.Hostname()
	} else {
		hostname = target
	}

	ips, err := net.LookupIP(hostname)
	if err != nil {
		fmt.Printf("Failed to lookup IP address for the target: %v. Error: %v", target, err)
		return ipAddr, ipStr, err
	}
	for _, ip := range ips {
		// check for ipv4 and ipv6 here
		// if gIPVersion == ethrIPAny || (gIPVersion == ethrIPv4 && ip.To4() != nil) || (gIPVersion == ethrIPv6 && ip.To16() != nil) {
		ipAddr = ip
		ipStr = ip.String()
		fmt.Printf("Resolved target: %v to IP address: %v\n", target, ip)
		return ipAddr, ipStr, nil
	}
	fmt.Printf("Unable to resolve the given target: %v to an IP address.", target)
	return ipAddr, ipStr, os.ErrNotExist
}

// TODO: having both of these makes no sense.
// maybe want lookupIPForHostname + lookupHostnameForIP?
func lookupIpHostname(addr string) (string, string) {
	name := ""
	tname := ""
	if addr == "" {
		return tname, name
	}
	// fmt.Println(addr)
	names, err := net.LookupAddr(addr)
	// fmt.Println(len(names))
	if err == nil && len(names) > 0 {

		name = names[0]
		sz := len(name)

		if sz > 0 && name[sz-1] == '.' {
			name = name[:sz-1]
		}
		// tname = truncateStringFromEnd(name, 16)
		tname = name
	}
	return tname, name
}
