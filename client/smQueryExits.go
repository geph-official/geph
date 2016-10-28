package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"gopkg.in/bunsim/natrium.v1"
)

// smQueryExits is the QueryExits state.
// => FindEntry if successful
// => QueryBinder if unsuccessful
func (cmd *Command) smQueryExits() {
	log.Println("** => QueryExits **")
	defer log.Println("** <= QueryExits **")

	entries := make(map[string][]entryInfo)
	var err error
	for ext, kee := range cmd.exitCache {
		req, _ := http.NewRequest("POST",
			fmt.Sprintf("https://%v/exits/%v:8081/get-nodes", cFRONT, ext), nil)
		req.Host = cHOST
		var resp *http.Response
		resp, err = insecHTTP.Do(req)
		if err != nil {
			log.Println(err.Error())
			continue
		}
		var lol struct {
			Expires string
			Nodes   map[string][]byte
		}
		buf := new(bytes.Buffer)
		io.Copy(buf, resp.Body)
		// we have to verify at this point!
		hexsig := resp.Header.Get("X-Geph-Signature")
		var sig []byte
		sig, err = natrium.HexDecode(hexsig)
		if len(sig) != 64 || buf.Len() == 0 {
			continue
		}
		if err != nil {
			log.Println(err.Error())
			continue
		}
		err = natrium.EdDSAPublic(kee).Verify(buf.Bytes(), sig)
		if err != nil {
			log.Println(err.Error())
			continue
		}
		// now the thing has to be legit
		err = json.NewDecoder(buf).Decode(&lol)
		if err != nil {
			log.Println(err.Error())
			resp.Body.Close()
			continue
		}
		resp.Body.Close()
		for addr, cook := range lol.Nodes {
			entries[ext] = append(entries[ext], entryInfo{
				Addr:    addr,
				Cookie:  cook,
				ExitKey: natrium.EdDSAPublic(kee),
			})
		}
	}
	if len(entries) == 0 {
		log.Println("QueryExits: not a single entry node discovered")
		cmd.exitCache = nil
		cmd.smState = cmd.smQueryBinder
		return
	}

	cmd.entryCache = entries
	cmd.smState = cmd.smFindEntry
	return
}
