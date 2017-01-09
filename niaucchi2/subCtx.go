package niaucchi2

import (
	"errors"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/lunixbochs/struc"

	"gopkg.in/tomb.v2"
)

// ErrTimeout indicates a timeout error.
var ErrTimeout = errors.New("niaucchi2: watchdog timed out")

type subCtx struct {
	parent   *Context
	subid    subCtxID
	wire     net.Conn
	wirewlok sync.Mutex

	wdogkick chan struct{}

	death tomb.Tomb
}

func (sctx *subCtx) sendAliv() error {
	var wait int
	if sctx.parent.isClient {
		wait = 800
	} else {
		wait = 120
	}
	for {
		sctx.wirewlok.Lock()
		err := struc.Pack(sctx.wire, &segment{Flag: flAliv})
		sctx.wirewlok.Unlock()
		if err != nil {
			return err
		}
		//time.Sleep(time.Second * 5)
		time.Sleep(time.Second * time.Duration(wait+rand.Int()%60))
	}
}

func (sctx *subCtx) watchdog() error {
	go sctx.sendAliv() // This should not block the death
	var wait time.Duration
	if sctx.parent.isClient {
		wait = time.Minute * 4
	} else {
		wait = time.Minute * 20
	}
	for {
		select {
		case <-time.After(wait):
			log.Println("niaucchi2: watchdog timed out on", sctx.subid)
			return ErrTimeout
		case <-sctx.wdogkick:
		case <-sctx.death.Dying():
			return sctx.death.Err()
		}
	}
}

func (sctx *subCtx) mainThread() (err error) {
	// If our parent dies, so do we!
	sctx.death.Go(func() error {
		defer sctx.wire.Close()
		select {
		case <-sctx.death.Dying():
			return sctx.death.Err()
		case <-sctx.parent.death.Dying():
			return sctx.parent.death.Err()
		}
	})
	defer sctx.wire.Close()
	// Spin off watchdog
	sctx.wdogkick = make(chan struct{}, 1)
	sctx.death.Go(sctx.watchdog)
	// Main thread only takes care of reading from the wire.
	for {
		var newseg segment
		err = struc.Unpack(sctx.wire, &newseg)
		if err != nil {
			sctx.parent.death.Kill(err)
			log.Println("niaucchi2:", sctx.subid, "died due to wire:", err.Error())
			return
		}
		select {
		case sctx.wdogkick <- struct{}{}:
		default:
		}
		switch newseg.Flag {
		case flPing:
			log.Println("niaucchi2: PING on", sctx.subid)
			go func() {
				sctx.wirewlok.Lock()
				defer sctx.wirewlok.Unlock()
				struc.Pack(sctx.wire, &newseg)
			}()
		case flAliv:
			log.Println("niaucchi2: ALIV on", sctx.subid)
		case flOpen:
			log.Println("niaucchi2: OPEN", newseg.Sokid, "on", sctx.subid)
			// We have to be a server
			if sctx.parent.isClient {
				log.Println("niaucchi2:", sctx.subid, "got nonsensical OPEN, dying")
				err = ErrProtocolFail
				sctx.parent.death.Kill(err)
				return
			}
			// Construct a socket
			newsok := &socket{
				sockid:   socketID(newseg.Sokid),
				parent:   sctx,
				incoming: make(chan segment, 256),
				sendwind: make(chan struct{}, 256),
			}
			// Tie up the death of the socket with our death
			go func() {
				select {
				case <-sctx.death.Dying():
					newsok.death.Kill(sctx.death.Err())
				case <-newsok.death.Dying():
				}
			}()
			// Stuff the new socket into our parent
			sctx.parent.tabLock.Lock()
			sctx.parent.sokTable[newsok.sockid] = newsok
			sctx.parent.tabLock.Unlock()
			// Put into accept queue
			select {
			case sctx.parent.acptQueue <- newsok:
			default:
				log.Println("niaucchi2: overfull accept queue! dying")
				err = ErrProtocolFail
				sctx.parent.death.Kill(err)
				return
			}
		default:
			// Look up the correct socket
			var dest *socket
			sctx.parent.tabLock.RLock()
			dest = sctx.parent.sokTable[socketID(newseg.Sokid)]
			sctx.parent.tabLock.RUnlock()
			if dest == nil {
				log.Println("niaucchi2: stray", newseg, "on", sctx.subid, ", dying")
				err = ErrProtocolFail
				sctx.parent.death.Kill(err)
				return
			}
			if newseg.Flag == flIcwd {
				log.Println("niaucchi2: ICWD", newseg.Sokid, newseg.Body[0], "on", sctx.subid)
				for i := byte(0); i < newseg.Body[0]; i++ {
					select {
					case <-dest.sendwind:
					default:
						log.Println("niaucchi2: ICWD OVERFLOW", newseg.Sokid, "on", sctx.subid)
						err = ErrProtocolFail
						sctx.parent.death.Kill(err)
						return
					}
				}
			} else {
				if newseg.Flag == flClos {
					log.Println("niaucchi2: socket", dest.sockid, "got CLOS, deregistering")
					sctx.parent.tabLock.Lock()
					delete(sctx.parent.sokTable, dest.sockid)
					sctx.parent.tabLock.Unlock()
				}
				select {
				case dest.incoming <- newseg:
				default:
					log.Println("niaucchi2: overfull read buffer in", sctx.subid, ", dying")
					err = ErrProtocolFail
					sctx.parent.death.Kill(err)
					return
				}
			}
		}
	}
}
