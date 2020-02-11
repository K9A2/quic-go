package quic

import (
	"fmt"
	"net/http"
	"sync"

	"github.com/lucas-clemente/quic-go/order"
)

type concurrentStreamCounter struct {
	mutex   sync.Mutex
	counter int
}

func (c *concurrentStreamCounter) OnStart() {
	c.mutex.Lock()
	c.counter++
	fmt.Printf("%d\n", c.counter)
	c.mutex.Unlock()
}

func (c *concurrentStreamCounter) OnFinish() {
	c.mutex.Lock()
	c.counter--
	c.mutex.Unlock()
}

var ConcurrentStreamCounter = concurrentStreamCounter{}

// ResponseWriterScheduler 是调度器的抽象接口，所有调度器都必须实现接口中所定义的全部方法
type ResponseWriterScheduler interface {
	Name() string
	Run()
	AddNewResponseWriter(http.ResponseWriter, *http.Request, Stream, http.Handler)
	popNextResponseWriter() *ResponseWriterControlBlock
	tryExecuteResponseWriter()
}

// ResponseWriterControlBlock 用来暂存添加到调度器中的 ResponseWriter 及对应的 Request
type ResponseWriterControlBlock struct {
	writer  *http.ResponseWriter
	request *http.Request
	str     Stream
	handler http.Handler
}

// newResponseWriterControlBlock 使用给定的 ResponseWriter 和 Request 初始化一个新的
// 控制块，并返回其指针
func newResponseWriterControlBlock(writer http.ResponseWriter,
	request *http.Request, quicStream Stream, handler http.Handler) *ResponseWriterControlBlock {
	return &ResponseWriterControlBlock{
		writer:  &writer,
		request: request,
		str:     quicStream,
		handler: handler,
	}
}

var (
	// currentScheduler = staticOrderSchedulerName
	currentScheduler = roundRobinSchedulerName
	currentOrderList = order.YahooPerformanceList
)

// InitResponseWriterScheduler 根据配置的调度器类型初始化对应的调度器实例
func InitResponseWriterScheduler() ResponseWriterScheduler {
	var scheduler ResponseWriterScheduler
	switch currentScheduler {
	case roundRobinSchedulerName:
		{
			// fmt.Printf("using %v\n", roundRobinSchedulerName)
			scheduler = NewRoundRobinScheduler()
		}
	case staticOrderSchedulerName:
		{
			// fmt.Printf("using %v\n", staticOrderSchedulerName)
			scheduler = NewStaticOrderScheduler()
		}
	}
	return scheduler
}

// getFileName 根据接受的 url 返回对应的文件名
func getFileName(url string) string {
	if url == "/" {
		return "index.html"
	}
	return url[1:]
}
