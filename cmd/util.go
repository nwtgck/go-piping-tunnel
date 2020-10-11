package cmd

import "fmt"

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
