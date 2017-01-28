package exit

import (
	"bytes"
	"database/sql"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"gopkg.in/bunsim/natrium.v1"
	// SQLite interface
	_ "gopkg.in/mattn/go-sqlite3.v1"
)

type entryDB struct {
	dbHand *sql.DB
}

func newEntryDB() *entryDB {
	db, _ := sql.Open("sqlite3", "file::memory:?cache=shared")
	db.Exec("create table clients (cid integer unique not null)")
	db.Exec(`create table nodes (nid text unique not null, addr text not null,
		 asn text not null, lastseen integer not null)`)
	db.Exec(`create table mapping (
		cid integer,
		nid text,
		foreign key(cid) references clients(cid) on delete cascade,
		foreign key(uid) references nodes(nid) on delete cascade
	)`)
	// police based on lastseen
	go func() {
		for {
			time.Sleep(time.Minute)
			tx, err := db.Begin()
			if err != nil {
				continue
			}
			tx.Exec("pragma foreign_keys=1")
			tx.Exec("delete from nodes where lastseen<$1", time.Now().Add(-time.Minute*5))
			tx.Commit()
		}
	}()
	return &entryDB{db}
}

func (edb *entryDB) getASN(addr string) (string, error) {
	var asn string
	err := edb.dbHand.QueryRow("select asn from nodes where addr=$1", addr).Scan(&asn)
	if err != nil {
		// TODO do this locally
		resp, err := http.Get("https://ipinfo.io/" + strings.Split(addr, ":")[0] + "/org")
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, resp.Body)
		if err != nil {
			return "", err
		}
		log.Println("remote query for ASN got", string(buf.Bytes()))
		return strings.Split(string(buf.Bytes()), " ")[0], nil
	}
	return asn, nil
}

func (edb *entryDB) AddNode(addr string, cookie []byte) error {
	asn, err := edb.getASN(addr)
	if err != nil {
		return err
	}
	tx, err := edb.dbHand.Begin()
	if err != nil {
		return err
	}
	defer tx.Commit()
	tx.Exec("pragma foreign_keys=1")
	_, err = tx.Exec("insert or replace into nodes values($1, $2, $3, $4)",
		natrium.HexEncode(cookie), addr, asn, time.Now().Unix())
	return err
}

func (edb *entryDB) GetNodes(seed int) (nodes map[string][]byte) {
	tx, err := edb.dbHand.Begin()
	if err != nil {
		return make(map[string][]byte)
	}
	defer tx.Commit()
	tx.Exec("pragma foreign_keys=1")
	// put ourselves into the system first
	tx.Exec("insert or replace into clients", seed)
	for try := 0; try < 20; try++ {
		// reinitialize nodes
		nodes = make(map[string][]byte)
		// we try to use existing mapping if possible
		var existnum int
		err := tx.QueryRow("select count(*) from mapping where cid=$1", seed).Scan(&existnum)
		if err != nil {
			return
		}
		// if the existing mapping is acceptable use it
		var rows *sql.Rows
		if existnum == 3 {
			rows, err = tx.Query(`select nid,addr,asn from nodes
				natural join mapping where cid=$1`, seed)
		} else {
			tx.Exec("delete from mapping where cid=$1", seed)
			// fill with random nodes
			rows, err = tx.Query("select nid,addr,asn from nodes order by random() limit 3")
			if err != nil {
				return
			}
		}
		seenasns := make(map[string]bool)
		for rows.Next() {
			var nid, addr, asn string
			err = rows.Scan(&nid, &addr, &asn)
			if err != nil {
				return
			}
			bts, err := natrium.HexDecode(nid)
			if err != nil {
				panic(err.Error())
			}
			nodes[addr] = bts
			seenasns[asn] = true
			tx.Exec("insert into mapping values($1,$2)", seed, nid)
		}
		// enforce constraints
		if len(seenasns) > 1 {
			// we have more ASN, good to go
			return
		}
	}
	// give up, return with what we have
	return
}
