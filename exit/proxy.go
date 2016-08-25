package exit

import (
	"io"
	"log"
	"net"

	"github.com/bunsim/niaucchi"
)

func (cmd *Command) doProxy() {
	lsnr, err := niaucchi.Listen(nil, cmd.identity, ":2378")
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
		clnt, err := lsnr.Accept()
		if err != nil {
			panic(err.Error())
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
			log.Println("requested to proxy", string(addrbts))
			// resolve and connect
			addr, err := net.ResolveTCPAddr("tcp", string(addrbts))
			if err != nil {
				return
			}
			// block connections to things in the CIDR blacklist
			for _, n := range cidrBlacklist {
				if n.Contains(addr.IP) {
					log.Println("refusing to connect to local address", addr)
					return
				}
			}
			// go ahead and connect
			log.Println("proxying", addr)
			rmt, err := net.DialTCP("tcp", nil, addr)
			if err != nil {
				log.Println("failed to connect to", addr)
				return
			}
			// forward traffic
			defer rmt.Close()
			go func() {
				defer rmt.Close()
				defer clnt.Close()
				io.Copy(rmt, clnt)
			}()
			io.Copy(clnt, rmt)
		}()
	}
}
