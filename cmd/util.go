package cmd

import "fmt"

func generatePaths(args []string) (string, string, error) {
	var path1 string
	var path2 string

	switch len(args) {
	case 1:
		path1 = fmt.Sprintf("%s/c-to-s", args[0])
		path2 = fmt.Sprintf("%s/s-to-c", args[0])
	case 2:
		path1 = args[0]
		path2 = args[1]
	default:
		return "", "", fmt.Errorf("The number of paths should be one or two\n")
	}
	return path1, path2, nil
}
