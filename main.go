package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
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

func main() {
	port, _ := strconv.Atoi(os.Args[1])
	server := os.Args[2]
	path1 := os.Args[3]
	path2 := os.Args[4]

	conn, err := net.Dial("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	url2 := fmt.Sprintf("%s/%s", server, path2)
	postHttpClient := GetHttpClient(false)
	_, err = postHttpClient.Post(url2, "application/octet-stream", conn)
	if err != nil {
		panic(err)
	}
	fmt.Println("after POST")

	url1 := fmt.Sprintf("%s/%s", server, path1)
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
