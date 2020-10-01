package cmd

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/nwtgck/go-piping-tunnel/version"
	"github.com/spf13/cobra"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"time"
)

const (
	ServerUrlEnvName = "PIPING_SERVER_URL"
)

var serverUrl string
var tcpPort int
var insecure bool
var dnsServer string
var showsVersion bool

func init() {
	cobra.OnInitialize()
	defaultServer, ok := os.LookupEnv(ServerUrlEnvName)
	if !ok {
		defaultServer = "https://ppng.io"
	}
	RootCmd.Flags().StringVarP(&serverUrl,  "server",  "s", defaultServer, "Piping Server URL")
	RootCmd.Flags().IntVarP(&tcpPort,  "port",  "p", 0, "TCP port of server host")
	RootCmd.MarkFlagRequired("port")
	// NOTE: --insecure, -k is inspired by curl
	RootCmd.Flags().BoolVarP(&insecure, "insecure", "k", false, "Allow insecure server connections when using SSL")
	RootCmd.Flags().StringVar(&dnsServer, "dns-server", "", "DNS server (e.g. 1.1.1.1:53)")
	RootCmd.Flags().BoolVarP(&showsVersion, "version", "v", false, "show version")
}

var RootCmd = &cobra.Command{
	Use:   os.Args[0],
	Short: "piping-tunnel",
	Long:  "Tunnel over Piping Server",
	RunE: func(cmd *cobra.Command, args []string) error {
		if showsVersion {
			fmt.Println(version.Version)
			return nil
		}
		if len(args) != 2 {
			return fmt.Errorf("Path 1 and path 2 are required\n")
		}

		path1 := args[0]
		path2 := args[1]

		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", tcpPort))
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		httpClient := getHttpClient(insecure)
		if dnsServer != "" {
			// Set DNS resolver
			httpClient.Transport.(*http.Transport).DialContext = dialContext(dnsServer)
		}

		url2, err := urlJoin(serverUrl, path2)
		if err != nil {
			panic(err)
		}
		_, err = httpClient.Post(url2, "application/octet-stream", conn)
		if err != nil {
			panic(err)
		}
		fmt.Println("after POST")

		url1, err := urlJoin(serverUrl, path1)
		if err != nil {
			panic(err)
		}
		res, err := httpClient.Get(url1)
		if err != nil {
			panic(err)
		}
		_, err = io.Copy(conn, res.Body)
		if err != nil {
			panic(err)
		}
		fmt.Println("after GET")

		return nil
	},
}

// (base: https://stackoverflow.com/a/34668130/2885946)
func urlJoin(s string, p string) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	u.Path = path.Join(u.Path, p)
	return u.String(), nil
}

// Generate HTTP client
func getHttpClient(insecure bool) *http.Client {
	// Set insecure or not
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{ InsecureSkipVerify: insecure },
	}
	return &http.Client{Transport: tr}
}


// Set default resolver for HTTP client
func dialContext(dnsServer string) func(ctx context.Context, network, address string) (net.Conn, error) {
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, "udp", dnsServer)
		},
	}

	// Resolver for HTTP
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{
			Timeout:  time.Millisecond * time.Duration(10000),
			Resolver: resolver,
		}
		return d.DialContext(ctx, network, address)
	}
}
