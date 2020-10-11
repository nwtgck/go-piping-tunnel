package cmd

import (
	"fmt"
	"github.com/nwtgck/go-piping-tunnel/io_progress"
	"github.com/nwtgck/go-piping-tunnel/util"
	"github.com/spf13/cobra"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
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
		var progress *io_progress.IOProgress = nil
		if showProgress {
			p := io_progress.NewIOProgress(conn, os.Stderr, func(progress *io_progress.IOProgress) string {
				return fmt.Sprintf(
					"↑ %s (%s/s) | ↓ %s (%s/s)",
					util.HumanizeBytes(float64(progress.CurrReadBytes)),
					util.HumanizeBytes(float64(progress.CurrReadBytes)/time.Since(progress.StartTime).Seconds()),
					util.HumanizeBytes(float64(progress.CurrWriteBytes)),
					util.HumanizeBytes(float64(progress.CurrWriteBytes)/time.Since(progress.StartTime).Seconds()),
				)
			})
			progress = &p
		}

		url2, err := util.UrlJoin(serverUrl, path2)
		if err != nil {
			panic(err)
		}
		var reader io.Reader = conn
		if progress != nil {
			reader = progress
		}
		_, err = httpClient.Post(url2, "application/octet-stream", reader)
		if err != nil {
			panic(err)
		}

		url1, err := util.UrlJoin(serverUrl, path1)
		if err != nil {
			panic(err)
		}
		fmt.Println("==== Client host (socat + curl) ====")
		fmt.Printf(
			"socat TCP-LISTEN:31376 'EXEC:curl -NsS %s!!EXEC:curl -NsST - %s'\n",
			strings.Replace(url2, ":", "\\:", -1),
			strings.Replace(url1, ":", "\\:", -1),
		)
		fmt.Println()
		fmt.Println("==== Client host (piping-tunnel) ====")
		fmt.Printf(
			"piping-tunnel -s %s client -p 31376 %s %s\n",
			serverUrl,
			path1,
			path2,
		)

		res, err := httpClient.Get(url1)
		if err != nil {
			panic(err)
		}
		var writer io.Writer = conn
		if progress != nil {
			writer = io.MultiWriter(conn, progress)
		}
		_, err = io.Copy(writer, res.Body)
		if err != nil {
			panic(err)
		}
		fmt.Println("Finished")

		return nil
	},
}
