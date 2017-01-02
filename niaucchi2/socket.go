package niaucchi2

import (
	"bytes"
	"io"
	"log"
	"math/rand"
	"sync"

	"github.com/lunixbochs/struc"

	"gopkg.in/tomb.v2"
)

type socket struct {
	sockid   socketID
	parent   *subCtx
	incoming chan segment
	inbuf    bytes.Buffer

	sendwind  chan struct{}
	recvcount uint8 // HACK this acts like 256 when started...

	wlock sync.Mutex
	once  sync.Once

	death tomb.Tomb
}

// Close closes the socket, duh.
func (sok *socket) Close() (err error) {
	sok.once.Do(func() {
		sok.wlock.Lock()
		defer sok.wlock.Unlock()
		sok.parent.wirewlok.Lock()
		defer sok.parent.wirewlok.Unlock()
		struc.Pack(sok.parent.wire, &segment{Flag: flClos, Sokid: uint16(sok.sockid)})
		sok.death.Kill(io.ErrClosedPipe)
	})
	return
}

// Write writes from a buffer.
func (sok *socket) Write(p []byte) (n int, err error) {
	maxKbs := 8
	if len(p) > maxKbs*1024 {
		var fn int
		fn, err = sok.realWrite(p[:maxKbs*1024])
		if err != nil {
			return
		}
		var rn int
		rn, err = sok.Write(p[maxKbs*1024:])
		if err != nil {
			return
		}
		n = fn + rn
		return
	}
	// for smaller writes pass through
	return sok.realWrite(p)
}

func (sok *socket) realWrite(p []byte) (n int, err error) {
	sok.sendwind <- struct{}{}
	//log.Println("niaucchi2: sendwind decreased to", 256-len(sok.sendwind), "on", sok.sockid)
	sok.wlock.Lock()
	defer sok.wlock.Unlock()
	sok.parent.wirewlok.Lock()
	defer sok.parent.wirewlok.Unlock()
	if !sok.death.Alive() {
		err = sok.death.Err()
		return
	}
	err = struc.Pack(sok.parent.wire, &segment{Flag: flData,
		Sokid: uint16(sok.sockid),
		Body:  p})
	if err != nil {
		return
	}
	n = len(p)
	return
}

// Read reads into a buffer
func (sok *socket) Read(p []byte) (n int, err error) {
	// If there's anything in the buffer, we always read from there first
	if sok.inbuf.Len() != 0 {
		return sok.inbuf.Read(p)
	}
	// Is there an error?
	if !sok.death.Alive() {
		err = sok.death.Err()
		return
	}
	// Otherwise, we fill the buffer
	select {
	case newseg := <-sok.incoming:
		switch newseg.Flag {
		case flData:
			//log.Println("niaucchi2: socket", sok.sockid, "got DATA", len(newseg.Body))
			n = copy(p, newseg.Body)
			sok.inbuf.Write(newseg.Body[n:])
			sok.recvcount--
			//log.Println("niaucchi2: socket", sok.sockid, "recv window", sok.recvcount)
			if sok.recvcount < uint8(rand.Int()%64+32) {
				const COUNT = 64
				sok.recvcount += COUNT
				log.Println("niaucchi2: boosting recv window in",
					sok.sockid, "by 128 to", sok.recvcount)
				// tell the other side too
				go func() {
					sok.wlock.Lock()
					defer sok.wlock.Unlock()
					sok.parent.wirewlok.Lock()
					defer sok.parent.wirewlok.Unlock()
					// we must check aliveness here due to a potential race
					if sok.death.Alive() {
						struc.Pack(sok.parent.wire, &segment{
							Flag:  flIcwd,
							Sokid: uint16(sok.sockid),
							Body:  []byte{COUNT}})
					}
				}()
			}
			return
		case flClos:
			log.Println("niaucchi2: socket", sok.sockid, "got CLOS, deregistering")
			sok.parent.parent.tabLock.Lock()
			delete(sok.parent.parent.sokTable, sok.sockid)
			sok.parent.parent.tabLock.Unlock()
			sok.death.Kill(io.EOF)
			err = io.EOF
			return
		default:
			log.Println("niaucchi2: socket", sok.sockid, "got garbage!", newseg)
			sok.parent.parent.death.Kill(ErrProtocolFail)
			err = ErrProtocolFail
			return
		}
	}
}
