package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/lucas-clemente/quic-go/http3"
	"github.com/lucas-clemente/quic-go/internal/testdata"
)

// 新建 h3 客户端时的同步锁
var newClientLock = sync.Mutex{}

const (
	// 服务器地址
	serverAddr = "https://www.stormlin.com/"

	// 未就绪
	statusUnabailable = 1
	// 处理中
	statusPending = 2
	// 已下载
	statusFinished = 3
)

// Node 是配置文件中单个节点的定义
type Node struct {
	URL string   `json:"url"`
	Dep []string `json:"dep"`
}

// Nodes 是配置文件中节点列表的定义
type Nodes struct {
	Nodes []Node `json:"nodes_with_deps"`
}

// LoadJSONConfig 负责加载 JSON 格式的配置文件
func LoadJSONConfig(filename string) (*[]Node, error) {
	jsonFile, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()

	byteValue, _ := ioutil.ReadAll(jsonFile)
	var nodes Nodes
	json.Unmarshal(byteValue, &nodes)
	return &nodes.Nodes, nil
}

// 新建一个 h3 客户端
func newH3Client() *http.Client {
	// 使用同步方法的形式以免多个 go 程争用 http3 包下的 newClient 方法
	newClientLock.Lock()
	defer newClientLock.Unlock()

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

// 调度器定义
type scheduler struct {
	mutex     sync.Mutex
	statusMap map[string]int
	nodeList  *[]Node
	h3Client  *http.Client
	queue     []string
	signal    chan struct{}

	sumRequest      int
	finishedRequest int
}

// 新建一个调度器
func newScheduler(nodes *[]Node) *scheduler {
	numRequest := len(*nodes)
	// 初始化调度器
	schd := scheduler{
		mutex:     sync.Mutex{},
		statusMap: make(map[string]int),
		nodeList:  nodes,
		h3Client:  newH3Client(),
		queue:     make([]string, 0, numRequest),
		signal:    make(chan struct{}, numRequest),

		sumRequest:      numRequest,
		finishedRequest: 0,
	}
	// 初始化请求状态机
	for _, n := range *nodes {
		schd.statusMap[n.URL] = statusUnabailable
	}
	return &schd
}

// 返回所有就绪的请求
func (schd *scheduler) enqueueAllAvailableRequest() *[]string {
	schd.mutex.Lock()
	defer schd.mutex.Unlock()

	available := []string{}
	// 循环检查所有队列中的请求，并把就绪的请求添加到就绪队列中
	for _, node := range *schd.nodeList {
		status, _ := schd.statusMap[node.URL]
		if len(node.Dep) == 0 || status != statusUnabailable {
			// index.html 的依赖性列表为空，或者该请求已经完成，跳过这些请求
			continue
		}

		// 加载路径中是否有依赖项尚未满足，默认值为 false
		var unmet = false
		for _, dep := range node.Dep {
			status, _ := schd.statusMap[dep]
			if status != statusFinished {
				// 加载路径中有一项不满足就不能下发该请求
				unmet = true
				break
			}
		}
		if unmet {
			// 本项的加载路径尚未完全就绪，继续检查下一项
			continue
		}
		// 本项的加载路径完全满足，需要发送信号让调度器执行此请求
		schd.statusMap[node.URL] = statusPending
		available = append(available, node.URL)
	}
	return &available
}

func (schd *scheduler) allFinished() bool {
	schd.mutex.Lock()
	defer schd.mutex.Unlock()
	fmt.Printf("%v %v \n", schd.finishedRequest, schd.sumRequest)
	return !(schd.finishedRequest < schd.sumRequest)
}

// 模拟浏览器的主 go 程
func browser(wg *sync.WaitGroup, nodes *[]Node) {
	schd := newScheduler(nodes)
	// 请求总数
	numRequest := len(*nodes)

	resp, err := schd.h3Client.Get(serverAddr + "index.html")
	fmt.Printf("sent: <%v>\n", serverAddr+"index.html")

	if err != nil {
		fmt.Println(err.Error())
		return
	}
	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	schd.statusMap["index.html"] = statusFinished
	schd.signal <- struct{}{}

	finishedRequest := 0
	allFinished := false
	for !allFinished {
		availableRequests := schd.enqueueAllAvailableRequest()
		for _, req := range *availableRequests {
			// 在新的 go 程中执行请求
			go func(req string) {
				fmt.Printf("sent: <%v>\n", serverAddr+req)
				resp, err := schd.h3Client.Get(serverAddr + req)

				if err != nil {
					fmt.Println(err.Error())
					return
				}
				defer resp.Body.Close()
				_, err = ioutil.ReadAll(resp.Body)

				schd.mutex.Lock()
				schd.statusMap[req] = statusFinished

				schd.signal <- struct{}{}
				schd.mutex.Unlock()
			}(req)
		}
		select {
		case <-schd.signal:
			{
				finishedRequest++
				if finishedRequest >= numRequest {
					allFinished = true
				}
			}
		}
	}

	wg.Done()
}

// 主程序入口
// 命令行调用方式: ./main output.json 10
func main() {
	start := time.Now()
	// 加载 JSON 格式的配置文件
	// 要求第一个参数必须是 JSON 配置文件文件名
	nodes, err := LoadJSONConfig(os.Args[1])
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// 确定需要并发 browser 数目
	// 要求第二个参数为并发 browser 数目
	numBrowser, err := strconv.Atoi(os.Args[2])
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	var wg sync.WaitGroup
	wg.Add(numBrowser)
	for i := 0; i < numBrowser; i++ {
		go browser(&wg, nodes)
	}
	wg.Wait()
	end := time.Now()
	fmt.Printf("%v\n", end.Sub(start).Microseconds())
}
