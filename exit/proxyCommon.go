package exit

import (
	"context"
	"io"
	"net"
	"time"

	"golang.org/x/time/rate"

	"github.com/bunsim/geph/common"
)

func (cmd *Command) proxyCommon(consume func(int) bool, limit, harshlimit *rate.Limiter,
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
		return
	}
	// we check if the
	// resolve and connect
	addr, err := net.ResolveTCPAddr("tcp", string(addrbts))
	if err != nil {
		return
	}
	// block connections to things in the CIDR blacklist
	if isBlack(addr) {
		return
	}
	// block connections to forbidden ports
	if !common.AllowedPorts[addr.Port] {
		return
	}
	// go ahead and connect
	rmt, err := net.DialTimeout("tcp", addr.String(), time.Second*5)
	if err != nil {
		return
	}
	// forward traffic
	defer rmt.Close()
	go func() {
		defer rmt.Close()
		defer clnt.Close()
		buf := make([]byte, 32768)
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
	buf := make([]byte, 32768)
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
