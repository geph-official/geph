package exit

import (
	"io"
	"net"
	"sync"

	"gopkg.in/bunsim/natrium.v1"

	"github.com/bunsim/geph/niaucchi2"
	"github.com/bunsim/miniss"
)

// doProxy using the new protocol
func (cmd *Command) doProxy() {
	// Manually handle TCP and MiniSS
	lsnr, err := net.Listen("tcp", ":2379")
	if err != nil {
		panic(err.Error())
	}
	// Table of active contexts
	ctxTab := make(map[string]*niaucchi2.Context)
	var ctxTabLok sync.Mutex
	for {
		wire, err := lsnr.Accept()
		if err != nil {
			panic(err.Error())
		}
		go func() {
			// Handle MiniSS first
			mwire, err := miniss.Handshake(wire, cmd.identity.ToECDH())
			if err != nil {
				wire.Close()
				return
			}
			// Ignore the first 33 bytes
			_, err = io.ReadFull(mwire, make([]byte, 32))
			if err != nil {
				mwire.Close()
				return
			}
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
			// Check the ctxTab now
			ctxTabLok.Lock()
			if ctxTab[ctkey] == nil {
				ctxTab[ctkey] = niaucchi2.NewServerCtx()
				ctxTab[ctkey].Absorb(mwire)
				go func() {
					ctxTab[ctkey].Tomb().Wait()
					ctxTabLok.Lock()
					defer ctxTabLok.Unlock()
					delete(ctxTab, ctkey)
				}()
			}
			ctxTabLok.Unlock()
		}()
	}
}
