package cmd

import (
	"fmt"
	"github.com/nwtgck/go-piping-tunnel/crypto_duplex"
	"github.com/nwtgck/go-piping-tunnel/io_progress"
	piping_tunnel_util "github.com/nwtgck/go-piping-tunnel/piping-tunnel-util"
	"github.com/nwtgck/go-piping-tunnel/util"
	"io"
	"net/http"
	"os"
	"time"
)

func generatePaths(args []string) (string, string, error) {
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
		return "", "", fmt.Errorf("the number of paths should be one or two")
	}
	return clientToServerPath, serverToClientPath, nil
}

func makeProgressMessage(progress *io_progress.IOProgress) string {
	return fmt.Sprintf(
		"↑ %s (%s/s) | ↓ %s (%s/s)",
		util.HumanizeBytes(float64(progress.CurrReadBytes)),
		util.HumanizeBytes(float64(progress.CurrReadBytes)/time.Since(progress.StartTime).Seconds()),
		util.HumanizeBytes(float64(progress.CurrWriteBytes)),
		util.HumanizeBytes(float64(progress.CurrWriteBytes)/time.Since(progress.StartTime).Seconds()),
	)
}

func makeUserInputPassphraseIfEmpty(passphrase *string) (err error) {
	// If the passphrase is empty
	if *passphrase == "" {
		// Get user-input passphrase
		*passphrase, err = util.InputPassphrase()
		return err
	}
	return nil
}

func makeDuplexWithEncryptionAndProgressIfNeed(httpClient *http.Client, headers []piping_tunnel_util.KeyValue, uploadUrl, downloadUrl string, encrypts bool, passphrase string) (io.ReadWriteCloser, error) {
	var duplex io.ReadWriteCloser
	duplex, err := piping_tunnel_util.DuplexConnect(httpClient, headers, uploadUrl, downloadUrl)
	if err != nil {
		return nil, err
	}
	// If encryption is enabled
	if encrypts {
		// Encrypt
		duplex, err = crypto_duplex.EncryptDuplexWithAesCtr(duplex, duplex, []byte(passphrase))
		if err != nil {
			return nil, err
		}
		fmt.Println("[INFO] End-to-end encryption with AES-CTR")
	}
	if showProgress {
		duplex = io_progress.NewIOProgress(duplex, duplex, os.Stderr, makeProgressMessage)
	}
	return duplex, nil
}
