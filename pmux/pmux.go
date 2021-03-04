// TODO: should send fin to notify finish
package pmux

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
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
	"strings"
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
const syncSubPath = ""
const httpTimeout = 50 * time.Second

var IncompatiblePmuxVersion = errors.Errorf("incompatible pmux version, expected %d", pmuxVersion)
var NonPmuxMimeTypeError = errors.Errorf("invalid content-type, expected %s", pmuxMimeType)

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
	return server
}

type getSubPathStatusError struct {
	statusCode int
}

func (e *getSubPathStatusError) Error() string {
	return fmt.Sprintf("not status 200, found: %d", e.statusCode)
}

func (s *server) sendSubPath() (string, error) {
	uploadUrl, err := util.UrlJoin(s.baseUploadUrl, syncSubPath)
	if err != nil {
		return "", err
	}
	subPath := strings.Replace(uuid.New().String(), "-", "", 4)
	sync := syncJson{SubPath: subPath}
	jsonBytes, err := json.Marshal(sync)
	if err != nil {
		return "", err
	}
	versionBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(versionBytes, pmuxVersion)
	ctx, cancel := context.WithTimeout(context.Background(), httpTimeout)
	defer cancel()
	postRes, err := piping_util.PipingSendWithContext(ctx, s.httpClient, headersWithPmux(s.headers), uploadUrl, bytes.NewReader(append(versionBytes, jsonBytes...)))
	if err != nil {
		return "", err
	}
	_, err = io.Copy(ioutil.Discard, postRes.Body)
	if err != nil {
		return "", err
	}
	return subPath, nil
}

func (s *server) Accept() (io.ReadWriteCloser, error) {
	backoff := NewExponentialBackoff()
	var subPath string
	for {
		var err error
		subPath, err = s.sendSubPath()
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

func Client(httpClient *http.Client, headers []piping_util.KeyValue, baseUploadUrl string, baseDownloadUrl string) *client {
	client := &client{
		httpClient:      httpClient,
		headers:         headers,
		baseUploadUrl:   baseUploadUrl,
		baseDownloadUrl: baseDownloadUrl,
	}
	return client
}

func (c *client) getSubPath() (string, error) {
	downloadUrl, err := util.UrlJoin(c.baseDownloadUrl, syncSubPath)
	if err != nil {
		return "", err
	}
	res, err := piping_util.PipingGet(c.httpClient, c.headers, downloadUrl)
	if err != nil {
		return "", err
	}
	if res.StatusCode != 200 {
		return "", &getSubPathStatusError{statusCode: res.StatusCode}
	}
	if res.Header.Get("Content-Type") != pmuxMimeType {
		return "", NonPmuxMimeTypeError
	}
	resBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	versionBytes := resBytes[:4]
	serverVersion := binary.BigEndian.Uint32(versionBytes)
	if serverVersion != pmuxVersion {
		return "", IncompatiblePmuxVersion
	}
	var sync syncJson
	err = json.Unmarshal(resBytes[4:], &sync)
	if err != nil {
		return "", err
	}
	return sync.SubPath, nil
}

func (c *client) Open() (io.ReadWriteCloser, error) {
	backoff := NewExponentialBackoff()
	var subPath string
	for {
		var err error
		subPath, err = c.getSubPath()
		if err == nil {
			break
		}
		// If timeout
		if e, ok := err.(net.Error); ok && e.Timeout() {
			backoff.Reset()
			continue
		}
		// If fatal error
		if err == NonPmuxMimeTypeError || err == IncompatiblePmuxVersion {
			return nil, err
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
