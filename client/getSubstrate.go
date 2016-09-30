package client

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/bunsim/geph/niaucchi"
	"gopkg.in/bunsim/natrium.v1"
)

var binderPub natrium.EdDSAPublic

func init() {
	binderPub, _ = natrium.HexDecode("d25bcdc91961a6e9e6c74fbcd5eb977c18e7b1fe63a78ec62378b55aa5172654")
}

var myHTTP = &http.Client{
	Transport: &http.Transport{
		TLSHandshakeTimeout: time.Second * 10,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		//DisableKeepAlives:   true,
	},
	Timeout: time.Second * 10,
}

const cFRONT = "a0.awsstatic.com"
const cHOST = "dtnins2n354c4.cloudfront.net"

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
		log.Println("WARNING: bad time format in exit info, ignoring")
		return
	}
	if expTime.Before(time.Now()) {
		log.Println("WARNING: expire time before now, ignoring")
		return
	}
	nds = exinf.Exits
	return
}

type entryInfo struct {
	Addr    string
	Cookie  []byte
	ExitKey natrium.EdDSAPublic
}

func (cmd *Command) getSubstrate() (ss *niaucchi.Substrate, err error) {
	// step 1: obtain the list of exit nodes
	nds, err := cmd.getExitNodes()
	if err != nil {
		return
	}
	// step 2: for each exit node, ping all the entry nodes
	entries := make(map[string][]entryInfo)
	for ext, kee := range nds {
		req, _ := http.NewRequest("POST",
			fmt.Sprintf("https://%v/exits/%v:8081/get-nodes", cFRONT, ext), nil)
		req.Host = cHOST
		var resp *http.Response
		resp, err = myHTTP.Do(req)
		if err != nil {
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
			continue
		}
		err = natrium.EdDSAPublic(kee).Verify(buf.Bytes(), sig)
		if err != nil {
			continue
		}
		// now the thing has to be legit
		err = json.NewDecoder(buf).Decode(&lol)
		if err != nil {
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
		err = errors.New("nothing worked at all")
		return
	}

	// step 3: massive race
	retline := make(chan *niaucchi.Substrate)
	dedline := make(chan bool)
	for exit, entries := range entries {
		for _, xaxa := range entries {
			xaxa := xaxa
			log.Println(xaxa.Addr, "from", exit)
			go func() {
				cand, merr := niaucchi.DialSubstrate(xaxa.Cookie,
					cmd.identity,
					xaxa.ExitKey.ToECDH(),
					xaxa.Addr, 8)
				if merr != nil {
					log.Println(xaxa.Addr, "failed right away:", merr)
					return
				}
				select {
				case retline <- cand:
					log.Println(xaxa.Addr, "WINNER")
				case <-dedline:
					log.Println(xaxa.Addr, "failed race")
					cand.Tomb().Kill(io.ErrClosedPipe)
				}
			}()
		}
	}

	select {
	case ss = <-retline:
		close(dedline)
		return
	case <-time.After(time.Second * 10):
		close(dedline)
		err = errors.New("timeout")
		return
	}
}
