package niaucchi

import (
	"io"
	"net"
	"testing"
	"time"

	"gopkg.in/bunsim/natrium.v1"
)

func TestLol(t *testing.T) {
	FACTOR := 3
	identity := natrium.ECDHGenerateKey()

	go func() {
		zzz, err := Listen(nil, identity, "127.0.0.1:13371")
		if err != nil {
			panic(err.Error())
		}
		for {
			ss, err := zzz.AcceptSubstrate()
			if err != nil {
				panic(err.Error())
			}
			go func() {
				defer ss.Tomb().Kill(io.ErrClosedPipe)
				for {
					clnt, err := ss.AcceptConn()
					if err != nil {
						panic(err.Error())
					}
					io.Copy(clnt, clnt)
				}
			}()
		}
	}()
	time.Sleep(time.Second)

	xaxa, err := net.Listen("tcp", "127.0.0.1:13370")
	if err != nil {
		panic(err.Error())
	}
	lel, err := DialSubstrate(nil, natrium.ECDHGenerateKey(),
		identity.PublicKey(), "127.0.0.1:13371", FACTOR)
	if err != nil {
		panic(err.Error())
	}
	for {
		xa, err := xaxa.Accept()
		if err != nil {
			panic(err.Error())
		}
		go func() {
			defer xa.Close()
			remote, err := lel.OpenConn()
			if err != nil {
				panic(err.Error())
			}
			go func() {
				defer xa.Close()
				defer remote.Close()
				io.Copy(xa, remote)
			}()
			defer remote.Close()
			io.Copy(remote, xa)
		}()
	}
}
