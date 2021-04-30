package openssl_aes_ctr_duplex

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"github.com/pkg/errors"
	"golang.org/x/crypto/pbkdf2"
	"hash"
	"io"
)

type KeyAndIV struct {
	Key []byte
	Iv  []byte
}

const ivLen = 16

func DeriveKeyAndIvByPbkdf2(password []byte, salt []byte, iter int, keyLen int, h func() hash.Hash) KeyAndIV {
	keyAndIv := pbkdf2.Key(password, salt, iter, keyLen+ivLen, h)
	return KeyAndIV{
		Key: keyAndIv[:keyLen],
		Iv:  keyAndIv[keyLen:],
	}
}

func AesCtrEncryptWithPbkdf2(w io.Writer, password []byte, pbkdf2Iter int, keyLen int, h func() hash.Hash) (io.WriteCloser, error) {
	var salt [8]byte
	if _, err := io.ReadFull(rand.Reader, salt[:]); err != nil {
		return nil, err
	}
	keyAndIV := DeriveKeyAndIvByPbkdf2(password, salt[:], pbkdf2Iter, keyLen, h)
	block, err := aes.NewCipher(keyAndIV.Key)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(append([]byte("Salted__"), salt[:]...)); err != nil {
		return nil, err
	}
	encryptingWriter := &cipher.StreamWriter{
		S: cipher.NewCTR(block, keyAndIV.Iv),
		W: w,
	}
	return encryptingWriter, nil
}

func AesCtrDecryptWithPbkdf2(encryptedReader io.Reader, password []byte, pbkdf2Iter int, keyLen int, h func() hash.Hash) (io.Reader, error) {
	var eightBytes [8]byte
	if _, err := io.ReadFull(encryptedReader, eightBytes[:]); err != nil {
		return nil, err
	}
	if string(eightBytes[:]) != "Salted__" {
		return nil, errors.New("not start with Salted__")
	}
	// Read salt
	if _, err := io.ReadFull(encryptedReader, eightBytes[:]); err != nil {
		return nil, err
	}
	// Derive key and IV
	keyAndIV := DeriveKeyAndIvByPbkdf2(password, eightBytes[:], pbkdf2Iter, keyLen, h)
	block, err := aes.NewCipher(keyAndIV.Key)
	if err != nil {
		return nil, err
	}
	decryptedReader := &cipher.StreamReader{
		S: cipher.NewCTR(block, keyAndIV.Iv),
		R: encryptedReader,
	}
	return decryptedReader, nil
}
