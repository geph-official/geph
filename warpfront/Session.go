package warpfront

import (
	"bytes"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type session struct {
	rx  chan []byte
	tx  chan []byte
	ded chan bool
	buf *bytes.Buffer

	once sync.Once
}

func newSession() *session {
	return &session{
		rx:  make(chan []byte),
		tx:  make(chan []byte),
		ded: make(chan bool),
		buf: new(bytes.Buffer),
	}
}

func (sess *session) Write(pkt []byte) (int, error) {
	cpy := make([]byte, len(pkt))
	copy(cpy, pkt)
	select {
	case sess.tx <- cpy:
		return len(pkt), nil
	case <-sess.ded:
		return 0, io.ErrClosedPipe
	}
}

func (sess *session) Read(p []byte) (int, error) {
	if sess.buf.Len() > 0 {
		return sess.buf.Read(p)
	}
	select {
	case bts := <-sess.rx:
		sess.buf.Write(bts)
		return sess.Read(p)
	case <-sess.ded:
		return 0, io.ErrClosedPipe
	}
}

func (sess *session) Close() error {
	sess.once.Do(func() {
		close(sess.ded)
	})
	return nil
}

func (sess *session) LocalAddr() net.Addr {
	return nil
}

func (sess *session) RemoteAddr() net.Addr {
	return nil
}

func (sess *session) SetDeadline(t time.Time) error {
	log.Println("SetDeadline on warpfront.Session is currently no-op")
	return nil
}

func (sess *session) SetWriteDeadline(t time.Time) error {
	//log.Println("SetWriteDeadline on warpfront.Session is currently no-op")
	return nil
}

func (sess *session) SetReadDeadline(t time.Time) error {
	log.Println("SetReadDeadline on warpfront.Session is currently no-op")
	return nil
}
