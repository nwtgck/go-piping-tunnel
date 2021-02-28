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

type syncPacket struct {
	SubPath string `json:"sub_path"`
}

const pmuxVersion uint32 = 1
const pmuxMimeType = "application/pmux"
const syncSubPath = "sync"
const getTimeout = 50 * time.Second

var IncompatiblePmuxVersion = errors.Errorf("incompatible pmux version, expected %d", pmuxVersion)
var NonPmuxMimeTypeError = errors.Errorf("invalid content-type, expected %s", pmuxMimeType)

func headersWithPmux(headers []piping_util.KeyValue) []piping_util.KeyValue {
	return append(headers, piping_util.KeyValue{Key: "Content-Type", Value: pmuxMimeType})
}

func sendInitial(httpClient *http.Client, headers []piping_util.KeyValue, uploadUrl string) {
	backoff := NewExponentialBackoff()
	for {
		versionBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(versionBytes, pmuxVersion)
		res, err := piping_util.PipingSend(httpClient, headersWithPmux(headers), uploadUrl, bytes.NewReader(versionBytes))
		if err != nil {
			time.Sleep(backoff.NextDuration())
			continue
		}
		if res.StatusCode != 200 {
			time.Sleep(backoff.NextDuration())
			continue
		}
		_, err = io.Copy(ioutil.Discard, res.Body)
		if err != nil {
			continue
		}
		break
	}
}

func getInitial(httpClient *http.Client, headers []piping_util.KeyValue, downloadUrl string) error {
	backoff := NewExponentialBackoff()
	for {
		ctx, cancel := context.WithTimeout(context.Background(), getTimeout)
		defer cancel()
		getRes, err := piping_util.PipingGetWithContext(ctx, httpClient, headers, downloadUrl)
		if err != nil {
			// If timeout
			if e, ok := err.(net.Error); ok && e.Timeout() {
				backoff.Reset()
				continue
			}
			// backoff
			time.Sleep(backoff.NextDuration())
			continue
		}
		if getRes.StatusCode != 200 {
			time.Sleep(backoff.NextDuration())
			continue
		}
		contentType := getRes.Header.Get("Content-Type")
		if contentType != pmuxMimeType {
			return NonPmuxMimeTypeError
		}
		resBytes, err := ioutil.ReadAll(getRes.Body)
		if err != nil {
			continue
		}
		version := binary.BigEndian.Uint32(resBytes[0:4])
		if version != pmuxVersion {
			return IncompatiblePmuxVersion
		}
		return nil
	}
}

func Server(httpClient *http.Client, headers []piping_util.KeyValue, baseUploadUrl string, baseDownloadUrl string) (*server, error) {
	server := &server{
		httpClient:      httpClient,
		headers:         headers,
		baseUploadUrl:   baseUploadUrl,
		baseDownloadUrl: baseDownloadUrl,
	}
	err := server.initialConnect()
	return server, err
}

func (s *server) initialConnect() error {
	err := getInitial(s.httpClient, s.headers, s.baseDownloadUrl)
	sendInitial(s.httpClient, s.headers, s.baseUploadUrl)
	return err
}

type getSubPathStatusError struct {
	statusCode int
}

func (e *getSubPathStatusError) Error() string {
	return fmt.Sprintf("not status 200, found: %d", e.statusCode)
}

func (s *server) getSubPath() (string, error) {
	downloadUrl, err := util.UrlJoin(s.baseDownloadUrl, syncSubPath)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), getTimeout)
	defer cancel()
	getRes, err := piping_util.PipingGetWithContext(ctx, s.httpClient, s.headers, downloadUrl)
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
	var packet syncPacket
	err = json.Unmarshal(resBytes, &packet)
	if err != nil {
		return "", err
	}
	return packet.SubPath, nil
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
		// Skip error logging if status error
		if _, ok := err.(*getSubPathStatusError); !ok {
			fmt.Printf("get sync error: %+v\n", errors.WithStack(err))
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
	err := client.initialConnect()
	return client, err
}

func (c *client) initialConnect() error {
	sendInitial(c.httpClient, c.headers, c.baseUploadUrl)
	return getInitial(c.httpClient, c.headers, c.baseDownloadUrl)
}

func (c *client) sendSubPath() (string, error) {
	subPath := strings.Replace(uuid.New().String(), "-", "", 4)
	packet := syncPacket{SubPath: subPath}
	jsonBytes, err := json.Marshal(packet)
	if err != nil {
		return "", err
	}
	uploadUrl, err := util.UrlJoin(c.baseUploadUrl, syncSubPath)
	if err != nil {
		return "", err
	}
	res, err := piping_util.PipingSend(c.httpClient, c.headers, uploadUrl, bytes.NewReader(jsonBytes))
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
		fmt.Fprintln(os.Stderr, "send sync error", err)
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
