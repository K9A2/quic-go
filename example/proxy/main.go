package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
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
	// hostname := req.URL.Hostname()
	requestURL := req.URL.RequestURI()
	// scheme := req.URL.Scheme

	// if hostname != "www.stormlin.com" {
	// 	// 请求的不是本地服务器，直接返回 404 Not Found 错误
	// 	http.Error(wr, "Not Found", http.StatusNotFound)
	// 	// log.Printf("reject request with url = <%v>", hostname+requestURL)
	// 	return
	// }

	// if scheme != "http" && scheme != "https" {
	// 	msg := "unsupported protocol scheme <" + scheme + ">"
	// 	http.Error(wr, msg, http.StatusBadRequest)
	// 	log.Println(msg)
	// 	return
	// }

	// h2Client := &http.Client{}

	log.Printf("received request <%v>", requestURL)
	// 根据客户端的请求重新生成到远程服务器的请求
	req, err := http.NewRequest(http.MethodGet, "https://www.stormlin.com"+requestURL, nil)
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
	if resp.Body != nil {
		// 部分请求可能会出现空响应体的情况, 所以只能在响应体非空的时候关闭它
		defer resp.Body.Close()
	}

	// log.Printf("url = %v, statusCode = %v", requestURL, resp.StatusCode)
	// log.Printf("main: start to copy response body for url = <%v>", req.URL.RequestURI())
	copyHeader(wr.Header(), resp.Header)
	wr.WriteHeader(resp.StatusCode)
	if resp.Body != nil {
		io.Copy(wr, resp.Body)
	}
	log.Printf("request finished <%v>", requestURL)
}

// redirect 把收到的全部请求都重定向为 HTTP 协议
func redirect(w http.ResponseWriter, req *http.Request) {
	targetURL := "http://" + req.Host + req.URL.Path
	if len(req.URL.RawQuery) > 0 {
		targetURL += "?" + req.URL.RawQuery
	}
	log.Printf("redirect to: %s", targetURL)
	http.Redirect(w, req, targetURL, http.StatusMovedPermanently)
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

	logFile, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0777)
	if err != nil {
		log.Fatalf("error opening file: %v", err.Error())
		return
	}
	defer logFile.Close()

	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	log.Println("Starting proxy server on: ", defaultProxyAddr)

	// 把全部 HTTPS 请求重定向为 HTTP 请求t
	go http.ListenAndServeTLS(":443", "cert.pem", "cert.key", http.HandlerFunc(redirect))

	handler := &proxy{}
	if err := http.ListenAndServe(defaultProxyAddr, handler); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
