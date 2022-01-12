package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
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
			DestinationIP:       ip,
			DestinationHostname: target,
			OriginIP:            session.LocalIP,
			Hops:                make([]CTRDHop, session.MaxHops),
		}
		session.Traceroutes = append(session.Traceroutes, tr)
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
		// TODO - can we use another func to do this instead?
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
		session.OutputType = "file"
		session.OutputPath = outfilePath
	} else {
		session.OutputType = "terminal"
	}
}

// at this point, this is analogous to writeToFile, but servers as a stub for the future
func writeSessionToOutput(session *CTRDSession) {
	fmt.Printf("\nTraceroute session complete.\n\n")

	// this currently is always true
	if session.OutputType == "file" {
		f, err := os.Create(session.OutputPath)
		check(err)
		defer f.Close()

		writeTracerouteToFile(f, session)
	}
}

func writeTracerouteToFile(f *os.File, session *CTRDSession) {
	bs, err := json.Marshal(session)
	if err != nil {
		fmt.Println(err)
	}
	f.WriteString(string(bs))
}

func writeTracerouteMetadataToTerminal(tr CTRDTraceroute) {
	fmt.Println("\n\nRunning traceroute to " + tr.DestinationHostname + "...")
	fmt.Printf("Origin IP: %v\n", tr.OriginIP)
	fmt.Printf("Destination IP: %v\n", tr.DestinationIP)
}

func writeTracerouteHeadersToTerminal(tr CTRDTraceroute) {
	fmt.Println(strings.Repeat("-", 90))
	fmt.Printf("| %-3s | %-15s | %-50s | %-12s\n", "Hop", "IP", "Hostname", "Latency")
}

func writeHopToOutput(session *CTRDSession, hop CTRDHop) {
	if session.OutputType == "terminal" {
		fmt.Printf("| %-3d | %-15s | %-50s | %-12s\n", hop.Num, hop.Ip, hop.Hostname, hop.Latency)
	} else {
		fmt.Printf(".")
	}
}
