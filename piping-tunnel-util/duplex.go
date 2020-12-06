package piping_tunnel_util

import (
	"io"
	"net/http"
)

type PipingDuplex struct {
	uploadWriter   *io.PipeWriter
	downloadReader *io.PipeReader
}

func NewPipingDuplex(httpClient *http.Client, headers []KeyValue, uploadPath, downloadPath string) (*PipingDuplex, error) {
	uploadPr, uploadPw := io.Pipe()
	req, err := http.NewRequest("POST", uploadPath, uploadPr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	for _, kv := range headers {
		req.Header.Set(kv.Key, kv.Value)
	}
	_, err = httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	req, err = http.NewRequest("GET", downloadPath, nil)
	if err != nil {
		return nil, err
	}
	for _, kv := range headers {
		req.Header.Set(kv.Key, kv.Value)
	}
	downloadPr, downloadPw := io.Pipe()

	go func() {
		res, _ := httpClient.Do(req)
		io.Copy(downloadPw, res.Body)
	}()

	return &PipingDuplex{
		uploadWriter:   uploadPw,
		downloadReader: downloadPr,
	}, nil
}

func (pd *PipingDuplex) Read(b []byte) (n int, err error) {
	return pd.downloadReader.Read(b)
}

func (pd *PipingDuplex) Write(b []byte) (n int, err error) {
	return pd.uploadWriter.Write(b)
}

func (pd *PipingDuplex) Close() error {
	err := pd.uploadWriter.Close()
	if err != nil {
		return err
	}
	err = pd.downloadReader.Close()
	return err
}
