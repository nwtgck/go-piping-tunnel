package piping_util

import (
	"context"
	"io"
	"net/http"
)

func PipingSendWithContext(ctx context.Context, httpClient *http.Client, headers []KeyValue, uploadUrl string, reader io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", uploadUrl, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	for _, kv := range headers {
		req.Header.Set(kv.Key, kv.Value)
	}
	return httpClient.Do(req)
}

func PipingSend(httpClient *http.Client, headers []KeyValue, uploadUrl string, reader io.Reader) (*http.Response, error) {
	return PipingSendWithContext(context.Background(), httpClient, headers, uploadUrl, reader)
}

func PipingGetWithContext(ctx context.Context, httpClient *http.Client, headers []KeyValue, downloadUrl string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", downloadUrl, nil)
	if err != nil {
		return nil, err
	}
	for _, kv := range headers {
		req.Header.Set(kv.Key, kv.Value)
	}
	return httpClient.Do(req)
}

func PipingGet(httpClient *http.Client, headers []KeyValue, downloadUrl string) (*http.Response, error) {
	return PipingGetWithContext(context.Background(), httpClient, headers, downloadUrl)
}
