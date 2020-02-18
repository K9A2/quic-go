package quic

import (
	"net/http"
	"sync"
)

const (
	staticOrderSchedulerName    = "static-order-scheduler"
	maxConcurrentResponseWriter = 4
)

// StaticOrderScheduler 按照给定的顺序调整 ResponseWriter 的执行顺序
type StaticOrderScheduler struct {
	mutex sync.Mutex
	name  string

	// 已经安排好顺序的 ReponseWriter 会被放入此队列
	ManagedResponseWriter []*ResponseWriterControlBlock
	ManagedURLMap         map[string]int
	// ManagedResponseWriter 队列中的就绪 ResponseWriter 数目
	QueuedManagedResponseWriter int

	// 没有被安排传输顺序的 ResponseWriter 会被放入此队列
	UnmanagedResponseWriter []*ResponseWriterControlBlock
	// UnmanagedResponseWriter 队列中的就绪 ResponseWriter 数目
	QueuedUnmanagedResponseWriter int

	// 同时执行的 ResponseWriter 数目
	ConcurrentResponseWriter          int // 总数目
	ConcurrentManagedResponseWriter   int // managed 的数目
	ConcurrentUnmanagedResponseWriter int // unmanaged 的数目

	// 当有 block 就绪或者执行完毕的时候，调度器都需要向此 chan 发送消息，以便寻找并执行
	// 下一就选的 ResponseWriter
	tryRunChan chan struct{}
}

// NewStaticOrderScheduler 根据给定的传输顺序初始化调度器实例并返回其指针
func NewStaticOrderScheduler() *StaticOrderScheduler {
	// 实例化调度器
	scheduler := StaticOrderScheduler{
		ManagedResponseWriter:   make([]*ResponseWriterControlBlock, len(currentOrderList)),
		ManagedURLMap:           make(map[string]int),
		UnmanagedResponseWriter: make([]*ResponseWriterControlBlock, 0),
		tryRunChan:              make(chan struct{}, 100),
	}
	// 写入传输顺序
	for index, url := range currentOrderList {
		scheduler.ManagedURLMap[url] = index
	}

	return &scheduler
}

// Name 返回该调度器的名字
func (schd *StaticOrderScheduler) Name() string {
	return schd.name
}

// Run 在后台运行调度线程
func (schd *StaticOrderScheduler) Run() {
	go func() {
		for {
			select {
			case <-schd.tryRunChan:
				{
					// 有新的 ResponseWriter 就绪
					next := schd.popNextResponseWriter()
					if next != nil {
						go schd.executeResponseWriter(next)
					}
				}
			}
		}
	}()
}

// AddNewResponseWriter 向调度器添加一个就绪的 ResponseWriter
func (schd *StaticOrderScheduler) AddNewResponseWriter(writer http.ResponseWriter,
	request *http.Request, quicStr Stream, handler http.Handler) {
	schd.mutex.Lock()

	index, ok := schd.ManagedURLMap[getFileName((*request).RequestURI)]
	if ok {
		// 这个是已经安排好顺序的请求
		schd.ManagedResponseWriter[index] = newResponseWriterControlBlock(
			writer, request, quicStr, handler)
		schd.QueuedManagedResponseWriter++
	} else {
		// 这个是尚未安排好顺序的请求
		schd.UnmanagedResponseWriter = append(schd.UnmanagedResponseWriter,
			newResponseWriterControlBlock(writer, request, quicStr, handler))
		schd.QueuedUnmanagedResponseWriter++
	}

	// 发送信号
	schd.tryRunChan <- struct{}{}

	schd.mutex.Unlock()
}

func (schd *StaticOrderScheduler) tryExecuteResponseWriter() {
}

// findFirstAvailableResponseWriter 寻找调度器中首个处于就绪状态的 managed ResponseWriter
func (schd *StaticOrderScheduler) findFirstAvailableResponseWriter() *ResponseWriterControlBlock {
	var next *ResponseWriterControlBlock
	var index int
	for index, next = range schd.ManagedResponseWriter {
		if next != nil {
			schd.ManagedResponseWriter[index] = nil
			return next
		}
	}
	return next
}

// popNextResponseWriter 返回调度器中下一个应当被执行的 ResponseWriter
func (schd *StaticOrderScheduler) popNextResponseWriter() *ResponseWriterControlBlock {
	schd.mutex.Lock()
	defer schd.mutex.Unlock()

	if schd.ConcurrentResponseWriter > maxConcurrentResponseWriter {
		// 同时可以执行的 ResponseWriter 数目有最大限制
		return nil
	}

	var next *ResponseWriterControlBlock
	if schd.QueuedManagedResponseWriter > 0 {
		// 有就绪的 managed ResponseWriter
		next = schd.findFirstAvailableResponseWriter()
		if next != nil {
			schd.QueuedManagedResponseWriter--
			schd.ConcurrentManagedResponseWriter++
			return next
		}
	}

	if schd.ConcurrentManagedResponseWriter > 0 {
		// 在 managed 的 ResponseWriter 还没有结束的时候不执行新的 unmanaged 的 ResponseWriter
		return next
	}

	if schd.QueuedUnmanagedResponseWriter > 0 {
		// 有就绪的 unmanaged ResponseWriter
		next = schd.UnmanagedResponseWriter[0]
		schd.UnmanagedResponseWriter = schd.UnmanagedResponseWriter[1:]
		schd.QueuedUnmanagedResponseWriter--
		schd.ConcurrentUnmanagedResponseWriter++
	}

	if next != nil {
		schd.ConcurrentResponseWriter++
	}

	return next
}

func (schd *StaticOrderScheduler) executeResponseWriter(block *ResponseWriterControlBlock) {
	block.handler.ServeHTTP(*block.writer, block.request)
	schd.mutex.Lock()
	schd.ConcurrentResponseWriter--
	_, ok := schd.ManagedURLMap[getFileName((*block).request.RequestURI)]
	if ok {
		schd.ConcurrentManagedResponseWriter--
	} else {
		schd.ConcurrentUnmanagedResponseWriter--
	}
	schd.mutex.Unlock()
	block.str.Close()
	schd.tryRunChan <- struct{}{}
}
