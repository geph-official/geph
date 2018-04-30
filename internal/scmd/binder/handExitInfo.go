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
	w.Header().Add("content-type", "application/json")
	// JSON thingy
	var towrite struct {
		Exits     map[string][]byte
		Expires   string
		Blacklist map[string][]string
	}
	defer r.Body.Close()
	w.Header().Add("Cache-Control", "max-age=300")
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
	towrite.Expires = time.Now().Add(time.Hour * 24).Format(time.RFC3339)
	// Now we filter the exits
	clIP := r.Header.Get("X-Forwarded-For")
	if clIP != "" {
		cntry, err := ipToCountry(clIP)
		if err == nil {
			log.Println("handExitInfo: identified client", clIP, "from", cntry)
			for _, ex := range towrite.Blacklist[cntry] {
				delete(towrite.Exits, ex)
			}
		} else {
			log.Println("handExitInfo: unexpected error", err.Error())
		}
	}
	// Now we reserialize and sign
	bts, _ := json.MarshalIndent(&towrite, "", "  ")
	sig := cmd.identity.Sign(bts)
	w.Header().Add("X-Geph-Signature", natrium.HexEncode(sig))
	w.Write(bts)
}
