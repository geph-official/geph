package niaucchi3

import (
	"errors"
	"io"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/lunixbochs/struc"

	"gopkg.in/tomb.v2"
)

// ErrTablesFull means the internal tables used in the Context are full.
var ErrTablesFull = errors.New("niaucchi3: internal tables in Context are full")

// ErrProtocolFail means a fatal protocol violation.
var ErrProtocolFail = errors.New("niaucchi3: nonsensical network messages")

// ErrTimeout indicates a timeout error.
var ErrTimeout = errors.New("niaucchi3: timed out")

// Context represents a connection used to tunnel sockets.
type Context struct {
	isClient bool
	sctx     *subCtx
	sokTable map[socketID]*socket
	tabLock  sync.RWMutex

	acptQueue chan io.ReadWriteCloser

	death tomb.Tomb
}

// NewContext creates a new context.
func NewContext(isClient bool, conn net.Conn) (ctx *Context) {
	ctx = &Context{
		isClient:  isClient,
		sokTable:  make(map[socketID]*socket),
		acptQueue: make(chan io.ReadWriteCloser, 1024),
	}
	ctx.sctx = &subCtx{
		parent: ctx,
		wire:   conn,
	}
	ctx.death.Go(ctx.sctx.mainThread)
	return
}

// Tomb returns the tomb of the context.
func (ctx *Context) Tomb() *tomb.Tomb {
	return &ctx.death
}

// Accept must be called by only the server.
func (ctx *Context) Accept() (conn io.ReadWriteCloser, err error) {
	select {
	case <-ctx.death.Dying():
		err = ctx.death.Err()
		return
	case conn = <-ctx.acptQueue:
		return
	}
}

// Tunnel must be called by only the client.
func (ctx *Context) Tunnel() (conn io.ReadWriteCloser, err error) {
	ctx.tabLock.Lock()
	// select a socketID
	var sokid socketID
	for {
		rd := socketID(rand.Int() % 65536)
		if ctx.sokTable[rd] == nil {
			sokid = rd
			break
		}
	}
	newsok := &socket{
		sockid:   socketID(sokid),
		parent:   ctx.sctx,
		incoming: make(chan segment, 256),
		sendwind: make(chan struct{}, 256),
	}
	ctx.sokTable[sokid] = newsok
	ctx.tabLock.Unlock()
	// send open via the select subctx
	ctx.sctx.wirewlok.Lock()
	err = struc.Pack(ctx.sctx.wire, &segment{Flag: flOpen, Sokid: uint16(sokid)})
	ctx.sctx.wirewlok.Unlock()
	// return the newsok
	conn = newsok
	// Tie up the death of the socket with our death
	go func() {
		select {
		case <-ctx.death.Dying():
			newsok.death.Kill(ctx.death.Err())
		case <-newsok.death.Dying():
		}
	}()
	return
}

type subCtx struct {
	parent   *Context
	wire     net.Conn
	wirewlok sync.Mutex

	death tomb.Tomb
}

func (sctx *subCtx) sendAliv() error {
	log.Println("niaucchi3: sendAliv() started")
	wait := 240
	for {
		sctx.wirewlok.Lock()
		err := struc.Pack(sctx.wire, &segment{Flag: flAliv})
		log.Println("niaucchi3: sent keepalive")
		sctx.wirewlok.Unlock()
		if err != nil {
			return err
		}
		time.Sleep(time.Second * time.Duration(wait+rand.Int()%60))
	}
}

func (sctx *subCtx) mainThread() (err error) {
	// If our parent dies, so do we!
	sctx.death.Go(func() error {
		defer sctx.wire.Close()
		select {
		case <-sctx.death.Dying():
			return sctx.death.Err()
		}
	})
	defer sctx.wire.Close()
	sctx.death.Go(sctx.sendAliv)
	// Main thread only takes care of reading from the wire.
	for {
		sctx.wire.SetReadDeadline(time.Now().Add(time.Minute * 8))
		var newseg segment
		err = struc.Unpack(sctx.wire, &newseg)
		if err != nil {
			sctx.death.Kill(err)
			return
		}
		switch newseg.Flag {
		case flAliv:
		case flOpen:
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
				log.Println("niaucchi3: overfull accept queue! dying")
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
				log.Println("niaucchi3: stray", newseg, ", dying")
				err = ErrProtocolFail
				sctx.parent.death.Kill(err)
				return
			}
			if newseg.Flag == flIcwd {
				for i := byte(0); i < newseg.Body[0]; i++ {
					select {
					case <-dest.sendwind:
					default:
						log.Println("niaucchi3: ICWD OVERFLOW", newseg.Sokid)
						err = ErrProtocolFail
						sctx.parent.death.Kill(err)
						return
					}
				}
			} else {
				if newseg.Flag == flClos {
					sctx.parent.tabLock.Lock()
					delete(sctx.parent.sokTable, dest.sockid)
					sctx.parent.tabLock.Unlock()
				}
				select {
				case dest.incoming <- newseg:
				default:
					log.Println("niaucchi3: overfull read buffer in, dying")
					err = ErrProtocolFail
					sctx.parent.death.Kill(err)
					return
				}
			}
		}
	}
}
