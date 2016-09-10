package entry

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"golang.org/x/time/rate"

	"github.com/bunsim/kiss"
)

func (cmd *Command) doForward(lsnr net.Listener, cookie []byte, dest *string) {
	for {
		raw, err := lsnr.Accept()
		if err != nil {
			return
		}
		go func() {
			// rate limit every connection in the long run to 16 KiB/s
			ulmt := rate.NewLimiter(16*1024, 512*1024)
			dlmt := rate.NewLimiter(16*1024, 512*1024)
			ctx := context.Background()
			defer raw.Close()
			clnt, err := kiss.LLObfsServerHandshake(cookie, raw)
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
				buf := make([]byte, 16384)
				for {
					n, err := remote.Read(buf)
					if err != nil {
						return
					}
					dlmt.WaitN(ctx, n)
					_, err = clnt.Write(buf[:n])
					if err != nil {
						return
					}
				}
			}()
			buf := make([]byte, 16384)
			for {
				n, err := clnt.Read(buf)
				if err != nil {
					return
				}
				ulmt.WaitN(ctx, n)
				_, err = remote.Write(buf[:n])
				if err != nil {
					return
				}
			}
		}()
	}
}
