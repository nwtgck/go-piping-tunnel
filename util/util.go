package util

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/nwtgck/go-piping-tunnel/io_progress"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"path"
	"time"
)

// (base: https://stackoverflow.com/a/34668130/2885946)
func UrlJoin(s string, p string) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	u.Path = path.Join(u.Path, p)
	return u.String(), nil
}

// Generate HTTP client
func CreateHttpClient(insecure bool, writeBufSize int, readBufSize int) *http.Client {
	// Set insecure or not
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: insecure},
		WriteBufferSize: writeBufSize,
		ReadBufferSize:  readBufSize,
	}
	return &http.Client{Transport: tr}
}

// Set default resolver for HTTP client
func CreateDialContext(dnsServer string) func(ctx context.Context, network, address string) (net.Conn, error) {
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

// (base: https://github.com/schollz/progressbar/blob/9c6973820b2153b15d2e6a08d8705ec981fda59f/progressbar.go#L784-L799)
func HumanizeBytes(s float64) string {
	sizes := []string{" B", " kB", " MB", " GB", " TB", " PB", " EB"}
	base := 1024.0
	if s < 10 {
		return fmt.Sprintf("%2.0fB", s)
	}
	e := math.Floor(logn(s, base))
	suffix := sizes[int(e)]
	val := math.Floor(s/math.Pow(base, e)*10+0.5) / 10
	f := "%.0f"
	if val < 10 {
		f = "%.1f"
	}

	return fmt.Sprintf(f, val) + suffix
}

// (from: https://github.com/schollz/progressbar/blob/9c6973820b2153b15d2e6a08d8705ec981fda59f/progressbar.go#L784-L799)
func logn(n, b float64) float64 {
	return math.Log(n) / math.Log(b)
}

type IOProgressReadWriteCloser struct {
	pw         *io.PipeWriter
	progress   *io_progress.IOProgress
	baseDuplex io.ReadWriteCloser
}

func NewIOProgressReadWriteCloser(duplex io.ReadWriteCloser, messageWriter io.Writer, makeMessage func(progress *io_progress.IOProgress) string) IOProgressReadWriteCloser {
	pr, pw := io.Pipe()
	p := io_progress.NewIOProgress(pr, messageWriter, makeMessage)
	go func() {
		// TODO: hard code
		var buf = make([]byte, 16)
		io.CopyBuffer(duplex, &p, buf)
	}()
	return IOProgressReadWriteCloser{
		pw:         pw,
		progress:   &p,
		baseDuplex: duplex,
	}
}

func (p IOProgressReadWriteCloser) Read(b []byte) (n int, err error) {
	n, err = p.baseDuplex.Read(b)
	if err != nil {
		return n, err
	}
	_, err = p.progress.Write(b[:n])
	if err != nil {
		return n, err
	}
	return n, nil
}

func (p IOProgressReadWriteCloser) Write(b []byte) (n int, err error) {
	return p.pw.Write(b)
}

func (p IOProgressReadWriteCloser) Close() error {
	return p.baseDuplex.Close()
}
