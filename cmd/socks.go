package cmd

import (
	"fmt"
	"github.com/armon/go-socks5"
	"github.com/hashicorp/yamux"
	"github.com/nwtgck/go-piping-tunnel/io_progress"
	piping_tunnel_util "github.com/nwtgck/go-piping-tunnel/piping-tunnel-util"
	"github.com/nwtgck/go-piping-tunnel/util"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"os"
	"strings"
)

var socksYamux bool

func init() {
	RootCmd.AddCommand(socksCmd)
	socksCmd.Flags().BoolVarP(&socksYamux, "yamux", "", false, "Multiplex connection by hashicorp/yamux")
}

var socksCmd = &cobra.Command{
	Use:   "socks",
	Short: "Run SOCKS5 server",
	RunE: func(cmd *cobra.Command, args []string) error {
		clientToServerPath, serverToClientPath, err := generatePaths(args)
		if err != nil {
			return err
		}
		headers, err := piping_tunnel_util.ParseKeyValueStrings(headerKeyValueStrs)
		if err != nil {
			return err
		}
		httpClient := util.CreateHttpClient(insecure, httpWriteBufSize, httpReadBufSize)
		if dnsServer != "" {
			// Set DNS resolver
			httpClient.Transport.(*http.Transport).DialContext = util.CreateDialContext(dnsServer)
		}
		serverToClientUrl, err := util.UrlJoin(serverUrl, serverToClientPath)
		if err != nil {
			return err
		}
		clientToServerUrl, err := util.UrlJoin(serverUrl, clientToServerPath)
		if err != nil {
			return err
		}
		// Print hint
		socksPrintHintForClientHost(clientToServerUrl, serverToClientUrl, clientToServerPath, serverToClientPath)

		// If not use multiplexer with yamux
		if !socksYamux {
			return fmt.Errorf("--yamux must be specified")
		}

		fmt.Println("[INFO] Multiplexing with hashicorp/yamux")
		socks5Conf := &socks5.Config{}
		socks5Server, err := socks5.New(socks5Conf)
		if err != nil {
			panic(err)
		}
		return socksHandleWithYamux(socks5Server, httpClient, headers, clientToServerUrl, serverToClientUrl)
	},
}

func socksPrintHintForClientHost(clientToServerUrl string, serverToClientUrl string, clientToServerPath string, serverToClientPath string) {
	if !socksYamux {
		fmt.Println("[INFO] Hint: Client host (socat + curl)")
		fmt.Printf(
			"  socat TCP-LISTEN:31376 'EXEC:curl -NsS %s!!EXEC:curl -NsST - %s'\n",
			strings.Replace(serverToClientUrl, ":", "\\:", -1),
			strings.Replace(clientToServerUrl, ":", "\\:", -1),
		)
	}
	flags := ""
	if socksYamux {
		flags += "--yamux "
	}
	fmt.Println("[INFO] Hint: Client host (piping-tunnel)")
	fmt.Printf(
		"  piping-tunnel -s %s client -p 1080 %s%s %s\n",
		serverUrl,
		flags,
		clientToServerPath,
		serverToClientPath,
	)
}

func socksHandleWithYamux(socks5Server *socks5.Server, httpClient *http.Client, headers []piping_tunnel_util.KeyValue, clientToServerUrl string, serverToClientUrl string) error {
	duplex, err := piping_tunnel_util.NewPipingDuplex(httpClient, headers, serverToClientUrl, clientToServerUrl)
	if err != nil {
		return err
	}
	var readWriteCloser io.ReadWriteCloser = duplex
	if showProgress {
		readWriteCloser = io_progress.NewIOProgress(duplex, duplex, os.Stderr, makeProgressMessage)
	}
	yamuxSession, err := yamux.Server(readWriteCloser, nil)
	if err != nil {
		return err
	}
	for {
		yamuxStream, err := yamuxSession.Accept()
		if err != nil {
			return err
		}
		go socks5Server.ServeConn(yamuxStream)
	}
}
