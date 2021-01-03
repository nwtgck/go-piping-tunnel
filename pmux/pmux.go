package pmux

import (
	"bytes"
	"encoding/json"
	"fmt"
	piping_tunnel_util "github.com/nwtgck/go-piping-tunnel/piping-tunnel-util"
	"github.com/nwtgck/go-piping-tunnel/util"
	"github.com/pkg/errors"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"sync"
)

type server struct {
	httpClient      *http.Client
	requestingSeqId uint
	seqIdCh         chan uint
	mutex           *sync.Mutex
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

type stream struct {
	rc    io.ReadCloser
	pw    *io.PipeWriter
	mutex *sync.Mutex
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
	s := &server{
		httpClient:      httpClient,
		requestingSeqId: 1,
		seqIdCh:         make(chan uint),
		mutex:           new(sync.Mutex),
		headers:         headers,
		baseUploadUrl:   baseUploadUrl,
		baseDownloadUrl: baseDownloadUrl,
	}

	go func() {
		seqId, err := s.synchronizeID()
		if err != nil {
			fmt.Println("synchronizeID error", err)
			return
		}
		fmt.Println("server synced seqID", seqId)
		s.seqIdCh <- seqId
	}()

	return s
}

// TODO: name (case)
type negotiatePacket struct {
	SeqId uint
}

// TODO: place
const negotiateIDPath = "negotiate_id"

func (s *server) synchronizeID() (uint, error) {
	uploadUrl, err := util.UrlJoin(s.baseUploadUrl, negotiateIDPath)
	synchronizingSeqID := s.requestingSeqId
	fmt.Println("synchronizing synchronizingSeqID:", synchronizingSeqID)
	jsonBytes, err := json.Marshal(negotiatePacket{
		SeqId: synchronizingSeqID,
	})
	req, err := http.NewRequest("POST", uploadUrl, bytes.NewReader(jsonBytes))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	for _, kv := range s.headers {
		req.Header.Set(kv.Key, kv.Value)
	}
	_, err = s.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	downloadUrl, err := util.UrlJoin(s.baseDownloadUrl, negotiateIDPath)
	if err != nil {
		return 0, err
	}
	req, err = http.NewRequest("GET", downloadUrl, nil)
	if err != nil {
		return 0, err
	}
	res, err := s.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	if res.StatusCode != 200 {
		return 0, errors.Errorf("not status 200, found: %d", res.StatusCode)
	}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return 0, err
	}
	var resJson negotiatePacket
	err = json.Unmarshal(bodyBytes, &resJson)
	if err != nil {
		return 0, err
	}
	if resJson.SeqId != synchronizingSeqID {
		return 0, errors.Errorf("resJson.SeqId != synchronizingSeqID: %d, %d", resJson.SeqId, synchronizingSeqID)
	}
	return resJson.SeqId, nil
}

func (s *server) Accept() (io.ReadWriteCloser, error) {
	seqId := <-s.seqIdCh
	fmt.Println("server used seqID:", seqId)
	go func() {
		s.mutex.Lock()
		defer s.mutex.Unlock()
		for {
			// FIXME: handle overflow
			s.requestingSeqId += 1
			seqId, err := s.synchronizeID()
			if err != nil {
				fmt.Println("server synchronizeID error", err)
				continue
			}
			fmt.Println("server synced seqID", seqId)
			s.seqIdCh <- seqId
			break
		}
	}()
	downloadUrl, err := util.UrlJoin(s.baseDownloadUrl, strconv.Itoa(int(seqId)))
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
	fmt.Println("server seq id:", seqId)

	uploadUrl, err := util.UrlJoin(s.baseUploadUrl, strconv.Itoa(int(seqId)))
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
	fmt.Println("server seqID finished", seqId)
	return &stream{
		rc: getRes.Body,
		pw: uploadPw,
	}, nil
}

func Client(httpClient *http.Client, headers []piping_tunnel_util.KeyValue, baseUploadUrl string, baseDownloadUrl string) *client {
	c := &client{
		httpClient:      httpClient,
		headers:         headers,
		baseUploadUrl:   baseUploadUrl,
		baseDownloadUrl: baseDownloadUrl,
	}
	//go func() {
	//	for {
	//		seqId, err := c.getSeqId()
	//		if err != nil {
	//			// TODO: handle?
	//			fmt.Println("client seqid loop error:", err)
	//			continue
	//		}
	//		fmt.Println("client synced seqID", seqId)
	//		seqIdCh <- seqId
	//	}
	//}()
	return c
}

func (c *client) getSeqId() (uint, error) {
	downloadUrl, err := util.UrlJoin(c.baseDownloadUrl, negotiateIDPath)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest("GET", downloadUrl, nil)
	if err != nil {
		return 0, err
	}
	res, err := c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	if res.StatusCode != 200 {
		return 0, errors.Errorf("not status 200, found: %d", res.StatusCode)
	}
	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return 0, err
	}
	var resJson negotiatePacket
	err = json.Unmarshal(bodyBytes, &resJson)
	if err != nil {
		return 0, err
	}
	uploadUrl, err := util.UrlJoin(c.baseUploadUrl, negotiateIDPath)
	req, err = http.NewRequest("POST", uploadUrl, bytes.NewReader(bodyBytes))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	for _, kv := range c.headers {
		req.Header.Set(kv.Key, kv.Value)
	}
	res, err = c.httpClient.Do(req)
	if err != nil {
		return 0, err
	}
	if _, err := io.Copy(ioutil.Discard, res.Body); err != nil {
		return 0, err
	}
	fmt.Println("client synced", resJson.SeqId)
	return resJson.SeqId, nil
}

func (c *client) Open() (io.ReadWriteCloser, error) {
	var seqId uint
	for {
		id, err := c.getSeqId()
		if err != nil {
			// TODO: handle?
			fmt.Println("client seqid loop error:", err)
			continue
		}
		fmt.Println("client synced seqID", id)
		seqId = id
		break
	}
	fmt.Println("client used seqID:", seqId)
	uploadUrl, err := util.UrlJoin(c.baseUploadUrl, strconv.Itoa(int(seqId)))
	if err != nil {
		return nil, err
	}
	downloadUrl, err := util.UrlJoin(c.baseDownloadUrl, strconv.Itoa(int(seqId)))
	if err != nil {
		return nil, err
	}
	duplex, err := piping_tunnel_util.DuplexConnect(c.httpClient, c.headers, uploadUrl, downloadUrl)
	if err != nil {
		return nil, err
	}
	fmt.Println("client seqID finished", seqId)
	return duplex, err
}
