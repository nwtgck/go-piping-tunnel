package cmd

import (
	"fmt"
	"github.com/nwtgck/go-piping-tunnel/io_progress"
	"github.com/nwtgck/go-piping-tunnel/openpgp_duplex"
	"github.com/nwtgck/go-piping-tunnel/util"
	"io"
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

func openPGPEncryptedDuplex(duplex io.ReadWriteCloser, passphrase string) (io.ReadWriteCloser, error) {
	// If the passphrase is empty
	if passphrase == "" {
		var err error
		// Get user-input passphrase
		passphrase, err = util.InputPassphrase()
		if err != nil {
			return nil, err
		}
	}
	// Encrypt
	duplex, err := openpgp_duplex.NewSymmetricallyDuplex(duplex, duplex, []byte(passphrase))
	if err != nil {
		return nil, err
	}
	fmt.Println("[INFO] End-to-end encryption with OpenPGP")
	return duplex, nil
}
