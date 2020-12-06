package cmd

import (
	"fmt"
	"github.com/nwtgck/go-piping-tunnel/io_progress"
	"github.com/nwtgck/go-piping-tunnel/util"
	"time"
)

func generatePaths(args []string) (string, string, error) {
	var clientToServerPath string
	var serverToClientPath string

	switch len(args) {
	case 1:
		clientToServerPath = fmt.Sprintf("%s/c-to-s", args[0])
		serverToClientPath = fmt.Sprintf("%s/s-to-c", args[0])
	case 2:
		clientToServerPath = args[0]
		serverToClientPath = args[1]
	default:
		return "", "", fmt.Errorf("The number of paths should be one or two\n")
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
