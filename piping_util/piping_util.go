package piping_util

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

func PipingSend(httpClient *http.Client, headers []KeyValue, uploadUrl string, reader io.Reader) (*http.Response, error) {
	startTime := time.Now()
	fmt.Printf("==> POSTing %s\n", uploadUrl)
	req, err := http.NewRequest("POST", uploadUrl, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	for _, kv := range headers {
		req.Header.Set(kv.Key, kv.Value)
	}
	res, err := httpClient.Do(req)
	fmt.Printf("<== POSTing %s: %d\n", uploadUrl, time.Now().Sub(startTime).Milliseconds())
	return res, err
}

func PipingGet(httpClient *http.Client, headers []KeyValue, downloadUrl string) (*http.Response, error) {
	startTime := time.Now()
	fmt.Printf("==> GETing  %s\n", downloadUrl)
	req, err := http.NewRequest("GET", downloadUrl, nil)
	if err != nil {
		return nil, err
	}
	for _, kv := range headers {
		req.Header.Set(kv.Key, kv.Value)
	}
	res, err := httpClient.Do(req)
	fmt.Printf("<== GETing  %s: %d\n", downloadUrl, time.Now().Sub(startTime).Milliseconds())
	return res, err
}
