package binder

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/dchest/captcha"
)

func (cmd *Command) handExampleCaptcha(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "image/png")
	w.Header().Add("cache-control", "no-cache")
	id := captcha.NewLen(6)
	w.Header().Add("x-captcha-id", id)
	captcha.WriteImage(w, id, 200, 100)
}

func (cmd *Command) handFreshCaptcha(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "application/json")
	w.Header().Add("cache-control", "no-cache")
	var resp struct {
		CaptchaID  string
		CaptchaImg []byte
	}
	id := captcha.NewLen(6)
	resp.CaptchaID = id
	buf := bytes.NewBuffer(nil)
	captcha.WriteImage(buf, id, 200, 100)
	resp.CaptchaImg = buf.Bytes()
	j, _ := json.MarshalIndent(&resp, "", "  ")
	w.Write(j)
}
