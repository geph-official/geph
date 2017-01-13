package warpfront

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"testing"
	"time"
)

func TestServer(t *testing.T) {
	http.DefaultTransport.(*http.Transport).MaxIdleConnsPerHost = 32

	srv := NewServer()
	http.Handle("/", srv)
	go http.ListenAndServe(":8080", srv)
	go func() {
		for {
			fmt.Println("XAXA")
			lol, _ := srv.Accept()
			go func() {
				defer lol.Close()
				io.Copy(lol, lol)
			}()
		}
	}()
	time.Sleep(time.Millisecond * 50)
	sesh, err := Connect(&http.Client{}, "http://127.0.0.1:8080", "127.0.0.1")
	if err != nil {
		panic(err.Error())
	}
	//sesh = RWCNagle(sesh)

	defer sesh.Close()
	go func() {
		defer sesh.Close()
		for i := 0; i < 1000; i++ {
			fmt.Fprintln(sesh, "line", i)
			//time.Sleep(time.Millisecond * 200)
		}
	}()
	for {
		xaxa := make([]byte, 4096)
		n, err := sesh.Read(xaxa)
		if err != nil {
			log.Println(err.Error())
			break
		}
		log.Print(string(xaxa[:n]))
	}
}
