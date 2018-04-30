package warpfront

import (
	"bytes"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

type session struct {
	rx  chan []byte
	tx  chan []byte
	ded chan bool
	buf *bytes.Buffer

	rdded atomic.Value
	wrded atomic.Value

	once sync.Once
}

func newSession() *session {
	return &session{
		rx:  make(chan []byte, 1024),
		tx:  make(chan []byte, 1024),
		ded: make(chan bool),
		buf: new(bytes.Buffer),
	}
}

func (sess *session) Write(pkt []byte) (int, error) {
	deadline := sess.wrded.Load()
	var realded time.Duration
	if deadline == nil {
		realded = time.Hour
	} else {
		realded = deadline.(time.Time).Sub(time.Now())
	}
	cpy := make([]byte, len(pkt))
	copy(cpy, pkt)
	select {
	case sess.tx <- cpy:
		return len(pkt), nil
	case <-sess.ded:
		return 0, io.ErrClosedPipe
	case <-time.After(realded):
		return 0, io.ErrClosedPipe
	}
}

func (sess *session) Read(p []byte) (int, error) {
	deadline := sess.rdded.Load()
	var realded time.Duration
	if deadline == nil {
		realded = time.Hour
	} else {
		realded = deadline.(time.Time).Sub(time.Now())
	}
	if sess.buf.Len() > 0 {
		return sess.buf.Read(p)
	}
	select {
	case bts := <-sess.rx:
		sess.buf.Write(bts)
		return sess.Read(p)
	case <-sess.ded:
		return 0, io.ErrClosedPipe
	case <-time.After(realded):
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
	sess.SetReadDeadline(t)
	sess.SetWriteDeadline(t)
	return nil
}

func (sess *session) SetWriteDeadline(t time.Time) error {
	sess.wrded.Store(t)
	return nil
}

func (sess *session) SetReadDeadline(t time.Time) error {
	sess.rdded.Store(t)
	return nil
}
