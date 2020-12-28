package crypto_duplex

import (
	"crypto/aes"
	"crypto/cipher"
	"io"
)

type Duplex struct {
	w io.WriteCloser
	r io.Reader
}

// TODO: add passphrase
func NewDuplex(baseWriter io.WriteCloser, baseReader io.ReadCloser) (*Duplex, error) {
	// TODO: should derive from passphrase
	key := []byte("e05hjyKTJIOF5Umr")
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	// TODO: Should random
	iv1 := make([]byte, aes.BlockSize)
	stream := cipher.NewCTR(block, iv1)
	writer := &cipher.StreamWriter{S: stream, W: baseWriter}

	block, err = aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	var iv2 [aes.BlockSize]byte
	stream = cipher.NewCTR(block, iv2[:])
	reader := &cipher.StreamReader{S: stream, R: baseReader}

	return &Duplex{w: writer, r: reader}, nil
}

func (d *Duplex) Write(p []byte) (int, error) {
	return d.w.Write(p)
}

func (d *Duplex) Read(p []byte) (int, error) {
	return d.r.Read(p)
}

func (d *Duplex) Close() error {
	return d.w.Close()
}
