package exit

import (
	"encoding/base32"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	miniss "gopkg.in/bunsim/miniss.v1"
	natrium "gopkg.in/bunsim/natrium.v1"

	"github.com/niwl/geph/niaucchi2"
	"github.com/niwl/geph/warpfront"
)

func (cmd *Command) doFront() {
	log.Println("listening on 8088 for WarpFront connections")
	wserv := warpfront.NewServer()
	go func() {
		for {
			wire, err := wserv.Accept()
			if err != nil {
				panic(err.Error())
			}
			go func() {
				defer wire.Close()
				// Handle MiniSS first
				mwire, err := miniss.Handshake(wire, cmd.identity.ToECDH())
				if err != nil {
					wire.Close()
					return
				}
				defer mwire.Close()
				pub := mwire.RemotePK()
				uid := strings.ToLower(
					base32.StdEncoding.EncodeToString(
						natrium.SecureHash(pub, nil)[:10]))
				// Next 1 byte: 0x02
				buf := make([]byte, 1)
				_, err = io.ReadFull(mwire, buf)
				if err != nil {
					mwire.Close()
					return
				}
				if buf[0] != 0x02 {
					mwire.Close()
					return
				}
				// Pass to the context manager
				ctx := niaucchi2.NewServerCtx()
				ctx.Absorb(mwire)
				cmd.manageOneCtx(uid, ctx)
			}()
		}
	}()
	htsrv := &http.Server{
		Addr:         ":8088",
		ReadTimeout:  time.Minute * 2,
		WriteTimeout: time.Minute * 10,
		Handler:      wserv,
	}
	err := htsrv.ListenAndServe()
	if err != nil {
		panic(err.Error())
	}
}
