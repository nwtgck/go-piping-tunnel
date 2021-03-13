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
var verboseLoggerLevel int

func init() {
	cobra.OnInitialize()
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
	RootCmd.PersistentFlags().IntVarP(&verboseLoggerLevel, "verbose", "", 0, "Verbose logging level")
}

var RootCmd = &cobra.Command{
	Use:          os.Args[0],
	Short:        "piping-tunnel",
	Long:         "Tunnel over Piping Server",
	SilenceUsage: true,
	Example: fmt.Sprintf(`
Normal:
  piping-tunnel server -p 22 aaa bbb
  piping-tunnel client -p 1022 aaa bbb

Short:
  piping-tunnel server -p 22 aaa
  piping-tunnel client -p 1022 aaa

Multiplexing:
  piping-tunnel server -p 22 --yamux aaa bbb
  piping-tunnel client -p 1022 --yamux aaa bbb

SOCKS5 like VPN:
  piping-tunnel socks --yamux aaa bbb
  piping-tunnel client -p 1080 --yamux aaa bbb

Environment variable:
  $%s for default Piping Server
`, ServerUrlEnvName),
	RunE: func(cmd *cobra.Command, args []string) error {
		if showsVersion {
			fmt.Println(version.Version)
			return nil
		}
		return cmd.Help()
	},
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		vlog.Level = verboseLoggerLevel
	},
}
