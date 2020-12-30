package cmd

import (
	"fmt"
	"github.com/libp2p/go-yamux"
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
var serverSymmetricallyEncrypts bool
var serverSymmetricallyEncryptPassphrase string
var serverCipherType string

func init() {
	RootCmd.AddCommand(serverCmd)
	serverCmd.Flags().IntVarP(&serverHostPort, "port", "p", 0, "TCP port of server host")
	serverCmd.MarkFlagRequired("port")
	serverCmd.Flags().UintVarP(&serverClientToServerBufSize, "c-to-s-buf-size", "", 16, "Buffer size of client-to-server in bytes")
	serverCmd.Flags().BoolVarP(&serverYamux, yamuxFlagLongName, "", false, "Multiplex connection by libp2p/go-yamux")
	serverCmd.Flags().BoolVarP(&serverSymmetricallyEncrypts, symmetricallyEncryptsFlagLongName, symmetricallyEncryptsFlagShortName, false, "Encrypt symmetrically")
	serverCmd.Flags().StringVarP(&serverSymmetricallyEncryptPassphrase, symmetricallyEncryptPassphraseFlagLongName, "", "", "Passphrase for encryption")
	serverCmd.Flags().StringVarP(&serverCipherType, cipherTypeFlagLongName, "", defaultCipherType, fmt.Sprintf("Cipher type: %s, %s", cipherTypeAesCtr, cipherTypeOpenpgp))
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run server-host",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate cipher-type
		if serverSymmetricallyEncrypts {
			if err := validateClientCipher(serverCipherType); err != nil {
				return nil
			}
		}
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
		// Make user input passphrase if it is empty
		if serverSymmetricallyEncrypts {
			err = makeUserInputPassphraseIfEmpty(&serverSymmetricallyEncryptPassphrase)
			if err != nil {
				return err
			}
		}
		// Use multiplexer with yamux
		if serverYamux {
			fmt.Println("[INFO] Multiplexing with libp2p/go-yamux")
			return serverHandleWithYamux(httpClient, headers, clientToServerUrl, serverToClientUrl)
		}

		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", serverHostPort))
		if err != nil {
			return err
		}
		defer conn.Close()
		// If encryption is enabled
		if serverSymmetricallyEncrypts {
			duplex, err := makeDuplexWithEncryptionAndProgressIfNeed(httpClient, headers, serverToClientUrl, clientToServerUrl, serverSymmetricallyEncrypts, serverSymmetricallyEncryptPassphrase, serverCipherType)
			if err != nil {
				return err
			}
			fin := make(chan error)
			go func() {
				// TODO: hard code
				var buf = make([]byte, 16)
				_, err := io.CopyBuffer(duplex, conn, buf)
				fin <- err
			}()
			go func() {
				// TODO: hard code
				var buf = make([]byte, 16)
				_, err := io.CopyBuffer(conn, duplex, buf)
				fin <- err
			}()
			return util.CombineErrors(<-fin, <-fin)
		}
		var progress *io_progress.IOProgress = nil
		if showProgress {
			progress = io_progress.NewIOProgress(conn, conn, os.Stderr, makeProgressMessage)
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
			writer = progress
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

func printHintForClientHost(clientToServerUrl string, serverToClientUrl string, clientToServerPath string, serverToClientPath string) {
	if !serverYamux {
		fmt.Println("[INFO] Hint: Client host (socat + curl)")
		fmt.Printf(
			"  socat TCP-LISTEN:31376 'EXEC:curl -NsS %s!!EXEC:curl -NsST - %s'\n",
			strings.Replace(serverToClientUrl, ":", "\\:", -1),
			strings.Replace(clientToServerUrl, ":", "\\:", -1),
		)
	}
	flags := ""
	if serverSymmetricallyEncrypts {
		flags += fmt.Sprintf("-%s ", symmetricallyEncryptsFlagShortName)
		if serverCipherType != defaultCipherType {
			flags += fmt.Sprintf("--%s=%s ", cipherTypeFlagLongName, serverCipherType)
		}
	}
	if serverYamux {
		flags += fmt.Sprintf("--%s ", yamuxFlagLongName)
	}
	fmt.Println("[INFO] Hint: Client host (piping-tunnel)")
	fmt.Printf(
		"  piping-tunnel -s %s client -p 31376 %s%s %s\n",
		serverUrl,
		flags,
		clientToServerPath,
		serverToClientPath,
	)
}

func serverHandleWithYamux(httpClient *http.Client, headers []piping_tunnel_util.KeyValue, clientToServerUrl string, serverToClientUrl string) error {
	duplex, err := makeDuplexWithEncryptionAndProgressIfNeed(httpClient, headers, serverToClientUrl, clientToServerUrl, serverSymmetricallyEncrypts, serverSymmetricallyEncryptPassphrase, serverCipherType)
	if err != nil {
		return err
	}
	yamuxSession, err := yamux.Server(util.NewDuplexConn(duplex), yamuxConfig())
	if err != nil {
		return err
	}
	for {
		yamuxStream, err := yamuxSession.Accept()
		if err != nil {
			return err
		}
		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", serverHostPort))
		if err != nil {
			return err
		}
		fin := make(chan struct{})
		go func() {
			// TODO: hard code
			var buf = make([]byte, 16)
			io.CopyBuffer(yamuxStream, conn, buf)
			fin <- struct{}{}
		}()
		go func() {
			// TODO: hard code
			var buf = make([]byte, 16)
			io.CopyBuffer(conn, yamuxStream, buf)
			fin <- struct{}{}
		}()
		go func() {
			<-fin
			<-fin
			close(fin)
			conn.Close()
			yamuxStream.Close()
		}()
	}
}
