package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"regexp"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const (
	defaultMaxHops             = 12
	defaultTimeout             = 1 * time.Second // how long to wait for a response before going to next hop
	defaultTracerouteTargetSet = "data/traceroute_targets.txt"
	// const defaultOutput
)

// TODO - we may want a struct sitting above these to handle the session
// should these structs be interfaces?
// TODO - are these terms too generic? ctrdTraceroute, ctrdHop, etc
type traceroute struct {
	// originIP			net.IP
	destinationIP       net.IP
	destinationHostname string
	hops                []hop
}

type hop struct {
	num      int
	ip       net.IP
	hostname string
	latency  float32
}

func main() {
	/*********************** Misc / config ****************************/
	// TODO - create an init/setup func, put this in
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	/*********************** Flags ****************************/
	maxHops := flag.Int("m", defaultMaxHops, "Max hops in the traceroute (ie max ttl)")
	timeout := flag.Duration("t", defaultTimeout, "Timeout to wait for an answer in one hop")
	// trTarget := flag.String("u", "", "Traceroute target (url or ip)")
	flag.Parse()

	// fmt.Println(*maxHops)
	// fmt.Println(*timeout)

	/*********************** Input ****************************/
	targets := parseInfile()

	/****************** TRACEROUTE STARTS HERE *******************/

	/*********************** Destination parsing ****************************/

	for _, target := range targets {
		ip, hostname, err := IPLookup(target)
		check(err)

		tr := traceroute{
			destinationIP:       ip,
			destinationHostname: hostname,
			hops:                make([]hop, *maxHops),
		}
		trace(tr, *maxHops, *timeout)
	}
}

func trace(tr traceroute, maxHops int, timeout time.Duration) {

	/*********************** Traceroute ****************************/

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

	for i := 1; i <= maxHops; i++ {
		// set the time to live
		icmpConn.IPv4PacketConn().SetTTL(i)

		// start the clock for the RTT
		startTime := time.Now()

		wm := icmp.Message{
			Type: ipv4.ICMPTypeEcho, Code: 0,
			Body: &icmp.Echo{
				ID: os.Getpid() & 0xffff, Seq: 1,
				Data: []byte("CTRD probing..."),
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

		// hop# | url | ip | rtt1 | rtt2 | rtt3

		readBuffer := make([]byte, 1500)

		if err := icmpConn.SetDeadline(time.Now().Add(timeout)); err != nil {
			// TODO - clean up this error handling
			fmt.Fprintf(os.Stderr, "Could not set the read timeout on the ipv4 socket: %s\n", err)
			os.Exit(1)
		}

		n, peer, err := icmpConn.ReadFrom(readBuffer)
		// this is where it's dying. Is it something with the wrong interface? Or ICMP is rejected?
		// the ReadFrom never completes, n = 0, peer = nil
		// this doesn't exactly work. Once it hits something non-responsive, it just continues forever until hitting max hops
		// IS THIS STILL A PROBLEM?
		if err != nil {
			fmt.Printf("%v \t* \t* \t*\n", i)
			continue
		}

		// split off the port, since it'll choke the DNS lookup
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

		// we won't need this in the long run
		hostname, _ := lookupIpHostname(ip)

		fmt.Printf("%v | %v | %+v | %v\n", i, ip, hostname, latency)

		if icmpAnswer.Type == ipv4.ICMPTypeEchoReply {
			fmt.Println("Traceroute complete")
			break
		}
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

func check(e error) {
	if e != nil {
		log.Fatal(e)
	}
}

const URIPattern string = `^((ftp|http|https):\/\/)?(\S+(:\S*)?@)?((([1-9]\d?|1\d\d|2[01]\d|22[0-3])(\.(1?\d{1,2}|2[0-4]\d|25[0-5])){2}(?:\.([0-9]\d?|1\d\d|2[0-4]\d|25[0-4]))|(((([a-z\x{00a1}-\x{ffff}0-9]+-?-?_?)*[a-z\x{00a1}-\x{ffff}0-9]+)\.)?)?(([a-z\x{00a1}-\x{ffff}0-9]+-?-?_?)*[a-z\x{00a1}-\x{ffff}0-9]+)(?:\.([a-z\x{00a1}-\x{ffff}]{2,}))?)|localhost)(:(\d{1,5}))?((\/|\?|#)[^\s]*)?$`

func validateURI(uri string) bool {
	pattern := URIPattern
	match, err := regexp.MatchString(pattern, uri)
	check(err)
	return match
}

func parseInfile() []string {
	f, err := os.Open("data/traceroute_targets.txt")
	check(err)
	// do I need to defer this?
	defer f.Close()
	scanner := bufio.NewScanner(f)

	// is it better to instantiate a zero length slice, or make it big and then clear out the empty values later? make([]string, 50)
	targets := []string{}
	// it might also be better to create the output struct (session struct) here, since we're going to have to do it anyways...
	fmt.Println("Validating traceroute targets")
	for scanner.Scan() {
		uri := scanner.Text()
		// check for uri validity
		// check for other things?
		if validateURI(uri) {
			targets = append(targets, uri)
		} else {
			fmt.Printf("Found non-valid traceroute target '%v', skipping...\n", uri)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Fatal(err)
	}
	return targets
}
