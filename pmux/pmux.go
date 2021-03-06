// TODO: should send fin to notify finish
package pmux

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/nwtgck/go-piping-tunnel/early_piping_duplex"
	"github.com/nwtgck/go-piping-tunnel/heartbeat_duplex"
	"github.com/nwtgck/go-piping-tunnel/piping_util"
	"github.com/nwtgck/go-piping-tunnel/util"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"time"
)

type server struct {
	httpClient      *http.Client
	headers         []piping_util.KeyValue
	baseUploadUrl   string
	baseDownloadUrl string
}

type client struct {
	httpClient      *http.Client
	headers         []piping_util.KeyValue
	baseUploadUrl   string
	baseDownloadUrl string
}

type syncJson struct {
	SubPath string `json:"sub_path"`
}

const pmuxVersion uint32 = 1
const pmuxMimeType = "application/pmux"
const httpTimeout = 50 * time.Second

var pmuxVersionBytes [4]byte
var IncompatiblePmuxVersion = errors.Errorf("incompatible pmux version, expected %d", pmuxVersion)
var NonPmuxMimeTypeError = errors.Errorf("invalid content-type, expected %s", pmuxMimeType)

func init() {
	binary.BigEndian.PutUint32(pmuxVersionBytes[:], pmuxVersion)
}

func headersWithPmux(headers []piping_util.KeyValue) []piping_util.KeyValue {
	return append(headers, piping_util.KeyValue{Key: "Content-Type", Value: pmuxMimeType})
}

func Server(httpClient *http.Client, headers []piping_util.KeyValue, baseUploadUrl string, baseDownloadUrl string) *server {
	server := &server{
		httpClient:      httpClient,
		headers:         headers,
		baseUploadUrl:   baseUploadUrl,
		baseDownloadUrl: baseDownloadUrl,
	}
	go server.sendVersionLoop()
	return server
}

type getSubPathStatusError struct {
	statusCode int
}

func (e *getSubPathStatusError) Error() string {
	return fmt.Sprintf("not status 200, found: %d", e.statusCode)
}

func (s *server) sendVersionLoop() {
	backoff := NewExponentialBackoff()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
		defer cancel()
		postRes, err := piping_util.PipingSendWithContext(ctx, s.httpClient, headersWithPmux(s.headers), s.baseUploadUrl, bytes.NewReader(pmuxVersionBytes[:]))
		// If timeout
		if e, ok := err.(net.Error); ok && e.Timeout() {
			// reset backoff
			backoff.Reset()
			// No backoff
			continue
		}
		if err != nil {
			// backoff
			time.Sleep(backoff.NextDuration())
			continue
		}
		_, err = io.Copy(ioutil.Discard, postRes.Body)
		if err != nil {
			// backoff
			time.Sleep(backoff.NextDuration())
			continue
		}
	}
}

func (s *server) getSubPath() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()
	getRes, err := piping_util.PipingGetWithContext(ctx, s.httpClient, s.headers, s.baseDownloadUrl)
	if err != nil {
		return "", err
	}
	if getRes.StatusCode != 200 {
		return "", &getSubPathStatusError{statusCode: getRes.StatusCode}
	}
	resBytes, err := ioutil.ReadAll(getRes.Body)
	if err != nil {
		return "", err
	}
	var sync syncJson
	err = json.Unmarshal(resBytes, &sync)
	if err != nil {
		return "", err
	}
	return sync.SubPath, nil
}

func (s *server) Accept() (io.ReadWriteCloser, error) {
	backoff := NewExponentialBackoff()
	var subPath string
	for {
		var err error
		subPath, err = s.getSubPath()
		if err == nil {
			break
		}
		// If timeout
		if e, ok := err.(net.Error); ok && e.Timeout() {
			// reset backoff
			backoff.Reset()
			// No backoff
			continue
		}
		// backoff
		time.Sleep(backoff.NextDuration())
	}
	uploadUrl, err := util.UrlJoin(s.baseUploadUrl, subPath)
	if err != nil {
		return nil, err
	}
	downloadUrl, err := util.UrlJoin(s.baseDownloadUrl, subPath)
	if err != nil {
		return nil, err
	}
	duplex, err := early_piping_duplex.DuplexConnect(s.httpClient, s.headers, uploadUrl, downloadUrl)
	if err != nil {
		return nil, err
	}
	return heartbeat_duplex.Duplex(duplex), err
}

func Client(httpClient *http.Client, headers []piping_util.KeyValue, baseUploadUrl string, baseDownloadUrl string) (*client, error) {
	client := &client{
		httpClient:      httpClient,
		headers:         headers,
		baseUploadUrl:   baseUploadUrl,
		baseDownloadUrl: baseDownloadUrl,
	}
	return client, client.checkServerVersion()
}

func (c *client) checkServerVersion() error {
	backoff := NewExponentialBackoff()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
		defer cancel()
		postRes, err := piping_util.PipingGetWithContext(ctx, c.httpClient, c.headers, c.baseDownloadUrl)
		// If timeout
		if e, ok := err.(net.Error); ok && e.Timeout() {
			// reset backoff
			backoff.Reset()
			// No backoff
			continue
		}
		if err != nil {
			// backoff
			time.Sleep(backoff.NextDuration())
			continue
		}
		if postRes.Header.Get("Content-Type") != pmuxMimeType {
			return NonPmuxMimeTypeError
		}
		versionBytes := make([]byte, 4)
		_, err = io.ReadFull(postRes.Body, versionBytes)
		if err != nil {
			// backoff
			time.Sleep(backoff.NextDuration())
			continue
		}
		serverVersion := binary.BigEndian.Uint32(versionBytes)
		if serverVersion != pmuxVersion {
			return IncompatiblePmuxVersion
		}
		return nil
	}
}

func (c *client) sendSubPath() (string, error) {
	subPath, err := util.RandomHexString()
	if err != nil {
		return "", err
	}
	sync := syncJson{SubPath: subPath}
	jsonBytes, err := json.Marshal(sync)
	if err != nil {
		return "", err
	}
	res, err := piping_util.PipingSend(c.httpClient, c.headers, c.baseUploadUrl, bytes.NewReader(jsonBytes))
	if err != nil {
		return "", err
	}
	if res.StatusCode != 200 {
		return "", errors.Errorf("not status 200, found: %d", res.StatusCode)
	}
	_, err = io.Copy(ioutil.Discard, res.Body)
	return subPath, err
}

func (c *client) Open() (io.ReadWriteCloser, error) {
	backoff := NewExponentialBackoff()
	var subPath string
	for {
		var err error
		subPath, err = c.sendSubPath()
		if err == nil {
			break
		}
		// If timeout
		if e, ok := err.(net.Error); ok && e.Timeout() {
			backoff.Reset()
			continue
		}
		fmt.Fprintln(os.Stderr, "get sync error", err)
		time.Sleep(backoff.NextDuration())
	}
	uploadUrl, err := util.UrlJoin(c.baseUploadUrl, subPath)
	if err != nil {
		return nil, err
	}
	downloadUrl, err := util.UrlJoin(c.baseDownloadUrl, subPath)
	if err != nil {
		return nil, err
	}
	duplex, err := early_piping_duplex.DuplexConnect(c.httpClient, c.headers, uploadUrl, downloadUrl)
	if err != nil {
		return nil, err
	}
	return heartbeat_duplex.Duplex(duplex), err
}
