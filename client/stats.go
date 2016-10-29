package client

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"sync/atomic"
	"time"
)

func (cmd *Command) servSummary(w http.ResponseWriter, r *http.Request) {
	var resp struct {
		Status  string
		ErrCode string
		Uptime  int
		BytesTX int
		BytesRX int
	}
	cmd.stats.RLock()
	defer cmd.stats.RUnlock()
	resp.Status = cmd.stats.status
	resp.Uptime = int(time.Now().Sub(cmd.stats.stTime).Seconds())
	resp.BytesRX = int(cmd.stats.rxBytes)
	resp.BytesTX = int(cmd.stats.txBytes)
	w.Header().Add("content-type", "application/json")
	bts, _ := json.MarshalIndent(&resp, "", "    ")
	w.Write(bts)
}

func (cmd *Command) servNetinfo(w http.ResponseWriter, r *http.Request) {
	var resp struct {
		Exit        string
		Entry       string
		Protocol    string
		ActiveTunns map[string]string
	}
	eip, err := cmd.resolveName(cmd.stats.netinfo.exit)
	cmd.stats.RLock()
	defer cmd.stats.RUnlock()
	csn := cmd.stats.netinfo
	if err == nil {
		resp.Exit = eip
	} else {
		resp.Exit = "FAIL"
	}
	resp.Entry = csn.entry
	resp.Protocol = csn.prot
	resp.ActiveTunns = csn.tuns
	w.Header().Add("content-type", "application/json")
	bts, _ := json.MarshalIndent(&resp, "", "    ")
	w.Write(bts)
}

func (cmd *Command) servAccInfo(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PrivKey []byte
	}
	req.PrivKey = cmd.identity
	bts, _ := json.Marshal(&req)
	resp, err := cmd.proxclient.Post("https://binder.geph.io/account-summary",
		"application/json", bytes.NewReader(bts))
	// If the network is borked, go back to ConnEntry
	if err != nil {
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
		if err != nil {
			return
		}
		var resp struct {
			Username string
			AccID    string
			Balance  int
		}
		resp.Username = lol.Username
		resp.AccID = touid(cmd.identity.PublicKey())
		resp.Balance = lol.Balance
		bts, _ := json.MarshalIndent(&resp, "", "    ")
		w.Write(bts)
	} else {
		return
	}
}

// io.Copy, except it increments a counter in real-time using atomic primitives
func ctrCopy(dest io.Writer, orig io.Reader, ctr *uint64) error {
	buffer := make([]byte, 32768)
	for {
		n, err := orig.Read(buffer)
		if err != nil {
			return err
		}
		_, err = dest.Write(buffer[:n])
		if err != nil {
			return err
		}
		atomic.AddUint64(ctr, uint64(n))
	}
}
