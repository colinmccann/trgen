package main

import (
	"flag"
	"log"
	"net"
	"os"
	"time"
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
	MaxHops     int              `json:"maxHops"`
	Timeout     time.Duration    `json:"timeOut"`
	OutputType  string           `json:"outputType"`
	OutputPath  string           `json:"outputPath"`
	LocalIP     net.IP           `json:"localIP"`
	Traceroutes []CTRDTraceroute `json:"traceroutes"`
}

type CTRDTraceroute struct {
	OriginIP            net.IP    `json:"originIP"`
	DestinationIP       net.IP    `json:"destinationIP"`
	DestinationHostname string    `json:"destinationHostname"`
	Hops                []CTRDHop `json:"hops"`
}

type CTRDHop struct {
	Num      int           `json:"num"`
	Ip       string        `json:"ip"`
	Hostname string        `json:"hostname"`
	Latency  time.Duration `json:"latency"`
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
		Timeout: *timeout,
		LocalIP: localIP,
	}
	/*********************** I/O ****************************/
	// TODO - this still feels weird, too global objecty. Is it not better to have a return here?
	handleInput(&session, *trTarget, *infile, *infilePath)
	handleOutput(&session, *outfile, *outfilePath)

	/****************** Main *******************/
	session.runSession()

	/*********************** Cleanup and exit ****************************/
	os.Exit(0)
}
