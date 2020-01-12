package main

import (
	"log"
	"net/http"
	"os"

	"github.com/lucas-clemente/quic-go/http3"
	"golang.org/x/net/http2"
)

func runH2Server() {
	cwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
		return
	}
	srv := &http.Server{
		Addr:    "0.0.0.0:443",
		Handler: http.FileServer(http.Dir(cwd)),
	}
	http2.ConfigureServer(srv, &http2.Server{})
	log.Fatal(srv.ListenAndServeTLS("cert.pem", "cert.key"))
}

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

func runH3Server() {
	h3srv := http3.Server{
		Server: &http.Server{
			Addr:    "0.0.0.0:443",
			Handler: newH3FileServer(),
		},
	}
	log.Fatal(h3srv.ListenAndServeTLS("cert.pem", "cert.key"))
}

func main() {
	// 运行 HTTP2 服务器
	go runH2Server()
	// 运行 HTTP3 服务器
	runH3Server()
}
