package niaucchi

import (
	"io"
	"log"
	"net"
	"testing"
)

func TestLol(t *testing.T) {
	FACTOR := 20

	go func() {
		zzz, err := net.Listen("tcp", "127.0.0.1:13371")
		if err != nil {
			panic(err.Error())
		}

		tzes := make([]net.Conn, FACTOR)
		for i := 0; i < FACTOR; i++ {
			qqq, err := net.Dial("tcp", "127.0.0.1:13370")
			if err != nil {
				panic(err.Error())
			}
			tzes[i] = qqq
		}
		ss := NewSubstrate(tzes)

		for {
			clnt, err := zzz.Accept()
			if err != nil {
				panic(err.Error())
			}
			go func() {
				defer clnt.Close()
				rmt, err := ss.OpenConn()
				if err != nil {
					panic(err.Error())
				}
				defer rmt.Close()
				go func() {
					defer clnt.Close()
					defer rmt.Close()
					log.Println(io.Copy(rmt, clnt))
				}()
				log.Println(io.Copy(clnt, rmt))
			}()
		}
	}()

	xaxa, err := net.Listen("tcp", "127.0.0.1:13370")
	if err != nil {
		panic(err.Error())
	}
	for {
		transes := make([]net.Conn, FACTOR)
		for i := 0; i < FACTOR; i++ {
			lol, err := xaxa.Accept()
			if err != nil {
				panic(err.Error())
			}
			transes[i] = lol
		}
		go func() {
			ss := NewSubstrate(transes)
			for {
				clnt, err := ss.AcceptConn()
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
}
