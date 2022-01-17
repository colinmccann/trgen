package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

/*********************** Input ****************************/

// handleInput handles traceroute targets, depending on user input flags. Options are:
// - single user inputted url
// - input file
// - external (web) source
func handleInput(session *CTRDSession, target string, infile bool, infilePath string) []string {
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

	return targets
}

// parseInfile parses a file for traceroute targets
func parseInfile(infile string) []string {
	f, err := os.Open(infile)
	check(err)
	defer f.Close()

	scanner := bufio.NewScanner(f)

	// is it better to instantiate a zero length slice, or make it big and then clear out the empty values later? make([]string, 50)
	targets := []string{}
	for scanner.Scan() {
		uri := scanner.Text()
		targets = append(targets, uri)
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

// at this point, this is analogous to writeToFile, but serves as a stub for the future
func writeSessionToOutput(session *CTRDSession) {
	fmt.Printf("\nTraceroute session complete.\n\n")

	// this currently is always true
	if session.OutputType == "file" {
		f, err := os.Create(session.OutputPath)
		check(err)
		defer f.Close()

		writeSessionToFile(f, session)
	}
}

func writeSessionToFile(f *os.File, session *CTRDSession) {
	bs, err := json.Marshal(session)
	if err != nil {
		fmt.Println(err)
	}
	f.WriteString(string(bs))
}

func writeTracerouteToTerminal(tr CTRDTraceroute) {
	bs, err := json.Marshal(tr)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(bs))
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
		fmt.Printf("| %-3d | %-15s | %-50s | %-12s\n", hop.Num, hop.IP, hop.Hostname, time.Duration(hop.Latency).String())
	} else {
		fmt.Printf(".")
	}
}

type msDuration time.Duration

// implementing marshal for our duration
func (d msDuration) MarshalJSON() (b []byte, err error) {
	ms := float64(d) / float64(time.Millisecond)
	return []byte(fmt.Sprintf(`"%v"`, ms)), nil
}
