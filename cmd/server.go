package cmd

import (
	"fmt"
	"github.com/hashicorp/yamux"
	"github.com/nwtgck/go-piping-tunnel/io_progress"
	piping_tunnel_util "github.com/nwtgck/go-piping-tunnel/piping-tunnel-util"
	"github.com/nwtgck/go-piping-tunnel/util"
	"github.com/spf13/cobra"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
)

var serverHostPort int
var serverClientToServerBufSize uint
var serverYamux bool

func init() {
	serverCmd.Flags().IntVarP(&serverHostPort, "port", "p", 0, "TCP port of server host")
	serverCmd.MarkFlagRequired("port")
	serverCmd.Flags().UintVarP(&serverClientToServerBufSize, "c-to-s-buf-size", "", 16, "Buffer size of client-to-server in bytes")
	serverCmd.Flags().BoolVarP(&serverYamux, "yamux", "", false, "Multiplex connection by hashicorp/yamux")
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run server-host",
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
		printHintForClientHost(clientToServerUrl, serverToClientUrl, clientToServerPath, serverToClientPath)

		// Use multiplexer with yamux
		if serverYamux {
			fmt.Println("[INFO] Multiplexing with hashicorp/yamux")
			return serverHandleWithYamux(httpClient, headers, clientToServerUrl, serverToClientUrl)
		}

		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", serverHostPort))
		if err != nil {
			return err
		}
		defer conn.Close()
		var progress *io_progress.IOProgress = nil
		if showProgress {
			p := io_progress.NewIOProgress(conn, os.Stderr, makeProgressMessage)
			progress = &p
		}
		var reader io.Reader = conn
		if progress != nil {
			reader = progress
		}
		req, err := http.NewRequest("POST", serverToClientUrl, reader)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		for _, kv := range headers {
			req.Header.Set(kv.Key, kv.Value)
		}
		_, err = httpClient.Do(req)
		if err != nil {
			return err
		}

		req, err = http.NewRequest("GET", clientToServerUrl, nil)
		if err != nil {
			return err
		}
		for _, kv := range headers {
			req.Header.Set(kv.Key, kv.Value)
		}
		res, err := httpClient.Do(req)
		if err != nil {
			return err
		}
		var writer io.Writer = conn
		if progress != nil {
			writer = io.MultiWriter(conn, progress)
		}
		var buf = make([]byte, serverClientToServerBufSize)
		_, err = io.CopyBuffer(writer, res.Body, buf)
		fmt.Println()
		if err != nil {
			return err
		}
		fmt.Println("[INFO] Finished")

		return nil
	},
}

// TODO: proper hint when multiplexing
func printHintForClientHost(clientToServerUrl string, serverToClientUrl string, clientToServerPath string, serverToClientPath string) {
	fmt.Println("[INFO] Hint: Client host (socat + curl)")
	fmt.Printf(
		"  socat TCP-LISTEN:31376 'EXEC:curl -NsS %s!!EXEC:curl -NsST - %s'\n",
		strings.Replace(serverToClientUrl, ":", "\\:", -1),
		strings.Replace(clientToServerUrl, ":", "\\:", -1),
	)
	fmt.Println("[INFO] Hint: Client host (piping-tunnel)")
	fmt.Printf(
		"  piping-tunnel -s %s client -p 31376 %s %s\n",
		serverUrl,
		clientToServerPath,
		serverToClientPath,
	)
}

func serverHandleWithYamux(httpClient *http.Client, headers []piping_tunnel_util.KeyValue, clientToServerUrl string, serverToClientUrl string) error {
	duplex, err := piping_tunnel_util.NewPipingDuplex(httpClient, headers, serverToClientUrl, clientToServerUrl)
	if err != nil {
		return err
	}
	var readWriteCloser io.ReadWriteCloser = duplex
	if showProgress {
		readWriteCloser = util.NewIOProgressReadWriteCloser(duplex, os.Stderr, makeProgressMessage)
	}
	yamuxSession, err := yamux.Server(readWriteCloser, nil)
	if err != nil {
		return err
	}
	for {
		yamuxSession, err := yamuxSession.Accept()
		if err != nil {
			return err
		}
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", serverHostPort))
		if err != nil {
			return err
		}
		go func() {
			io.Copy(yamuxSession, conn)
		}()
		go func() {
			io.Copy(conn, yamuxSession)
		}()
	}
	return nil
}
