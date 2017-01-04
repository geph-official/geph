package client

import (
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/bunsim/geph/niaucchi2"
	"github.com/miekg/dns"
)

type dnsCacheEntry struct {
	response string
	deadline time.Time
}

// TODO don't make this global, this sucks
var dnsCache = make(map[string]*dnsCacheEntry)
var dnsCacheLock sync.Mutex

func (cmd *Command) resolveName(name string) (ip string, err error) {
	dnsCacheLock.Lock()
	if dnsCache[name] != nil && dnsCache[name].deadline.After(time.Now()) {
		ip = dnsCache[name].response
		dnsCacheLock.Unlock()
		return
	}
	dnsCacheLock.Unlock()
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
	tmr := time.AfterFunc(time.Second*15, func() {
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
	dnsCacheLock.Lock()
	dnsCache[name] = &dnsCacheEntry{
		response: ip,
		deadline: time.Now().Add(time.Hour),
	}
	dnsCacheLock.Unlock()
	return
}

func (cmd *Command) doDNS() {
	// our server
	serv := &dns.Server{
		Net:  "udp",
		Addr: "127.0.0.1:8753",
		Handler: dns.HandlerFunc(func(w dns.ResponseWriter, r *dns.Msg) {
			q := r.Question[0]
			// we can't do anything if not A or CNAME
			if q.Qtype == dns.TypeA || q.Qtype == dns.TypeCNAME {
				ans, err := cmd.resolveName(q.Name)
				if err != nil {
					dns.HandleFailed(w, r)
					return
				}
				ip, _ := net.ResolveIPAddr("ip4", ans)
				m := new(dns.Msg)
				m.SetReply(r)
				m.Answer = append(m.Answer, &dns.A{
					Hdr: dns.RR_Header{
						Name:   q.Name,
						Rrtype: dns.TypeA,
						Class:  dns.ClassINET,
						Ttl:    60,
					},
					A: ip.IP,
				})
				w.WriteMsg(m)
			}
			dns.HandleFailed(w, r)
		}),
	}
	err := serv.ListenAndServe()
	if err != nil {
		panic(err.Error())
	}
}
