package warpfront

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync/atomic"
)

func getWithHost(client *http.Client, url string, host string) (resp *http.Response, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return
	}
	req.Host = host
	return client.Do(req)
}

func postWithHost(client *http.Client, url string, host string, body io.Reader) (resp *http.Response, err error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return
	}
	req.Host = host
	req.Header.Add("Content-Type", "application/octet-stream")
	return client.Do(req)
}

var clGetCount int64
var clConnCount int64

// Connect returns a warpfront session connected to the given front and real host. The front must contain a protocol scheme (http:// or https://).
func Connect(client *http.Client, frontHost string, realHost string) (net.Conn, error) {
	// generate session number
	num := make([]byte, 32)
	rand.Read(num)
	// register our session
	resp, err := getWithHost(client, fmt.Sprintf("%v/register?id=%x", frontHost, num), realHost)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, errors.New("wtf")
	}

	atomic.AddInt64(&clGetCount, 1)
	atomic.AddInt64(&clConnCount, 1)
	sesh := newSession()

	go func() {
		defer sesh.Close()
		// poll and stuff into rx
		for i := 0; ; i++ {
			atomic.AddInt64(&clGetCount, 1)
			log.Println("warpfront: GET count", i)
			resp, err := getWithHost(client,
				fmt.Sprintf("%v/%x?serial=%v", frontHost, num, i),
				realHost)
			if err != nil {
				return
			}
			if resp.StatusCode != http.StatusOK {
				log.Println("WAT DIE")
				resp.Body.Close()
				return
			}

			for {
				lbts := make([]byte, 4)
				_, err := io.ReadFull(resp.Body, lbts)
				if err != nil {
					resp.Body.Close()
					return
				}
				if binary.BigEndian.Uint32(lbts) == 0 {
					//log.Println("warpfront: client got continuation signal, looping around")
					resp.Body.Close()
					goto OUT
				}
				buf := make([]byte, binary.BigEndian.Uint32(lbts))
				_, err = io.ReadFull(resp.Body, buf)
				if err != nil {
					resp.Body.Close()
					return
				}
				select {
				case sesh.rx <- buf:
				case <-sesh.ded:
					resp.Body.Close()
					return
				}
				if err != nil {
					resp.Body.Close()
					return
				}
			}
		OUT:
		}
	}()
	go func() {
		defer sesh.Close()
		// drain something from tx
		for i := 0; ; i++ {
			select {
			case bts := <-sesh.tx:
				atomic.AddInt64(&clGetCount, 1)
				//log.Println("warpfront: POST count", clGetCount, cid)
				resp, err := postWithHost(client,
					fmt.Sprintf("%v/%x?serial=%v", frontHost, num, i),
					realHost,
					bytes.NewBuffer(bts))
				if err != nil {
					return
				}

				dummy := new(bytes.Buffer)
				io.Copy(dummy, resp.Body)
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					return
				}
			case <-sesh.ded:
				return
			}
		}
	}()

	// couple closing the session with deletion
	go func() {
		<-sesh.ded
		getWithHost(client, fmt.Sprintf("%v/delete?id=%x", frontHost, num), realHost)
	}()

	// return the sesh
	return sesh, nil
}
