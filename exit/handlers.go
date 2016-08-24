package exit

import (
	"encoding/json"
	"log"
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
	var tosend struct {
		Expires string
		Nodes   map[string][]byte
	}
	tosend.Expires = time.Now().Add(time.Hour).Format(time.RFC3339)
	tosend.Nodes = cmd.edb.GetNodes(0)
	bts, _ := json.Marshal(&tosend)
	w.Write(bts)
}
