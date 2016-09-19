package niaucchi

import (
	"bytes"
	"io"
	"net"
	"sync"
	"time"

	"gopkg.in/tomb.v2"
)

// ssConn is a substrate-supported connection.
type ssConn struct {
	daddy *Substrate

	sorted chan segment // JUST the flData and flClos segments
	dnbuf  bytes.Buffer
	dneof  bool
	connid uint16

	sendbar chan struct{} // flow control barrier; new flData segments
	sendctr uint64
	sendlok sync.Mutex

	tmb *tomb.Tomb
}

// Write writes from a buffer.
func (sc *ssConn) Write(p []byte) (n int, err error) {
	sc.sendlok.Lock()
	defer sc.sendlok.Unlock()
	pcp := make([]byte, len(p))
	copy(pcp, p)
	tosend := segment{
		Flag:   flData,
		ConnID: sc.connid,
		Serial: sc.sendctr,
		Body:   pcp,
	}
	sc.sendctr++
	// send off the tosend
	select {
	case sc.daddy.upch <- tosend:
	case <-sc.tmb.Dying():
		err = io.ErrClosedPipe
		return
	}
	// wait for the ack
	select {
	case sc.sendbar <- struct{}{}:
	case <-sc.tmb.Dying():
		err = io.ErrClosedPipe
		return
	}
	n = len(p)
	return
}

// Close does what it says on the tin.
func (sc *ssConn) Close() (err error) {
	sc.sendlok.Lock()
	defer sc.sendlok.Unlock()
	tosend := segment{
		Flag:   flClos,
		ConnID: sc.connid,
		Serial: sc.sendctr,
	}
	select {
	case sc.daddy.upch <- tosend:
	case <-sc.daddy.mtmb.Dying():
		return
	}
	go func() {
		time.Sleep(time.Second * 2)
		sc.tmb.Kill(io.ErrClosedPipe)
	}()
	return
}

// Read reads into a buffer.
func (sc *ssConn) Read(p []byte) (n int, err error) {
	if sc.dnbuf.Len() != 0 {
		return sc.dnbuf.Read(p)
	}

	if sc.dneof {
		err = io.EOF
		return
	}

	select {
	case <-sc.tmb.Dying():
		err = io.ErrClosedPipe
		return
	case seg := <-sc.sorted:
		//log.Println(seg)
		if seg.Flag == flData {
			// get ack back first
			select {
			case sc.daddy.upch <- segment{
				Flag:   flAck,
				ConnID: sc.connid,
			}:
			case <-sc.tmb.Dying():
				err = io.ErrClosedPipe
				return
			}
			// then copy into buf
			n = copy(p, seg.Body)
			if n < len(seg.Body) {
				sc.dnbuf.Write(seg.Body[n:])
			}
			return
		} else if seg.Flag == flClos {
			sc.dneof = true
			err = io.EOF
			sc.tmb.Kill(io.ErrClosedPipe)
			return
		}
	}
	return
}

// LocalAddr **returns nil**
func (sc *ssConn) LocalAddr() net.Addr {
	return nil
}

// RemoteAddr **returns nil**
func (sc *ssConn) RemoteAddr() net.Addr {
	return nil
}

// SetDeadline **does nothing**
func (sc *ssConn) SetDeadline(t time.Time) error {
	return nil
}

// SetReadDeadline is currently **identical to SetDeadline**
func (sc *ssConn) SetReadDeadline(t time.Time) error { return sc.SetDeadline(t) }

// SetWriteDeadline is currently **identical to SetDeadline**
func (sc *ssConn) SetWriteDeadline(t time.Time) error { return sc.SetDeadline(t) }

func newSsConn(tmb *tomb.Tomb, daddy *Substrate, incoming chan segment, connid uint16) *ssConn {
	sorted := make(chan segment, 1024)
	tmb.Go(func() error {
		select {
		case <-daddy.mtmb.Dying():
			return daddy.mtmb.Err()
		case <-tmb.Dying():
			return nil
		}
	})

	ssc := &ssConn{
		daddy:  daddy,
		sorted: sorted,
		connid: connid,
		tmb:    tmb,

		sendbar: make(chan struct{}, 64),
	}

	go func() {
		<-tmb.Dead()
		go func() {
			daddy.cblok.Lock()
			daddy.delCallback(connid)
			daddy.cblok.Unlock()
		}()
	}()

	// dispatch goroutine
	tmb.Go(func() error {
		srtr := newSorter()
		for {
			select {
			case <-ssc.tmb.Dying():
				return nil
			case seg := <-incoming:
				switch seg.Flag {
				case flClos:
					fallthrough
				case flData:
					srtr.Push(seg.Serial, &seg)
					segs := srtr.Pop()
					for _, v := range segs {
						select {
						case sorted <- *(v.(*segment)):
						case <-ssc.tmb.Dying():
							return nil
						}
					}
				case flAck:
					select {
					case <-ssc.sendbar:
					case <-tmb.Dying():
						return nil
					}
				}
			}
		}
	})

	return ssc
}
