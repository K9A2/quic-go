package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"time"

	quic "github.com/lucas-clemente/quic-go"
)

func main() {
	// 以第一个参数确定是否为客户端，该参数值为 client 时为客户端，
	// 值为 server 时为服务器端。
	role := os.Args[1]

	switch role {
	case "client":
		if err := clientMain(); err != nil {
			fmt.Println(err.Error())
		}
	case "server":
		if err := serverMain(); err != nil {
			fmt.Println(err.Error())
		}
	default:
		fmt.Println("unknown role")
	}
}

func serverMain() error {
	listener, err := quic.ListenAddr("0.0.0.0:5001", generateTLSConfig(), nil)
	if err != nil {
		return err
	}
	sess, err := listener.Accept(context.Background())
	if err != nil {
		return err
	}
	// 只是单纯地接受一个单向流
	_, err = sess.AcceptUniStream(context.Background())
	if err != nil {
		return err
	}
	return nil
}

func clientMain() error {
	tlsConf := &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"quic-echo-example"},
	}
	session, err := quic.DialAddr("www.stormlin.com:5001", tlsConf, nil)
	if err != nil {
		return err
	}

	for i := 0; i < 50; i++ {
		fmt.Println("rtt =", session.GetConnectionRTT())
		time.Sleep(time.Duration(1) * time.Second)
	}

	return nil
}

// Setup a bare-bones TLS config for the server
func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		Certificates: []tls.Certificate{tlsCert},
		NextProtos:   []string{"quic-echo-example"},
	}
}
