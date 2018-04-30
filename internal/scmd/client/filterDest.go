package client

import (
	"log"
	"net"
	"regexp"
	"strings"
)

const cGoogleDNS = "8.8.8.8:53"

var cLanRxp = regexp.MustCompile("\\.(lan|local)$")

// filterDest takes in an address of the form "host:port" and return true iff it should
// go through the proxy
func (cmd *Command) filterDest(addr string) bool {
	// blacklist of local networks
	var cidrBlacklist []*net.IPNet
	for _, s := range []string{
		"127.0.0.1/8",
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
	} {
		_, n, _ := net.ParseCIDR(s)
		cidrBlacklist = append(cidrBlacklist, n)
	}
	// split host and port
	host, _, _ := net.SplitHostPort(addr)
	if host == "binder.geph.io" {
		return true
	}
	if !strings.Contains(host, ".") ||
		cLanRxp.MatchString(host) {
		log.Println("DENYING", addr, "due to host pattern")
		// local or internal address
		return false
	}
	// if it's already an IP, don't bother
	ip := net.ParseIP(host)
	if len(ip) == 0 {
		if cmd.geodbloc == "" && addr != cGoogleDNS {
			return true
		}
		// otherwise, resolve through the tunnel
		r, e := cmd.resolveName(host)
		if e != nil || len(ip) == 0 {
			return true
		}
		ip = net.ParseIP(r)
	} else {
		// deny local networks
		for _, n := range cidrBlacklist {
			if n.Contains(ip) {
				log.Println("DENYING", addr, ip.String(), "because it's local")
				return false
			}
		}
	}
	// check the country
	if cmd.geodbloc != "" {
		ctry := cmd.geodb.GetCountry(ip)
		for _, v := range cmd.whitegeo {
			if ctry == v {
				log.Println("DENYING", addr, ip.String(), ctry, "due to country")
				return false
			}
		}
		return true
	}
	if addr != cGoogleDNS {
		return true
	}
	return true
}
