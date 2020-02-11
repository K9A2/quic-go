package main

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"log"
	"net/http"

	"github.com/lucas-clemente/quic-go/http3"
	"github.com/lucas-clemente/quic-go/internal/testdata"
)

// 本代理的默认监听地址
const defaultProxyAddr = "127.0.0.1:8080"

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

	log.Printf("received request <%v>", requestURL)

	if scheme != "http" && scheme != "https" {
		msg := "unsupported protocal scheme " + req.URL.Scheme
		http.Error(wr, msg, http.StatusBadRequest)
		log.Println(msg)
		return
	}

	// h2Client := &http.Client{}

	// 根据客户端的请求重新生成到远程服务器的请求
	resp, err := h3Client.Get("https://" + hostname + requestURL)
	if err != nil {
		http.Error(wr, "Server Error", http.StatusInternalServerError)
		log.Printf("ServeHTTP: %v", err)
		wr.WriteHeader(http.StatusNotFound)
		return
	}
	defer resp.Body.Close()

	copyHeader(wr.Header(), resp.Header)
	wr.WriteHeader(resp.StatusCode)
	io.Copy(wr, resp.Body)
	log.Printf("request finished <%v>", requestURL)
}

func main() {
	log.Println("Starting proxy server on: ", defaultProxyAddr)
	handler := &proxy{}
	if err := http.ListenAndServe(defaultProxyAddr, handler); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
