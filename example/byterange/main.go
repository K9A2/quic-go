package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/lucas-clemente/quic-go/http3"
)

// 使用锁机制避免多 go 程出现争用现象
var createClientLock sync.Mutex

const logFilePath = "output.log"

// 单 go 程下载方法
// func download(targetURL string, rangeStart int, rangeEnd int, wg *sync.WaitGroup) *[]byte {
// 	createClientLock.Lock()
// 	client := http.Client{}
// 	roundTripper := &http3.RoundTripper{}
// 	defer roundTripper.Close()
// 	client.Transport = roundTripper
// 	createClientLock.Unlock()

// 	// 创建请求
// 	req, err := http.NewRequest(http.MethodGet, targetURL, nil)
// 	if err != nil {
// 		fmt.Println(err.Error())
// 		return nil
// 	}
// 	// 设置分段请求
// 	req.Header.Set("Range", "bytes="+strconv.Itoa(rangeStart)+"-"+strconv.Itoa(rangeEnd))

// 	// 发起请求
// 	resp, err := client.Do(req)
// 	if err != nil {
// 		fmt.Println(err.Error())
// 		return nil
// 	}
// 	defer resp.Body.Close()
// 	data, err := ioutil.ReadAll(resp.Body)
// 	if err != nil {
// 		fmt.Println(err.Error())
// 		return nil
// 	}
// 	wg.Done()
// 	return &data
// }

// func main() {
// 	var processTime int64
// 	url := "https://www.stormlin.com/4MB"
// 	targetFileSize := 131072
// 	// 读取第一个参数为 go 程数目
// 	numGoroutine, err := strconv.Atoi(os.Args[1])
// 	blockSize := targetFileSize / numGoroutine
// 	if err != nil {
// 		fmt.Println(err.Error())
// 		return
// 	}

// 	for i := 0; i < 50; i++ {
// 		fmt.Println(i)
// 		timeStart := time.Now()
// 		var wg sync.WaitGroup
// 		wg.Add(numGoroutine)

// 		for j := 0; j < numGoroutine; j++ {
// 			go download(url, j*blockSize, (j+1)*blockSize, &wg)
// 		}

// 		wg.Wait()
// 		// 记录总时间
// 		timeEnd := time.Now()
// 		processTime += timeEnd.Sub(timeStart).Milliseconds()
// 	}

// 	fmt.Println(processTime / 50)
// }

func main() {
	if _, err := os.Stat(logFilePath); err == nil {
		// 日志文件已存在，删除此日志文件
		os.Remove(logFilePath)
		fmt.Println("old log file removed")
	}

	logFile, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err.Error())
		return
	}
	defer logFile.Close()

	log.SetOutput(logFile)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
	log.Println("program started")
	var result int64
	repeatFor := 50
	for i := 0; i < repeatFor; i++ {
		fmt.Println(i)
		timeStart := time.Now()
		url := "https://www.stormlin.com/4MB"

		// defaultBlockSize := int64(512 * 1024)

		client := http.Client{}
		roundTripper := &http3.RoundTripper{}
		defer roundTripper.Close()
		client.Transport = roundTripper

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		fmt.Println(req.URL.Hostname(), req.URL.RequestURI())

		resp, err := client.Do(req)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		log.Printf("res addr = <%p>", &resp)
		log.Printf("body addr = <%v>", &resp.Body)
		if resp.Body == nil {
			log.Println("nil response body")
		}
		data, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		// for err == nil {
		// 	// timeStart := time.Now()

		// 	buf := bytes.Buffer{}
		// 	_, err := io.CopyN(&buf, resp.Body, defaultBlockSize)
		// 	if err != nil {
		// 		fmt.Println(err.Error())
		// 		break
		// 	}
		// 	// 追加读取到的字节到结果数组
		// 	data = append(data, buf.Bytes()...)

		// 	// timeEnd := time.Now()
		// 	// timeUsed := timeEnd.Sub(timeStart).Milliseconds()
		// 	// bandwidth := float64(defaultBlockSize) / float64(timeUsed) / 1000.0
		// 	// fmt.Println(bandwidth)
		// }

		err = ioutil.WriteFile("output", data, 0777)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		// fmt.Println("output done")
		timeEnd := time.Now()
		timeUsed := timeEnd.Sub(timeStart).Milliseconds()
		result += timeUsed
	}
	fmt.Println(result / int64(repeatFor))
}
