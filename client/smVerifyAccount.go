package client

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
)

// smVerifyAccount is the VerifyAccount state where the account info is verified.
// => SteadyState if the account info is okay
// => BadAuth if the account info is not okay
func (cmd *Command) smVerifyAccount() {
	log.Println("** => VerifyAccount **")
	defer log.Println("** <= VerifyAccount **")
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
		cmd.smState = cmd.smConnEntry
		return
	}
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
		cmd.smState = cmd.smSteadyState
	} else {
		log.Println("** FATAL: account info is wrong! **")
		os.Exit(403)
	}
}
