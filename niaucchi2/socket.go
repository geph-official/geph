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
	mlen := 8192
	if len(p) > mlen {
		var fn int
		fn, err = sok.realWrite(p[:mlen])
		if err != nil {
			return
		}
		var rn int
		rn, err = sok.Write(p[mlen:])
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
	// calculate a good value for pad
	var pad int
	if len(p) > 4096 {
		// no overhead for full-8192 payloads
		pad = 8192 + 1 + 2 + 2
	} else if len(p) > 2048 {
		pad = 4096
	} else if len(p) > 1024 {
		pad = 2048
	} else if len(p) > 512 {
		pad = 1024 + rand.Int()%512
	} else if len(p) > 256 {
		pad = 512 + rand.Int()%256
	} else if len(p) > 128 {
		pad = 256 + rand.Int()%128
	} else {
		pad = 128 + rand.Int()%64
	}
	// calculate how many bytes to add to make the write be pad
	fpadlen := pad
	pad -= len(p)          // decrease length of p
	pad -= (1 + 2 + 2) * 2 // overhead of two packets
	if pad <= 0 {
		err = struc.Pack(sok.parent.wire, &segment{Flag: flData,
			Sokid: uint16(sok.sockid),
			Body:  p})
	} else {
		buf := new(bytes.Buffer)
		struc.Pack(buf, &segment{Flag: flData,
			Sokid: uint16(sok.sockid), Body: p})
		struc.Pack(buf, &segment{Flag: flAliv,
			Body: make([]byte, pad)})
		if buf.Len() != fpadlen {
			panic("didn't reach the correct padding length")
		}
		_, err = sok.parent.wire.Write(buf.Bytes())
	}
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
			sok.death.Kill(io.EOF)
			err = io.EOF
			return
		default:
			log.Println("niaucchi2: socket", sok.sockid, "got garbage!", newseg)
			sok.parent.parent.death.Kill(ErrProtocolFail)
			err = ErrProtocolFail
			return
		}
	case <-sok.death.Dying():
		err = sok.death.Err()
		return
	}
}
