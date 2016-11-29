package niaucchi

import (
	"errors"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lunixbochs/struc"
	"gopkg.in/bunsim/miniss.v1"

	"gopkg.in/bunsim/natrium.v1"

	"gopkg.in/tomb.v2"
)

// ErrProtocolFail indicates a fatal protocol failure.
var ErrProtocolFail = errors.New("protocol failure")

// ErrOperationTimeout indicates that an operation (such as waiting for an ack) timed out fatally.
var ErrOperationTimeout = errors.New("operation timed out")

// ErrWatchdogTimeout indicates that the watchdog timed out fatally.
var ErrWatchdogTimeout = errors.New("watchdog timed out")

// Substrate represents a pool of connections over which carried connections are multiplexed.
type Substrate struct {
	upch chan segment
	opch chan segment
	mtmb *tomb.Tomb

	cbtab map[uint16]chan segment
	cblok sync.Mutex

	transport []net.Conn

	queuebts uint32
}

func (ss *Substrate) incQueueBts(siz int) {
	atomic.AddUint32(&ss.queuebts, uint32(siz))
}

func (ss *Substrate) decQueueBts(siz int) {
	atomic.AddUint32(&ss.queuebts, ^uint32(siz-1))
}

func (ss *Substrate) regCallback(connid uint16) chan segment {
	if ss.cbtab[connid] != nil {
		return ss.cbtab[connid]
	}
	ch := make(chan segment, 256)
	ss.cbtab[connid] = ch
	return ch
}

func (ss *Substrate) delCallback(connid uint16) {
	if ss.cbtab[connid] != nil {
		delete(ss.cbtab, connid)
	}
}

// RemotePK gets their PK.
func (ss *Substrate) RemotePK() natrium.ECDHPublic {
	return ss.transport[0].(*miniss.Socket).RemotePK()
}

// Tomb returns the substrate's associated tomb.
func (ss *Substrate) Tomb() *tomb.Tomb {
	return ss.mtmb
}

// AcceptConn accepts a new tunneled connection over the substrate.
func (ss *Substrate) AcceptConn() (cn net.Conn, data []byte, err error) {
	var msg segment
	select {
	case <-ss.mtmb.Dying():
		err = ss.mtmb.Err()
		return
	case msg = <-ss.opch:
	}
	// get the callback
	tmb := new(tomb.Tomb)
	ss.cblok.Lock()
	down := ss.regCallback(msg.ConnID)
	ss.cblok.Unlock()
	// we now ack the opening if it's a legacy Open
	if msg.Flag == flOpen {
		log.Println("niaucchi: legacy OPEN (must ack) received on", msg.ConnID)
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
	} else {
		// it's a FastOpen, copy the additional data
		log.Println("niaucchi: FASTOPEN received on", msg.ConnID)
		data = msg.Body
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
	tmb := new(tomb.Tomb)
	log.Println("niaucchi: opening", connid)
	down := ss.regCallback(connid)
	ss.cblok.Unlock()
	tosend := segment{
		Flag:   flFastOpen,
		ConnID: connid,
	}
	select {
	case ss.upch <- tosend:
	case <-ss.mtmb.Dying():
		err = ss.mtmb.Err()
		return
	case <-time.After(time.Second * 15):
		ss.mtmb.Kill(ErrOperationTimeout)
		err = ss.mtmb.Err()
		return
	}
	/*select {
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
		ss.mtmb.Kill(ErrOperationTimeout)
		err = ss.mtmb.Err()
	}*/
	cn = newSsConn(tmb, ss, down, connid)
	return
}

// NewSubstrate creates a Substrate from the given connections.
func NewSubstrate(transport []net.Conn) *Substrate {
	toret := &Substrate{
		upch:      make(chan segment),
		opch:      make(chan segment, 256),
		mtmb:      new(tomb.Tomb),
		cbtab:     make(map[uint16]chan segment),
		transport: transport,
	}

	// the watchdog
	kikr := make(chan bool)
	toret.mtmb.Go(func() error {
		for {
			select {
			case <-toret.mtmb.Dying():
				return nil
			case <-time.After(time.Minute * 5):
				return ErrWatchdogTimeout
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
				// regardless of the above, we do this
				if lol.Flag == flOpen || lol.Flag == flFastOpen {
					select {
					case toret.opch <- lol:
					default:
						log.Println("niaucchi: overfull accept buffer!")
						return ErrProtocolFail
					}
					continue
				}
				toret.cblok.Lock()
				f, ok := toret.cbtab[lol.ConnID]
				toret.cblok.Unlock()
				// if the thing doesn't exist yet, create it
				if !ok {
					if lol.Flag == flClos {
						// just ignore
						continue
					}
					log.Println("niaucchi: IMPLICITLY opening", lol.ConnID)
					toret.cblok.Lock()
					f = toret.regCallback(lol.ConnID)
					toret.cblok.Unlock()
				}
				select {
				case f <- lol:
				default:
					log.Println("niaucchi: overfull read buffer!")
					return ErrProtocolFail
				}
			}
		})
	}

	// keep alive thread
	tmb.Go(func() error {
		for {
			select {
			case <-time.After(time.Minute * 2):
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
