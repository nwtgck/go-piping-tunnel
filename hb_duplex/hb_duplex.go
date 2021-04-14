package hb_duplex

import (
	"crypto/rand"
	"encoding/binary"
	"github.com/pkg/errors"
	"io"
	"sync"
	"time"
)

const (
	dataType byte = iota
	heartbeatType
)

type hbDuplex struct {
	inner      io.ReadWriteCloser
	rest       uint32
	writeMutex *sync.Mutex
}

func Duplex(duplex io.ReadWriteCloser) io.ReadWriteCloser {
	d := &hbDuplex{inner: duplex, rest: 0, writeMutex: new(sync.Mutex)}
	go func() {
		heartbeatInterval := 30 * time.Second
		for {
			d.writeMutex.Lock()
			randomBytes := make([]byte, 1)
			io.ReadFull(rand.Reader, randomBytes)
			d.inner.Write([]byte{heartbeatType, randomBytes[0]})
			d.writeMutex.Unlock()
			time.Sleep(heartbeatInterval)
		}
	}()
	return d
}

func (d *hbDuplex) Read(p []byte) (int, error) {
	if d.rest == 0 {
		b := make([]byte, 1)
		_, err := io.ReadFull(d.inner, b)
		if err != nil {
			return 0, err
		}
		flag := b[0]
		switch flag {
		case heartbeatType:
			// Discard one random byte
			b := make([]byte, 1)
			_, err := io.ReadFull(d.inner, b)
			if err != nil {
				return 0, err
			}
			return d.Read(p)
		case dataType:
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

func (d *hbDuplex) Write(p []byte) (int, error) {
	length := uint32(len(p))
	lengthBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBytes, length)
	d.writeMutex.Lock()
	defer d.writeMutex.Unlock()
	bytes := append([]byte{dataType}, lengthBytes...)
	n, err := d.inner.Write(bytes)
	if n != len(bytes) {
		return n, io.ErrShortWrite
	}
	if err != nil {
		return 0, err
	}
	return d.inner.Write(p)
}

func (d *hbDuplex) Close() error {
	return d.inner.Close()
}
