package client

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"time"

	"gopkg.in/bunsim/miniss.v1"
	"gopkg.in/bunsim/natrium.v1"

	"github.com/rensa-labs/geph/internal/niaucchi3"
	"gopkg.in/bunsim/cluttershirt.v1"
)

// smConnEntry is the ConnEntry state, where a connection to some entry node is established.
// => VerifyAccount if successful
// => ClearCache if unsuccessful
func (cmd *Command) smConnEntry() {
	log.Println("** => ConnEntry **")
	defer log.Println("** <= ConnEntry **")

	retline := make(chan *miniss.Socket)
	dedline := make(chan bool)
	for exit, entries := range cmd.ecache.GetEntries() {
		for _, xaxa := range entries {
			exit := exit
			xaxa := xaxa
			log.Printf("%v (%x) from %v\n", xaxa.Addr, natrium.SecureHash(xaxa.Cookie, nil)[:4], exit)

			getWire := func() (mconn *miniss.Socket, err error) {
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
				mconn, err = miniss.Handshake(oconn, natrium.ECDHGenerateKey())
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
				return
			}
			go func() {
				mconn, err := getWire()
				if err != nil {
					log.Println("getConn to", xaxa.Addr, err.Error())
					return
				}
				log.Println("getWire returned")
				select {
				case retline <- mconn:
					cmd.stats.Lock()
					cmd.stats.netinfo.entry = natrium.HexEncode(
						natrium.SecureHash(xaxa.Cookie, nil)[:8])
					cmd.stats.netinfo.exit = exit
					cmd.stats.netinfo.prot = "cl-ni-3"
					cmd.stats.Unlock()
					log.Println(xaxa.Addr, "WINNER")
				case <-dedline:
					log.Println(xaxa.Addr, "failed race")
					mconn.Close()
				}
			}()
		}
	}

	select {
	case <-time.After(time.Second * 15):
		log.Println("ConnEntry: failed to connect to anything within 15 seconds")
		close(dedline)
		cmd.smState = cmd.smFindEntry
		return
	case mconn := <-retline:
		close(dedline)
		// 0x03
		_, err := mconn.Write([]byte{0x03})
		if err != nil {
			log.Println("niaucchi3", err.Error())
			mconn.Close()
			return
		}
		// JSON authentication
		authObject := map[string]string{
			"Username": cmd.uname,
			"Password": cmd.pwd,
		}
		marsh, _ := json.Marshal(authObject)
		binary.Write(mconn, binary.BigEndian, uint16(len(marsh)))
		mconn.Write(marsh)
		zz := make([]byte, 1)
		_, err = mconn.Read(zz)
		if err != nil {
			log.Println(err.Error())
			return
		}
		log.Println("GOOOOD", zz)
		switch zz[0] {
		case 'y':
			log.Println("GOOD!")
		case 'n':
			log.Println("niaucchi3 AUTH BAD")
			log.Println("** FATAL: account info is wrong! **")
			os.Exit(43)
			return
		default:
			log.Println("niaucchi3 auth interrupted")
			err = io.ErrClosedPipe
			mconn.Close()
			return
		}
		cmd.currTunn = niaucchi3.NewContext(true, mconn)
		cmd.smState = cmd.smSteadyState
		return
	}
}
