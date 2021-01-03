package piping_util

import (
	"io"
	"net/http"
)

func PipingSend(httpClient *http.Client, headers []KeyValue, uploadUrl string, reader io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", uploadUrl, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	for _, kv := range headers {
		req.Header.Set(kv.Key, kv.Value)
	}
	return httpClient.Do(req)
}

func PipingGet(httpClient *http.Client, headers []KeyValue, downloadUrl string) (*http.Response, error) {
	req, err := http.NewRequest("GET", downloadUrl, nil)
	if err != nil {
		return nil, err
	}
	for _, kv := range headers {
		req.Header.Set(kv.Key, kv.Value)
	}
	return httpClient.Do(req)
}
