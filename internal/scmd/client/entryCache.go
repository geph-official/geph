package client

import (
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
