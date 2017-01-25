// tinysocks project main.go
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/ProjectNiwl/tinysocks"
)

func main() {
	var port int
	flag.IntVar(&port, "port", 18123, "port on which to listen for SOCKS clients")
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%v", port))
	if err != nil {
		log.Printf("failed to bind SOCKS5 listener: %v", err.Error())
		os.Exit(-1)
	}
	log.Printf("SOCKS5 listener successfully bound at %v", listener.Addr())
	for {
		clnt, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleSocks(clnt)
	}
}

func handleSocks(clnt io.ReadWriteCloser) {
	defer clnt.Close()
	destin, err := tinysocks.ReadRequest(clnt)
	if err != nil {
		log.Printf("problem while reading SOCKS5 request: %v", err.Error())
		return
	}

	log.Printf("requesting %v", destin)

	remote, err := net.Dial("tcp", destin)
	if err != nil {
		log.Printf("failed to connect to %v (%v)", destin, err.Error())
		tinysocks.CompleteRequest(0x04, clnt)
		return
	}
	tinysocks.CompleteRequest(0x00, clnt)

	log.Printf("succesfully connected to %v", destin)
	defer log.Printf("freed %v", destin)

	// forward between local and remote
	go func() {
		defer clnt.Close()
		defer remote.Close()
		io.Copy(remote, clnt)
	}()
	io.Copy(clnt, remote)
}
