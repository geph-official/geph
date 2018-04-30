package entry

import (
	"fmt"
	"net"
)

func listenTCP(p int) net.Listener {
	l, e := net.Listen("tcp", fmt.Sprintf(":%v", p))
	if e != nil {
		panic(e)
	}
	return l
}
