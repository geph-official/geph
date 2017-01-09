package client

import (
	"io"
	"log"
	"net"
	"strings"
	"time"

	"gopkg.in/bunsim/miniss.v1"
	"gopkg.in/bunsim/natrium.v1"

	"github.com/bunsim/cluttershirt"
	"github.com/bunsim/geph/niaucchi2"
	"github.com/bunsim/geph/warpfront"
)

// smConnEntry is the ConnEntry state, where a connection to some entry node is established.
// => VerifyAccount if successful
// => ClearCache if unsuccessful
func (cmd *Command) smConnEntry() {
	log.Println("** => ConnEntry **")
	defer log.Println("** <= ConnEntry **")

	FANOUT := 1
	if !cmd.powersave {
		FANOUT = 8
	}

	retline := make(chan *niaucchi2.Context)
	dedline := make(chan bool)
	// barrier to prevent the initial connection from ruining the race
	barr := time.After(time.Second * 2)
	for exit, entries := range cmd.entryCache {
		for _, xaxa := range entries {
			exit := exit
			xaxa := xaxa
			log.Println(xaxa.Addr, "from", exit)
			if xaxa.Addr == "warpfront" {
				go func() {
					//xaxa.Cookie = []byte("http://localhost:8088;d2hk1ucgmi0pgc.cloudfront.net")
					select {
					case <-time.After(time.Second * 6):
						log.Println("time's up, falling back to", string(xaxa.Cookie))
					case <-dedline:
						return
					}
					splitted := strings.Split(string(xaxa.Cookie), ";")
					xaxa.Addr = string(xaxa.Cookie)
					rawconn, err := warpfront.Connect(cleanHTTP, splitted[0], splitted[1])
					if err != nil {
						log.Println("warpfront to", string(xaxa.Cookie), err.Error())
						return
					}
					rawconn = warpfront.RWCNagle(rawconn)
					mconn, err := miniss.Handshake(rawconn, cmd.identity)
					if err != nil {
						log.Println("miniss to", xaxa.Addr, err.Error())
						rawconn.Close()
						return
					}
					/*if natrium.CTCompare(mconn.RemotePK(), xaxa.ExitKey.ToECDH()) != 0 {
						log.Println("miniss to", xaxa.Addr, "bad auth")
						rawconn.Close()
						return
					}*/
					mconn.Write([]byte{0x02})
					// No ctxid for warpfront
					cand := niaucchi2.NewClientCtx()
					cand.Absorb(mconn)
					select {
					case retline <- cand:
						cmd.stats.Lock()
						cmd.stats.netinfo.entry = natrium.HexEncode(
							natrium.SecureHash(xaxa.Cookie, nil)[:8])
						cmd.stats.netinfo.exit = exit
						cmd.stats.netinfo.prot = "wf-ni-2"
						cmd.stats.Unlock()
						log.Println(xaxa.Addr, "WINNER")
					case <-dedline:
						log.Println(xaxa.Addr, "failed race")
						cand.Tomb().Kill(io.ErrClosedPipe)
					}
				}()
			} else {
				getWire := func(ctxid []byte) (mconn *miniss.Socket, err error) {
					rawconn, err := net.DialTimeout("tcp", xaxa.Addr, time.Second*10)
					if err != nil {
						log.Println("dial to", xaxa.Addr, err.Error())
						return
					}
					oconn, err := cluttershirt.Client(xaxa.Cookie, rawconn)
					if err != nil {
						log.Println("cluttershirt to", xaxa.Addr, err.Error())
						rawconn.Close()
						return
					}
					log.Println("cluttershirt to", xaxa.Addr, "okay")
					// make the race fair
					<-barr
					// 0x00 for a negotiable protocol
					oconn.Write([]byte{0x00})
					mconn, err = miniss.Handshake(oconn, cmd.identity)
					if err != nil {
						log.Println("miniss to", xaxa.Addr, err.Error())
						oconn.Close()
						return
					}
					if natrium.CTCompare(mconn.RemotePK(), xaxa.ExitKey.ToECDH()) != 0 {
						log.Println("miniss to", xaxa.Addr, "bad auth")
						oconn.Close()
						return
					}
					log.Println("miniss to", xaxa.Addr, "okay")
					// 0x02
					log.Println("gonna send", len(append([]byte{0x02}, ctxid...)))
					_, err = mconn.Write(append([]byte{0x02}, ctxid...))
					if err != nil {
						log.Println("ctxid to", xaxa.Addr, err.Error())
						mconn.Close()
						return
					}
					log.Println("id to", xaxa.Addr, "okay")
					return
				}
				go func() {
					cand := niaucchi2.NewClientCtx()
					ctxid := make([]byte, 32)
					natrium.RandBytes(ctxid)
					mconn, err := getWire(ctxid)
					if err != nil {
						log.Println("getConn to", xaxa.Addr, err.Error())
						return
					}
					err = cand.Absorb(mconn)
					if err != nil {
						log.Println("absorb to", xaxa.Addr, err.Error())
						mconn.Close()
						return
					}
					select {
					case retline <- cand:
						cmd.stats.Lock()
						cmd.stats.netinfo.entry = natrium.HexEncode(
							natrium.SecureHash(xaxa.Cookie, nil)[:8])
						cmd.stats.netinfo.exit = exit
						cmd.stats.netinfo.prot = "cl-ni-2"
						cmd.stats.Unlock()
						log.Println(xaxa.Addr, "WINNER")
						// fan out
						FANOUT--
						go func() {
							for i := 0; i < FANOUT; i++ {
								mconn, err := getWire(ctxid)
								if err == nil {
									cand.Absorb(mconn)
								}
							}
						}()
					case <-dedline:
						log.Println(xaxa.Addr, "failed race")
						cand.Tomb().Kill(io.ErrClosedPipe)
					}
				}()
			}
		}
	}

	select {
	case <-time.After(time.Second * 15):
		log.Println("ConnEntry: failed to connect to anything within 15 seconds")
		cmd.smState = cmd.smClearCache
		return
	case ss := <-retline:
		close(dedline)
		cmd.currTunn = ss
		cmd.smState = cmd.smSteadyState
		return
	}
}

// smClearCache clears the cache and goes back to the entry point.
// => FindEntry always
func (cmd *Command) smClearCache() {
	log.Println("** => ClearCache **")
	defer log.Println("** <= ClearCache **")
	cmd.entryCache = nil
	cmd.exitCache = nil
	cmd.smState = cmd.smFindEntry
	if cmd.cdb != nil {
		cmd.cdb.Exec("DELETE FROM main WHERE k='bst.entries'")
	}
}
