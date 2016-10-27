package entry

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"gopkg.in/bunsim/cluttershirt.v1"
)

func (cmd *Command) doForward(lsnr net.Listener, cookie []byte, dest *string) {
	for {
		raw, err := lsnr.Accept()
		if err != nil {
			return
		}
		go func() {
			defer raw.Close()
			clnt, err := cluttershirt.Server(cookie, raw)
			if err != nil {
				return
			}
			defer clnt.Close()
			remote, err := net.DialTimeout("tcp", fmt.Sprintf("%v:2378", *dest), time.Second*2)
			if err != nil {
				log.Println("WARNING: failed to forward to", *dest, ":", err.Error())
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
