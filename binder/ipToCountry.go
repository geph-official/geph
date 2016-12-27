package binder

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"
)

var cleanHTTP = &http.Client{
	Transport: &http.Transport{
		TLSHandshakeTimeout: time.Second * 10,
		IdleConnTimeout:     time.Second * 10,
	},
	Timeout: time.Second * 10,
}

var ipcache = make(map[string]struct {
	cntry string
	exp   time.Time
})
var ipcachelk sync.Mutex

func ipToCountry(ip string) (reg string, err error) {
	ipcachelk.Lock()
	cr, ok := ipcache[ip]
	ipcachelk.Unlock()
	if ok && cr.exp.After(time.Now()) {
		reg = cr.cntry
		return
	}
	resp, err := cleanHTTP.Get(fmt.Sprintf("https://freegeoip.net/json/%v", ip))
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		err = errors.New("non-OK status")
		return
	}
	var jsxp struct {
		CountryCode string `json:"country_code"`
	}
	err = json.NewDecoder(resp.Body).Decode(&jsxp)
	reg = jsxp.CountryCode
	ipcachelk.Lock()
	ipcache[ip] = struct {
		cntry string
		exp   time.Time
	}{reg, time.Now().Add(time.Hour * 24)}
	ipcachelk.Unlock()
	return
}

func init() {
	go func() {
		// garbage collect
		for {
			time.Sleep(time.Hour)
			var todel []string
			ipcachelk.Lock()
			for k, v := range ipcache {
				if v.exp.After(time.Now()) {
					todel = append(todel, k)
				}
			}
			for _, td := range todel {
				delete(ipcache, td)
			}
			ipcachelk.Unlock()
		}
	}()
}
