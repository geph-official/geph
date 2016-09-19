package main

import (
	"flag"
	"io"
	"log"
	"net"

	"github.com/bunsim/niaucchi"
	"github.com/google/subcommands"
	"golang.org/x/net/context"
)

type clientCmd struct {
	proxy string
}

func (*clientCmd) Name() string     { return "client" }
func (*clientCmd) Synopsis() string { return "client" }
func (*clientCmd) Usage() string    { return "client" }

func (sc *clientCmd) SetFlags(f *flag.FlagSet) {
	f.StringVar(&sc.proxy, "proxy", "", "address of remote proxy")
}

func (sc *clientCmd) Execute(_ context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	if sc.proxy == "" {
		panic("wtf")
	}

	xaxa, err := net.Listen("tcp", "127.0.0.1:13131")
	if err != nil {
		panic(err.Error())
	}
	ss, err := niaucchi.DialSubstrate(nil, nil, sc.proxy)
	if err != nil {
		panic(err.Error())
	}
	for {
		lol, err := xaxa.Accept()
		if err != nil {
			panic(err.Error())
		}
		go func() {
			defer lol.Close()
			rmt, err := ss.OpenConn()
			if err != nil {
				panic(err.Error())
			}
			defer rmt.Close()
			go func() {
				defer lol.Close()
				defer rmt.Close()
				log.Println(io.Copy(rmt, lol))
			}()
			log.Println(io.Copy(lol, rmt))
		}()
	}
}
