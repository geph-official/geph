// +build uftcp

package entry

import (
	"net"

	"github.com/rensa-labs/unfairtcp"
)

func listenTCP(p int) net.Listener {
	l, e := unfairtcp.Listen(uint16(p))
	if e != nil {
		panic(e)
	}
	return l
}
