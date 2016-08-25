package client

import (
	"flag"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/ProjectNiwl/tinysocks"
	"github.com/bunsim/niaucchi"
	"github.com/google/subcommands"
	"golang.org/x/net/context"
)

// Command is the client subcommand.
type Command struct {
}

// Name returns the name "client".
func (*Command) Name() string { return "client" }

// Synopsis returns a description of the subcommand.
func (*Command) Synopsis() string { return "Run as the client" }

// Usage returns a string describing usage.
func (*Command) Usage() string { return "" }

// SetFlags sets the flag on the binder subcommand.
func (cmd *Command) SetFlags(f *flag.FlagSet) {
}

// Execute executes a client subcommand.
func (cmd *Command) Execute(_ context.Context,
	f *flag.FlagSet,
	args ...interface{}) subcommands.ExitStatus {
	var ss *niaucchi.Substrate
	var sl sync.Mutex
	// one thread does all the SOCKS stuff
	go func() {
		lsnr, err := net.Listen("tcp", "127.0.0.1:9999")
		if err != nil {
			panic(err.Error())
		}
		for {
			clnt, err := lsnr.Accept()
			if err != nil {
				panic(err.Error())
			}
			go func() {
				defer clnt.Close()
				var myss *niaucchi.Substrate
				sl.Lock()
				myss = ss
				sl.Unlock()
				if myss == nil {
					return
				}
				dest, err := tinysocks.ReadRequest(clnt)
				if err != nil {
					return
				}
				conn, err := myss.OpenConn()
				if err != nil {
					return
				}
				defer conn.Close()
				tinysocks.CompleteRequest(0x00, clnt)
				conn.Write([]byte{byte(len(dest))})
				conn.Write([]byte(dest))
				// forward
				log.Println("proxying to", dest)
				go func() {
					defer conn.Close()
					defer clnt.Close()
					io.Copy(clnt, conn)
				}()
				io.Copy(conn, clnt)
			}()
		}
	}()
	// the other constantly revives the stuff
	for {
	retry:
		nss, err := cmd.getSubstrate()
		if err != nil {
			log.Println("failed in obtaining substrate:", err.Error())
			time.Sleep(time.Second)
			goto retry
		}
		sl.Lock()
		ss = nss
		sl.Unlock()
		nss.Tomb().Wait()
	}
}
