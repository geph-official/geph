package exit

import (
	"database/sql"
	"encoding/base32"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/rensa-labs/geph/internal/common"
	natrium "gopkg.in/bunsim/natrium.v1"
)

type userID int

func toLegacyUid(b []byte) string {
	uid := strings.ToLower(
		base32.StdEncoding.EncodeToString(
			natrium.SecureHash(b, nil)[:10]))
	return uid
}

func (cmd *Command) authUser(uname, pwd string) (uid userID, limit int, err error) {
	tx, err := cmd.pgdb.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	tx.Exec("SET TRANSACTION ISOLATION LEVEL SERIALIZABLE")
	row := tx.QueryRow("SELECT ID, PwdHash FROM Users WHERE Username = $1", uname)
	var PwdHash string
	err = row.Scan(&uid, &PwdHash)
	if err == sql.ErrNoRows {
		err = nil
		uPriv := common.DeriveKey(uname, pwd).ToECDH()
		legacyUid := toLegacyUid(uPriv.PublicKey())
		log.Println("attempting to upgrade legacy user", uname, legacyUid)
		var mbs int
		err = tx.QueryRow("SELECT mbs FROM accbalances WHERE uid = $1", legacyUid).Scan(&mbs)
		if err != nil {
			return
		}
		hshPwd := natrium.PasswordHash([]byte(pwd), 5, 16*1024*1024)
		log.Println("hshPwd", []byte(hshPwd))
		_, err = tx.Exec("INSERT INTO Users (Username, PwdHash, FreeBalance, CreateTime)"+
			"VALUES ($1, $2, $3, $4)", uname, hshPwd, mbs, time.Now())
		if err != nil {
			log.Println("cannot insert! bad!")
			return
		}
		err = tx.QueryRow("SELECT ID FROM Users WHERE Username = $1", uname).Scan(&uid)
		if err != nil {
			return
		}
		log.Println("legacy user", uname, "successfully upgraded!")
		err = tx.Commit()
		return
	}
	if !natrium.PasswordVerify([]byte(pwd), PwdHash) {
		err = errors.New("wrong password")
		return
	}
	limit = 1000
	tx.QueryRow("SELECT maxspeed FROM Subscriptions NATURAL JOIN PremiumPlans WHERE ID = $1",
		uid).Scan(&limit)
	err = tx.Commit()
	return
}

func (cmd *Command) decAccBalance(uid userID, amt int) (rem int, err error) {
	// grab a TX at the database
	tx, err := cmd.pgdb.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	tx.Exec("SET TRANSACTION ISOLATION LEVEL SERIALIZABLE")
	var isUnlimited bool
	err = tx.QueryRow(
		"SELECT unlimited FROM subscriptions NATURAL JOIN PremiumPlans WHERE id = $1",
		uid).Scan(&isUnlimited)
	if err != nil && err != sql.ErrNoRows {
		return
	}
	if isUnlimited {
		err = nil
		rem = 1000000
		return
	}
	err = tx.QueryRow("SELECT freebalance FROM users WHERE id = $1", uid).Scan(&rem)
	if err != nil {
		return
	}
	rem -= amt
	if rem < 0 {
		rem = 0
	}
	_, err = tx.Exec("UPDATE users SET freebalance = $1 WHERE id = $2", rem, uid)
	if err != nil {
		return
	}
	err = tx.Commit()
	return
}

func (cmd *Command) decLegacyAccBalance(uid string, amt int) (rem int, err error) {
	// grab a TX at the database
	tx, err := cmd.pgdb.Begin()
	if err != nil {
		return
	}
	defer tx.Rollback()
	tx.Exec("SET TRANSACTION ISOLATION LEVEL SERIALIZABLE")
	// get the current value
	rw := tx.QueryRow("SELECT Mbs FROM AccBalances WHERE Uid = $1", uid)
	err = rw.Scan(&rem)
	if err != nil {
		return
	}
	// we don't really care about whether the remaining fails or succeeds
	// we deduct amt from rem
	rem -= amt
	if rem < 0 {
		rem = 0
	}
	// set the thing in the database to rem
	tx.Exec("UPDATE AccBalances SET Mbs = $1 WHERE Uid = $2", rem, uid)
	tx.Commit()
	return
}
