package client

import (
	"io"
	"log"
	"net"
	"time"

	"golang.org/x/net/proxy"

	"github.com/niwl/geph/tinysocks"
	"github.com/niwl/geph/niaucchi2"
)

// smSteadyState represents the steady state of the client.
// => FindEntry when the network fails.
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
	// do the account verification in parallel
	go cmd.verifyAccount()
	// wait until death
	<-cmd.currTunn.Tomb().Dying()
	reason := cmd.currTunn.Tomb().Err()
	if reason == nil {
		log.Println("WTF??? Null??")
	} else {
		log.Println("network failed in steady state:", reason.Error())
	}
	// clear everything and go to FindEntry
	cmd.currTunn = nil
	cmd.smState = cmd.smFindEntry
}

func (cmd *Command) dialTun(dest string) (conn net.Conn, err error) {
	sks, err := proxy.SOCKS5("tcp", "127.0.0.1:8781", nil, proxy.FromEnvironment())
	if err != nil {
		return
	}
	return sks.Dial("tcp", dest)
}

func (cmd *Command) dialTunRaw(dest string) (conn io.ReadWriteCloser, code byte) {
	var err error
	if !cmd.filterDest(dest) {
		conn, err = net.Dial("tcp", dest)
		if err != nil {
			code = 0x03
		}
		return
	}
	var myss *niaucchi2.Context
	myss = cmd.currTunn
	if myss == nil {
		code = 0x03
		return
	}
	conn, err = myss.Tunnel()
	if err != nil {
		code = 0x03
		return
	}
	conn.Write(append([]byte{byte(len(dest))}, []byte(dest)...))
	// wait for the status code
	tmr := time.AfterFunc(time.Second*15, func() {
		myss.Tomb().Kill(niaucchi2.ErrTimeout)
	})
	b := make([]byte, 1)
	_, err = io.ReadFull(conn, b)
	if err != nil {
		conn.Close()
		code = 0x03
	}
	code = b[0]
	if code != 0x00 {
		conn.Close()
	}
	tmr.Stop()
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
			conn, code := cmd.dialTunRaw(dest)
			if code != 0x00 {
				tinysocks.CompleteRequest(code, clnt)
				return
			}
			defer conn.Close()
			tinysocks.CompleteRequest(0x00, clnt)
			// forward
			cmd.stats.Lock()
			log.Println("OPEN", dest)
			cmd.stats.netinfo.tuns[clnt.RemoteAddr().String()] = dest
			cmd.stats.Unlock()
			defer func() {
				cmd.stats.Lock()
				log.Println("CLOS", dest)
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
