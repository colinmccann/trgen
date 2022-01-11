package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

/*********************** Input ****************************/

func handleInput(session *CTRDSession, target string, infile bool, infilePath string) {
	var targets []string

	// if the user entered a single target
	// else if the user entered the path to their input file
	// 	 OR if the user has entered nothing, use the default file path
	// else err
	if target != "" {
		targets = append(targets, target)
	} else if infilePath != "" {
		targets = parseInfile(infilePath)
	} else {
		log.Fatal("No input file or traceroute target specified")
		os.Exit(1)
	}

	for _, target := range targets {
		ip, _, err := IPLookup(target)
		check(err)

		tr := CTRDTraceroute{
			destinationIP:       ip,
			destinationHostname: target,
			originIP:            session.localIP,
			hops:                make([]CTRDHop, session.maxHops),
		}
		session.traceroutes = append(session.traceroutes, tr)
	}
}

func parseInfile(infile string) []string {
	f, err := os.Open(infile)
	check(err)
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

/*********************** Output ****************************/

// these are mostly stubs for when we eventually write via eg a websocket
func handleOutput(session *CTRDSession, outfile bool, outfilePath string) {
	// handle output to websocket

	// explicit check on outfile, given that the path has a default
	if outfile {
		session.outputType = "file"
		session.outputPath = outfilePath
	} else {
		session.outputType = "terminal"
	}
}

func writeSessionToOutput(session *CTRDSession) {
	fmt.Printf("Traceroute session complete.\n\n")

	// this currently is always true, but there will be future options such as websockets
	if session.outputType == "file" {
		f, err := os.Create(session.outputPath)
		check(err)
		defer f.Close()

		for _, tr := range session.traceroutes {
			writeTracerouteHeadersToFile(f, tr)
			for _, hop := range tr.hops {
				writeHopToFile(f, hop)
			}
		}
	}
}

func writeTracerouteHeadersToFile(f *os.File, tr CTRDTraceroute) {
	fmt.Printf("TR origin: %v", tr.originIP)
	f.WriteString(fmt.Sprintf("%v\n", tr.originIP))
	f.WriteString(tr.destinationHostname + "\n")
	f.WriteString(fmt.Sprintf("%v\n", tr.destinationIP))
	f.WriteString(strings.Repeat("-", 90) + "\n")
	f.WriteString("Hop      IP      Hostname      Latency" + "\n")
}

func writeHopToFile(f *os.File, hop CTRDHop) {
	f.WriteString(strconv.Itoa(hop.num) + "   " + hop.ip + "   " + hop.hostname + "   " + fmt.Sprintf("%v", hop.latency) + "\n")
}

func writeTracerouteHeadersToOutput(tr CTRDTraceroute) {
	fmt.Printf("\nOrigin IP: %v\n", tr.originIP)
	fmt.Printf("Destination hostname: %v\n", tr.destinationHostname)
	fmt.Printf("Destination IP: %v\n", tr.destinationIP)
	fmt.Println(strings.Repeat("-", 90))
	fmt.Printf("| %-3s | %-15s | %-50s | %-12s\n", "Hop", "IP", "Hostname", "Latency")
}

func writeHopToOutput(hop CTRDHop) {
	fmt.Printf("| %-3d | %-15s | %-50s | %-12s\n", hop.num, hop.ip, hop.hostname, hop.latency)
}
