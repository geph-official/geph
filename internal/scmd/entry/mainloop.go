package entry

import (
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"gopkg.in/bunsim/cluttershirt.v1"
)

func (cmd *Command) doForward(lsnr net.Listener, cookie []byte, dest string) {
	for {
		raw, err := lsnr.Accept()
		if err != nil {
			return
		}
		go func() {
			defer raw.Close()
			raw.SetDeadline(time.Now().Add(time.Minute))
			clnt, err := cluttershirt.Server(cookie, raw)
			if err != nil {
				return
			}
			defer clnt.Close()
			// Read 1 byte to determine version
			lol := make([]byte, 1)
			_, err = io.ReadFull(clnt, lol)
			if err != nil {
				return
			}
			var remote net.Conn
			if lol[0] != 0 {
				return // Cannot support legacy anymore!
			}
			remote, err = net.DialTimeout("tcp", fmt.Sprintf("%v:2379", dest), time.Second*10)
			if err != nil {
				log.Println("WARNING: failed to forward to", dest, ":", err.Error())
				return
			}
			remote.Write(lol)
			raw.SetDeadline(time.Time{})
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
