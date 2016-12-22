package binder

import (
	"encoding/base32"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/dchest/captcha"

	"gopkg.in/bunsim/natrium.v1"
)

func (cmd *Command) handRegisterAccount(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "application/json")
	var req struct {
		Username    string
		PubKey      []byte
		CaptchaID   string
		CaptchaSoln string
	}
	// read json
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Println("handRegisterAccount:", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// check for sanity
	if len(req.PubKey) != natrium.ECDHKeyLength {
		log.Println("handRegisterAccount: insane PubKey length")
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	uid := strings.ToLower(
		base32.StdEncoding.EncodeToString(
			natrium.SecureHash(req.PubKey, nil)[:10]))
	// check the captcha
	if !captcha.VerifyString(req.CaptchaID, req.CaptchaSoln) {
		log.Println("handRegisterAccount:", req.CaptchaSoln, "does not solve", req.CaptchaID)
		w.WriteHeader(http.StatusForbidden)
		return
	}
	// now we attempt to insert to db
	tx, err := cmd.pgdb.Begin()
	if err != nil {
		log.Println("handRegisterAccount: failed to start tx", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	tx.Exec("SET TRANSACTION ISOLATION LEVEL SERIALIZABLE")
	// does the account already exist?
	var cnt int
	err = tx.QueryRow("SELECT COUNT(*) FROM AccInfo WHERE Uname = $1 OR Uid = $2",
		req.Username, uid).Scan(cnt)
	if err != nil {
		log.Println("handRegisterAccount: failed to query for count", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if cnt > 0 {
		log.Println("handRegisterAccount: attempt to register duplicate username",
			req.Username, "failed")
		w.WriteHeader(http.StatusConflict)
		return
	}
	// we are safe now, so we can proceed with registration
	_, err = tx.Exec("INSERT INTO AccInfo VALUES ($1, $2, $3)", uid, req.Username, time.Now())
	if err != nil {
		log.Println("handRegisterAccount: failed to update AccInfo:", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// put an entry for the balance too
	_, err = tx.Exec("INSERT INTO AccBalances VALUES ($1, 1000)", uid)
	if err != nil {
		log.Println("handRegisterAccount: failed to update AccBalances:", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// success!
	err = tx.Commit()
	if err != nil {
		log.Println("handRegisterAccount: wasn't able to commit:", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	return
}
