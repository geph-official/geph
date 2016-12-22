package client

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

// verifyAccount should be run during the SteadyState state to check that the account info is
// okay. It kills the program if it is not okay, and returns an error only when the network fails.
func (cmd *Command) verifyAccount() (err error) {
	// We check by calling the account-summary interface over the tunnel
	var req struct {
		PrivKey []byte
	}
	req.PrivKey = cmd.identity
	bts, _ := json.Marshal(&req)
	resp, err := cmd.proxclient.Post("https://binder.geph.io/account-summary",
		"application/json", bytes.NewReader(bts))
	// If the network is borked, go back to ConnEntry
	if err != nil {
		log.Println("account verification failed since network is bad:", err.Error())
		return
	}
	log.Println("binder thing sent away")
	// See if the status is 200
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		var lol struct {
			Username string
			RegDate  string
			Balance  int
		}
		err = json.NewDecoder(resp.Body).Decode(&lol)
		if err == nil {
			log.Println("account verified: balance =", lol.Balance, "MB")
		}
	} else {
		log.Println("** FATAL: account info is wrong! **")
		os.Exit(43)
	}
	return
}
