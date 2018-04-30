package client

import (
	"database/sql"
	"encoding/json"
	"sync"
)

type entryCache interface {
	GetEntries() map[string][]entryInfo
	SetEntries(map[string][]entryInfo)
}

type memEntryCache struct {
	entries map[string][]entryInfo
	lk      sync.Mutex
}

func (mec *memEntryCache) GetEntries() map[string][]entryInfo {
	mec.lk.Lock()
	defer mec.lk.Unlock()
	return mec.entries
}

func (mec *memEntryCache) SetEntries(ei map[string][]entryInfo) {
	mec.lk.Lock()
	defer mec.lk.Unlock()
	mec.entries = ei
}

type sqliteEntryCache struct {
	sdb *sql.DB
}

func (sec *sqliteEntryCache) GetEntries() map[string][]entryInfo {
	var bts []byte
	sec.sdb.QueryRow("SELECT v FROM main WHERE k='bst.entries'").Scan(&bts)
	if bts == nil {
		return nil
	}
	var toret map[string][]entryInfo
	json.Unmarshal(bts, &toret)
	return toret
}

func (sec *sqliteEntryCache) SetEntries(ei map[string][]entryInfo) {
	bts, _ := json.Marshal(&ei)
	sec.sdb.Exec("INSERT OR REPLACE INTO main VALUES('bst.entries', $1)", bts)
}
