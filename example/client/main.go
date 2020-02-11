package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/lucas-clemente/quic-go/http3"
	"github.com/lucas-clemente/quic-go/internal/testdata"
)

func main() {
	urls := []string{
		"https://www.stormlin.com/desktop_polymer_v2.js",
	}

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
	defer roundTripper.Close()
	hclient := &http.Client{
		Transport: roundTripper,
	}

	var wg sync.WaitGroup
	wg.Add(len(urls))
	for _, addr := range urls {
		fmt.Printf("GET %s\n", addr)
		go func(addr string) {
			_, err := hclient.Get(addr)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Println("finished")
			wg.Done()
		}(addr)
	}
	wg.Wait()
}
