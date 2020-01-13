package main

import (
	"log"
	"net/http"

	"github.com/lucas-clemente/quic-go/http3"
)

type h3FileHandler struct {
	handler http.Handler
}

func newH3FileServer() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("./")))
	return &h3FileHandler{
		handler: mux,
	}
}

func (h *h3FileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handler.ServeHTTP(w, r)
}

func main() {
	// 同时运行 HTTP/3 和 HTTP/2 服务器
	log.Fatal(http3.ListenAndServe("0.0.0.0:443", "cert.pem", "cert.key", newH3FileServer()))
}
