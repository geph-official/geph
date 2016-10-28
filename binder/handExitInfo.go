package binder

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"gopkg.in/bunsim/natrium.v1"
)

func (cmd *Command) handExitInfo(w http.ResponseWriter, r *http.Request) {
	// JSON thingy
	var towrite struct {
		Expires string
		Exits   map[string][]byte
	}
	defer r.Body.Close()
	w.Header().Add("Cache-Control", "max-age=120")
	// Some sanity-checking to make sure we don't sign something stupid
	hand, err := os.Open(cmd.exitConf)
	if err != nil {
		log.Println("handExitInfo: cannot open exit configuration", cmd.exitConf)
		return
	}
	defer hand.Close()
	err = json.NewDecoder(hand).Decode(&towrite)
	if err != nil {
		log.Println("handExitInfo: exit configuration is bad JSON:", err.Error())
		return
	}
	zeit, err := time.Parse(time.RFC3339, towrite.Expires)
	if err != nil {
		log.Println("handExitInfo: exit configuration contains a malformed date:", towrite.Expires)
		return
	}
	if zeit.Before(time.Now()) {
		log.Println("handExitInfo: exit configuration has already expired:", towrite.Expires)
		return
	}
	if zeit.Before(time.Now().Add(time.Hour * 24 * 7)) {
		log.Println("handExitInfo: exit configuration is going to expire soon:", towrite.Expires)
	}
	// Now we reserialize and sign
	bts, err := json.Marshal(&towrite)
	if err != nil {
		panic(err.Error())
	}
	sig := cmd.identity.Sign(bts)
	w.Header().Add("X-Geph-Signature", natrium.HexEncode(sig))
	w.Write(bts)
}
