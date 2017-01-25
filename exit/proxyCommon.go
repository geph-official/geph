package exit

import (
	"context"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"golang.org/x/time/rate"

	"github.com/niwl/geph/common"
)

func (cmd *Command) proxyCommon(doAck bool, consume func(int) bool, limit, harshlimit *rate.Limiter,
	uid string, clnt io.ReadWriteCloser) {
	// other things
	ctx := context.Background()
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
	// convenience functions
	isBlack := func(addr *net.TCPAddr) bool {
		for _, n := range cidrBlacklist {
			if n.Contains(addr.IP) {
				return true
			}
		}
		return false
	}
	defer clnt.Close()
	// pascal string of the address
	lb := make([]byte, 1)
	_, err := io.ReadFull(clnt, lb)
	if err != nil {
		return
	}
	addrbts := make([]byte, lb[0])
	_, err = io.ReadFull(clnt, addrbts)
	if err != nil {
		if doAck {
			clnt.Write([]byte{0x01})
		}
		return
	}
	log.Println("gonna tun", string(addrbts))
	// is it actually a DNS request?
	if len(addrbts) > 4 && string(addrbts[:4]) == "dns:" {
		// comma-separated array of addresses
		addrs, zerr := net.LookupHost(string(addrbts[4:]))
		if zerr != nil {
			return
		}
		if len(addrs) > 3 {
			addrs = addrs[:3]
		}
		towr := strings.Join(addrs, ",")
		log.Println("got", string(addrbts[4:]), "->", towr)
		clnt.Write(append([]byte{byte(len(towr))}, towr...))
		return
	}
	// resolve and connect
	addr, err := net.ResolveTCPAddr("tcp", string(addrbts))
	if err != nil {
		if doAck {
			clnt.Write([]byte{0x04})
		}
		return
	}
	// block connections to things in the CIDR blacklist
	if isBlack(addr) {
		if doAck {
			clnt.Write([]byte{0x02})
		}
		return
	}
	// block connections to forbidden ports
	if !common.AllowedPorts[addr.Port] {
		if doAck {
			clnt.Write([]byte{0x02})
		}
		return
	}
	// go ahead and connect
	rmt, err := net.DialTimeout("tcp", addr.String(), time.Second*5)
	if err != nil {
		if doAck {
			clnt.Write([]byte{0x05})
		}
		return
	}
	if doAck {
		clnt.Write([]byte{0x00})
	}
	// forward traffic
	defer rmt.Close()
	go func() {
		defer rmt.Close()
		defer clnt.Close()
		buf := make([]byte, 8192)
		for {
			n, err := rmt.Read(buf)
			if err != nil {
				return
			}
			if !consume(n) {
				// if we are over the limit, apply fascist limit
				harshlimit.WaitN(ctx, n)
			} else {
				limit.WaitN(ctx, n)
			}
			_, err = clnt.Write(buf[:n])
			if err != nil {
				return
			}
		}
	}()
	buf := make([]byte, 8192)
	for {
		n, err := clnt.Read(buf)
		if err != nil {
			return
		}
		if !consume(n) {
			// if we are over the limit, apply fascist limit
			harshlimit.WaitN(ctx, n)
		}
		limit.WaitN(ctx, n)
		_, err = rmt.Write(buf[:n])
		if err != nil {
			return
		}
	}
}
