package client

import (
	"errors"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"gopkg.in/bunsim/miniss.v1"
	"gopkg.in/bunsim/natrium.v1"

	"github.com/rensa-labs/geph/niaucchi2"
	"github.com/rensa-labs/geph/warpfront"
	"gopkg.in/bunsim/cluttershirt.v1"
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
	for exit, entries := range cmd.ecache.GetEntries() {
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
					if natrium.CTCompare(mconn.RemotePK(), xaxa.ExitKey.ToECDH()) != 0 {
						log.Println("miniss to", xaxa.Addr, "bad auth")
						rawconn.Close()
						return
					}
					mconn.Write([]byte{0x02})
					// No ctxid for warpfront
					cand := niaucchi2.NewClientCtx()
					cand.Absorb(mconn)
					select {
					case retline <- cand:
						cmd.stats.Lock()
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
						err = errors.New("wrong public key")
						return
					}
					// 0x02
					_, err = mconn.Write(append([]byte{0x02}, ctxid...))
					if err != nil {
						log.Println("ctxid to", xaxa.Addr, err.Error())
						mconn.Close()
						return
					}
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
					// for the first one, we send stuff back and forth to eliminate low-latency but congested links
					dun := make(chan bool)
					go func() {
						//dur, _ := cand.Ping(make([]byte, 32000))
						//log.Println("32k to", xaxa.Addr, dur)
						var lol time.Duration
						var i int
						for ; i < 2; i++ {
							dur, err := cand.Ping(49) // get 50K
							if err != nil {
								break
							}
							lol += dur
						}
						if i != 0 {
							log.Println("ping+100K to", xaxa.Addr, lol/time.Duration(i))
						}
						close(dun)
					}()
					select {
					case <-dun:
					case <-dedline:
						log.Println(xaxa.Addr, "failed race")
						cand.Tomb().Kill(io.ErrClosedPipe)
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
		close(dedline)
		cmd.smState = cmd.smFindEntry
		return
	case ss := <-retline:
		close(dedline)
		cmd.currTunn = ss
		cmd.smState = cmd.smSteadyState
		return
	}
}
