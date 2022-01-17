package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const ipLookupURL = "http://checkip.amazonaws.com"

func check(e error) {
	if e != nil {
		// log.Fatal(e)
		fmt.Println(e)
	}
}

const URIPattern string = `^((ftp|http|https):\/\/)?(\S+(:\S*)?@)?((([1-9]\d?|1\d\d|2[01]\d|22[0-3])(\.(1?\d{1,2}|2[0-4]\d|25[0-5])){2}(?:\.([0-9]\d?|1\d\d|2[0-4]\d|25[0-4]))|(((([a-z\x{00a1}-\x{ffff}0-9]+-?-?_?)*[a-z\x{00a1}-\x{ffff}0-9]+)\.)?)?(([a-z\x{00a1}-\x{ffff}0-9]+-?-?_?)*[a-z\x{00a1}-\x{ffff}0-9]+)(?:\.([a-z\x{00a1}-\x{ffff}]{2,}))?)|localhost)(:(\d{1,5}))?((\/|\?|#)[^\s]*)?$`

// weak validation of URIs
func validateURI(uri string) bool {
	pattern := URIPattern
	match, err := regexp.MatchString(pattern, uri)
	check(err)
	return match
}

// check for URI validity
func validTarget(target string) bool {
	// check for other things?
	if validateURI(target) {
		return true
	}

	fmt.Printf("Found non-valid traceroute target '%v', skipping...\n", target)
	return false
}

// return user's local IP, as seen from external source (in this case, aws as defined in a const)
func getLocalIP() (net.IP, error) {
	req, err := http.Get(ipLookupURL)
	if err != nil {
		return nil, err
	}
	defer req.Body.Close()

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	cleanedBody := strings.TrimSpace(string(body))

	netIP := net.ParseIP(cleanedBody)
	// this is a weird error check. TODO - fix?
	if netIP == nil {
		return nil, nil
	}

	return netIP, nil
}

// for a given IP or hostname string 'target', return the IP, hostname and err
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

	// otherwise we assume it's a hostname
	ipAddr, err := lookupIPForHostname(target)

	if ipAddr != nil && err == nil {
		return ipAddr, target, nil
	}

	// TODO - handle this error properly
	// fmt.Printf("Unable to resolve the given target: %v to an IP address.", target)
	return ipAddr, ipStr, err
}

// return a net.IP ip address for a given hostname 'target'
func lookupIPForHostname(target string) (net.IP, error) {
	hostname := cleanedHostname(target)

	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil, err
	}

	// check if there's an IPv4 to return
	for _, ip := range ips {
		if ipv4 := ip.To4(); ipv4 != nil {
			return ip, nil
		}
	}

	// No IPv4, instead return the first IPv6
	if len(ips) > 0 {
		return ips[0], nil
	}

	return nil, errors.New("Given hostname was a valid host URL, but somehow does not have an IP")
}

// return a string hostname for a given ip address
func lookupHostnameForIP(ip string) (string, error) {
	name := ""
	if ip == "" {
		return name, errors.New("Given IP is blank")
	}

	names, err := net.LookupAddr(ip)

	if err == nil && len(names) > 0 {
		name = names[0]

		// remove trailing '.' if exists (unicode 46)
		length := len(name)
		if length > 0 && name[length-1] == 46 {
			name = name[:length-1]
		}
	}
	return name, nil
}

func cleanedHostname(target string) string {
	// removes the schema, port, etc
	u, _ := url.Parse(target)
	if u.Hostname() != "" {
		return u.Hostname()
	}

	return target
}
