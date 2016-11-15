package proxbinder

import (
	"encoding/base32"
	"encoding/json"
	"net/http"
	"strings"

	natrium "gopkg.in/bunsim/natrium.v1"
)

func (cmd *Command) handDeriveKeys(w http.ResponseWriter, r *http.Request) {
	uname := r.URL.Query().Get("uname")
	pwd := r.URL.Query().Get("pwd")
	prek := natrium.SecureHash([]byte(pwd), []byte(uname))

	SK := natrium.EdDSADeriveKey(
		natrium.StretchKey(prek, make([]byte, natrium.PasswordSaltLen), 8, 64*1024*1024)).ToECDH()
	PK := SK.PublicKey()
	UID := strings.ToLower(
		base32.StdEncoding.EncodeToString(
			natrium.SecureHash(PK, nil)[:10]))
	var resp struct {
		PubKey  []byte
		PrivKey []byte
		UID     string
	}
	resp.PubKey = PK
	resp.PrivKey = SK
	resp.UID = UID
	b, _ := json.MarshalIndent(&resp, "", "  ")
	w.Write(b)
}
