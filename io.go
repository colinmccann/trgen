package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// this is populated with the -b flag. To be used in comparison with the trgen_validator.sh
// to confirm that this trgen creates the same TR outputs as an OS level traceroute
const validatorOutputFile = "data/validator_trgen_output.txt"

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
		logError("No input file or traceroute target specified")
		os.Exit(1)
	}

	return targets
}

// parseInfile parses a file for traceroute targets
func parseInfile(infile string) []string {
	f, err := os.Open(infile)
	if err != nil {
		logError(err.Error())
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	// is it better to instantiate a zero length slice, or make it big and then clear out the empty values later? make([]string, 50)
	targets := []string{}
	for scanner.Scan() {
		uri := scanner.Text()
		targets = append(targets, uri)
	}
	if err := scanner.Err(); err != nil {
		logError(err.Error())
	}

	return targets
}

/*********************** Output ****************************/

// these are mostly stubs for when we eventually write via eg a websocket
func handleOutput(session *CTRDSession, outfile bool, outfilePath string) {
	// handle output to server/websocket

	// explicit check on outfile, given that the path has a default
	if outfile {
		session.OutputType = File
		session.OutputPath = outfilePath
	} else {
		session.OutputType = Terminal
	}
}

// at this point, this is analogous to printToFile, but serves as a stub for the future
func printSessionToOutput(session *CTRDSession) {
	fmt.Println("Traceroute session complete.")

	// this currently is always true
	if session.OutputType == File {
		f, err := os.Create(session.OutputPath)
		if err != nil {
			logError(err.Error())
		}
		defer f.Close()

		printSessionToFile(f, session)
	}

	// validate whether these TRs follow the same path as the OS would generate
	if session.ValidatorOutput {
		printSessionToValidatorOutput(session)
	}
}

func printSessionToFile(f *os.File, session *CTRDSession) {
	bs, err := json.Marshal(session)
	if err != nil {
		fmt.Println(err)
	}
	f.WriteString(string(bs))
}

func printTracerouteToTerminal(tr CTRDTraceroute) {
	bs, err := json.Marshal(tr)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println(string(bs))
}

func printTracerouteMetadataToTerminal(tr CTRDTraceroute) {
	logInfo(fmt.Sprintf("Running traceroute to '" + tr.DestinationHostname + "'"))
	logInfo(fmt.Sprintf("Origin IP: %v", tr.OriginIP))
	logInfo(fmt.Sprintf("Destination IP: %v", tr.DestinationIP))
}

func printTracerouteHeadersToTerminal(tr CTRDTraceroute) {
	fmt.Println(strings.Repeat("-", 90))
	fmt.Printf("| %-3s | %-15s | %-50s | %-12s\n", "Hop", "IP", "Hostname", "RTT")
}

func printHopToOutput(session *CTRDSession, hop CTRDHop) {
	if session.OutputType == Terminal {
		fmt.Printf("| %-3d | %-15s | %-50s | %-12s\n", hop.Num, hop.IP, hop.Hostname, time.Duration(hop.RTT).String())
	} else {
		fmt.Printf(".")
	}
}

func printSessionToValidatorOutput(session *CTRDSession) {
	f, err := os.Create(validatorOutputFile)
	if err != nil {
		logError(err.Error())
	}
	defer f.Close()

	for _, tr := range session.Traceroutes {
		f.WriteString(tr.DestinationHostname)
		for _, h := range tr.Hops {
			if h.IP == "*" {
				f.WriteString("\n")
			} else {
				f.WriteString("\n" + h.IP)
			}

		}
		f.WriteString("\n" + strings.Repeat("-", 20) + "\n\n")
	}
}

type msDuration time.Duration

// implementing marshal for our duration
func (d msDuration) MarshalJSON() (b []byte, err error) {
	ms := float64(d) / float64(time.Millisecond)
	return []byte(fmt.Sprintf(`"%v"`, ms)), nil
}
