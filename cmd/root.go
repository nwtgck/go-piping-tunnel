package cmd

import (
	"fmt"
	"github.com/nwtgck/go-piping-tunnel/version"
	"github.com/spf13/cobra"
	"os"
)

const (
	ServerUrlEnvName = "PIPING_SERVER"
)

var serverUrl string
var insecure bool
var dnsServer string
var showsVersion bool
var showProgress bool
var headerKeyValueStrs []string
var httpWriteBufSize int
var httpReadBufSize int

func init() {
	cobra.OnInitialize()
	RootCmd.AddCommand(serverCmd)
	defaultServer, ok := os.LookupEnv(ServerUrlEnvName)
	if !ok {
		defaultServer = "https://ppng.io"
	}
	RootCmd.PersistentFlags().StringVarP(&serverUrl, "server", "s", defaultServer, "Piping Server URL")
	RootCmd.PersistentFlags().StringVar(&dnsServer, "dns-server", "", "DNS server (e.g. 1.1.1.1:53)")
	// NOTE: --insecure, -k is inspired by curl
	RootCmd.PersistentFlags().BoolVarP(&insecure, "insecure", "k", false, "Allow insecure server connections when using SSL")
	RootCmd.PersistentFlags().StringArrayVarP(&headerKeyValueStrs, "header", "H", []string{}, "HTTP header")
	RootCmd.PersistentFlags().IntVarP(&httpWriteBufSize, "http-write-buf-size", "", 16, "HTTP write-buffer size in bytes")
	RootCmd.PersistentFlags().IntVarP(&httpReadBufSize, "http-read-buf-size", "", 16, "HTTP read-buffer size in bytes")
	RootCmd.PersistentFlags().BoolVarP(&showProgress, "progress", "", true, "Show progress")
	RootCmd.Flags().BoolVarP(&showsVersion, "version", "v", false, "show version")
}

var RootCmd = &cobra.Command{
	Use:   os.Args[0],
	Short: "piping-tunnel",
	Long:  "Tunnel over Piping Server",
	RunE: func(cmd *cobra.Command, args []string) error {
		if showsVersion {
			fmt.Println(version.Version)
			return nil
		}
		return cmd.Help()
	},
}
