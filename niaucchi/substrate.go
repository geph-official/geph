package niaucchi

import (
	"errors"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/lunixbochs/struc"

	"gopkg.in/bunsim/natrium.v1"

	"gopkg.in/tomb.v2"
)

// ErrProtocolFail indicates an error due to a protocol failure.
var ErrProtocolFail = errors.New("protocol failure")

// Substrate represents a pool of connections over which carried connections are multiplexed.
type Substrate struct {
	upch chan segment
	opch chan segment
	mtmb *tomb.Tomb

	cbtab map[uint16]func(segment)
	cblok sync.Mutex
}

func (ss *Substrate) regCallback(connid uint16, cback func(segment)) bool {
	if ss.cbtab[connid] != nil {
		return false
	}
	ss.cbtab[connid] = cback
	return true
}

func (ss *Substrate) delCallback(connid uint16) {
	if ss.cbtab[connid] != nil {
		delete(ss.cbtab, connid)
	}
	return
}

// Tomb returns the substrate's associated tomb.
func (ss *Substrate) Tomb() *tomb.Tomb {
	return ss.mtmb
}

// AcceptConn accepts a new tunneled connection over the substrate.
func (ss *Substrate) AcceptConn() (cn net.Conn, err error) {
	var msg segment
	select {
	case <-ss.mtmb.Dying():
		err = ss.mtmb.Err()
		return
	case msg = <-ss.opch:
		log.Println("niaucchi: AcceptConn() accepted flOpen", msg.ConnID)
	}
	// register a callback
	down := make(chan segment, 128)
	tmb := new(tomb.Tomb)
	ss.cblok.Lock()
	ss.regCallback(msg.ConnID, func(s segment) {
		select {
		case down <- s:
		case <-tmb.Dying():
		}
	})
	ss.cblok.Unlock()
	// we now ack the opening
	ack := segment{
		Flag:   flAck,
		ConnID: msg.ConnID,
	}
	select {
	case ss.upch <- ack:
	case <-ss.mtmb.Dying():
		err = ss.mtmb.Err()
		return
	}
	// create the context
	cn = newSsConn(tmb, ss, down, msg.ConnID)
	return
}

// OpenConn opens a new tunneled connection over the substrate.
func (ss *Substrate) OpenConn() (cn net.Conn, err error) {
	ss.cblok.Lock()
	connid := uint16(0)
	for {
		connid = uint16(natrium.RandUint32())
		if ss.cbtab[connid] == nil {
			break
		}
	}
	down := make(chan segment, 1)
	tmb := new(tomb.Tomb)
	ss.regCallback(connid, func(s segment) {
		select {
		case down <- s:
		case <-tmb.Dying():
		}
	})
	ss.cblok.Unlock()
	tosend := segment{
		Flag:   flOpen,
		ConnID: connid,
	}
	select {
	case ss.upch <- tosend:
	case <-ss.mtmb.Dying():
		err = ss.mtmb.Err()
		return
	case <-time.After(time.Second * 15):
		ss.mtmb.Kill(errors.New("timeout"))
		err = ss.mtmb.Err()
		return
	}
	select {
	case seg := <-down:
		if seg.Flag != flAck {
			log.Println("niaucchi: app data before ack for open, requeuing")
			go func() {
				select {
				case down <- seg:
				case <-ss.mtmb.Dying():
				}
			}()
		}
		cn = newSsConn(tmb, ss, down, connid)
	case <-ss.mtmb.Dying():
		err = ss.mtmb.Err()
	case <-time.After(time.Second * 15):
		ss.mtmb.Kill(errors.New("timeout"))
		err = ss.mtmb.Err()
	}
	return
}

// NewSubstrate creates a Substrate from the given connections.
func NewSubstrate(transport []net.Conn) *Substrate {
	toret := &Substrate{
		upch:  make(chan segment),
		opch:  make(chan segment, 256),
		mtmb:  new(tomb.Tomb),
		cbtab: make(map[uint16]func(segment)),
	}

	// the watchdog
	kikr := make(chan bool)
	toret.mtmb.Go(func() error {
		for {
			select {
			case <-toret.mtmb.Dying():
				return nil
			case <-time.After(time.Minute * 2):
				return io.ErrClosedPipe
			case <-kikr:
			}
		}
	})
	tmb := toret.mtmb
	// we spin up the worker threads
	for _, cn := range transport {
		cn := cn
		tmb.Go(func() error {
			defer cn.Close()
			for i := 0; ; i++ {
				select {
				case <-tmb.Dying():
					return nil
				case towr := <-toret.upch:
					struc.Pack(cn, &towr)
				}
			}
		})
		// a thread executes callbacks and tracks flOpen packets
		tmb.Go(func() error {
			defer cn.Close()
			for {
				var lol segment
				err := struc.Unpack(cn, &lol)
				if err != nil {
					return err
				}
				if lol.Flag == flAliv {
					select {
					case kikr <- true:
					default:
					}
				}
				toret.cblok.Lock()
				f, ok := toret.cbtab[lol.ConnID]
				toret.cblok.Unlock()
				if !ok {

				} else {
					f(lol)
				}
				if lol.Flag == flOpen {
					select {
					case toret.opch <- lol:
					default:
						log.Println("niaucchi: overfull accept buffer!")
						return ErrProtocolFail
					}
				}
			}
		})
	}

	// keep alive thread
	tmb.Go(func() error {
		for {
			select {
			case <-time.After(time.Minute):
			case <-tmb.Dying():
				return nil
			}
			select {
			case toret.upch <- segment{
				Flag: flAliv,
			}:
			case <-tmb.Dying():
				return nil
			}
		}
	})
	return toret
}
