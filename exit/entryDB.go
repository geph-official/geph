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

func newEntryDB(fname string) *entryDB {
	if fname == "" {
		fname = "file::memory:?cache=shared"
	}
	fname = fname + "?foreign_keys=on"
	db, _ := sql.Open("sqlite3", fname)
	tx, _ := db.Begin()
	tx.Exec("create table if not exists clients (cid integer unique not null)")
	tx.Exec(`create table if not exists nodes (nid text unique not null, addr text not null,
		 asn text not null, lastseen integer not null)`)
	tx.Exec(`create table if not exists mapping (
		cid integer,
		nid text,
		foreign key(cid) references clients(cid) on delete cascade,
		foreign key(nid) references nodes(nid) on delete cascade
	)`)
	tx.Commit()
	// police based on lastseen
	go func() {
		for {
			time.Sleep(time.Second * 10)
			tx, err := db.Begin()
			if err != nil {
				continue
			}
			tx.Exec("delete from nodes where lastseen<$1", time.Now().Add(-time.Minute*3).Unix())
			tx.Commit()
		}
	}()
	return &entryDB{db}
}

func (edb *entryDB) getASN(addr string) (string, error) {
	var asn string
	tx, err := edb.dbHand.Begin()
	if err != nil {
		return "", err
	}
	err = tx.QueryRow("select asn from nodes where addr=$1", addr).Scan(&asn)
	tx.Commit()
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
		log.Println("remote query for ASN got", strings.Split(string(buf.Bytes()), " ")[0])
		return strings.Split(string(buf.Bytes()), " ")[0], nil
	}
	return asn, nil
}

func (edb *entryDB) AddNode(addr string, cookie []byte) error {
	// TODO validate that the node can be connected to
	asn, err := edb.getASN(addr)
	if err != nil {
		return err
	}
	tx, err := edb.dbHand.Begin()
	if err != nil {
		return err
	}
	defer tx.Commit()
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
	// put ourselves into the system first
	tx.Exec("insert or replace into clients values($1)", seed)
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
			if existnum != 3 {
				tx.Exec("insert into mapping values($1,$2)", seed, nid)
			}
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
