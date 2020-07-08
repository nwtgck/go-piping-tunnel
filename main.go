package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
)

// Generate HTTP client
func GetHttpClient(insecure bool) *http.Client {
	// Set insecure or not
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{ InsecureSkipVerify: insecure },
	}
	return &http.Client{Transport: tr}
}

// (base: https://stackoverflow.com/a/34668130/2885946)
func urlJoin(s string, p string) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	u.Path = path.Join(u.Path, p)
	return u.String(), nil
}

func main() {
	port, _ := strconv.Atoi(os.Args[1])
	serverUrl := os.Args[2]
	path1 := os.Args[3]
	path2 := os.Args[4]

	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	url2, err := urlJoin(serverUrl, path2)
	if err != nil {
		panic(err)
	}
	postHttpClient := GetHttpClient(false)
	_, err = postHttpClient.Post(url2, "application/octet-stream", conn)
	if err != nil {
		panic(err)
	}
	fmt.Println("after POST")

	url1, err := urlJoin(serverUrl, path1)
	if err != nil {
		panic(err)
	}
	getHttpClient := GetHttpClient(false)
	res, err := getHttpClient.Get(url1)
	if err != nil {
		panic(err)
	}
	_, err = io.Copy(conn, res.Body)
	if err != nil {
		panic(err)
	}
	fmt.Println("after GET")
}
