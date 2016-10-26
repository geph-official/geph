package exit

import (
	"log"
	"math/rand"
	"net"
	"sync"
	"time"
)

type entryDB struct {
	expireTab map[string]time.Time
	cookieTab map[string][]byte
	lok       sync.RWMutex
}

func newEntryDB() *entryDB {
	toret := &entryDB{
		expireTab: make(map[string]time.Time),
		cookieTab: make(map[string][]byte),
	}
	go func() {
		for {
			time.Sleep(time.Minute)
			var toDelete []string
			toret.lok.RLock()
			for addr, expire := range toret.expireTab {
				if expire.Before(time.Now()) {
					toDelete = append(toDelete, addr)
				}
			}
			toret.lok.RUnlock()
			toret.lok.Lock()
			for _, v := range toDelete {
				delete(toret.expireTab, v)
				delete(toret.cookieTab, v)
			}
			toret.lok.Unlock()
		}
	}()
	return toret
}

func (edb *entryDB) AddNode(addr string, cookie []byte) error {
	wire, err := net.DialTimeout("tcp", addr, time.Second*5)
	if err != nil {
		return err
	}
	wire.Close()
	edb.lok.Lock()
	defer edb.lok.Unlock()
	log.Println("FIXME: AddNode does not do proper checking")
	edb.cookieTab[addr] = cookie
	edb.expireTab[addr] = time.Now().Add(time.Minute * 5)
	return nil
}

func (edb *entryDB) GetNodes(seed uint64) (nodes map[string][]byte) {
	edb.lok.RLock()
	defer edb.lok.RUnlock()
	// seed an insecure RNG; the insecurity of the RNG shouldn't be an issue
	badrng := rand.New(rand.NewSource(int64(seed)))
	var allnodes []string
	for k := range edb.cookieTab {
		allnodes = append(allnodes, k)
	}
	// now we shuffle
	for i := 0; i < len(allnodes)-1; i++ {
		j := badrng.Int()%(len(allnodes)-i) + i
		tmp := allnodes[j]
		allnodes[j] = allnodes[i]
		allnodes[i] = tmp
	}
	// take first 3 at most
	nodes = make(map[string][]byte)
	for i := 0; i < 3 && i < len(allnodes); i++ {
		nodes[allnodes[i]] = edb.cookieTab[allnodes[i]]
	}
	return
}
