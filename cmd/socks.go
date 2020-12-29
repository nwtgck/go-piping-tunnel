package cmd

import (
	"fmt"
	"github.com/armon/go-socks5"
	"github.com/hashicorp/yamux"
	piping_tunnel_util "github.com/nwtgck/go-piping-tunnel/piping-tunnel-util"
	"github.com/nwtgck/go-piping-tunnel/util"
	"github.com/spf13/cobra"
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
			return fmt.Errorf("--yamux must be specified")
		}

		fmt.Println("[INFO] Multiplexing with hashicorp/yamux")
		socks5Conf := &socks5.Config{}
		socks5Server, err := socks5.New(socks5Conf)
		if err != nil {
			return err
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

func socksHandleWithYamux(socks5Server *socks5.Server, httpClient *http.Client, headers []piping_tunnel_util.KeyValue, clientToServerUrl string, serverToClientUrl string) error {
	duplex, err := makeDuplexWithEncryptionAndProgressIfNeed(httpClient, headers, serverToClientUrl, clientToServerUrl, socksSymmetricallyEncrypts, socksSymmetricallyEncryptPassphrase, socksCipherType)
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
		go socks5Server.ServeConn(yamuxStream)
	}
}
