package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/lucas-clemente/quic-go/http3"
	"github.com/lucas-clemente/quic-go/internal/testdata"
)

// 本代理的默认监听地址
const defaultProxyAddr = "127.0.0.1:8080"

const logFilePath = "output.log"

// 复制 HTTP 头
func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func newH3Client() *http.Client {
	pool, err := x509.SystemCertPool()
	if err != nil {
		log.Fatal(err)
	}
	testdata.AddRootCA(pool)
	roundTripper := &http3.RoundTripper{
		TLSClientConfig: &tls.Config{
			RootCAs: pool,
		},
	}
	// defer roundTripper.Close()
	h3Client := &http.Client{
		Transport: roundTripper,
	}
	return h3Client
}

type proxy struct{}

var h3Client = newH3Client()

func (p *proxy) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	hostname := req.URL.Hostname()
	requestURL := req.URL.RequestURI()
	scheme := req.URL.Scheme

	if hostname != "www.stormlin.com" {
		// 请求的不是本地服务器，直接返回 404 Not Found 错误
		http.Error(wr, "Not Found", http.StatusNotFound)
		log.Printf("rejuect request with url = <%v>", hostname+requestURL)
		return
	}

	// log.Printf("received request <%v>", requestURL)
	if scheme != "http" && scheme != "https" {
		msg := "unsupported protocal scheme <" + scheme + ">"
		http.Error(wr, msg, http.StatusBadRequest)
		log.Println(msg)
		return
	}

	// h2Client := &http.Client{}

	// 根据客户端的请求重新生成到远程服务器的请求
	req, err := http.NewRequest(http.MethodGet, "https://"+hostname+requestURL, nil)
	if err != nil {
		http.Error(wr, "Server Error", http.StatusInternalServerError)
		log.Printf("ServeHTTP: %v", err.Error())
		wr.WriteHeader(http.StatusNotFound)
		return
	}
	resp, err := h3Client.Do(req)
	if err != nil {
		http.Error(wr, "Server Error", http.StatusInternalServerError)
		log.Printf("ServeHTTP: %v", err)
		wr.WriteHeader(http.StatusNotFound)
		return
	}
	defer resp.Body.Close()

	// log.Printf("url = %v, statusCode = %v", requestURL, resp.StatusCode)
	copyHeader(wr.Header(), resp.Header)
	wr.WriteHeader(resp.StatusCode)
	io.Copy(wr, resp.Body)
	// log.Printf("request finished <%v>", requestURL)
}

func main() {
	if _, err := os.Stat(logFilePath); err == nil {
		// 日志文件已存在，删除此日志文件
		os.Remove(logFilePath)
		fmt.Println("old log file removed")
	} else {
		fmt.Println("error in removing old log file:", err.Error())
		return
	}

	logFile, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err.Error())
		return
	}
	defer logFile.Close()

	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	log.Println("Starting proxy server on: ", defaultProxyAddr)
	handler := &proxy{}
	if err := http.ListenAndServe(defaultProxyAddr, handler); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
