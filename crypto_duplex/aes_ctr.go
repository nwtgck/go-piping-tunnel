package crypto_duplex

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"github.com/nwtgck/go-piping-tunnel/util"
	"golang.org/x/crypto/pbkdf2"
	"io"
)

const saltLen = 64
const pbkdf2Iter = 4096
const keyLen = 32

type AesCtrDuplex struct {
	encryptWriter   io.WriteCloser
	decryptedReader io.Reader
	closeBaseReader func() error
}

func EncryptDuplexWithAesCtr(baseWriter io.WriteCloser, baseReader io.ReadCloser, passphrase []byte) (*AesCtrDuplex, error) {
	// Generate salt
	salt1, err := util.GenerateRandomBytes(saltLen)
	if err != nil {
		return nil, err
	}
	// Send the salt
	if _, err := baseWriter.Write(salt1); err != nil {
		return nil, err
	}
	// Derive key from passphrase
	key1 := pbkdf2.Key(passphrase, salt1, pbkdf2Iter, keyLen, crypto.SHA512.New)
	block, err := aes.NewCipher(key1)
	if err != nil {
		return nil, err
	}
	// Generate IV
	iv1, err := util.GenerateRandomBytes(aes.BlockSize)
	if err != nil {
		return nil, err
	}
	// Send the IV
	if _, err := baseWriter.Write(iv1); err != nil {
		return nil, err
	}
	encryptWriter := &cipher.StreamWriter{
		S: cipher.NewCTR(block, iv1),
		W: baseWriter,
	}
	block, err = aes.NewCipher(key1)
	if err != nil {
		return nil, err
	}

	// Read salt from peer
	salt2 := make([]byte, saltLen)
	if _, err := io.ReadFull(baseReader, salt2); err != nil {
		return nil, err
	}
	// Read IV from peer
	iv2 := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(baseReader, iv2); err != nil {
		return nil, err
	}
	// Derive key from passphrase
	key2 := pbkdf2.Key(passphrase, salt2, pbkdf2Iter, keyLen, crypto.SHA512.New)
	block2, err := aes.NewCipher(key2)
	if err != nil {
		return nil, err
	}
	decryptedReader := &cipher.StreamReader{
		S: cipher.NewCTR(block2, iv2),
		R: baseReader,
	}

	return &AesCtrDuplex{encryptWriter: encryptWriter, decryptedReader: decryptedReader, closeBaseReader: baseReader.Close}, nil
}

func (d *AesCtrDuplex) Write(p []byte) (int, error) {
	return d.encryptWriter.Write(p)
}

func (d *AesCtrDuplex) Read(p []byte) (int, error) {
	return d.decryptedReader.Read(p)
}

func (d *AesCtrDuplex) Close() error {
	wErr := d.encryptWriter.Close()
	rErr := d.closeBaseReader()
	return util.CombineErrors(wErr, rErr)
}
