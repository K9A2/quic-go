package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/lucas-clemente/quic-go/http3"
	"github.com/lucas-clemente/quic-go/internal/testdata"
)

const serverAddr = "https://www.stormlin.com/"

// 新建一个 h3 客户端
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

// 实际执行测试的方法
func runTest(url string) {
	client := http.Client{}

	// 使用 H3 时的配置
	// roundTripper := &http3.RoundTripper{}
	// defer roundTripper.Close()
	// client.Transport = roundTripper

	timeStart := time.Now()
	resp, err := client.Get(url)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	timeEnd := time.Now()
	fmt.Println(timeEnd.Sub(timeStart).Milliseconds())
}

// 该程序以单线程发起请求，循环执行指定次数的请求并输出每次请求所用时间
// 调用命令 ./main 1KB 50，第一项参数是目标文件大小，第二项是重复次数
func main() {
	target, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	repeat := 50

	var targetURL string
	switch target {
	case 1:
		targetURL = serverAddr + "1KB"
	case 2:
		targetURL = serverAddr + "10KB"
	case 3:
		targetURL = serverAddr + "100KB"
	case 4:
		targetURL = serverAddr + "1MB"
	default:
		targetURL = serverAddr + "10MB"
	}
	// 重复指定次数
	for i := 0; i < repeat; i++ {
		// 实际执行测试
		runTest(targetURL)
		// 睡眠 3 秒以使得已发送的数据包能够完全离开网络
		time.Sleep(time.Duration(500) * time.Millisecond)
	}
}
