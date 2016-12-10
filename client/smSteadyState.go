package client

import (
	"errors"
	"log"
	"net"

	"golang.org/x/net/proxy"

	"github.com/ProjectNiwl/tinysocks"
	"github.com/bunsim/geph/niaucchi"
)

// smSteadyState represents the steady state of the client.
// => ConnEntry when the network fails.
func (cmd *Command) smSteadyState() {
	log.Println("** => SteadyState **")
	defer log.Println("** <= SteadyState **")
	// change stats
	cmd.stats.Lock()
	cmd.stats.status = "connected"
	cmd.stats.Unlock()
	defer func() {
		cmd.stats.Lock()
		cmd.stats.status = "connecting"
		cmd.stats.Unlock()
	}()
	// do the account verification in parallel, tied to currTunn's tomb
	cmd.currTunn.Tomb().Go(cmd.verifyAccount)
	// wait until death
	reason := cmd.currTunn.Tomb().Wait()
	log.Println("network failed in steady state:", reason.Error())
	// clear everything and go to ConnEntry
	cmd.currTunn = nil
	cmd.smState = cmd.smConnEntry
}

func (cmd *Command) dialTun(dest string) (conn net.Conn, err error) {
	sks, err := proxy.SOCKS5("tcp", "127.0.0.1:8781", nil, proxy.FromEnvironment())
	if err != nil {
		return
	}
	return sks.Dial("tcp", dest)
}

func (cmd *Command) dialTunRaw(dest string) (conn net.Conn, err error) {
	if !cmd.filterDest(dest) {
		return net.Dial("tcp", dest)
	}
	var myss *niaucchi.Substrate
	myss = cmd.currTunn
	if myss == nil {
		err = errors.New("null")
		return
	}
	conn, err = myss.OpenConn()
	if err != nil {
		return
	}
	conn.Write(append([]byte{byte(len(dest))}, []byte(dest)...))
	return
}

func (cmd *Command) doSocks(lsnr net.Listener) {
	for {
		clnt, err := lsnr.Accept()
		if err != nil {
			return
		}
		go func() {
			defer clnt.Close()
			dest, err := tinysocks.ReadRequest(clnt)
			if err != nil {
				return
			}
			conn, err := cmd.dialTunRaw(dest)
			if err != nil {
				tinysocks.CompleteRequest(0x03, clnt)
				return
			}
			tinysocks.CompleteRequest(0x00, clnt)
			// forward
			cmd.stats.Lock()
			cmd.stats.netinfo.tuns[clnt.RemoteAddr().String()] = dest
			cmd.stats.Unlock()
			defer func() {
				cmd.stats.Lock()
				delete(cmd.stats.netinfo.tuns, clnt.RemoteAddr().String())
				cmd.stats.Unlock()
			}()
			go func() {
				defer conn.Close()
				defer clnt.Close()
				ctrCopy(clnt, conn, &cmd.stats.rxBytes)
			}()
			ctrCopy(conn, clnt, &cmd.stats.txBytes)
		}()
	}
}
