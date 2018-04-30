package exit

import (
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/rensa-labs/geph/internal/legacy/niaucchi2"
	"github.com/rensa-labs/geph/internal/niaucchi3"

	"gopkg.in/bunsim/miniss.v1"
	"gopkg.in/bunsim/natrium.v1"
)

func (cmd *Command) handleNewClient(mwire *miniss.Socket) {
	// JSON uname and pwd
	var upLen uint16
	binary.Read(mwire, binary.BigEndian, &upLen)
	upwdBts := make([]byte, upLen)
	binary.Read(mwire, binary.BigEndian, &upwdBts)
	var upwd map[string]string
	json.Unmarshal(upwdBts, &upwd)
	log.Println("user", upwd["Username"], "authenticating...")
	uid, limit, e := cmd.authUser(upwd["Username"], upwd["Password"])
	if e != nil {
		log.Println("user", upwd["Username"], "cannot authenticate:", e.Error())
		mwire.Write([]byte("n"))
		mwire.Close()
		return
	}
	mwire.Write([]byte("y"))
	log.Println("wrote y")
	mwire.SetDeadline(time.Time{})
	ctx := niaucchi3.NewContext(false, mwire)
	go cmd.manageCtx(uid, limit, ctx)
	<-ctx.Tomb().Dying()
}

// doProxy using the new protocol
func (cmd *Command) doProxy() {
	// Manually handle TCP and MiniSS
	lsnr, err := net.Listen("tcp", ":2379")
	if err != nil {
		panic(err.Error())
	}
	log.Println("MiniSS listening on port 2379")
	// Table of active niaucchi2 contexts
	ctxTab := make(map[string]*niaucchi2.Context)
	var ctxTabLok sync.Mutex
	for {
		iwire, err := lsnr.Accept()
		if err != nil {
			panic(err.Error())
		}
		wire := iwire.(*net.TCPConn)
		go func() {
			wire.SetDeadline(time.Now().Add(time.Minute))
			io.ReadFull(wire, make([]byte, 1))
			// Handle MiniSS first
			mwire, err := miniss.Handshake(wire, cmd.identity.ToECDH())
			if err != nil {
				wire.Close()
				return
			}
			pub := mwire.RemotePK()
			// read protocol
			var protID uint8
			binary.Read(mwire, binary.BigEndian, &protID)
			switch protID {
			case 0x03:
				cmd.handleNewClient(mwire)
			case 0x02:
				uid := strings.ToLower(
					base32.StdEncoding.EncodeToString(
						natrium.SecureHash(pub, nil)[:10]))
				// Next 32 bytes is ctxid
				buf := make([]byte, 32)
				_, err = io.ReadFull(mwire, buf)
				if err != nil {
					mwire.Close()
					return
				}
				ctkey := natrium.HexEncode(buf)
				// Check the ctxTab nowx
				var sctx *niaucchi2.Context
				ctxTabLok.Lock()
				if ctxTab[ctkey] == nil {
					ctxTab[ctkey] = niaucchi2.NewServerCtx()
					go cmd.manageLegacyCtx(uid, ctxTab[ctkey])
					tmb := ctxTab[ctkey].Tomb()
					go func() {
						<-tmb.Dying()
						ctxTabLok.Lock()
						defer ctxTabLok.Unlock()
						delete(ctxTab, ctkey)
					}()
				}
				sctx = ctxTab[ctkey]
				ctxTabLok.Unlock()
				// clear deadline
				wire.SetDeadline(time.Time{})
				sctx.Absorb(mwire)
			}
		}()
	}
}
