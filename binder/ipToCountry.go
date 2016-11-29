package binder

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var cleanHTTP = &http.Client{
	Transport: &http.Transport{
		TLSHandshakeTimeout: time.Second * 10,
		IdleConnTimeout:     time.Second * 10,
	},
	Timeout: time.Second * 10,
}

func ipToCountry(ip string) (reg string, err error) {
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
	return
}
