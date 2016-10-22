package client

import (
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
