package client

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/yamux"
	"github.com/nwtgck/go-piping-tunnel/cmd"
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
)

var flag struct {
	clientHostPort                 int
	clientHostUnixSocket           string
	serverToClientBufSize          uint
	yamux                          bool
	pmux                           bool
	pmuxConfig                     string
	symmetricallyEncrypts          bool
	symmetricallyEncryptPassphrase string
	cipherType                     string
	pbkdf2JsonString               string
}

func init() {
	cmd.RootCmd.AddCommand(clientCmd)
	clientCmd.Flags().IntVarP(&flag.clientHostPort, "port", "p", 0, "TCP port of client host")
	clientCmd.Flags().StringVarP(&flag.clientHostUnixSocket, "unix-socket", "", "", "Unix socket of client host")
	clientCmd.Flags().UintVarP(&flag.serverToClientBufSize, "sc-buf-size", "", 16, "Buffer size of server-to-client in bytes")
	clientCmd.Flags().BoolVarP(&flag.yamux, cmd.YamuxFlagLongName, "", false, "Multiplex connection by hashicorp/yamux")
	clientCmd.Flags().BoolVarP(&flag.pmux, cmd.PmuxFlagLongName, "", false, "Multiplex connection by pmux (experimental)")
	clientCmd.Flags().StringVarP(&flag.pmuxConfig, cmd.PmuxConfigFlagLongName, "", `{"hb": true}`, "pmux config in JSON (experimental)")
	clientCmd.Flags().BoolVarP(&flag.symmetricallyEncrypts, cmd.SymmetricallyEncryptsFlagLongName, cmd.SymmetricallyEncryptsFlagShortName, false, "Encrypt symmetrically")
	clientCmd.Flags().StringVarP(&flag.symmetricallyEncryptPassphrase, cmd.SymmetricallyEncryptPassphraseFlagLongName, "", "", "Passphrase for encryption")
	clientCmd.Flags().StringVarP(&flag.cipherType, cmd.CipherTypeFlagLongName, "", cmd.DefaultCipherType, fmt.Sprintf("Cipher type: %s, %s, %s, %s ", piping_util.CipherTypeAesCtr, piping_util.CipherTypeOpensslAes128Ctr, piping_util.CipherTypeOpensslAes256Ctr, piping_util.CipherTypeOpenpgp))
	// NOTE: default value of --pbkdf2 should be empty to detect key derive derivation from multiple algorithms in the future.
	clientCmd.Flags().StringVarP(&flag.pbkdf2JsonString, cmd.Pbkdf2FlagLongName, "", "", fmt.Sprintf("e.g. %s", cmd.ExamplePbkdf2JsonStr()))
}

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Run client-host",
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
		clientToServerUrl, err := util.UrlJoin(cmd.ServerUrl, clientToServerPath)
		if err != nil {
			return err
		}
		serverToClientUrl, err := util.UrlJoin(cmd.ServerUrl, serverToClientPath)
		if err != nil {
			return err
		}
		var ln net.Listener
		if flag.clientHostUnixSocket == "" {
			ln, err = net.Listen("tcp", fmt.Sprintf(":%d", flag.clientHostPort))
		} else {
			ln, err = net.Listen("unix", flag.clientHostUnixSocket)
		}
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
		printHintForServerHost(ln, clientToServerUrl, serverToClientUrl, clientToServerPath, serverToClientPath, opensslAesCtrParams)
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
			return clientHandleWithYamux(ln, httpClient, headers, clientToServerUrl, serverToClientUrl)
		}
		// If pmux is enabled
		if flag.pmux {
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
		if flag.symmetricallyEncrypts {
			var duplex io.ReadWriteCloser
			duplex, err := piping_util.DuplexConnect(httpClient, headers, clientToServerUrl, serverToClientUrl)
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
		err = piping_util.HandleDuplex(httpClient, conn, headers, clientToServerUrl, serverToClientUrl, flag.serverToClientBufSize, nil, cmd.ShowProgress, cmd.MakeProgressMessage)
		fmt.Println()
		if err != nil {
			return err
		}
		fmt.Println("[INFO] Finished")

		return nil
	},
}

func printHintForServerHost(ln net.Listener, clientToServerUrl string, serverToClientUrl string, clientToServerPath string, serverToClientPath string, opensslAesCtrParams *cmd.OpensslAesCtrParams) {
	var listeningOn string
	if addr, ok := ln.Addr().(*net.TCPAddr); ok {
		// (base: https://stackoverflow.com/a/43425461)
		flag.clientHostPort = addr.Port
		listeningOn = strconv.Itoa(addr.Port)
	} else {
		listeningOn = flag.clientHostUnixSocket
	}
	fmt.Printf("[INFO] Client host listening on %s ...\n", listeningOn)
	if !flag.yamux && !flag.pmux {
		if flag.symmetricallyEncrypts {
			if opensslAesCtrParams != nil {
				fmt.Println("[INFO] Hint: Server host. <PORT> should be replaced (nc + curl + openssl)")
				fmt.Printf(
					"  read -p \"passphrase: \" -s pass && curl -sSN %s | stdbuf -i0 -o0 openssl aes-%d-ctr -d -pass \"pass:$pass\" -bufsize 1 -pbkdf2 -iter %d -md %s | nc 127.0.0.1 <YOUR PORT> | stdbuf -i0 -o0 openssl aes-%d-ctr -pass \"pass:$pass\" -bufsize 1 -pbkdf2 -iter %d -md %s | curl -sSNT - %s; unset pass\n",
					clientToServerUrl,
					opensslAesCtrParams.KeyBits,
					opensslAesCtrParams.Pbkdf2.Iter,
					opensslAesCtrParams.Pbkdf2.HashNameForCommandHint,
					opensslAesCtrParams.KeyBits,
					opensslAesCtrParams.Pbkdf2.Iter,
					opensslAesCtrParams.Pbkdf2.HashNameForCommandHint,
					serverToClientUrl,
				)
			}
		} else {
			fmt.Println("[INFO] Hint: Server host (nc + curl)")
			fmt.Printf("  curl -sSN %s | nc 127.0.0.1 <YOUR PORT> | curl -sSNT - %s\n", clientToServerUrl, serverToClientUrl)
		}
	}
	fmt.Println("[INFO] Hint: Server host (piping-tunnel)")
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
	fmt.Printf(
		"  piping-tunnel -s %s server -p <YOUR PORT> %s%s %s\n",
		cmd.ServerUrl,
		flags,
		clientToServerPath,
		serverToClientPath,
	)
	fmt.Println("    OR")
	fmt.Printf(
		"  piping-tunnel -s %s socks %s%s %s\n",
		cmd.ServerUrl,
		flags,
		clientToServerPath,
		serverToClientPath,
	)
}

func clientHandleWithYamux(ln net.Listener, httpClient *http.Client, headers []piping_util.KeyValue, clientToServerUrl string, serverToClientUrl string) error {
	var duplex io.ReadWriteCloser
	duplex, err := piping_util.DuplexConnectWithHandlers(
		func(body io.Reader) (*http.Response, error) {
			return piping_util.PipingSend(httpClient, cmd.HeadersWithYamux(headers), clientToServerUrl, body)
		},
		func() (*http.Response, error) {
			res, err := piping_util.PipingGet(httpClient, headers, serverToClientUrl)
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
	var config cmd.ClientPmuxConfigJson
	if json.Unmarshal([]byte(flag.pmuxConfig), &config) != nil {
		return errors.Errorf("invalid pmux config format")
	}
	pmuxClient, err := pmux.Client(httpClient, headers, clientToServerUrl, serverToClientUrl, config.Hb, flag.symmetricallyEncrypts, flag.symmetricallyEncryptPassphrase, flag.cipherType)
	if err != nil {
		if err == pmux.NonPmuxMimeTypeError {
			return errors.Errorf("--%s may be missing in server", cmd.PmuxFlagLongName)
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
			cmd.Vlog.Log(
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
				cmd.Vlog.Log(
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
				cmd.Vlog.Log(
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
