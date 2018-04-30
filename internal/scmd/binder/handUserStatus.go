package binder

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

func (cmd *Command) handUserStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("content-type", "application/json")
	var req struct {
		Username string
	}
	defer r.Body.Close()
	// populate from json
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		log.Println("handUserStatus:", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var resp struct {
		FreeBalance int
		PremiumInfo struct {
			Plan      string
			Desc      json.RawMessage
			Unlimited bool
			ExpUnix   int
		}
	}
	// query the database
	tx, err := cmd.pgdb.Begin()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()
	tx.Exec("SET TRANSACTION ISOLATION LEVEL SERIALIZABLE")
	var uid int
	tx.QueryRow("SELECT ID, FreeBalance FROM Users WHERE Username = $1", req.Username).
		Scan(&uid, &resp.FreeBalance)
	if uid == 0 {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var expTime time.Time
	err = tx.QueryRow(
		"SELECT Plan, Description, Unlimited, Expires FROM subscriptions NATURAL JOIN premiumplans WHERE ID = $1", uid).
		Scan(&resp.PremiumInfo.Plan, &resp.PremiumInfo.Desc, &resp.PremiumInfo.Unlimited,
			&expTime)
	if err != nil && err != sql.ErrNoRows {
		log.Println("handUserStatus:", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	resp.PremiumInfo.ExpUnix = int(expTime.Unix())
	tx.Commit()
	// write back response
	j, _ := json.MarshalIndent(&resp, "", "  ")
	w.Write(j)
}
