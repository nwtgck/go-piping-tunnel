// TODO: should send fin to notify finish
package pmux

import (
	"bytes"
	"encoding/json"
	"github.com/google/uuid"
	piping_tunnel_util "github.com/nwtgck/go-piping-tunnel/piping-tunnel-util"
	"github.com/nwtgck/go-piping-tunnel/util"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

type server struct {
	httpClient      *http.Client
	headers         []piping_tunnel_util.KeyValue
	baseUploadUrl   string
	baseDownloadUrl string
}

type client struct {
	httpClient      *http.Client
	headers         []piping_tunnel_util.KeyValue
	baseUploadUrl   string
	baseDownloadUrl string
}

type syncPacket struct {
	SubPath string `json:"sub_path"`
}

const syncPath = "sync"

func Server(httpClient *http.Client, headers []piping_tunnel_util.KeyValue, baseUploadUrl string, baseDownloadUrl string) *server {
	return &server{
		httpClient:      httpClient,
		headers:         headers,
		baseUploadUrl:   baseUploadUrl,
		baseDownloadUrl: baseDownloadUrl,
	}
}

func (s *server) getSubPath() (string, error) {
	downloadUrl, err := util.UrlJoin(s.baseDownloadUrl, syncPath)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("GET", downloadUrl, nil)
	if err != nil {
		return "", err
	}
	for _, kv := range s.headers {
		req.Header.Set(kv.Key, kv.Value)
	}
	getRes, err := s.httpClient.Do(req)
	if err != nil {
		return "", err
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
	var subPath string
	for {
		var err error
		subPath, err = s.getSubPath()
		if err == nil {
			break
		}
	}
	uploadUrl, err := util.UrlJoin(s.baseUploadUrl, subPath)
	if err != nil {
		return nil, err
	}
	downloadUrl, err := util.UrlJoin(s.baseDownloadUrl, subPath)
	if err != nil {
		return nil, err
	}
	duplex, err := piping_tunnel_util.DuplexConnect(s.httpClient, s.headers, uploadUrl, downloadUrl)
	if err != nil {
		return nil, err
	}
	return duplex, err
}

func Client(httpClient *http.Client, headers []piping_tunnel_util.KeyValue, baseUploadUrl string, baseDownloadUrl string) *client {
	return &client{
		httpClient:      httpClient,
		headers:         headers,
		baseUploadUrl:   baseUploadUrl,
		baseDownloadUrl: baseDownloadUrl,
	}
}

func (c *client) sendSubPath() (string, error) {
	subPath := strings.Replace(uuid.New().String(), "-", "", 4)
	packet := syncPacket{SubPath: subPath}
	jsonBytes, err := json.Marshal(packet)
	if err != nil {
		return "", err
	}
	uploadUrl, err := util.UrlJoin(c.baseUploadUrl, syncPath)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", uploadUrl, bytes.NewReader(jsonBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	for _, kv := range c.headers {
		req.Header.Set(kv.Key, kv.Value)
	}
	res, err := c.httpClient.Do(req)
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
	var subPath string
	for {
		var err error
		subPath, err = c.sendSubPath()
		if err == nil {
			break
		}
	}
	uploadUrl, err := util.UrlJoin(c.baseUploadUrl, subPath)
	if err != nil {
		return nil, err
	}
	downloadUrl, err := util.UrlJoin(c.baseDownloadUrl, subPath)
	if err != nil {
		return nil, err
	}
	duplex, err := piping_tunnel_util.DuplexConnect(c.httpClient, c.headers, uploadUrl, downloadUrl)
	if err != nil {
		return nil, err
	}
	return duplex, err
}
