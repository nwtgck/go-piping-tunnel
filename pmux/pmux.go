// TODO: should send fin to notify finish
package pmux

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/nwtgck/go-piping-tunnel/early_piping_duplex"
	"github.com/nwtgck/go-piping-tunnel/piping_util"
	"github.com/nwtgck/go-piping-tunnel/util"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/http"
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

const syncPath = "sync"

func Server(httpClient *http.Client, headers []piping_util.KeyValue, baseUploadUrl string, baseDownloadUrl string) *server {
	return &server{
		httpClient:      httpClient,
		headers:         headers,
		baseUploadUrl:   baseUploadUrl,
		baseDownloadUrl: baseDownloadUrl,
	}
}

func (s *server) getSubPath() (string, error) {
	startTime := time.Now()
	downloadUrl, err := util.UrlJoin(s.baseDownloadUrl, syncPath)
	if err != nil {
		return "", err
	}
	getRes, err := piping_util.PipingGet(s.httpClient, s.headers, downloadUrl)
	if err != nil {
		return "", err
	}
	if getRes.StatusCode != 200 {
		return "", errors.Errorf("not status 200, found: %d", getRes.StatusCode)
	}
	resBytes, err := ioutil.ReadAll(getRes.Body)
	if err != nil {
		return "", err
	}
	if err = getRes.Body.Close(); err != nil {
		return "", err
	}
	fmt.Println(string(resBytes), time.Now().Sub(startTime).Milliseconds())
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
		fmt.Printf("get sync error: %+v\n", errors.WithStack(err))
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
	return duplex, nil
}

func Client(httpClient *http.Client, headers []piping_util.KeyValue, baseUploadUrl string, baseDownloadUrl string) *client {
	return &client{
		httpClient:      httpClient,
		headers:         headers,
		baseUploadUrl:   baseUploadUrl,
		baseDownloadUrl: baseDownloadUrl,
	}
}

func (c *client) sendSubPath() (string, error) {
	startTime := time.Now()
	subPath := strings.Replace(uuid.New().String(), "-", "", 4)
	packet := syncPacket{SubPath: subPath}
	fmt.Println("sending          ", packet)
	jsonBytes, err := json.Marshal(packet)
	if err != nil {
		return "", err
	}
	uploadUrl, err := util.UrlJoin(c.baseUploadUrl, syncPath)
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
	fmt.Println("send   got status", packet, time.Now().Sub(startTime).Milliseconds())
	if _, err = io.Copy(ioutil.Discard, res.Body); err != nil {
		return "", err
	}
	if err = res.Body.Close(); err != nil {
		return "", err
	}
	fmt.Println("send read up body", packet, time.Now().Sub(startTime).Milliseconds())
	return subPath, nil
}

func (c *client) Open() (io.ReadWriteCloser, error) {
	startTime := time.Now()
	var subPath string
	for {
		var err error
		subPath, err = c.sendSubPath()
		if err == nil {
			break
		}
		fmt.Println("send sync error", err)
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
	fmt.Println("open: create duplex connect       ", time.Now().Sub(startTime).Milliseconds())
	if err != nil {
		return nil, err
	}
	return duplex, nil
}
