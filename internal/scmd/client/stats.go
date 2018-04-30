package client

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
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
	w.Header().Add("Connection", "close")
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
		ExitCountry string
		Entry       string
		Protocol    string
		ActiveTunns map[string]string
	}
	cmd.stats.RLock()
	defer cmd.stats.RUnlock()
	w.Header().Add("Connection", "close")
	csn := cmd.stats.netinfo
	resp.Exit = cmd.stats.netinfo.exit
	if cmd.geodbloc != "" {
		resp.ExitCountry = cmd.geodb.GetCountry(net.ParseIP(resp.Exit))
	}
	resp.Entry = csn.entry
	resp.Protocol = csn.prot
	resp.ActiveTunns = csn.tuns
	w.Header().Add("content-type", "application/json")
	bts, _ := json.MarshalIndent(&resp, "", "    ")
	w.Write(bts)
}

func (cmd *Command) servAccInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Connection", "close")
	var req struct {
		Username string
	}
	req.Username = cmd.uname
	bts, _ := json.Marshal(&req)
	resp, err := cmd.proxclient.Post("https://binder.geph.io/user-status",
		"application/json", bytes.NewReader(bts))
	// If the network is borked, just die
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		return
	}
	// See if the status is 200
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		io.Copy(w, resp.Body)
	} else {
		w.WriteHeader(resp.StatusCode)
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
