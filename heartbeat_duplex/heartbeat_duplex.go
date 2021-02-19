package heartbeat_duplex

import (
	"encoding/binary"
	"github.com/nwtgck/go-piping-tunnel/util"
	"github.com/pkg/errors"
	"io"
	"sync"
	"time"
)

const (
	flagData byte = iota
	flagHeartbeat
)

type duplexWithHeartbeat struct {
	inner      io.ReadWriteCloser
	rest       uint32
	writeMutex *sync.Mutex
}

func Duplex(duplex io.ReadWriteCloser) io.ReadWriteCloser {
	d := &duplexWithHeartbeat{inner: duplex, rest: 0, writeMutex: new(sync.Mutex)}
	go func() {
		heartbeatInterval := 30 * time.Second
		for {
			d.writeMutex.Lock()
			d.inner.Write([]byte{flagHeartbeat})
			d.writeMutex.Unlock()
			time.Sleep(heartbeatInterval)
		}
	}()
	return d
}

func (d *duplexWithHeartbeat) Read(p []byte) (int, error) {
	if d.rest == 0 {
		b := make([]byte, 1)
		_, err := io.ReadFull(d.inner, b)
		if err != nil {
			return 0, err
		}
		flag := b[0]
		switch flag {
		case flagHeartbeat:
			return d.Read(p)
		case flagData:
			lengthBytes := make([]byte, 4)
			_, err = io.ReadFull(d.inner, lengthBytes)
			if err != nil {
				return 0, err
			}
			// Get length of data body
			d.rest = binary.BigEndian.Uint32(lengthBytes)
			return d.Read(p)
		default:
			return 0, errors.Errorf("unexpecrted flag: %d", flag)
		}
	}
	if len(p) >= int(d.rest) {
		p = p[0:d.rest]
	}
	n, err := d.inner.Read(p)
	d.rest -= uint32(n)
	return n, err
}

func (d *duplexWithHeartbeat) Write(p []byte) (int, error) {
	length := uint32(len(p))
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, length)
	d.writeMutex.Lock()
	defer d.writeMutex.Unlock()
	err := util.WriteFull(d.inner, append([]byte{flagData}, lengthBytes...))
	if err != nil {
		return 0, err
	}
	return d.inner.Write(p)
}

func (d *duplexWithHeartbeat) Close() error {
	return d.inner.Close()
}
