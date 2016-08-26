package exit

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"time"
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
		log.Println("handUpdateNode: node updated:", req.Addr, "/", req.Cookie)
	}
	return
}

func (cmd *Command) handGetNodes(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Cache-Control", "no-cache")
	var tosend struct {
		Expires string
		Nodes   map[string][]byte
	}
	tosend.Expires = time.Now().Add(time.Hour).Format(time.RFC3339)
	tosend.Nodes = cmd.edb.GetNodes(0)
	bts, _ := json.Marshal(&tosend)
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
