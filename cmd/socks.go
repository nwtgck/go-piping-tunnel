package cmd

import (
	"fmt"
	"github.com/cybozu-go/usocksd"
	"github.com/cybozu-go/well"
	"github.com/hashicorp/yamux"
	piping_tunnel_util "github.com/nwtgck/go-piping-tunnel/piping-tunnel-util"
	"github.com/nwtgck/go-piping-tunnel/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"io/ioutil"
	"net/http"
	"strings"
)

var socksYamux bool
var socksSymmetricallyEncrypts bool
var socksSymmetricallyEncryptPassphrase string
var socksCipherType string

func init() {
	RootCmd.AddCommand(socksCmd)
	socksCmd.Flags().BoolVarP(&socksYamux, "yamux", "", false, "Multiplex connection by hashicorp/yamux")
	socksCmd.Flags().BoolVarP(&socksSymmetricallyEncrypts, symmetricallyEncryptsFlagLongName, symmetricallyEncryptsFlagShortName, false, "Encrypt symmetrically")
	socksCmd.Flags().StringVarP(&socksSymmetricallyEncryptPassphrase, symmetricallyEncryptPassphraseFlagLongName, "", "", "Passphrase for encryption")
	socksCmd.Flags().StringVarP(&socksCipherType, cipherTypeFlagLongName, "", defaultCipherType, fmt.Sprintf("Cipher type: %s, %s", cipherTypeAesCtr, cipherTypeOpenpgp))
}

var socksCmd = &cobra.Command{
	Use:   "socks",
	Short: "Run SOCKS5 server",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate cipher-type
		if socksSymmetricallyEncrypts {
			if err := validateClientCipher(socksCipherType); err != nil {
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
		socksPrintHintForClientHost(clientToServerUrl, serverToClientUrl, clientToServerPath, serverToClientPath)
		// Make user input passphrase if it is empty
		if socksSymmetricallyEncrypts {
			err = makeUserInputPassphraseIfEmpty(&socksSymmetricallyEncryptPassphrase)
			if err != nil {
				return err
			}
		}
		// If not use multiplexer with yamux
		if !socksYamux {
			return errors.Errorf("--%s must be specified", yamuxFlagLongName)
		}

		fmt.Println("[INFO] Multiplexing with hashicorp/yamux")
		return socksHandleWithYamux(httpClient, headers, clientToServerUrl, serverToClientUrl)
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
	if socksSymmetricallyEncrypts {
		flags += fmt.Sprintf("-%s ", symmetricallyEncryptsFlagShortName)
		if socksCipherType != defaultCipherType {
			flags += fmt.Sprintf("--%s=%s ", cipherTypeFlagLongName, socksCipherType)
		}
	}
	if socksYamux {
		flags += fmt.Sprintf("--%s ", yamuxFlagLongName)
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

func socksHandleWithYamux(httpClient *http.Client, headers []piping_tunnel_util.KeyValue, clientToServerUrl string, serverToClientUrl string) error {
	duplex, err := makeDuplexWithEncryptionAndProgressIfNeed(httpClient, headers, serverToClientUrl, clientToServerUrl, socksSymmetricallyEncrypts, socksSymmetricallyEncryptPassphrase, socksCipherType)
	if err != nil {
		return err
	}

	yamuxSession, err := yamux.Server(duplex, nil)
	socksServer := usocksd.NewServer(usocksd.NewConfig())
	socksServer.Serve(yamuxSession)
	// NOTE: discard log output
	socksServer.Logger.SetOutput(ioutil.Discard)
	return well.Wait()
}
