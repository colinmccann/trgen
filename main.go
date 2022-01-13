package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"
)

const (
	defaultMaxHops = 24
	defaultTimeout = 1 * time.Second // how long to wait for a response before going to next hop
	defaultInfile  = "data/traceroute_targets.txt"
	defaultOutfile = "data/results.txt"
)

// should these structs be interfaces?
// outputType should a be restricted set of options
type CTRDSession struct {
	MaxHops     int              `json:"maxHops"`
	Timeout     msDuration       `json:"timeOut"`
	OutputType  string           `json:"outputType"`
	OutputPath  string           `json:"outputPath"`
	LocalIP     net.IP           `json:"localIP"`
	Traceroutes []CTRDTraceroute `json:"traceroutes"`
}

type CTRDTraceroute struct {
	OriginIP            net.IP    `json:"originIP"`
	DestinationIP       net.IP    `json:"destinationIP"`
	DestinationHostname string    `json:"destinationHostname"`
	Length              int       `json:"length"`
	Terminated          bool      `json:"terminated"`
	Hops                []CTRDHop `json:"hops"`
}

type CTRDHop struct {
	Num      int        `json:"num"`
	Ip       string     `json:"ip"`
	Hostname string     `json:"hostname"`
	Latency  msDuration `json:"latency"`
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

	localIP, _ := getLocalIP()
	// what other vals go in here?
	// - session time
	session := CTRDSession{
		MaxHops: *maxHops,
		Timeout: msDuration(*timeout),
		LocalIP: localIP,
	}

	/*********************** I/O ****************************/
	targets := handleInput(&session, *trTarget, *infile, *infilePath)
	// TODO - this still feels weird, too global objecty. Is it not better to have a return here?
	handleOutput(&session, *outfile, *outfilePath)

	/*********************** Setup TRs object for session ****************************/
	for _, target := range targets {
		if validTarget(target) {
			ip, _, err := IPLookup(target)
			check(err)

			if err != nil {
				fmt.Printf("Traceroute target %v not reachable, skipping...\n", target)
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
	session.runSession()

	/*********************** Cleanup and exit ****************************/
	os.Exit(0)
}
