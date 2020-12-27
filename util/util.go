package util

import (
	"context"
	"crypto/tls"
	"fmt"
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

type CombinedError struct {
	e1 error
	e2 error
}

func (e CombinedError) Error() string {
	return fmt.Sprintf("%v and %v", e.e1, e.e2)
}

func CombineErrors(e1 error, e2 error) error {
	if e1 == nil {
		return e2
	}
	if e2 == nil {
		return e1
	}
	return &CombinedError{e1: e1, e2: e2}
}
