package pmux

import (
	"fmt"
	piping_tunnel_util "github.com/nwtgck/go-piping-tunnel/piping-tunnel-util"
	"github.com/nwtgck/go-piping-tunnel/util"
	"io"
	"net/http"
	"strconv"
)

type server struct {
	httpClient      *http.Client
	seqId           uint
	headers         []piping_tunnel_util.KeyValue
	baseUploadUrl   string
	baseDownloadUrl string
}

type client struct {
	httpClient      *http.Client
	seqId           uint
	headers         []piping_tunnel_util.KeyValue
	baseUploadUrl   string
	baseDownloadUrl string
}

type stream struct {
	rc io.ReadCloser
	pw *io.PipeWriter
}

func (s *stream) Read(p []byte) (int, error) {
	return s.rc.Read(p)
}

func (s *stream) Write(p []byte) (int, error) {
	return s.pw.Write(p)
}

func (s *stream) Close() error {
	rErr := s.rc.Close()
	wErr := s.pw.Close()
	return util.CombineErrors(rErr, wErr)
}

func Server(httpClient *http.Client, headers []piping_tunnel_util.KeyValue, baseUploadUrl string, baseDownloadUrl string) *server {
	return &server{
		httpClient:      httpClient,
		seqId:           1,
		headers:         headers,
		baseUploadUrl:   baseUploadUrl,
		baseDownloadUrl: baseDownloadUrl,
	}
}

func (s *server) Accept() (io.ReadWriteCloser, error) {
	downloadUrl, err := util.UrlJoin(s.baseDownloadUrl, strconv.Itoa(int(s.seqId)))
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", downloadUrl, nil)
	if err != nil {
		// TODO: remove
		fmt.Println("get loop error:", err)
		return nil, err
	}
	for _, kv := range s.headers {
		req.Header.Set(kv.Key, kv.Value)
	}
	getRes, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	uploadUrl, err := util.UrlJoin(s.baseUploadUrl, strconv.Itoa(int(s.seqId)))
	uploadPr, uploadPw := io.Pipe()
	req, err = http.NewRequest("POST", uploadUrl, uploadPr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	for _, kv := range s.headers {
		req.Header.Set(kv.Key, kv.Value)
	}
	_, err = s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	// FIXME: handle overflow
	s.seqId += 1
	return &stream{
		rc: getRes.Body,
		pw: uploadPw,
	}, nil
}

func Client(httpClient *http.Client, headers []piping_tunnel_util.KeyValue, baseUploadUrl string, baseDownloadUrl string) *client {
	return &client{
		httpClient:      httpClient,
		seqId:           0,
		headers:         headers,
		baseUploadUrl:   baseUploadUrl,
		baseDownloadUrl: baseDownloadUrl,
	}
}

func (c *client) Open() (io.ReadWriteCloser, error) {
	// FIXME: handle overflow
	c.seqId += 1
	uploadUrl, err := util.UrlJoin(c.baseUploadUrl, strconv.Itoa(int(c.seqId)))
	if err != nil {
		return nil, err
	}
	downloadUrl, err := util.UrlJoin(c.baseDownloadUrl, strconv.Itoa(int(c.seqId)))
	if err != nil {
		return nil, err
	}
	duplex, err := piping_tunnel_util.DuplexConnect(c.httpClient, c.headers, uploadUrl, downloadUrl)
	if err != nil {
		return nil, err
	}
	return duplex, err
}
