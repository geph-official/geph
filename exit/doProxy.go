package exit

import (
	"encoding/base32"
	"io"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"gopkg.in/bunsim/miniss.v1"
	"gopkg.in/bunsim/natrium.v1"

	"github.com/niwl/geph/niaucchi2"
)

func (cmd *Command) manageOneCtx(uid string, nct *niaucchi2.Context) {
	defer nct.Tomb().Kill(io.ErrClosedPipe)
	// check balance first
	bal, err := cmd.decAccBalance(uid, 0)
	if err != nil {
		log.Println("error authenticating user", uid, ":", err)
	} else {
		log.Println(uid, "connected with", bal, "MiB left")
	}
	bal *= 1000000
	limit := rate.NewLimiter(rate.Limit(cmd.bwLimit*1024), 512*1024)
	harshlimit := rate.NewLimiter(rate.Limit(32*1024), 128*1024)
	// little balance
	lbal := 0
	var lblk sync.Mutex
	// consume bytes, returns true if succeeds, otherwise returns false
	consume := func(dec int) bool {
		lblk.Lock()
		defer lblk.Unlock()
		lbal += dec
		bal -= dec
		if bal <= 0 {
			return false
		}
		return true
	}
	// periodically sync our balance with the global balance
	go func() {
		for {
			select {
			case <-nct.Tomb().Dying():
				return
			case <-time.After(time.Second * 10):
				lblk.Lock()
				olbal := lbal
				lblk.Unlock()
				if olbal > 1000000 {
					// olbal has what we need ATM
					// decrement by olbal
					nbal, err := cmd.decAccBalance(uid, (olbal / 1000000))
					if err != nil {
						log.Println("error", err.Error())
						nbal = 0
					}
					// then possibly decrement by another MiB, dithered
					if rand.Float64() < float64(olbal%1000000)/1000000.0 {
						nbal, err = cmd.decAccBalance(uid, (olbal / 1000000))
						if err != nil {
							log.Println("error", err.Error())
							nbal = 0
						}
					}
					// update bal
					lblk.Lock()
					bal = nbal * 1000000
					lbal = 0
					lblk.Unlock()
				}
			}
		}
	}()
	// Accept loop
	for {
		clnt, err := nct.Accept()
		if err != nil {
			log.Println("exiting manageOneCtx:", err.Error())
			return
		}
		go cmd.proxyCommon(true, consume, limit, harshlimit, uid, clnt)
	}
}

// doProxy using the new protocol
func (cmd *Command) doProxy() {
	// Manually handle TCP and MiniSS
	lsnr, err := net.Listen("tcp", ":2379")
	if err != nil {
		panic(err.Error())
	}
	log.Println("niaucchi2 listening on port 2379")
	// Table of active contexts
	ctxTab := make(map[string]*niaucchi2.Context)
	var ctxTabLok sync.Mutex
	for {
		wire, err := lsnr.Accept()
		if err != nil {
			panic(err.Error())
		}
		go func() {
			io.ReadFull(wire, make([]byte, 1))
			// Handle MiniSS first
			mwire, err := miniss.Handshake(wire, cmd.identity.ToECDH())
			if err != nil {
				wire.Close()
				return
			}
			pub := mwire.RemotePK()
			uid := strings.ToLower(
				base32.StdEncoding.EncodeToString(
					natrium.SecureHash(pub, nil)[:10]))
			// Next 33 bytes: 0x02, then ctxId
			buf := make([]byte, 33)
			_, err = io.ReadFull(mwire, buf)
			if err != nil {
				mwire.Close()
				return
			}
			if buf[0] != 0x02 {
				mwire.Close()
				return
			}
			ctkey := natrium.HexEncode(buf[1:])
			// Check the ctxTab nowx
			ctxTabLok.Lock()
			if ctxTab[ctkey] == nil {
				ctxTab[ctkey] = niaucchi2.NewServerCtx()
				ctxTab[ctkey].Absorb(mwire)
				go cmd.manageOneCtx(uid, ctxTab[ctkey])
				go func() {
					ctxTab[ctkey].Tomb().Wait()
					ctxTabLok.Lock()
					defer ctxTabLok.Unlock()
					delete(ctxTab, ctkey)
				}()
			} else {
				ctxTab[ctkey].Absorb(mwire)
			}
			ctxTabLok.Unlock()
		}()
	}
}
