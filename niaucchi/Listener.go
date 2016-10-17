package niaucchi

import (
	"io"
	"net"
	"sync"
	"time"

	//"github.com/bunsim/miniss"

	"gopkg.in/bunsim/cluttershirt.v1"
	"gopkg.in/bunsim/miniss.v1"
	"gopkg.in/bunsim/natrium.v1"

	"gopkg.in/tomb.v2"
)

// Listener is a niaucchi listener.
type Listener struct {
	rl   net.Listener
	accq chan *Substrate
	tmb  *tomb.Tomb
}

// Addr returns the listening address.
func (ln *Listener) Addr() net.Addr {
	return ln.rl.Addr()
}

// AcceptSubstrate accepts a new client.
func (ln *Listener) AcceptSubstrate() (*Substrate, error) {
	select {
	case sok := <-ln.accq:
		return sok, nil
	case <-ln.tmb.Dying():
		return nil, ln.tmb.Err()
	}
}

// Listen returns a niaucchi listener. If the given obfuscation cookie is nil, the unobfuscated protocol will be used.
func Listen(ocookie []byte, identity natrium.ECDHPrivate, addr string) (lsnr *Listener, err error) {
	lsnr = new(Listener)
	lsnr.rl, err = net.Listen("tcp", addr)
	if err != nil {
		return
	}
	lsnr.accq = make(chan *Substrate)
	lsnr.tmb = new(tomb.Tomb)
	go func() {
		var cue = make(map[string][]net.Conn)
		var lok sync.Mutex

		for {
			raw, err := lsnr.rl.Accept()
			if err != nil {
				lsnr.tmb.Kill(err)
				return
			}
			go func() {
				var pipe net.Conn
				var err error
				// optionally do obfuscation
				if ocookie != nil {
					pipe, err = cluttershirt.Server(ocookie, raw)
					if err != nil {
						raw.Close()
						return
					}
				} else {
					pipe = raw
				}
				// always do kiss
				pipe, err = miniss.Handshake(pipe, identity)
				if err != nil {
					raw.Close()
					return
				}
				// read the multiplier
				mlt := make([]byte, 1)
				io.ReadFull(pipe, mlt)
				mult := int(mlt[0])
				// read the bunch identifier
				bid := make([]byte, 32)
				_, err = io.ReadFull(pipe, bid)
				if err != nil {
					pipe.Close()
					return
				}
				bids := string(bid)
				// put in cue
				lok.Lock()
				old := cue[bids]
				// if the first one, schedule cleanup
				if len(old) == 0 {
					go func() {
						time.Sleep(time.Second * 60)
						lok.Lock()
						defer lok.Unlock()
						_, ok := cue[bids]
						if ok {
							for _, v := range cue[bids] {
								v.Close()
							}
							delete(cue, bids)
						}
					}()
				}
				cue[bids] = append(old, pipe)
				// if already mult, break off
				noo := cue[bids]
				if len(noo) == mult {
					delete(cue, bids)
					go func() {
						select {
						case lsnr.accq <- NewSubstrate(noo):
						case <-lsnr.tmb.Dying():
							return
						}
					}()
				}
				lok.Unlock()
			}()
		}
	}()
	return
}
