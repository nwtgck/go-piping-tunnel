package piping_util

import (
	"github.com/nwtgck/go-piping-tunnel/io_progress"
	"github.com/pkg/errors"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	CipherTypeOpenpgp string = "openpgp"
	CipherTypeAesCtr         = "aes-ctr"
)

type KeyValue struct {
	Key   string
	Value string
}

func ParseKeyValueStrings(strKeyValues []string) ([]KeyValue, error) {
	var keyValues []KeyValue
	for _, str := range strKeyValues {
		splitted := strings.SplitN(str, ":", 2)
		if len(splitted) != 2 {
			return nil, errors.Errorf("invalid header format '%s'", str)
		}
		keyValues = append(keyValues, KeyValue{Key: splitted[0], Value: splitted[1]})
	}
	return keyValues, nil
}

// NOTE: duplex is usually conn
func HandleDuplex(httpClient *http.Client, duplex io.ReadWriteCloser, headers []KeyValue, uploadUrl string, downloadUrl string, downloadBufSize uint, arriveCh chan<- struct{}, showProgress bool, makeProgressMessage func(progress *io_progress.IOProgress) string) error {
	var progress *io_progress.IOProgress = nil
	if showProgress {
		progress = io_progress.NewIOProgress(duplex, duplex, os.Stderr, makeProgressMessage)
	}
	var reader io.Reader = duplex
	if progress != nil {
		reader = progress
	}
	_, err := PipingSend(httpClient, headers, uploadUrl, reader)
	if err != nil {
		return err
	}
	res, err := PipingGet(httpClient, headers, downloadUrl)
	if err != nil {
		return err
	}
	if arriveCh != nil {
		arriveCh <- struct{}{}
	}
	var writer io.Writer = duplex
	if progress != nil {
		writer = progress
	}
	var buf = make([]byte, downloadBufSize)
	_, err = io.CopyBuffer(writer, res.Body, buf)
	return err
}
