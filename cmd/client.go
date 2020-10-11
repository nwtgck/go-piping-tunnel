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

var clientHostPort int

func init() {
	RootCmd.AddCommand(clientCmd)
	clientCmd.Flags().IntVarP(&clientHostPort, "port", "p", 0, "TCP port of client host")
}

var clientCmd = &cobra.Command{
	Use:   "client",
	Short: "Run client-host",
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return fmt.Errorf("Path 1 and path 2 are required\n")
		}
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", clientHostPort))
		if err != nil {
			return err
		}

		path1 := args[0]
		path2 := args[1]
		url1, err := util.UrlJoin(serverUrl, path1)
		if err != nil {
			return err
		}
		url2, err := util.UrlJoin(serverUrl, path2)
		if err != nil {
			return err
		}
		// (from: https://stackoverflow.com/a/43425461)
		clientHostPort = ln.Addr().(*net.TCPAddr).Port
		fmt.Printf("Client host listening on %d ...\n", clientHostPort)
		fmt.Println("==== Server host (socat + curl) ====")
		fmt.Printf(
			"socat 'EXEC:curl -NsS %s!!EXEC:curl -NsST - %s' TCP:127.0.0.1:<YOUR PORT>\n",
			strings.Replace(url1, ":", "\\:", -1),
			strings.Replace(url2, ":", "\\:", -1),
		)
		fmt.Println()
		fmt.Println("==== Server host (piping-tunnel) ====")
		fmt.Printf(
			"piping-tunnel -s %s server -p <YOUR PORT> %s %s\n",
			serverUrl,
			path1,
			path2,
		)
		conn, err := ln.Accept()
		if err != nil {
			return err
		}
		// Refuse another new connection
		ln.Close()
		httpClient := util.CreateHttpClient(insecure)
		if dnsServer != "" {
			// Set DNS resolver
			httpClient.Transport.(*http.Transport).DialContext = util.CreateDialContext(dnsServer)
		}
		progress := io_progress.NewIOProgress(conn, os.Stdout, func(progress *io_progress.IOProgress) string {
			return fmt.Sprintf(
				"↑ %s (%s/s) | ↓ %s (%s/s)",
				util.HumanizeBytes(float64(progress.CurrReadBytes)),
				util.HumanizeBytes(float64(progress.CurrReadBytes)/time.Since(progress.StartTime).Seconds()),
				util.HumanizeBytes(float64(progress.CurrWriteBytes)),
				util.HumanizeBytes(float64(progress.CurrWriteBytes)/time.Since(progress.StartTime).Seconds()),
			)
		})
		_, err = httpClient.Post(url1, "application/octet-stream", &progress)
		if err != nil {
			return err
		}
		res, err := httpClient.Get(url2)
		if err != nil {
			return err
		}
		_, err = io.Copy(io.MultiWriter(conn, &progress), res.Body)
		if err != nil {
			return err
		}
		fmt.Println("Finished")

		return nil
	},
}
