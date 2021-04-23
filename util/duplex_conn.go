package util

import (
	"io"
	"net"
	"time"
)

type duplexConn struct {
	duplex io.ReadWriteCloser
}

func NewDuplexConn(d io.ReadWriteCloser) *duplexConn {
	return &duplexConn{duplex: d}
}

func (d *duplexConn) Read(p []byte) (int, error) {
	return d.duplex.Read(p)
}

func (d *duplexConn) Write(p []byte) (int, error) {
	return d.duplex.Write(p)
}

func (d *duplexConn) Close() error {
	return d.duplex.Close()
}

func (d *duplexConn) LocalAddr() net.Addr {
	return nil
}

func (d *duplexConn) RemoteAddr() net.Addr {
	return nil
}

func (d *duplexConn) SetDeadline(t time.Time) error {
	return nil
}

func (d *duplexConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (d *duplexConn) SetWriteDeadline(t time.Time) error {
	return nil
}
