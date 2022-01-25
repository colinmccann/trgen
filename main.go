package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"
)

const (
	defaultMaxHops = 24
	defaultTimeout = 1 * time.Second // how long to wait for a response before going to next hop
	defaultInfile  = "data/traceroute_targets.txt"
	defaultOutfile = "data/results.txt"
	defaultLogfile = "log/out.log"
)

// what other vals go in here?
// - submitterName
// - submitterIp
// - submitterPostCode
type CTRDSession struct {
	MaxHops         int              `json:"maxHops"`
	Timeout         msDuration       `json:"timeOut"`
	OutputType      OutputType       `json:"outputType"`
	OutputPath      string           `json:"outputPath"`
	LocalIP         net.IP           `json:"localIP"`
	LogLevel        LogLevel         `json:"logLevel"`
	StartedAt       time.Time        `json:"startedAt"`
	EndedAt         time.Time        `json:"endedAt"`
	ValidatorOutput bool             `json:"validatorOutput"`
	Traceroutes     []CTRDTraceroute `json:"traceroutes"`
}

// what other vals go in here?
type CTRDTraceroute struct {
	OriginIP            net.IP    `json:"originIP"`
	DestinationIP       net.IP    `json:"destinationIP"`
	DestinationHostname string    `json:"destinationHostname"`
	Length              int       `json:"length"`
	Terminated          bool      `json:"terminated"`
	StartedAt           time.Time `json:"startedAt"`
	EndedAt             time.Time `json:"endedAt"`
	Hops                []CTRDHop `json:"hops"`
}

// what other vals go in here?
// - AttemptNum - or does an attempt have it's own struct? Probably...
// - RTT is better than latency?
// - minRTT
type CTRDHop struct {
	Num      int        `json:"num"`
	IP       string     `json:"ip"`
	Hostname string     `json:"hostname"`
	RTT      msDuration `json:"RTT"`
}

type OutputType int

const (
	Terminal OutputType = iota
	File
	Server
)

func main() {
	/*********************** Flags ****************************/
	debug := flag.Bool("debug", false, "")
	logToStdOut := flag.Bool("stdout", false, "Set to log to standard out")
	maxHops := flag.Int("m", defaultMaxHops, "Max hops in the traceroute (ie max ttl)")
	timeout := flag.Duration("t", defaultTimeout, "Timeout to wait for an answer in one hop")
	trTarget := flag.String("u", "", "Traceroute target (url or ip)")
	infile := flag.Bool("i", false, "Set to allow an input file")
	infilePath := flag.String("ipath", defaultInfile, "Specify path to traceroute targets input file")
	outfile := flag.Bool("o", false, "Set to allow an output file")
	outfilePath := flag.String("opath", defaultOutfile, "Specify path to results output file")
	validatorOutput := flag.Bool("b", false, "Validator output only includes IP - used for trgen validation. Diff ")
	flag.Parse()

	/*********************** Session ****************************/

	localIP, _ := getLocalIP()

	session := CTRDSession{
		MaxHops:         *maxHops,
		Timeout:         msDuration(*timeout),
		LocalIP:         localIP,
		LogLevel:        LogLevelInfo,
		StartedAt:       time.Now().UTC(),
		ValidatorOutput: *validatorOutput,
	}

	/*********************** Log setup ****************************/
	if *debug {
		session.LogLevel = LogLevelDebug
	}
	initLogging(*logToStdOut, defaultLogfile)

	/*********************** I/O ****************************/
	targets := handleInput(&session, *trTarget, *infile, *infilePath)
	// TODO - this still feels weird, too global objecty. Is it not better to have a return here?
	handleOutput(&session, *outfile, *outfilePath)

	/*********************** Setup TRs object for session ****************************/
	for _, target := range targets {
		if validTarget(target) {
			ip, _, err := IPLookup(target)
			if err != nil {
				logInfo(fmt.Sprintf("Traceroute target %v not reachable, skipping...\n", target))
				continue
			}

			tr := CTRDTraceroute{
				DestinationIP:       ip,
				DestinationHostname: cleanedHostname(target),
				OriginIP:            session.LocalIP,
				Hops:                make([]CTRDHop, session.MaxHops),
			}
			session.Traceroutes = append(session.Traceroutes, tr)
		}
	}

	/****************** Main *******************/
	logInfo(fmt.Sprintf("Starting session at %v", time.Now().UTC().Format("2006-01-02 15:04:05")))
	session.runSession()
	logInfo(fmt.Sprintf("Ending session at %v", time.Now().UTC().Format("2006-01-02 15:04:05")))

	/*********************** Cleanup and exit ****************************/
	session.EndedAt = time.Now().UTC()
	printSessionToOutput(&session)
	fmt.Printf("Completed in %+v", session.EndedAt.Sub(session.StartedAt))
	terminateLogging()
	os.Exit(0)
}
