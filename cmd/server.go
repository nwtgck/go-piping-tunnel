package cmd

import (
	"fmt"
	"github.com/nwtgck/go-piping-tunnel/util"
	"github.com/spf13/cobra"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
)

var serverHostPort int

func init() {
	serverCmd.Flags().IntVarP(&serverHostPort, "port", "p", 0, "TCP port of server host")
	serverCmd.MarkFlagRequired("port")
}

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Run server-host",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return fmt.Errorf("Path 1 and path 2 are required\n")
		}

		path1 := args[0]
		path2 := args[1]

		conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", serverHostPort))
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		httpClient := util.CreateHttpClient(insecure)
		if dnsServer != "" {
			// Set DNS resolver
			httpClient.Transport.(*http.Transport).DialContext = util.CreateDialContext(dnsServer)
		}

		url2, err := urlJoin(serverUrl, path2)
		if err != nil {
			panic(err)
		}
		_, err = httpClient.Post(url2, "application/octet-stream", conn)
		if err != nil {
			panic(err)
		}

		url1, err := urlJoin(serverUrl, path1)
		if err != nil {
			panic(err)
		}
		fmt.Println("==== Client host (socat + curl) ====")
		fmt.Printf(
			"socat TCP-LISTEN:31376 'EXEC:curl -NsS %s!!EXEC:curl -NsST - %s'\n",
			strings.Replace(url2, ":", "\\:", -1),
			strings.Replace(url1, ":", "\\:", -1),
		)

		res, err := httpClient.Get(url1)
		if err != nil {
			panic(err)
		}
		_, err = io.Copy(conn, res.Body)
		if err != nil {
			panic(err)
		}
		fmt.Println("Finished")

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
