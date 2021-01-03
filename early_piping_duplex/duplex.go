// TODO: duplicate code
package early_piping_duplex

import (
	"github.com/nwtgck/go-piping-tunnel/piping_util"
	"github.com/nwtgck/go-piping-tunnel/util"
	"io"
	"net/http"
)

type pipingDuplex struct {
	uploadWriterChan   <-chan interface{} // *io.PipeWriter or error
	downloadReaderChan <-chan interface{} // io.ReadCloser or error
	uploadWriter       *io.PipeWriter
	downloadReader     io.ReadCloser
}

func DuplexConnect(httpClient *http.Client, headers []piping_util.KeyValue, uploadUrl, downloadUrl string) (*pipingDuplex, error) {
	uploadWriterChan := make(chan interface{})

	go func() {
		uploadPr, uploadPw := io.Pipe()
		req, err := http.NewRequest("POST", uploadUrl, uploadPr)
		if err != nil {
			uploadWriterChan <- err
			return
		}
		req.Header.Set("Content-Type", "application/octet-stream")
		for _, kv := range headers {
			req.Header.Set(kv.Key, kv.Value)
		}
		_, err = httpClient.Do(req)
		if err != nil {
			uploadWriterChan <- err
			return
		}
		uploadWriterChan <- uploadPw
	}()

	downloadReaderChan := make(chan interface{})
	go func() {
		req, err := http.NewRequest("GET", downloadUrl, nil)
		if err != nil {
			downloadReaderChan <- err
			return
		}
		for _, kv := range headers {
			req.Header.Set(kv.Key, kv.Value)
		}
		res, err := httpClient.Do(req)
		if err != nil {
			downloadReaderChan <- err
			return
		}
		downloadReaderChan <- res.Body
	}()

	return &pipingDuplex{
		uploadWriterChan:   uploadWriterChan,
		downloadReaderChan: downloadReaderChan,
	}, nil
}

func (pd *pipingDuplex) Read(b []byte) (n int, err error) {
	if pd.downloadReaderChan != nil {
		// Get io.ReadCloser or error
		result := <-pd.downloadReaderChan
		// If result is error
		if err, ok := result.(error); ok {
			return 0, err
		}
		pd.downloadReader = result.(io.ReadCloser)
		pd.downloadReaderChan = nil
	}
	return pd.downloadReader.Read(b)
}

func (pd *pipingDuplex) Write(b []byte) (n int, err error) {
	// Get *io.PipeWriter or error
	if pd.uploadWriterChan != nil {
		result := <-pd.uploadWriterChan
		// If result is error
		if err, ok := result.(error); ok {
			return 0, err
		}
		pd.uploadWriter = result.(*io.PipeWriter)
		pd.uploadWriterChan = nil
	}
	return pd.uploadWriter.Write(b)
}

func (pd *pipingDuplex) Close() error {
	wErr := pd.uploadWriter.Close()
	rErr := pd.downloadReader.Close()
	return util.CombineErrors(wErr, rErr)
}
