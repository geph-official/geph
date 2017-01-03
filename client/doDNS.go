package client

import (
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/bunsim/geph/niaucchi2"
	"github.com/miekg/dns"
)

type dnsCacheEntry struct {
	response interface{}
	deadline time.Time
}

func (cmd *Command) resolveName(name string) (ip string, err error) {
	var myss *niaucchi2.Context
	myss = cmd.currTunn
	if myss == nil {
		err = io.ErrClosedPipe
		return
	}
	conn, err := myss.Tunnel()
	if err != nil {
		err = io.ErrClosedPipe
		return
	}
	conn.Write(append([]byte{byte(len(name) + 4)}, []byte("dns:"+name)...))
	// wait for the response
	tmr := time.AfterFunc(time.Second*5, func() {
		myss.Tomb().Kill(niaucchi2.ErrTimeout)
	})
	blen := make([]byte, 1)
	_, err = io.ReadFull(conn, blen)
	if err != nil {
		conn.Close()
		return
	}
	stuff := make([]byte, int(blen[0]))
	_, err = io.ReadFull(conn, stuff)
	if err != nil {
		conn.Close()
		return
	}
	ip = strings.Split(string(stuff), ",")[0]
	tmr.Stop()
	return
}

func (cmd *Command) doDNSCache() {
	// client that connects to our own TCP
	clnt := &dns.Client{
		Net:     "tcp",
		Timeout: time.Second * 5,
	}

	tbl := make(map[string]dnsCacheEntry)
	var lok sync.Mutex

	// thread that cleans things up
	go func() {
		for {
			time.Sleep(time.Hour)
			lok.Lock()
			var todel []string
			for k, v := range tbl {
				if v.deadline.After(time.Now()) {
					todel = append(todel, k)
				}
			}
			for _, k := range todel {
				delete(tbl, k)
			}
			lok.Unlock()
		}
	}()

	// our server currently just forwards to the TCP
	serv := &dns.Server{
		Net:  "udp",
		Addr: "127.0.0.1:8753",
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			q := r.Question[0]
			// check cache if A or CNAME
			if q.Qtype == dns.TypeA || q.Qtype == dns.TypeCNAME {
				lok.Lock()
				rsp := tbl[q.Name]
				if rsp.response != nil && rsp.deadline.After(time.Now()) {
					msg := rsp.response.(*dns.Msg)
					msg.Id = r.Id
					for _, v := range msg.Answer {
						v.Header().Ttl = uint32(rsp.deadline.Sub(time.Now()).Seconds())
					}
					w.WriteMsg(msg)
					lok.Unlock()
					return
				}
				lok.Unlock()
				// we still don't need to ask Google just yet.
			}
			// well I guess we need to ask Google...
			in, _, err := clnt.Exchange(r, "127.0.0.1:8753")
			if err != nil {
				log.Println("tunneled DNS resolution of", r.Question[0].Name, "failed:", err.Error())
				return
			}
			// truncate it
			for _, a := range in.Answer {
				a.Header().Ttl = 3600
			}
			in.Extra = nil
			in.Ns = nil

			w.WriteMsg(in)
			// now put into cache
			if q.Qtype == dns.TypeA || q.Qtype == dns.TypeCNAME {
				lok.Lock()
				var zaza dnsCacheEntry
				zaza.deadline = time.Now().Add(time.Hour)
				zaza.response = in
				tbl[q.Name] = zaza
				lok.Unlock()
			}
		}),
	}
	err := serv.ListenAndServe()
	if err != nil {
		panic(err.Error())
	}
}

func (cmd *Command) doDNS() {
	lsner, err := net.Listen("tcp", "127.0.0.1:8753")
	if err != nil {
		panic(err.Error())
	}
	// the TCP tunnels to Comodo's DNS
	go func() {
		for {
			clnt, err := lsner.Accept()
			if err != nil {
				panic(err.Error())
			}
			go func() {
				defer clnt.Close()
				rmt, err := cmd.dialTun("8.8.8.8:53")
				if err != nil {
					log.Println("failed to tunnel to DNS server:", err.Error())
					return
				}
				defer rmt.Close()
				go func() {
					defer rmt.Close()
					defer clnt.Close()
					io.Copy(clnt, rmt)
				}()
				io.Copy(rmt, clnt)
			}()
		}
	}()
	// the UDP does caching at stuff
	cmd.doDNSCache()
}
