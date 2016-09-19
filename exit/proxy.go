package exit

import (
	"context"
	"io"
	"log"
	"net"
	"time"

	"golang.org/x/time/rate"

	"github.com/bunsim/geph/niaucchi"
)

func (cmd *Command) doProxy() {
	lsnr, err := niaucchi.Listen(nil, cmd.identity.ToECDH(), ":2378")
	if err != nil {
		panic(err.Error())
	}
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

	log.Println("niaucchi listening on port 2378")
	for {
		ss, err := lsnr.AcceptSubstrate()
		if err != nil {
			panic(err.Error())
		}
		go func() {
			// per-substrate rate limit
			limit := rate.NewLimiter(rate.Limit(cmd.bwLimit*1024), 512*1024)
			ctx := context.Background()
			defer ss.Tomb().Kill(io.ErrClosedPipe)
			for {
				clnt, err := ss.AcceptConn()
				if err != nil {
					return
				}
				go func() {
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
					// resolve and connect
					addr, err := net.ResolveTCPAddr("tcp", string(addrbts))
					if err != nil {
						return
					}
					// block connections to things in the CIDR blacklist
					for _, n := range cidrBlacklist {
						if n.Contains(addr.IP) {
							return
						}
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
						buf := make([]byte, 16384)
						for {
							n, err := rmt.Read(buf)
							if err != nil {
								return
							}
							limit.WaitN(ctx, n)
							_, err = clnt.Write(buf[:n])
							if err != nil {
								return
							}
						}
					}()
					buf := make([]byte, 16384)
					for {
						n, err := clnt.Read(buf)
						if err != nil {
							return
						}
						limit.WaitN(ctx, n)
						_, err = rmt.Write(buf[:n])
						if err != nil {
							return
						}
					}
				}()
			}
		}()
	}
}
