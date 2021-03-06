package openpgp_duplex

import (
	"github.com/nwtgck/go-piping-tunnel/util"
	"golang.org/x/crypto/openpgp"
	"io"
)

type symmetricallyDuplex struct {
	encryptWriter     io.WriteCloser
	decryptedReader   io.Reader
	decryptedReaderCh chan interface{} // io.Reader or error
	closeBaseReader   func() error
}

func SymmetricallyEncryptDuplexWithOpenPGP(baseWriter io.WriteCloser, baseReader io.ReadCloser, passphrase []byte) (*symmetricallyDuplex, error) {
	encryptWriter, err := openpgp.SymmetricallyEncrypt(baseWriter, passphrase, nil, nil)
	if err != nil {
		return nil, err
	}
	decryptedReaderCh := make(chan interface{})
	go func() {
		// (base: https://github.com/golang/crypto/blob/a2144134853fc9a27a7b1e3eb4f19f1a76df13c9/openpgp/write_test.go#L129)
		md, err := openpgp.ReadMessage(baseReader, nil, func(keys []openpgp.Key, symmetric bool) ([]byte, error) {
			return passphrase, nil
		}, nil)
		if err != nil {
			decryptedReaderCh <- err
			return
		}
		decryptedReaderCh <- md.UnverifiedBody
	}()

	return &symmetricallyDuplex{
		encryptWriter:     encryptWriter,
		decryptedReaderCh: decryptedReaderCh,
		closeBaseReader:   baseReader.Close,
	}, nil
}

func (o *symmetricallyDuplex) Write(p []byte) (int, error) {
	return o.encryptWriter.Write(p)
}

func (o *symmetricallyDuplex) Read(p []byte) (int, error) {
	if o.decryptedReaderCh != nil {
		// Get io.Reader or error
		result := <-o.decryptedReaderCh
		// If result is error
		if err, ok := result.(error); ok {
			return 0, err
		}
		o.decryptedReader = result.(io.Reader)
		o.decryptedReaderCh = nil
	}
	return o.decryptedReader.Read(p)
}

func (o *symmetricallyDuplex) Close() error {
	wErr := o.encryptWriter.Close()
	rErr := o.closeBaseReader()
	return util.CombineErrors(wErr, rErr)
}
