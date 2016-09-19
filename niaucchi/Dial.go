package niaucchi

import (
	"errors"
	"net"
	"time"

	"gopkg.in/bunsim/miniss.v1"

	"gopkg.in/bunsim/cluttershirt.v1"
	"gopkg.in/bunsim/natrium.v1"
	"gopkg.in/tomb.v2"
)

// DialSubstrate dials to the given destination and returns a substrate. A nil obfuscation cookie means the unobfuscated protocol would be used.
func DialSubstrate(ocookie []byte, theirPK natrium.ECDHPublic, addr string, mult int) (ss *Substrate, err error) {
	conns := make([]net.Conn, mult)
	connid := make([]byte, 32)
	natrium.RandBytes(connid)
	xaxa := new(tomb.Tomb)
	for i := 0; i < mult; i++ {
		i := i
		xaxa.Go(func() (err error) {
			z, err := net.DialTimeout("tcp", addr, time.Second*10)
			if err != nil {
				return
			}
			if ocookie != nil {
				z, err = cluttershirt.Client(ocookie, z)
				if err != nil {
					return
				}
			}
			crypt, err := miniss.Handshake(z, natrium.ECDHGenerateKey())
			if err != nil {
				return
			}
			if natrium.CTCompare(crypt.RemotePK(), theirPK) != 0 {
				err = errors.New("authentication failure")
				return
			}
			conns[i] = crypt
			_, err = conns[i].Write([]byte{byte(mult)})
			_, err = conns[i].Write(connid)
			if err != nil {
				return
			}
			return
		})
	}
	err = xaxa.Wait()
	if err != nil {
		for _, v := range conns {
			if v != nil {
				v.Close()
			}
		}
		return
	}
	ss = NewSubstrate(conns)
	return
}
