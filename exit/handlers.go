package exit

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"time"

	"gopkg.in/bunsim/natrium.v1"
)

func (cmd *Command) handUpdateNode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Addr   string
		Cookie []byte
	}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Println("handUpdateNode: bad json:", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	err = cmd.edb.AddNode(req.Addr, req.Cookie)
	if err != nil {
		log.Println("handUpdateNode: node claiming to be", req.Addr, "doesn't check out")
	} else {
		log.Println("handUpdateNode: node updated:", req.Addr, "/", natrium.HexEncode(req.Cookie))
	}
	return
}

func (cmd *Command) handGetNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "max-age=500")
	var tosend struct {
		Expires string
		Nodes   map[string][]byte
	}

	log.Println(r.Header)

	// get the IP of the client. if the request comes from the binder, we trust the X-Forwarded-For
	binderips, err := net.LookupHost("binder.geph.io")
	if err != nil {
		log.Println("cannot lookup binder.geph.io:", err.Error())
		return
	}
	var rmadr string
	if r.RemoteAddr == binderips[0] {
		rmadr = r.Header.Get("X-Forwarded-For")
		log.Println("IP of client in get-nodes:", rmadr, "(forwarded)")
	} else {
		rmadr = net.ResolveTCPAddr("tcp", r.RemoteAddr).IP.String()
		log.Println("IP of client in get-nodes:", rmadr)
	}

	tosend.Expires = time.Now().Add(time.Hour).Format(time.RFC3339)
	tosend.Nodes = cmd.edb.GetNodes(
		binary.BigEndian.Uint64(natrium.SecureHash([]byte(rmadr), nil)[:8]))
	bts, _ := json.Marshal(&tosend)
	sig := cmd.identity.Sign(bts)
	w.Header().Add("X-Geph-Signature", natrium.HexEncode(sig))
	w.Write(bts)
}

func (cmd *Command) handTestSpeed(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Length", fmt.Sprintf("%v", 1024*1024))
	for i := 0; i < 256; i++ {
		lol := make([]byte, 4096)
		rand.Read(lol)
		w.Write(lol)
	}
}
