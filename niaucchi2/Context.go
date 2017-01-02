package niaucchi2

import (
	"encoding/binary"
	"errors"
	"io"
	"log"
	"math/rand"
	"net"
	"sync"

	"github.com/lunixbochs/struc"

	"gopkg.in/tomb.v2"
)

type subCtxID uint16
type socketID uint16

// ErrTablesFull means the internal tables used in the Context are full.
var ErrTablesFull = errors.New("niaucchi2: internal tables in Context are full")

// ErrProtocolFail means a fatal protocol violation.
var ErrProtocolFail = errors.New("niaucchi2: nonsensical network messages")

// Context represents a collection of secure connections used to tunnel sockets.
type Context struct {
	isClient bool
	subTable map[subCtxID]*subCtx
	sokTable map[socketID]*socket
	tabLock  sync.RWMutex

	acptQueue chan io.ReadWriteCloser

	death tomb.Tomb
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
	// select a subctx
	var cands []*subCtx
	for _, v := range ctx.subTable {
		cands = append(cands, v)
		break
	}
	sctx := cands[rand.Int()%len(cands)]
	// select a socketID
	var sokid socketID
	for i := socketID(1); i < 65535; i++ {
		if ctx.sokTable[i] == nil {
			sokid = i
			break
		}
	}
	newsok := &socket{
		sockid:   socketID(sokid),
		parent:   sctx,
		incoming: make(chan segment, 256),
		sendwind: make(chan struct{}, 256),
	}
	ctx.sokTable[sokid] = newsok
	ctx.tabLock.Unlock()
	// send open via the select subctx
	sctx.wirewlok.Lock()
	err = struc.Pack(sctx.wire, &segment{Flag: flOpen, Sokid: uint16(sokid)})
	sctx.wirewlok.Unlock()
	// return the newsok
	conn = newsok
	return
}

// Absorb absorbs a new network connection into the context.
func (ctx *Context) Absorb(conn net.Conn) (err error) {
	// Fail early if the whole thing is dead
	if !ctx.death.Alive() {
		return ctx.death.Err()
	}
	var subid subCtxID
	// Depending on whether we are client or server, generate or get subCtxId
	if ctx.isClient {
		// Grab the tabLock
		ctx.tabLock.Lock()
		defer ctx.tabLock.Unlock()
		for i := subCtxID(1); i < 65535; i++ {
			if ctx.subTable[i] == nil {
				subid = i
				break
			}
		}
		if subid == 0 {
			ctx.death.Kill(ErrTablesFull)
			err = ErrTablesFull
			return
		}
		// Send over whatever we selected
		conn.Write([]byte{byte(subid / 256), byte(subid % 256)})
		log.Println("niaucchi2: Context absorbed", subid, "as client")
	} else {
		// Read the subid from the network
		bts := make([]byte, 2)
		_, err = io.ReadFull(conn, bts)
		if err != nil {
			return
		}
		subid = subCtxID(binary.BigEndian.Uint16(bts))
		// Grab the tabLock after we read
		ctx.tabLock.Lock()
		defer ctx.tabLock.Unlock()
		log.Println("niaucchi2: Context absorbed", subid, "as server")
	}
	// Construct a subCtx wrapping this connection
	nsc := &subCtx{
		parent: ctx,
		subid:  subid,
		wire:   conn,
	}
	// Run the main thread for the subCtx
	nsc.death.Go(nsc.mainThread)
	// Stuff the subCtx into the table
	ctx.subTable[subid] = nsc
	return
}

// NewServerCtx creates a Context for servers.
func NewServerCtx() *Context {
	return &Context{
		isClient: false,
		subTable: make(map[subCtxID]*subCtx),
		sokTable: make(map[socketID]*socket),

		acptQueue: make(chan io.ReadWriteCloser, 256),
	}
}

// NewClientCtx creates a Context for clients.
func NewClientCtx() *Context {
	return &Context{
		isClient: true,
		subTable: make(map[subCtxID]*subCtx),
		sokTable: make(map[socketID]*socket),
	}
}
