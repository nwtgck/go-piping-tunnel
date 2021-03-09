package cmd

import (
	"fmt"
	"github.com/hashicorp/yamux"
	"github.com/nwtgck/go-piping-tunnel/backoff"
	"github.com/nwtgck/go-piping-tunnel/piping_util"
	"github.com/nwtgck/go-piping-tunnel/pmux"
	"github.com/nwtgck/go-piping-tunnel/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

var serverHostPort int
var serverClientToServerBufSize uint
var serverYamux bool
var serverPmux bool
var serverSymmetricallyEncrypts bool
var serverSymmetricallyEncryptPassphrase string
var serverCipherType string

func init() {
	RootCmd.AddCommand(serverCmd)
	serverCmd.Flags().IntVarP(&serverHostPort, "port", "p", 0, "TCP port of server host")
	serverCmd.MarkFlagRequired("port")
	serverCmd.Flags().UintVarP(&serverClientToServerBufSize, "c-to-s-buf-size", "", 16, "Buffer size of client-to-server in bytes")
	serverCmd.Flags().BoolVarP(&serverYamux, yamuxFlagLongName, "", false, "Multiplex connection by hashicorp/yamux")
	serverCmd.Flags().BoolVarP(&serverPmux, pmuxFlagLongName, "", false, "Multiplex connection by pmux (experimental)")
	serverCmd.Flags().BoolVarP(&serverSymmetricallyEncrypts, symmetricallyEncryptsFlagLongName, symmetricallyEncryptsFlagShortName, false, "Encrypt symmetrically")
	serverCmd.Flags().StringVarP(&serverSymmetricallyEncryptPassphrase, symmetricallyEncryptPassphraseFlagLongName, "", "", "Passphrase for encryption")
	serverCmd.Flags().StringVarP(&serverCipherType, cipherTypeFlagLongName, "", defaultCipherType, fmt.Sprintf("Cipher type: %s, %s", piping_util.CipherTypeAesCtr, piping_util.CipherTypeOpenpgp))
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
		headers, err := piping_util.ParseKeyValueStrings(headerKeyValueStrs)
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
			fmt.Println("[INFO] Multiplexing with hashicorp/yamux")
			return serverHandleWithYamux(httpClient, headers, clientToServerUrl, serverToClientUrl)
		}

		// If pmux is enabled
		if serverPmux {
			fmt.Println("[INFO] Multiplexing with pmux")
			return serverHandleWithPmux(httpClient, headers, clientToServerUrl, serverToClientUrl)
		}

		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", serverHostPort))
		if err != nil {
			return err
		}
		defer conn.Close()
		// If encryption is enabled
		if serverSymmetricallyEncrypts {
			var duplex io.ReadWriteCloser
			duplex, err := piping_util.DuplexConnect(httpClient, headers, serverToClientUrl, clientToServerUrl)
			if err != nil {
				return err
			}
			duplex, err = makeDuplexWithEncryptionAndProgressIfNeed(duplex, serverSymmetricallyEncrypts, serverSymmetricallyEncryptPassphrase, serverCipherType)
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
		err = piping_util.HandleDuplex(httpClient, conn, headers, serverToClientUrl, clientToServerUrl, serverClientToServerBufSize, nil, showProgress, makeProgressMessage)
		fmt.Println()
		if err != nil {
			return err
		}
		fmt.Println("[INFO] Finished")

		return nil
	},
}

func printHintForClientHost(clientToServerUrl string, serverToClientUrl string, clientToServerPath string, serverToClientPath string) {
	if !serverYamux && !serverPmux {
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
	if serverPmux {
		flags += fmt.Sprintf("--%s ", pmuxFlagLongName)
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

func serverHandleWithYamux(httpClient *http.Client, headers []piping_util.KeyValue, clientToServerUrl string, serverToClientUrl string) error {
	var duplex io.ReadWriteCloser
	duplex, err := piping_util.DuplexConnectWithHandlers(
		func(body io.Reader) (*http.Response, error) {
			return piping_util.PipingSend(httpClient, headersWithYamux(headers), serverToClientUrl, body)
		},
		func() (*http.Response, error) {
			res, err := piping_util.PipingGet(httpClient, headers, clientToServerUrl)
			if err != nil {
				return nil, err
			}
			contentType := res.Header.Get("Content-Type")
			// NOTE: application/octet-stream is for compatibility
			if contentType != yamuxMimeType && contentType != "application/octet-stream" {
				return nil, errors.Errorf("invalid content-type: %s", contentType)
			}
			return res, nil
		},
	)
	if err != nil {
		return err
	}
	duplex, err = makeDuplexWithEncryptionAndProgressIfNeed(duplex, serverSymmetricallyEncrypts, serverSymmetricallyEncryptPassphrase, serverCipherType)
	if err != nil {
		return err
	}
	yamuxSession, err := yamux.Server(duplex, nil)
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

func dialLoop(network string, address string) net.Conn {
	b := backoff.NewExponentialBackoff()
	for {
		conn, err := net.Dial(network, address)
		if err != nil {
			// backoff
			time.Sleep(b.NextDuration())
			continue
		}
		return conn
	}
}

func serverHandleWithPmux(httpClient *http.Client, headers []piping_util.KeyValue, clientToServerUrl string, serverToClientUrl string) error {
	pmuxServer := pmux.Server(httpClient, headers, serverToClientUrl, clientToServerUrl, serverSymmetricallyEncrypts, serverSymmetricallyEncryptPassphrase, serverCipherType)
	for {
		stream, err := pmuxServer.Accept()
		if err != nil {
			return err
		}
		conn := dialLoop("tcp", fmt.Sprintf("localhost:%d", serverHostPort))
		go func() {
			// TODO: hard code
			var buf = make([]byte, 16)
			_, err := io.CopyBuffer(conn, stream, buf)
			if err != nil {
				// TODO:
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				conn.Close()
				return
			}
		}()

		go func() {
			// TODO: hard code
			var buf = make([]byte, 16)
			_, err := io.CopyBuffer(stream, conn, buf)
			if err != nil {
				// TODO:
				fmt.Fprintf(os.Stderr, "error: %v\n", err)
				conn.Close()
				return
			}
		}()
	}
}
