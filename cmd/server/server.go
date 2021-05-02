package server

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/yamux"
	"github.com/nwtgck/go-piping-tunnel/backoff"
	"github.com/nwtgck/go-piping-tunnel/cmd"
	"github.com/nwtgck/go-piping-tunnel/piping_util"
	"github.com/nwtgck/go-piping-tunnel/pmux"
	"github.com/nwtgck/go-piping-tunnel/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"io"
	"net"
	"net/http"
	"time"
)

var flag struct {
	targetHost                     string
	serverHostPort                 int
	serverHostUnixSocket           string
	clientToServerBufSize          uint
	yamux                          bool
	pmux                           bool
	pmuxConfig                     string
	symmetricallyEncrypts          bool
	symmetricallyEncryptPassphrase string
	cipherType                     string
	pbkdf2JsonString               string
}

func init() {
	cmd.RootCmd.AddCommand(serverCmd)
	serverCmd.Flags().StringVarP(&flag.targetHost, "host", "", "localhost", "Target host")
	serverCmd.Flags().IntVarP(&flag.serverHostPort, "port", "p", 0, "TCP port of server host")
	serverCmd.Flags().StringVarP(&flag.serverHostUnixSocket, "unix-socket", "", "", "Unix socket of server host")
	serverCmd.Flags().UintVarP(&flag.clientToServerBufSize, "cs-buf-size", "", 16, "Buffer size of client-to-server in bytes")
	serverCmd.Flags().BoolVarP(&flag.yamux, cmd.YamuxFlagLongName, "", false, "Multiplex connection by hashicorp/yamux")
	serverCmd.Flags().BoolVarP(&flag.pmux, cmd.PmuxFlagLongName, "", false, "Multiplex connection by pmux (experimental)")
	serverCmd.Flags().StringVarP(&flag.pmuxConfig, cmd.PmuxConfigFlagLongName, "", `{"hb": true}`, "pmux config in JSON (experimental)")
	serverCmd.Flags().BoolVarP(&flag.symmetricallyEncrypts, cmd.SymmetricallyEncryptsFlagLongName, cmd.SymmetricallyEncryptsFlagShortName, false, "Encrypt symmetrically")
	serverCmd.Flags().StringVarP(&flag.symmetricallyEncryptPassphrase, cmd.SymmetricallyEncryptPassphraseFlagLongName, "", "", "Passphrase for encryption")
	serverCmd.Flags().StringVarP(&flag.cipherType, cmd.CipherTypeFlagLongName, "", cmd.DefaultCipherType, fmt.Sprintf("Cipher type: %s, %s, %s, %s ", piping_util.CipherTypeAesCtr, piping_util.CipherTypeOpensslAes128Ctr, piping_util.CipherTypeOpensslAes256Ctr, piping_util.CipherTypeOpenpgp))
	// NOTE: default value of --pbkdf2 should be empty to detect key derive derivation from multiple algorithms in the future.
	serverCmd.Flags().StringVarP(&flag.pbkdf2JsonString, cmd.Pbkdf2FlagLongName, "", "", fmt.Sprintf("e.g. %s", cmd.ExamplePbkdf2JsonStr()))
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run server-host",
	RunE: func(_ *cobra.Command, args []string) error {
		// Validate cipher-type
		if flag.symmetricallyEncrypts {
			if err := cmd.ValidateClientCipher(flag.cipherType); err != nil {
				return err
			}
		}
		clientToServerPath, serverToClientPath, err := cmd.GeneratePaths(args)
		if err != nil {
			return err
		}
		headers, err := piping_util.ParseKeyValueStrings(cmd.HeaderKeyValueStrs)
		if err != nil {
			return err
		}
		httpClient := util.CreateHttpClient(cmd.Insecure, cmd.HttpWriteBufSize, cmd.HttpReadBufSize)
		if cmd.DnsServer != "" {
			// Set DNS resolver
			httpClient.Transport.(*http.Transport).DialContext = util.CreateDialContext(cmd.DnsServer)
		}
		serverToClientUrl, err := util.UrlJoin(cmd.ServerUrl, serverToClientPath)
		if err != nil {
			return err
		}
		clientToServerUrl, err := util.UrlJoin(cmd.ServerUrl, clientToServerPath)
		if err != nil {
			return err
		}
		var opensslAesCtrParams *cmd.OpensslAesCtrParams = nil
		if flag.symmetricallyEncrypts {
			opensslAesCtrParams, err = cmd.ParseOpensslAesCtrParams(flag.cipherType, flag.pbkdf2JsonString)
			if err != nil {
				return err
			}
		}
		// Print hint
		printHintForClientHost(clientToServerUrl, serverToClientUrl, clientToServerPath, serverToClientPath, opensslAesCtrParams)
		// Make user input passphrase if it is empty
		if flag.symmetricallyEncrypts {
			err = cmd.MakeUserInputPassphraseIfEmpty(&flag.symmetricallyEncryptPassphrase)
			if err != nil {
				return err
			}
		}
		// Use multiplexer with yamux
		if flag.yamux {
			fmt.Println("[INFO] Multiplexing with hashicorp/yamux")
			return serverHandleWithYamux(httpClient, headers, clientToServerUrl, serverToClientUrl)
		}

		// If pmux is enabled
		if flag.pmux {
			fmt.Println("[INFO] Multiplexing with pmux")
			return serverHandleWithPmux(httpClient, headers, clientToServerUrl, serverToClientUrl)
		}

		conn, err := serverHostDial()
		if err != nil {
			return err
		}
		defer conn.Close()
		// If encryption is enabled
		if flag.symmetricallyEncrypts {
			var duplex io.ReadWriteCloser
			duplex, err := piping_util.DuplexConnect(httpClient, headers, serverToClientUrl, clientToServerUrl)
			if err != nil {
				return err
			}
			duplex, err = cmd.MakeDuplexWithEncryptionAndProgressIfNeed(duplex, flag.symmetricallyEncrypts, flag.symmetricallyEncryptPassphrase, flag.cipherType, flag.pbkdf2JsonString)
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
		err = piping_util.HandleDuplex(httpClient, conn, headers, serverToClientUrl, clientToServerUrl, flag.clientToServerBufSize, nil, cmd.ShowProgress, cmd.MakeProgressMessage)
		fmt.Println()
		if err != nil {
			return err
		}
		fmt.Println("[INFO] Finished")

		return nil
	},
}

func serverHostDial() (net.Conn, error) {
	if flag.serverHostUnixSocket == "" {
		return net.Dial("tcp", fmt.Sprintf("%s:%d", flag.targetHost, flag.serverHostPort))
	} else {
		return net.Dial("unix", flag.serverHostUnixSocket)
	}
}

func printHintForClientHost(clientToServerUrl string, serverToClientUrl string, clientToServerPath string, serverToClientPath string, opensslAesCtrParams *cmd.OpensslAesCtrParams) {
	if !flag.yamux && !flag.pmux {
		if flag.symmetricallyEncrypts {
			if opensslAesCtrParams != nil {
				fmt.Println("[INFO] Hint: Client host. <PORT> should be replaced (socat + curl + openssl)")
				fmt.Printf(
					"  read -p \"passphrase: \" -s pass && curl -NsS %s | stdbuf -i0 -o0 openssl aes-%d-ctr -d -pass \"pass:$pass\" -bufsize 1 -pbkdf2 -iter %d -md %s | socat TCP-LISTEN:31376 - | stdbuf -i0 -o0 openssl aes-%d-ctr -pass \"pass:$pass\" -bufsize 1 -pbkdf2 -iter %d -md %s | curl -NsST - %s; unset pass\n",
					serverToClientUrl,
					opensslAesCtrParams.KeyBits,
					opensslAesCtrParams.Pbkdf2.Iter,
					opensslAesCtrParams.Pbkdf2.HashNameForCommandHint,
					opensslAesCtrParams.KeyBits,
					opensslAesCtrParams.Pbkdf2.Iter,
					opensslAesCtrParams.Pbkdf2.HashNameForCommandHint,
					clientToServerUrl,
				)
			}
		} else {
			fmt.Println("[INFO] Hint: Client host (socat + curl)")
			// NOTE: nc can be used instead of socat but nc has variant: `nc -l 31376` in BSD version, `nc -l -p 31376` in GNU version.
			fmt.Printf("  curl -NsS %s | socat TCP-LISTEN:31376 - | curl -NsST - %s\n", serverToClientUrl, clientToServerUrl)
		}
	}
	flags := ""
	if flag.symmetricallyEncrypts {
		flags += fmt.Sprintf("-%s ", cmd.SymmetricallyEncryptsFlagShortName)
		flags += fmt.Sprintf("--%s=%s ", cmd.CipherTypeFlagLongName, flag.cipherType)
		switch flag.cipherType {
		case piping_util.CipherTypeOpensslAes128Ctr:
			fallthrough
		case piping_util.CipherTypeOpensslAes256Ctr:
			flags += fmt.Sprintf("--%s='%s' ", cmd.Pbkdf2FlagLongName, flag.pbkdf2JsonString)
		}
	}
	if flag.yamux {
		flags += fmt.Sprintf("--%s ", cmd.YamuxFlagLongName)
	}
	if flag.pmux {
		flags += fmt.Sprintf("--%s ", cmd.PmuxFlagLongName)
	}
	fmt.Println("[INFO] Hint: Client host (piping-tunnel)")
	fmt.Printf(
		"  piping-tunnel -s %s client -p 31376 %s%s %s\n",
		cmd.ServerUrl,
		flags,
		clientToServerPath,
		serverToClientPath,
	)
}

func serverHandleWithYamux(httpClient *http.Client, headers []piping_util.KeyValue, clientToServerUrl string, serverToClientUrl string) error {
	var duplex io.ReadWriteCloser
	duplex, err := piping_util.DuplexConnectWithHandlers(
		func(body io.Reader) (*http.Response, error) {
			return piping_util.PipingSend(httpClient, cmd.HeadersWithYamux(headers), serverToClientUrl, body)
		},
		func() (*http.Response, error) {
			res, err := piping_util.PipingGet(httpClient, headers, clientToServerUrl)
			if err != nil {
				return nil, err
			}
			contentType := res.Header.Get("Content-Type")
			// NOTE: application/octet-stream is for compatibility
			if contentType != cmd.YamuxMimeType && contentType != "application/octet-stream" {
				return nil, errors.Errorf("invalid content-type: %s", contentType)
			}
			return res, nil
		},
	)
	if err != nil {
		return err
	}
	duplex, err = cmd.MakeDuplexWithEncryptionAndProgressIfNeed(duplex, flag.symmetricallyEncrypts, flag.symmetricallyEncryptPassphrase, flag.cipherType, flag.pbkdf2JsonString)
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
		conn, err := serverHostDial()
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

func dialLoop() net.Conn {
	b := backoff.NewExponentialBackoff()
	for {
		conn, err := serverHostDial()
		if err != nil {
			// backoff
			time.Sleep(b.NextDuration())
			continue
		}
		return conn
	}
}

func serverHandleWithPmux(httpClient *http.Client, headers []piping_util.KeyValue, clientToServerUrl string, serverToClientUrl string) error {
	var config cmd.ServerPmuxConfigJson
	if json.Unmarshal([]byte(flag.pmuxConfig), &config) != nil {
		return errors.Errorf("invalid pmux config format")
	}
	pmuxServer := pmux.Server(httpClient, headers, serverToClientUrl, clientToServerUrl, config.Hb, flag.symmetricallyEncrypts, flag.symmetricallyEncryptPassphrase, flag.cipherType)
	for {
		stream, err := pmuxServer.Accept()
		if err != nil {
			return err
		}
		conn := dialLoop()
		go func() {
			// TODO: hard code
			var buf = make([]byte, 16)
			_, err := io.CopyBuffer(conn, stream, buf)
			if err != nil {
				cmd.Vlog.Log(
					fmt.Sprintf("error(pmux stream → conn): %v", errors.WithStack(err)),
					fmt.Sprintf("error(pmux stream → conn): %+v", errors.WithStack(err)),
				)
				conn.Close()
				return
			}
		}()

		go func() {
			// TODO: hard code
			var buf = make([]byte, 16)
			_, err := io.CopyBuffer(stream, conn, buf)
			if err != nil {
				cmd.Vlog.Log(
					fmt.Sprintf("error(conn → pmux stream): %v", errors.WithStack(err)),
					fmt.Sprintf("error(conn → pmux stream): %+v", errors.WithStack(err)),
				)
				conn.Close()
				return
			}
		}()
	}
}
