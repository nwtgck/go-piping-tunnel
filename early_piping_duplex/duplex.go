// TODO: duplicate code
package early_piping_duplex

import (
	"github.com/nwtgck/go-piping-tunnel/piping_util"
	"github.com/nwtgck/go-piping-tunnel/util"
	"github.com/pkg/errors"
	"io"
	"net/http"
)

type pipingDuplex struct {
	uploadWriter       *io.PipeWriter
	uploadErrChan      <-chan error
	downloadReaderChan <-chan interface{} // io.ReadCloser or error
	downloadReader     io.ReadCloser
}

func DuplexConnect(httpClient *http.Client, headers []piping_util.KeyValue, uploadUrl, downloadUrl string) (*pipingDuplex, error) {
	uploadPr, uploadPw := io.Pipe()
	uploadErrChan := make(chan error)
	go func() {
		defer close(uploadErrChan)
		res, err := piping_util.PipingSend(httpClient, headers, uploadUrl, uploadPr)
		if err != nil {
			uploadErrChan <- err
			return
		}
		if res.StatusCode != 200 {
			uploadErrChan <- errors.Errorf("not status 200, found: %d", res.StatusCode)
			return
		}
		uploadErrChan <- nil
	}()

	downloadReaderChan := make(chan interface{})
	go func() {
		defer close(downloadReaderChan)
		res, err := piping_util.PipingGet(httpClient, headers, downloadUrl)
		if err != nil {
			downloadReaderChan <- err
			return
		}
		if res.StatusCode != 200 {
			downloadReaderChan <- errors.Errorf("not status 200, found: %d", res.StatusCode)
		}
		downloadReaderChan <- res.Body
	}()

	return &pipingDuplex{
		uploadWriter:       uploadPw,
		uploadErrChan:      uploadErrChan,
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
	select {
	case err := <-pd.uploadErrChan:
		if err != nil {
			return 0, err
		}
	default:
	}
	return pd.uploadWriter.Write(b)
}

func (pd *pipingDuplex) Close() error {
	var wErr, rErr error
	if pd.uploadWriter != nil {
		wErr = pd.uploadWriter.Close()
	}
	if pd.downloadReader != nil {
		rErr = pd.downloadReader.Close()
	}
	return util.CombineErrors(wErr, rErr)
}
