package niaucchi2

import (
	"fmt"
	"io"
	"net"
	"testing"
	"time"
)

func BenchmarkLol(b *testing.B) {
	go func() {
		lsnr, err := net.Listen("tcp", "127.0.0.1:13371")
		if err != nil {
			panic(err.Error())
		}
		cont := NewServerCtx()
		for {
			zzz, _ := lsnr.Accept()
			go func() {
				defer zzz.Close()
				err := cont.Absorb(zzz)
				if err != nil {
					panic(err.Error())
				}
				for {
					clnt, err := cont.Accept()
					if err != nil {
						panic(err.Error())
					}
					go func() {
						defer clnt.Close()
						io.Copy(clnt, clnt)
					}()
				}
			}()
		}
	}()
	time.Sleep(time.Millisecond * 200)
	cont := NewClientCtx()
	for i := 0; i < 10; i++ {
		wire, err := net.Dial("tcp", "127.0.0.1:13371")
		if err != nil {
			panic(err.Error())
		}
		err = cont.Absorb(wire)
		if err != nil {
			panic(err.Error())
		}
	}
	for i := 0; i < 10; i++ {
		dur, _ := cont.Ping(make([]byte, 50000))
		fmt.Println(dur, i)
	}
	lsnr, _ := net.Listen("tcp", "127.0.0.1:13370")
	for {
		clnt, _ := lsnr.Accept()
		go func() {
			rmt, err := cont.Tunnel()
			if err != nil {
				panic(err.Error())
			}
			go func() {
				defer rmt.Close()
				defer clnt.Close()
				io.Copy(clnt, rmt)
			}()
			defer rmt.Close()
			defer clnt.Close()
			io.Copy(rmt, clnt)
		}()
	}
}
