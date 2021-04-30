package main

import (
	"fmt"
	"github.com/nwtgck/go-piping-tunnel/cmd"
	_ "github.com/nwtgck/go-piping-tunnel/cmd/client"
	_ "github.com/nwtgck/go-piping-tunnel/cmd/server"
	_ "github.com/nwtgck/go-piping-tunnel/cmd/socks"
	"os"
)

func main() {
	if err := cmd.RootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(-1)
	}
}
