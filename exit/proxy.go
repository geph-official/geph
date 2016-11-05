package exit

import (
	"context"
	"encoding/base32"
	"io"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/bunsim/geph/niaucchi"
	"gopkg.in/bunsim/natrium.v1"
)

// TODO refactor this function, it's getting too messy
func (cmd *Command) doProxy() {
	lsnr, err := niaucchi.Listen(nil, cmd.identity.ToECDH(), ":2378")
	if err != nil {
		panic(err.Error())
	}
	// lock on all the lists
	var lslok sync.RWMutex
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
	// whitelist of free networks
	freeWhitelist := map[string]bool{
		"binder.geph.io:443": true,
		"dl.geph.io:443":     true,
		"8.8.8.8:53":         true,
	}
	// we also periodically update the whitelist by resolving the names
	go func() {
		for {
			lslok.Lock()
			var nlst []string
			for v := range freeWhitelist {
				addr, err := net.ResolveTCPAddr("tcp", string(v))
				if err != nil {
					continue
				}
				nlst = append(nlst, addr.String())
			}
			for _, v := range nlst {
				freeWhitelist[v] = true
			}
			lslok.Unlock()
		}
	}()
	// convenience functions
	isBlack := func(addr *net.TCPAddr) bool {
		lslok.RLock()
		defer lslok.RUnlock()
		for _, n := range cidrBlacklist {
			if n.Contains(addr.IP) {
				return true
			}
		}
		return false
	}
	isWhite := func(addr string) bool {
		lslok.RLock()
		defer lslok.RUnlock()
		return freeWhitelist[addr]
	}
	log.Println("niaucchi listening on port 2378")
	for {
		ss, err := lsnr.AcceptSubstrate()
		if err != nil {
			panic(err.Error())
		}
		go func() {
			defer ss.Tomb().Kill(io.ErrClosedPipe)
			// get their pk first
			pub := ss.RemotePK()
			uid := strings.ToLower(
				base32.StdEncoding.EncodeToString(
					natrium.SecureHash(pub, nil)[:10]))
			// per-substrate rate limit
			limit := rate.NewLimiter(rate.Limit(cmd.bwLimit*1024), 512*1024)
			ctx := context.Background()
			// check balance first
			bal, err := cmd.decAccBalance(uid, 0)
			if err != nil {
				log.Println("error authenticating user", uid, ":", err)
			} else {
				log.Println(uid, "connected with", bal, "MiB left")
			}
			// little balance
			lbal := 0
			var lblk sync.Mutex
			// consume bytes, returns true if succeeds, otherwise returns false and kills everything
			consume := func(dec int) bool {
				lblk.Lock()
				defer lblk.Unlock()
				lbal -= dec
				if lbal <= 0 {
					// take between 1 and 10 MiB randomly to prevent consistent delays
					num := rand.Int()%9 + 1
					bal, err := cmd.decAccBalance(uid, num)
					if err != nil || bal == 0 {
						return false
					}
					lbal += 1000 * 1000 * num
				}
				return true
			}
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
					// is this connection free?
					isfree := isWhite(string(addrbts))
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
							if !consume(n) && !isfree {
								return
							}
							limit.WaitN(ctx, n)
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
						if !consume(n) && !isfree {
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
