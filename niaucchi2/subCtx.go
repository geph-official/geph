package niaucchi2

import (
	"log"
	"net"
	"sync"

	"github.com/lunixbochs/struc"

	"gopkg.in/tomb.v2"
)

type subCtx struct {
	parent   *Context
	subid    subCtxID
	wire     net.Conn
	wirewlok sync.Mutex

	death tomb.Tomb
}

func (sctx *subCtx) mainThread() (err error) {
	// If our parent dies, so do we!
	sctx.death.Go(func() error {
		select {
		case <-sctx.death.Dying():
			return sctx.death.Err()
		case <-sctx.parent.death.Dying():
			return sctx.parent.death.Err()
		}
	})
	defer sctx.wire.Close()
	// Main thread only takes care of reading from the wire.
	for {
		var newseg segment
		err = struc.Unpack(sctx.wire, &newseg)
		if err != nil {
			log.Println("niaucchi2:", sctx.subid, "died due to wire:", err.Error())
			return
		}
		switch newseg.Flag {
		case flOpen:
			log.Println("niaucchi2: OPEN", newseg.Sokid, "on", sctx.subid)
			// We have to be a server
			if sctx.parent.isClient {
				log.Println("niaucchi2:", sctx.subid, "got nonsensical OPEN, dying")
				err = ErrProtocolFail
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
				<-sctx.death.Dying()
				newsok.death.Kill(sctx.death.Err())
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
				return
			}
		default:
			// Look up the correct socket
			var dest *socket
			sctx.parent.tabLock.RLock()
			dest = sctx.parent.sokTable[socketID(newseg.Sokid)]
			sctx.parent.tabLock.RUnlock()
			if dest == nil {
				log.Println("niaucchi2: stray", newseg.Flag, "on", sctx.subid, ", dying")
				err = ErrProtocolFail
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
						return
					}
				}

			} else {
				select {
				case dest.incoming <- newseg:
				default:
					log.Println("niaucchi2: overfull read buffer in", sctx.subid, ", dying")
					err = ErrProtocolFail
					return
				}
			}
		}
	}
}
