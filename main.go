package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
)

// Generate HTTP client
func GetHttpClient(insecure bool) *http.Client {
	// Set insecure or not
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{ InsecureSkipVerify: insecure },
	}
	return &http.Client{Transport: tr}

}

func main() {
	fmt.Println("hello, world")

	// TODO: Hard code
	path1 := "mypath1"
	path2 := "mypath2"
	// TODO: Hard code
	conn, err := net.Dial("tcp", "localhost:8000")
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// TODO: hard code server
	url1 := fmt.Sprintf("https://ppng.io/%s", path1)
	postHttpClient := GetHttpClient(false)
	postHttpClient.Post(url1, "application/octet-stream", conn)
	fmt.Println("after POST")


	// TODO: hard code server
	url2 := fmt.Sprintf("https://ppng.io/%s", path2)
	getHttpClient := GetHttpClient(false)
	res, err := getHttpClient.Get(url2)
	if err != nil {
		panic(err)
	}
	io.Copy(conn, res.Body)
	fmt.Println("after GET")
}
