package cmd

import (
	"crypto"
	_ "crypto/sha1"
	_ "crypto/sha256"
	_ "crypto/sha512"
	"encoding/json"
	"fmt"
	"github.com/nwtgck/go-piping-tunnel/aes_ctr_duplex"
	"github.com/nwtgck/go-piping-tunnel/io_progress"
	"github.com/nwtgck/go-piping-tunnel/openpgp_duplex"
	"github.com/nwtgck/go-piping-tunnel/openssl_aes_ctr_duplex"
	"github.com/nwtgck/go-piping-tunnel/piping_util"
	"github.com/nwtgck/go-piping-tunnel/util"
	"github.com/nwtgck/go-piping-tunnel/verbose_logger"
	"github.com/pkg/errors"
	"hash"
	"io"
	"os"
	"time"
)

const DefaultCipherType = piping_util.CipherTypeAesCtr

const (
	YamuxFlagLongName                          = "yamux"
	PmuxFlagLongName                           = "pmux"
	PmuxConfigFlagLongName                     = "pmux-config"
	SymmetricallyEncryptsFlagLongName          = "symmetric"
	SymmetricallyEncryptsFlagShortName         = "c"
	SymmetricallyEncryptPassphraseFlagLongName = "passphrase"
	CipherTypeFlagLongName                     = "cipher-type"
	Pbkdf2FlagLongName                         = "pbkdf2"
)

const YamuxMimeType = "application/yamux"

type ServerPmuxConfigJson struct {
	Hb bool `json:"hb"`
}

type ClientPmuxConfigJson struct {
	Hb bool `json:"hb"`
}

type pbkdf2ConfigJson struct {
	Iter int    `json:"iter"`
	Hash string `json:"hash"`
}

type Pbkdf2Config struct {
	Iter                   int
	Hash                   func() hash.Hash
	HashNameForCommandHint string // for command hint
}

type OpensslAesCtrParams struct {
	KeyBits uint16
	Pbkdf2  *Pbkdf2Config
}

var Vlog *verbose_logger.Logger

func init() {
	Vlog = &verbose_logger.Logger{}
}

func ValidateClientCipher(str string) error {
	switch str {
	case piping_util.CipherTypeAesCtr:
		return nil
	case piping_util.CipherTypeOpensslAes128Ctr:
		return nil
	case piping_util.CipherTypeOpensslAes256Ctr:
		return nil
	case piping_util.CipherTypeOpenpgp:
		return nil
	default:
		return errors.Errorf("invalid cipher type: %s", str)
	}
}

func validateHashFunctionName(str string) (func() hash.Hash, error) {
	switch str {
	case "sha1":
		return crypto.SHA1.New, nil
	case "sha256":
		return crypto.SHA256.New, nil
	case "sha512":
		return crypto.SHA512.New, nil
	default:
		return nil, errors.Errorf("unsupported hash: %s", str)
	}
}

func ParsePbkdf2(str string) (*Pbkdf2Config, error) {
	var configJson pbkdf2ConfigJson
	if json.Unmarshal([]byte(str), &configJson) != nil {
		return nil, errors.Errorf("invalid pbkdf2 JSON format: e.g. --%s='%s'", Pbkdf2FlagLongName, ExamplePbkdf2JsonStr())
	}
	h, err := validateHashFunctionName(configJson.Hash)
	if err != nil {
		return nil, err
	}
	return &Pbkdf2Config{Iter: configJson.Iter, Hash: h, HashNameForCommandHint: configJson.Hash}, nil
}

func ParseOpensslAesCtrParams(cipherType string, pbkdf2ConfigJsonStr string) (*OpensslAesCtrParams, error) {
	var keyBits uint16
	switch cipherType {
	case piping_util.CipherTypeOpensslAes128Ctr:
		keyBits = 128
	case piping_util.CipherTypeOpensslAes256Ctr:
		keyBits = 256
	}
	switch cipherType {
	case piping_util.CipherTypeOpensslAes128Ctr:
		fallthrough
	case piping_util.CipherTypeOpensslAes256Ctr:
		pbkdf2Config, err := ParsePbkdf2(pbkdf2ConfigJsonStr)
		if err != nil {
			return nil, err
		}
		return &OpensslAesCtrParams{KeyBits: keyBits, Pbkdf2: pbkdf2Config}, nil
	}
	return nil, nil
}

func ExamplePbkdf2JsonStr() string {
	b, err := json.Marshal(&pbkdf2ConfigJson{Iter: 100000, Hash: "sha256"})
	if err != nil {
		panic(err)
	}
	return string(b)
}

func GeneratePaths(args []string) (string, string, error) {
	var clientToServerPath string
	var serverToClientPath string

	switch len(args) {
	case 1:
		// NOTE: "cs": from client-host to server-host
		clientToServerPath = fmt.Sprintf("%s/cs", args[0])
		// NOTE: "sc": from server-host to client-host
		serverToClientPath = fmt.Sprintf("%s/sc", args[0])
	case 2:
		clientToServerPath = args[0]
		serverToClientPath = args[1]
	default:
		return "", "", errors.New("the number of paths should be one or two")
	}
	return clientToServerPath, serverToClientPath, nil
}

func MakeProgressMessage(progress *io_progress.IOProgress) string {
	return fmt.Sprintf(
		"↑ %s (%s/s) | ↓ %s (%s/s)",
		util.HumanizeBytes(float64(progress.CurrReadBytes)),
		util.HumanizeBytes(float64(progress.CurrReadBytes)/time.Since(progress.StartTime).Seconds()),
		util.HumanizeBytes(float64(progress.CurrWriteBytes)),
		util.HumanizeBytes(float64(progress.CurrWriteBytes)/time.Since(progress.StartTime).Seconds()),
	)
}

func MakeUserInputPassphraseIfEmpty(passphrase *string) (err error) {
	// If the passphrase is empty
	if *passphrase == "" {
		// Get user-input passphrase
		*passphrase, err = util.InputPassphrase()
		return err
	}
	return nil
}

func MakeDuplexWithEncryptionAndProgressIfNeed(duplex io.ReadWriteCloser, encrypts bool, passphrase string, cipherType string, pbkdf2JsonStr string) (io.ReadWriteCloser, error) {
	var err error
	// If encryption is enabled
	if encrypts {
		var cipherName string
		switch cipherType {
		case piping_util.CipherTypeAesCtr:
			// Encrypt with AES-CTR
			duplex, err = aes_ctr_duplex.Duplex(duplex, duplex, []byte(passphrase))
			cipherName = "AES-CTR"
		case piping_util.CipherTypeOpensslAes128Ctr:
			pbkdf2, err := ParsePbkdf2(pbkdf2JsonStr)
			if err != nil {
				return nil, err
			}
			duplex, err = openssl_aes_ctr_duplex.Duplex(duplex, duplex, []byte(passphrase), pbkdf2.Iter, 128/8, pbkdf2.Hash)
			cipherName = "OpenSSL-AES-128-CTR-compatible"
		case piping_util.CipherTypeOpensslAes256Ctr:
			pbkdf2, err := ParsePbkdf2(pbkdf2JsonStr)
			if err != nil {
				return nil, err
			}
			duplex, err = openssl_aes_ctr_duplex.Duplex(duplex, duplex, []byte(passphrase), pbkdf2.Iter, 256/8, pbkdf2.Hash)
			cipherName = "OpenSSL-AES-256-CTR-compatible"
		case piping_util.CipherTypeOpenpgp:
			duplex, err = openpgp_duplex.SymmetricallyEncryptDuplexWithOpenPGP(duplex, duplex, []byte(passphrase))
			cipherName = "OpenPGP"
		default:
			return nil, errors.Errorf("unexpected cipher type: %s", cipherType)
		}
		if err != nil {
			return nil, err
		}
		fmt.Printf("[INFO] End-to-end encryption with %s\n", cipherName)
	}
	if ShowProgress {
		duplex = io_progress.NewIOProgress(duplex, duplex, os.Stderr, MakeProgressMessage)
	}
	return duplex, nil
}

func HeadersWithYamux(headers []piping_util.KeyValue) []piping_util.KeyValue {
	return append(headers, piping_util.KeyValue{Key: "Content-Type", Value: YamuxMimeType})
}
