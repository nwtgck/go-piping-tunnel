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

var ServerUrl string
var Insecure bool
var DnsServer string
var showsVersion bool
var ShowProgress bool
var HeaderKeyValueStrs []string
var HttpWriteBufSize int
var HttpReadBufSize int
var verboseLoggerLevel int

func init() {
	cobra.OnInitialize()
	defaultServer, ok := os.LookupEnv(ServerUrlEnvName)
	if !ok {
		defaultServer = "https://ppng.io"
	}
	RootCmd.PersistentFlags().StringVarP(&ServerUrl, "server", "s", defaultServer, "Piping Server URL")
	RootCmd.PersistentFlags().StringVar(&DnsServer, "dns-server", "", "DNS server (e.g. 1.1.1.1:53)")
	// NOTE: --insecure, -k is inspired by curl
	RootCmd.PersistentFlags().BoolVarP(&Insecure, "insecure", "k", false, "Allow insecure server connections when using SSL")
	RootCmd.PersistentFlags().StringArrayVarP(&HeaderKeyValueStrs, "header", "H", []string{}, "HTTP header")
	RootCmd.PersistentFlags().IntVarP(&HttpWriteBufSize, "http-write-buf-size", "", 4096, "HTTP write-buffer size in bytes")
	RootCmd.PersistentFlags().IntVarP(&HttpReadBufSize, "http-read-buf-size", "", 4096, "HTTP read-buffer size in bytes")
	RootCmd.PersistentFlags().BoolVarP(&ShowProgress, "progress", "", true, "Show progress")
	RootCmd.Flags().BoolVarP(&showsVersion, "version", "v", false, "show version")
	RootCmd.PersistentFlags().IntVarP(&verboseLoggerLevel, "verbose", "", 0, "Verbose logging level")
}

var RootCmd = &cobra.Command{
	Use:          os.Args[0],
	Short:        "piping-tunnel",
	Long:         "Tunneling from anywhere with Piping Server",
	SilenceUsage: true,
	Example: fmt.Sprintf(`
Normal:
  piping-tunnel server -p 22 aaa bbb
  piping-tunnel client -p 1022 aaa bbb

Short:
  piping-tunnel server -p 22 mypath
  piping-tunnel client -p 1022 mypath

Multiplexing:
  piping-tunnel server -p 22 --yamux aaa bbb
  piping-tunnel client -p 1022 --yamux aaa bbb

SOCKS proxy like VPN:
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
		Vlog.Level = verboseLoggerLevel
	},
}
