package entry

import (
	"io"
	"log"
	"net"

	"github.com/bunsim/kiss"
)

func (cmd *Command) doForward(lsnr net.Listener, cookie []byte, dest string) {
	log.Println("obfuscation listening on", lsnr.Addr(), "forwards to", dest)
	for {
		raw, err := lsnr.Accept()
		if err != nil {
			panic(err.Error())
		}
		go func() {
			defer raw.Close()
			clnt, err := kiss.LLObfsServerHandshake(cookie, raw)
			if err != nil {
				return
			}
			defer clnt.Close()
			remote, err := net.Dial("tcp", dest)
			if err != nil {
				log.Println("WARNING: failed to forward to", dest, ":", err.Error())
				return
			}
			defer remote.Close()
			go func() {
				defer remote.Close()
				defer clnt.Close()
				io.Copy(clnt, remote)
			}()
			io.Copy(remote, clnt)
		}()
	}
}
