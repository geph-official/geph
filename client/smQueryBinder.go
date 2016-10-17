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

	natrium "gopkg.in/bunsim/natrium.v1"
)

// smQueryBinder is the QueryBinder state.
// => QueryExits if successful
// => QueryBinder if fails
func (cmd *Command) smQueryBinder() {
	log.Println("** => QueryBinder **")
	defer log.Println("** <= QueryBinder **")
	nds, err := cmd.getExitNodes()
	if err != nil {
		cmd.smState = cmd.smQueryBinder
		time.Sleep(time.Second)
		return
	}
	cmd.exitCache = nds
	cmd.smState = cmd.smQueryExits
	return
}

func (cmd *Command) getExitNodes() (nds map[string][]byte, err error) {
	// request the data
	req, _ := http.NewRequest("GET", fmt.Sprintf("https://%v/exit-info", cFRONT), nil)
	req.Host = cHOST
	resp, err := myHTTP.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	io.Copy(buf, resp.Body)
	// verify the data
	hexsig := resp.Header.Get("X-Geph-Signature")
	sig, err := natrium.HexDecode(hexsig)
	if err != nil {
		return
	}
	if len(sig) != 64 || buf.Len() == 0 {
		err = errors.New("lol so broken")
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
