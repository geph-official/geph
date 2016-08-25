package exit

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/ProjectNiwl/natrium"
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
				if expire.After(time.Now()) {
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
	if seed != 0 {
		panic("GetNodes doesn't actually support a seed yet")
	}
	log.Println("FIXME: GetNodes does not use the seed")
	var allnodes []string
	for k := range edb.cookieTab {
		allnodes = append(allnodes, k)
	}
	// now we shuffle
	for i := 0; i < len(allnodes)-1; i++ {
		j := int(natrium.RandUint32LT(uint32(len(allnodes)-i))) + i
		tmp := allnodes[j]
		allnodes[j] = allnodes[i]
		allnodes[i] = tmp
	}
	// take first 5 at most
	nodes = make(map[string][]byte)
	for i := 0; i < 5 && i < len(allnodes); i++ {
		nodes[allnodes[i]] = edb.cookieTab[allnodes[i]]
	}
	return
}
