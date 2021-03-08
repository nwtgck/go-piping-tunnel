package piping_util

import (
	"github.com/nwtgck/go-piping-tunnel/util"
	"io"
	"net/http"
)

type pipingDuplex struct {
	downloadReaderChan <-chan interface{} // io.ReadCloser or error
	uploadWriter       *io.PipeWriter
	downloadReader     io.ReadCloser
}

func DuplexConnect(httpClient *http.Client, postHeaders []KeyValue, getHeaders []KeyValue, uploadUrl, downloadUrl string) (*pipingDuplex, error) {
	return DuplexConnectWithHandlers(
		func(body io.Reader) (*http.Response, error) {
			return PipingSend(httpClient, postHeaders, uploadUrl, body)
		},
		func() (*http.Response, error) {
			return PipingGet(httpClient, getHeaders, downloadUrl)
		},
	)
}

type postHandler = func(body io.Reader) (*http.Response, error)
type getHandler = func() (*http.Response, error)

func DuplexConnectWithHandlers(post postHandler, get getHandler) (*pipingDuplex, error) {
	uploadPr, uploadPw := io.Pipe()
	_, err := post(uploadPr)
	if err != nil {
		return nil, err
	}

	downloadReaderChan := make(chan interface{})
	go func() {
		res, err := get()
		if err != nil {
			downloadReaderChan <- err
			return
		}
		downloadReaderChan <- res.Body
	}()

	return &pipingDuplex{
		downloadReaderChan: downloadReaderChan,
		uploadWriter:       uploadPw,
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
	return pd.uploadWriter.Write(b)
}

func (pd *pipingDuplex) Close() error {
	var rErr error
	wErr := pd.uploadWriter.Close()
	if pd.downloadReader != nil {
		rErr = pd.downloadReader.Close()
	}
	return util.CombineErrors(wErr, rErr)
}
