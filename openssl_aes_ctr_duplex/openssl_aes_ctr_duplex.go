package openssl_aes_ctr_duplex

import (
	"github.com/nwtgck/go-piping-tunnel/util"
	"hash"
	"io"
)

type opensslAesCtrDuplex struct {
	encryptWriter   io.WriteCloser
	decryptedReader io.Reader
	closeBaseReader func() error
}

func Duplex(baseWriter io.WriteCloser, baseReader io.ReadCloser, passphrase []byte, pbkdf2Iter int, keyLen int, h func() hash.Hash) (*opensslAesCtrDuplex, error) {
	encryptWriter, err := AesCtrEncryptWithPbkdf2(baseWriter, passphrase, pbkdf2Iter, keyLen, h)
	if err != nil {
		return nil, err
	}
	decryptedReader, err := AesCtrDecryptWithPbkdf2(baseReader, passphrase, pbkdf2Iter, keyLen, h)
	if err != nil {
		return nil, err
	}
	return &opensslAesCtrDuplex{encryptWriter: encryptWriter, decryptedReader: decryptedReader, closeBaseReader: baseReader.Close}, nil
}

func (d *opensslAesCtrDuplex) Write(p []byte) (int, error) {
	return d.encryptWriter.Write(p)
}

func (d *opensslAesCtrDuplex) Read(p []byte) (int, error) {
	return d.decryptedReader.Read(p)
}

func (d *opensslAesCtrDuplex) Close() error {
	wErr := d.encryptWriter.Close()
	rErr := d.closeBaseReader()
	return util.CombineErrors(wErr, rErr)
}
