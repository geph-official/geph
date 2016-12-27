package main

import (
	"flag"
	"io"
	"log"
	"net"

	"github.com/bunsim/natrium"
	"github.com/bunsim/tinysocks"
	"github.com/bunsim/niaucchi"
	"github.com/google/subcommands"

	"golang.org/x/net/context"
)

const FACTOR = 8

type serverCmd struct {
}

func (*serverCmd) Name() string     { return "server" }
func (*serverCmd) Synopsis() string { return "server" }
func (*serverCmd) Usage() string    { return "server" }

func (sc *serverCmd) SetFlags(f *flag.FlagSet) {
}

func (sc *serverCmd) Execute(_ context.Context, f *flag.FlagSet, args ...interface{}) subcommands.ExitStatus {
	lsnr, err := niaucchi.Listen(nil, natrium.EdDSAGenerateKey(), ":9876")
	if err != nil {
		panic(err.Error())
	}
	log.Println("listening at", lsnr.Addr())
	for {
		clnt, err := lsnr.Accept()
		if err != nil {
			log.Println(err.Error())
			return -1
		}
		go func() {
			defer clnt.Close()
			dest, err := tinysocks.ReadRequest(clnt)
			if err != nil {
				log.Println(err.Error())
				log.Println("gotta call close now")
				return
			}
			log.Println("dest =", dest)
			rmt, err := net.Dial("tcp", dest)
			if err != nil {
				log.Println(err.Error())
				return
			}
			defer rmt.Close()
			tinysocks.CompleteRequest(0x00, clnt)

			go func() {
				defer rmt.Close()
				defer clnt.Close()
				io.Copy(clnt, rmt)
			}()
			io.Copy(rmt, clnt)
		}()
	}
}
