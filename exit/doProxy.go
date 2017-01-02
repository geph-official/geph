package exit

import (
	"encoding/base32"
	"io"
	"log"
	"math/rand"
	"net"
	"strings"
	"sync"

	"golang.org/x/time/rate"

	"gopkg.in/bunsim/natrium.v1"

	"github.com/bunsim/geph/niaucchi2"
	"github.com/bunsim/miniss"
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
	limit := rate.NewLimiter(rate.Limit(cmd.bwLimit*1024), 512*1024)
	harshlimit := rate.NewLimiter(rate.Limit(32*1024), 128*1024)
	// little balance
	lbal := 0
	var lblk sync.Mutex
	// consume bytes, returns true if succeeds, otherwise returns false and kills everything
	consume := func(dec int) bool {
		lblk.Lock()
		defer lblk.Unlock()
		lbal -= dec
		if lbal <= 0 {
			num := rand.Int()%10 + 5
			bal, err := cmd.decAccBalance(uid, num)
			if err != nil || bal == 0 {
				return false
			}
			lbal += 1000 * 1000 * num
		}
		return true
	}
	// at the very end, return the small balance
	defer func() {
		cmd.decAccBalance(uid, -lbal/1000000)
	}()
	// Accept loop
	for {
		clnt, err := nct.Accept()
		if err != nil {
			return
		}
		go cmd.proxyCommon(consume, limit, harshlimit, uid, clnt)
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
			log.Println("accepted 2379")
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
			log.Println("miniss finished")
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
			log.Println("ctkey =", ctkey)
			// Check the ctxTab now
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
