package client

import (
	"io"
	"log"
	"net"

	"github.com/ProjectNiwl/tinysocks"
	"github.com/bunsim/geph/niaucchi"
)

// smSteadyState represents the steady state of the client.
// => ConnEntry when the network fails.
func (cmd *Command) smSteadyState() {
	log.Println("** => SteadyState **")
	defer log.Println("** <= SteadyState **")
	// spawn the SOCKS5 server
	socksListener, err := net.Listen("tcp", "127.0.0.1:8781")
	if err != nil {
		panic(err.Error())
	}
	go cmd.doSocks(socksListener)
	defer socksListener.Close()
	// wait until death
	cmd.currTunn.Tomb().Wait()
	// clear everything and go to ConnEntry
	cmd.currTunn = nil
	cmd.smState = cmd.smConnEntry
}

func (cmd *Command) doSocks(lsnr net.Listener) {
	for {
		clnt, err := lsnr.Accept()
		if err != nil {
			return
		}
		go func() {
			defer clnt.Close()
			var myss *niaucchi.Substrate
			myss = cmd.currTunn
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
			log.Println("tunnel open", dest)
			defer log.Println("tunnel clos", dest)
			go func() {
				defer conn.Close()
				defer clnt.Close()
				io.Copy(clnt, conn)
			}()
			io.Copy(conn, clnt)
		}()
	}
}
