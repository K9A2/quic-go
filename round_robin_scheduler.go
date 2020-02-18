package quic

import (
	"fmt"
	"net/http"
	"sync"
)

const roundRobinSchedulerName = "round-robin-scheduler"

// RoundRobinScheduler 在收到任何一个新加入的 ResponseWriter 之后都会立刻将其触发
type RoundRobinScheduler struct {
	mutex sync.Mutex
	name  string

	// 当有新的 ResponseWriter 就绪时把该 ResponseWriter 投入此 chan 中
	blockArriveChan chan *ResponseWriterControlBlock
	// 并行 go 程数目
	concurrentStreamNumber uint16
}

// NewRoundRobinScheduler 初始化一个 roundRobinScheduler 实例并返回其指针
func NewRoundRobinScheduler() *RoundRobinScheduler {
	return &RoundRobinScheduler{
		mutex:           sync.Mutex{},
		name:            roundRobinSchedulerName,
		blockArriveChan: make(chan *ResponseWriterControlBlock, 300),
	}
}

// Name 返回该调度器的名字
func (schd *RoundRobinScheduler) Name() string {
	return schd.name
}

// Run 在后台运行该调度器
func (schd *RoundRobinScheduler) Run() {
	go func() {
		for true {
			select {
			case newBlock := <-schd.blockArriveChan:
				{
					if newBlock != nil {
						go schd.executeResponseWriter(newBlock)
					}
				}
			}
		}
	}()
}

// AddNewResponseWriter 向调度器中添加一个新的控制块
func (schd *RoundRobinScheduler) AddNewResponseWriter(writer http.ResponseWriter,
	request *http.Request, quicStr Stream, handler http.Handler) {
	// 立即出发此 ResponseWriter
	schd.blockArriveChan <- newResponseWriterControlBlock(writer, request, quicStr, handler)
}

// tryExecuteResponseWriter 尝试从队列中获取并执行就绪的 ResponseWriter
func (schd *RoundRobinScheduler) tryExecuteResponseWriter() {
}

// popNextResponseWriter 给出下一个应当触发的控制块
func (schd *RoundRobinScheduler) popNextResponseWriter() *ResponseWriterControlBlock {
	// RoundRobinScheduler 不需要使用此方法
	return nil
}

// executeResponseWriter 实际执行给定的 ResponseWriter
func (schd *RoundRobinScheduler) executeResponseWriter(block *ResponseWriterControlBlock) {
	schd.onStart()
	// 执行该 ResponseWriter
	block.handler.ServeHTTP(*block.writer, block.request)
	// ResponseWriter 执行完之后需要手动关闭对应的 QUIC Stream
	block.str.Close()
	schd.onFinish()
}

// 在调度器开始返回新的响应体时被触发
func (schd *RoundRobinScheduler) onStart() {
	schd.mutex.Lock()
	schd.concurrentStreamNumber++
	fmt.Printf("%d\n", schd.concurrentStreamNumber)
	schd.mutex.Unlock()
}

// 在响应体返回后触发
func (schd *RoundRobinScheduler) onFinish() {
	schd.mutex.Lock()
	schd.concurrentStreamNumber--
	schd.mutex.Unlock()
}
