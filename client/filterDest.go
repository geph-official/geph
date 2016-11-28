package client

import (
	"encoding/binary"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"

	"github.com/bunsim/geph/common"
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
	host, portstr, err := net.SplitHostPort(addr)
	if !strings.Contains(host, ".") ||
		cLanRxp.MatchString(host) {
		log.Println("DENYING", addr, "due to host pattern")
		// local or internal address
		return false
	}
	// next we check the port
	port, err := strconv.Atoi(portstr)
	if err != nil {
		panic("atoi on portstr should never fail")
	}
	if !common.AllowedPorts[port] {
		log.Println("DENYING", addr, "due to port")
		// Forbidden port
		return false
	}
	// if it's already an IP, don't bother
	ip := net.ParseIP(host)
	if len(ip) == 0 {
		if cmd.geosql == nil && addr != cGoogleDNS {
			log.Println("ACCEPTING", addr)
			return true
		}
		// otherwise, resolve through the tunnel
		r, e := cmd.resolveName(host)
		if e != nil {
			log.Println("ACCEPTING", addr, "to be safe, but host is unresolvable")
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
	if cmd.geosql != nil {
		var ctry string
		ipint := int(binary.BigEndian.Uint32(ip[12:]))
		err = cmd.geosql.QueryRow(
			"SELECT ctry FROM iptocountry WHERE idx = $1 AND start <= $2 AND $2 <= end",
			ipint-(ipint%16777216),
			ipint,
		).Scan(&ctry)
		if err != nil {
			log.Println("ACCEPTING", addr, ip.String(), "to be safe, but:", err.Error())
			return true
		}
		for _, v := range cmd.whitegeo {
			if ctry == v {
				log.Println("DENYING", addr, ip.String(), ctry, "due to country")
				return false
			}
		}
		if addr != cGoogleDNS { // reduce log spam
			log.Println("ACCEPTING", addr, ip.String(), ctry)
		}
		return true
	}
	if addr != cGoogleDNS {
		log.Println("ACCEPTING", addr)
		return true
	}
	return true
}
