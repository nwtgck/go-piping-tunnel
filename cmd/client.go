package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/yamux"
	"github.com/nwtgck/go-piping-tunnel/piping_util"
	"github.com/nwtgck/go-piping-tunnel/pmux"
	"github.com/nwtgck/go-piping-tunnel/util"
	"github.com/nwtgck/go-piping-tunnel/version"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
)

var clientHostPort int
var clientHostUnixSocket string
var clientServerToClientBufSize uint
var clientYamux bool
var clientPmux bool
var clientPmuxConfig string
var clientSymmetricallyEncrypts bool
var clientSymmetricallyEncryptPassphrase string
var clientCipherType string

func init() {
	RootCmd.AddCommand(clientCmd)
	clientCmd.Flags().IntVarP(&clientHostPort, "port", "p", 0, "TCP port of client host")
	clientCmd.Flags().StringVarP(&clientHostUnixSocket, "unix-socket", "", "", "Unix socket of client host")
	clientCmd.Flags().UintVarP(&clientServerToClientBufSize, "sc-buf-size", "", 16, "Buffer size of server-to-client in bytes")
	clientCmd.Flags().BoolVarP(&clientYamux, yamuxFlagLongName, "", false, "Multiplex connection by hashicorp/yamux")
	clientCmd.Flags().BoolVarP(&clientPmux, pmuxFlagLongName, "", false, "Multiplex connection by pmux (experimental)")
	clientCmd.Flags().StringVarP(&clientPmuxConfig, pmuxConfigFlagLongName, "", `{"hb": true}`, "pmux config in JSON (experimental)")
	clientCmd.Flags().BoolVarP(&clientSymmetricallyEncrypts, symmetricallyEncryptsFlagLongName, symmetricallyEncryptsFlagShortName, false, "Encrypt symmetrically")
	clientCmd.Flags().StringVarP(&clientSymmetricallyEncryptPassphrase, symmetricallyEncryptPassphraseFlagLongName, "", "", "Passphrase for encryption")
	clientCmd.Flags().StringVarP(&clientCipherType, cipherTypeFlagLongName, "", defaultCipherType, fmt.Sprintf("Cipher type: %s, %s", piping_util.CipherTypeAesCtr, piping_util.CipherTypeOpenpgp))
}

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Run client-host",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate cipher-type
		if clientSymmetricallyEncrypts {
			if err := validateClientCipher(clientCipherType); err != nil {
				return err
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
		clientToServerUrl, err := util.UrlJoin(serverUrl, clientToServerPath)
		if err != nil {
			return err
		}
		serverToClientUrl, err := util.UrlJoin(serverUrl, serverToClientPath)
		if err != nil {
			return err
		}
		var ln net.Listener
		if clientHostUnixSocket == "" {
			ln, err = net.Listen("tcp", fmt.Sprintf(":%d", clientHostPort))
		} else {
			ln, err = net.Listen("unix", clientHostUnixSocket)
		}
		if err != nil {
			return err
		}
		// Print hint
		printHintForServerHost(ln, clientToServerUrl, serverToClientUrl, clientToServerPath, serverToClientPath)
		// Make user input passphrase if it is empty
		if clientSymmetricallyEncrypts {
			err = makeUserInputPassphraseIfEmpty(&clientSymmetricallyEncryptPassphrase)
			if err != nil {
				return err
			}
		}
		// Use multiplexer with yamux
		if clientYamux {
			fmt.Println("[INFO] Multiplexing with hashicorp/yamux")
			return clientHandleWithYamux(ln, httpClient, headers, clientToServerUrl, serverToClientUrl)
		}
		// If pmux is enabled
		if clientPmux {
			fmt.Println("[INFO] Multiplexing with pmux")
			return clientHandleWithPmux(ln, httpClient, headers, clientToServerUrl, serverToClientUrl)
		}
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		fmt.Println("[INFO] accepted")
		// Refuse another new connection
		ln.Close()
		// If encryption is enabled
		if clientSymmetricallyEncrypts {
			var duplex io.ReadWriteCloser
			duplex, err := piping_util.DuplexConnect(httpClient, headers, clientToServerUrl, serverToClientUrl)
			if err != nil {
				return err
			}
			duplex, err = makeDuplexWithEncryptionAndProgressIfNeed(duplex, clientSymmetricallyEncrypts, clientSymmetricallyEncryptPassphrase, clientCipherType)
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
		err = piping_util.HandleDuplex(httpClient, conn, headers, clientToServerUrl, serverToClientUrl, clientServerToClientBufSize, nil, showProgress, makeProgressMessage)
		fmt.Println()
		if err != nil {
			return err
		}
		fmt.Println("[INFO] Finished")

		return nil
	},
}

func printHintForServerHost(ln net.Listener, clientToServerUrl string, serverToClientUrl string, clientToServerPath string, serverToClientPath string) {
	var listeningOn string
	if addr, ok := ln.Addr().(*net.TCPAddr); ok {
		// (base: https://stackoverflow.com/a/43425461)
		clientHostPort = addr.Port
		listeningOn = strconv.Itoa(addr.Port)
	} else {
		listeningOn = clientHostUnixSocket
	}
	fmt.Printf("[INFO] Client host listening on %s ...\n", listeningOn)
	if !clientYamux && !clientPmux {
		fmt.Println("[INFO] Hint: Server host (socat + curl)")
		fmt.Printf(
			"  socat 'EXEC:curl -NsS %s!!EXEC:curl -NsST - %s' TCP:127.0.0.1:<YOUR PORT>\n",
			strings.Replace(clientToServerUrl, ":", "\\:", -1),
			strings.Replace(serverToClientUrl, ":", "\\:", -1),
		)
	}
	fmt.Println("[INFO] Hint: Server host (piping-tunnel)")
	flags := ""
	if clientSymmetricallyEncrypts {
		flags += fmt.Sprintf("-%s ", symmetricallyEncryptsFlagShortName)
		if clientCipherType != defaultCipherType {
			flags += fmt.Sprintf("--%s=%s ", cipherTypeFlagLongName, clientCipherType)
		}
	}
	if clientYamux {
		flags += fmt.Sprintf("--%s ", yamuxFlagLongName)
	}
	if clientPmux {
		flags += fmt.Sprintf("--%s ", pmuxFlagLongName)
	}
	fmt.Printf(
		"  piping-tunnel -s %s server -p <YOUR PORT> %s%s %s\n",
		serverUrl,
		flags,
		clientToServerPath,
		serverToClientPath,
	)
	fmt.Println("    OR")
	fmt.Printf(
		"  piping-tunnel -s %s socks %s%s %s\n",
		serverUrl,
		flags,
		clientToServerPath,
		serverToClientPath,
	)
}

func clientHandleWithYamux(ln net.Listener, httpClient *http.Client, headers []piping_util.KeyValue, clientToServerUrl string, serverToClientUrl string) error {
	var duplex io.ReadWriteCloser
	duplex, err := piping_util.DuplexConnectWithHandlers(
		func(body io.Reader) (*http.Response, error) {
			return piping_util.PipingSend(httpClient, headersWithYamux(headers), clientToServerUrl, body)
		},
		func() (*http.Response, error) {
			res, err := piping_util.PipingGet(httpClient, headers, serverToClientUrl)
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
	duplex, err = makeDuplexWithEncryptionAndProgressIfNeed(duplex, clientSymmetricallyEncrypts, clientSymmetricallyEncryptPassphrase, clientCipherType)
	if err != nil {
		return err
	}
	yamuxSession, err := yamux.Client(duplex, nil)
	if err != nil {
		return err
	}

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		yamuxStream, err := yamuxSession.Open()
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

func clientHandleWithPmux(ln net.Listener, httpClient *http.Client, headers []piping_util.KeyValue, clientToServerUrl string, serverToClientUrl string) error {
	var config clientPmuxConfigJson
	if json.Unmarshal([]byte(clientPmuxConfig), &config) != nil {
		return errors.Errorf("invalid pmux config format")
	}
	pmuxClient, err := pmux.Client(httpClient, headers, clientToServerUrl, serverToClientUrl, config.Hb, clientSymmetricallyEncrypts, clientSymmetricallyEncryptPassphrase, clientCipherType)
	if err != nil {
		if err == pmux.NonPmuxMimeTypeError {
			return errors.Errorf("--%s may be missing in server", pmuxFlagLongName)
		}
		if err == pmux.IncompatiblePmuxVersion {
			return errors.Errorf("%s, hint: use the same piping-tunnel version (current: %s)", err.Error(), version.Version)
		}
		if err == pmux.IncompatibleServerConfigError {
			return errors.Errorf("%s, hint: use the same piping-tunnel version (current: %s)", err.Error(), version.Version)
		}
		return err
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			break
		}
		stream, err := pmuxClient.Open()
		if err != nil {
			vlog.Log(
				fmt.Sprintf("error(pmux open): %v", errors.WithStack(err)),
				fmt.Sprintf("error(pmux open): %+v", errors.WithStack(err)),
			)
			continue
		}
		fin := make(chan struct{})
		go func() {
			// TODO: hard code
			var buf = make([]byte, 16)
			_, err := io.CopyBuffer(conn, stream, buf)
			fin <- struct{}{}
			if err != nil {
				vlog.Log(
					fmt.Sprintf("error(pmux stream → conn): %v", errors.WithStack(err)),
					fmt.Sprintf("error(pmux stream → conn): %+v", errors.WithStack(err)),
				)
				return
			}
		}()

		go func() {
			// TODO: hard code
			var buf = make([]byte, 16)
			_, err := io.CopyBuffer(stream, conn, buf)
			fin <- struct{}{}
			if err != nil {
				vlog.Log(
					fmt.Sprintf("error(conn → pmux stream): %v", errors.WithStack(err)),
					fmt.Sprintf("error(conn → pmux stream): %+v", errors.WithStack(err)),
				)
				return
			}
		}()

		go func() {
			<-fin
			<-fin
			conn.Close()
			stream.Close()
			close(fin)
		}()
	}
	return nil
}
