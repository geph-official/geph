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

func (cmd *Command) getEntryNodes(exits map[string][]byte) map[string][]entryInfo {
	FRONT, REAL := getFrontDomain()
	entries := make(map[string][]entryInfo)
	var err error
	for ext, kee := range exits {
		req, _ := http.NewRequest("POST",
			fmt.Sprintf("https://%v/exits/%v:8081/get-nodes", FRONT, ext), nil)
		req.Host = REAL
		var resp *http.Response
		resp, err = cleanHTTP.Do(req)
		if err != nil {
			log.Println(err.Error())
			continue
		}
		var lol struct {
			Expires  string
			Nodes    map[string][]byte
			Fallback map[string]string
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
		if lol.Fallback["Front"] != "" {
			entries[ext] = append(entries[ext], entryInfo{
				Addr:    "warpfront",
				Cookie:  []byte(lol.Fallback["Front"] + ";" + lol.Fallback["Host"]),
				ExitKey: natrium.EdDSAPublic(kee),
			})
		}
	}
	return entries
}
