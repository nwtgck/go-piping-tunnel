package socks

import (
	"encoding/json"
	"fmt"
	"github.com/hashicorp/yamux"
	"github.com/nwtgck/go-piping-tunnel/cmd"
	"github.com/nwtgck/go-piping-tunnel/piping_util"
	"github.com/nwtgck/go-piping-tunnel/pmux"
	"github.com/nwtgck/go-piping-tunnel/util"
	"github.com/nwtgck/go-socks"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"strings"
)

var socksYamux bool
var socksPmux bool
var socksPmuxConfig string
var socksSymmetricallyEncrypts bool
var socksSymmetricallyEncryptPassphrase string
var socksCipherType string
var socksPbkdf2JsonString string

func init() {
	cmd.RootCmd.AddCommand(socksCmd)
	socksCmd.Flags().BoolVarP(&socksYamux, "yamux", "", false, "Multiplex connection by hashicorp/yamux")
	socksCmd.Flags().BoolVarP(&socksPmux, cmd.PmuxFlagLongName, "", false, "Multiplex connection by pmux (experimental)")
	socksCmd.Flags().StringVarP(&socksPmuxConfig, cmd.PmuxConfigFlagLongName, "", `{"hb": true}`, "pmux config in JSON (experimental)")
	socksCmd.Flags().BoolVarP(&socksSymmetricallyEncrypts, cmd.SymmetricallyEncryptsFlagLongName, cmd.SymmetricallyEncryptsFlagShortName, false, "Encrypt symmetrically")
	socksCmd.Flags().StringVarP(&socksSymmetricallyEncryptPassphrase, cmd.SymmetricallyEncryptPassphraseFlagLongName, "", "", "Passphrase for encryption")
	socksCmd.Flags().StringVarP(&socksCipherType, cmd.CipherTypeFlagLongName, "", cmd.DefaultCipherType, fmt.Sprintf("Cipher type: %s, %s, %s, %s ", piping_util.CipherTypeAesCtr, piping_util.CipherTypeOpensslAes128Ctr, piping_util.CipherTypeOpensslAes256Ctr, piping_util.CipherTypeOpenpgp))
	socksCmd.Flags().StringVarP(&socksPbkdf2JsonString, cmd.Pbkdf2FlagLongName, "", "", fmt.Sprintf("e.g. %s", cmd.ExamplePbkdf2JsonStr()))
}

var socksCmd = &cobra.Command{
	Use:   "socks",
	Short: "Run SOCKS server",
	RunE: func(_ *cobra.Command, args []string) error {
		// Validate cipher-type
		if socksSymmetricallyEncrypts {
			if err := cmd.ValidateClientCipher(socksCipherType); err != nil {
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
		// Print hint
		socksPrintHintForClientHost(clientToServerUrl, serverToClientUrl, clientToServerPath, serverToClientPath)
		// Make user input passphrase if it is empty
		if socksSymmetricallyEncrypts {
			err = cmd.MakeUserInputPassphraseIfEmpty(&socksSymmetricallyEncryptPassphrase)
			if err != nil {
				return err
			}
		}

		// If not using multiplexer
		if !socksYamux && !socksPmux {
			return errors.Errorf("--%s or --%s must be specified", cmd.YamuxFlagLongName, cmd.PmuxFlagLongName)
		}

		socksConf := &socks.Config{}
		socksServer, err := socks.New(socksConf)

		// If yamux is enabled
		if socksYamux {
			fmt.Println("[INFO] Multiplexing with hashicorp/yamux")
			return socksHandleWithYamux(socksServer, httpClient, headers, clientToServerUrl, serverToClientUrl)
		}

		// If pmux is enabled
		fmt.Println("[INFO] Multiplexing with pmux")
		return socksHandleWithPmux(socksServer, httpClient, headers, clientToServerUrl, serverToClientUrl)
	},
}

func socksPrintHintForClientHost(clientToServerUrl string, serverToClientUrl string, clientToServerPath string, serverToClientPath string) {
	if !socksYamux && !socksPmux {
		fmt.Println("[INFO] Hint: Client host (socat + curl)")
		fmt.Printf(
			"  socat TCP-LISTEN:31376 'EXEC:curl -NsS %s!!EXEC:curl -NsST - %s'\n",
			strings.Replace(serverToClientUrl, ":", "\\:", -1),
			strings.Replace(clientToServerUrl, ":", "\\:", -1),
		)
	}
	flags := ""
	if socksSymmetricallyEncrypts {
		flags += fmt.Sprintf("-%s ", cmd.SymmetricallyEncryptsFlagShortName)
		if socksCipherType != cmd.DefaultCipherType {
			flags += fmt.Sprintf("--%s=%s ", cmd.CipherTypeFlagLongName, socksCipherType)
		}
	}
	if socksYamux {
		flags += fmt.Sprintf("--%s ", cmd.YamuxFlagLongName)
	}
	if socksPmux {
		flags += fmt.Sprintf("--%s ", cmd.PmuxFlagLongName)
	}
	fmt.Println("[INFO] Hint: Client host (piping-tunnel)")
	fmt.Printf(
		"  piping-tunnel -s %s client -p 1080 %s%s %s\n",
		cmd.ServerUrl,
		flags,
		clientToServerPath,
		serverToClientPath,
	)
}

func socksHandleWithYamux(socksServer *socks.Server, httpClient *http.Client, headers []piping_util.KeyValue, clientToServerUrl string, serverToClientUrl string) error {
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
	duplex, err = cmd.MakeDuplexWithEncryptionAndProgressIfNeed(duplex, socksSymmetricallyEncrypts, socksSymmetricallyEncryptPassphrase, socksCipherType, socksPbkdf2JsonString)
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
		go socksServer.ServeConn(yamuxStream)
	}
}

func socksHandleWithPmux(socksServer *socks.Server, httpClient *http.Client, headers []piping_util.KeyValue, clientToServerUrl string, serverToClientUrl string) error {
	var config cmd.ServerPmuxConfigJson
	if json.Unmarshal([]byte(socksPmuxConfig), &config) != nil {
		return errors.Errorf("invalid pmux config format")
	}
	pmuxServer := pmux.Server(httpClient, headers, serverToClientUrl, clientToServerUrl, config.Hb, socksSymmetricallyEncrypts, socksSymmetricallyEncryptPassphrase, socksCipherType)
	for {
		stream, err := pmuxServer.Accept()
		if err != nil {
			return err
		}
		go func() {
			err := socksServer.ServeConn(util.NewDuplexConn(stream))
			if err != nil {
				cmd.Vlog.Log(
					fmt.Sprintf("error(serve conn): %v", errors.WithStack(err)),
					fmt.Sprintf("error(serve conn): %+v", errors.WithStack(err)),
				)
			}
		}()
	}
}
