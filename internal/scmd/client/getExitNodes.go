package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"gopkg.in/bunsim/natrium.v1"
)

func (cmd *Command) getExitNodes() (nds map[string][]byte, err error) {
	FRONT, REAL := getFrontDomain()
	// request the data
	req, _ := http.NewRequest("GET", fmt.Sprintf("https://%v/exit-info", FRONT), nil)
	req.Host = REAL
	resp, err := cleanHTTP.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	log.Println("exit-info gotten")
	buf := new(bytes.Buffer)
	io.Copy(buf, resp.Body)
	// verify the data
	hexsig := resp.Header.Get("X-Geph-Signature")
	sig, err := natrium.HexDecode(hexsig)
	if err != nil {
		return
	}
	if len(sig) != 64 {
		err = errors.New("signature not of right length")
		log.Println("malformed exit-info:")
		log.Println(string(buf.Bytes()))
		return
	}
	err = binderPub.Verify(buf.Bytes(), sig)
	if err != nil {
		return
	}
	// now everything must be fine
	var exinf struct {
		Expires string
		Exits   map[string][]byte
	}
	err = json.Unmarshal(buf.Bytes(), &exinf)
	if err != nil {
		log.Println("WARNING: bad json encountered in exit info:", err.Error())
		return
	}
	expTime, err := time.Parse(time.RFC3339, exinf.Expires)
	if err != nil {
		log.Println("WARNING: bad time format in exit info")
		return
	}
	if expTime.Before(time.Now()) {
		log.Println("WARNING: expire time before now in exit info")
		return
	}
	nds = exinf.Exits
	return
}
