package warpfront

import (
	"bytes"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type rwcNagle struct {
	buffer    *bytes.Buffer
	transport net.Conn

	wdedch chan time.Time
	clch   chan bool
	err    error

	once sync.Once
	sync.Mutex
}

// RWCNagle does something like the Nagle algorithm.
func RWCNagle(transport net.Conn) net.Conn {
	toret := &rwcNagle{
		buffer:    new(bytes.Buffer),
		transport: transport,

		wdedch: make(chan time.Time, 256),
		clch:   make(chan bool),
	}
	go toret.thrFlush()
	return toret
}

func (rcn *rwcNagle) thrFlush() {
	dline := time.Now().Add(time.Second * 10)

	for {
		select {
		case ndl := <-rcn.wdedch:
			dline = ndl
		case <-time.After(dline.Sub(time.Now())):
			rcn.Lock()
			log.Println("rwcNagle flushing", rcn.buffer.Len(), "bytes")
			_, rcn.err = io.Copy(rcn.transport, rcn.buffer)
			if rcn.err != nil {
				rcn.transport.Close()
				rcn.Unlock()
				return
			}
			rcn.Unlock()
			dline = time.Now().Add(time.Second * 60)
		case <-rcn.clch:
			return
		}
	}
}

func (rcn *rwcNagle) Write(p []byte) (n int, err error) {
	rcn.Lock()
	err = rcn.err
	n, _ = rcn.buffer.Write(p)
	buflen := rcn.buffer.Len()
	rcn.Unlock()

	if err != nil {
		return
	}

	// if buflen too long, flush immediately
	if buflen > 512*1024 {
		rcn.Lock()
		_, rcn.err = io.Copy(rcn.transport, rcn.buffer)
		if rcn.err != nil {
			err = rcn.err
			rcn.transport.Close()
			rcn.Unlock()
			return
		}
		rcn.Unlock()
	}

	// otherwise we flush after 50ms / KiB, but at least 20ms
	select {
	case rcn.wdedch <- time.Now().Add(time.Millisecond * (50*time.Duration(len(p)/1024) + 20)):
	default:
	}
	return
}

func (rcn *rwcNagle) Read(p []byte) (int, error) {
	return rcn.transport.Read(p)
}

func (rcn *rwcNagle) Close() error {
	rcn.once.Do(func() {
		go func() {
			time.Sleep(time.Second * 10)
			rcn.transport.Close()
		}()

		rcn.Lock()
		io.Copy(rcn.transport, rcn.buffer)
		rcn.Unlock()
		close(rcn.clch)
		rcn.transport.Close()
	})
	return nil
}

func (rcn *rwcNagle) LocalAddr() net.Addr {
	return rcn.transport.LocalAddr()
}

func (rcn *rwcNagle) RemoteAddr() net.Addr {
	return rcn.transport.RemoteAddr()
}

func (rcn *rwcNagle) SetDeadline(t time.Time) error {
	return rcn.transport.SetDeadline(t)
}

func (rcn *rwcNagle) SetReadDeadline(t time.Time) error {
	return rcn.transport.SetReadDeadline(t)
}

func (rcn *rwcNagle) SetWriteDeadline(t time.Time) error {
	return rcn.transport.SetWriteDeadline(t)
}
