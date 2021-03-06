package http3

import (
	"sync"

	"github.com/lucas-clemente/quic-go"
)

// 是调度器中原始 quicSession 的一个封装
type sessionControlblock struct {
	mutex sync.Mutex

	id int

	session       *quic.Session // 对应的 quic 连接
	canDispatched bool          // 该连接是否能够被调度器用于承载其他请求

	dataToFetch int // 还需要加载的字节数

	// 基于最新样本计算的信道参数
	rtt       float64 // 该连接的 rtt
	bandwidth float64 // 该连接的带宽

	pendingRequest   int // 该 session 上承载的请求数目
	remainingDataLen int // 该 session 上仍需加载的数据量
}

// newSessionControlBlock 方法新建一个 sessionControlBlock 并返回其指针
func newSessionControlBlock(id int, session *quic.Session, canDispatched bool) *sessionControlblock {
	return &sessionControlblock{mutex: sync.Mutex{}, id: id, session: session, canDispatched: canDispatched}
}

/* 以下三个函数负责处理对 pendingRequest 字段的操作 */
func (block *sessionControlblock) setBusy(requestURL string) {
	block.mutex.Lock()
	defer block.mutex.Unlock()
	block.pendingRequest++
}

func (block *sessionControlblock) setIdle(requestURL string) {
	block.mutex.Lock()
	defer block.mutex.Unlock()
	block.pendingRequest--
	// log.Printf("setIdle: session = <%d>, pendingRequests = <%d>", block.id, block.pendingRequest)
}

func (block *sessionControlblock) dispatchable() bool {
	block.mutex.Lock()
	defer block.mutex.Unlock()
	// session 上有超过一个正在处理的请求则不可以被用来处理新的请求
	return block.pendingRequest < 1
}

/* 以下是对 bandwidth 字段的处理方法 */
func (block *sessionControlblock) getBandwidth() float64 {
	block.mutex.Lock()
	defer block.mutex.Unlock()
	return block.bandwidth
}

func (block *sessionControlblock) setBandwidth(bandwidth float64) {
	block.mutex.Lock()
	defer block.mutex.Unlock()
	block.bandwidth = bandwidth
}

/* 以下函数是对 remainingDataLen 字段的操作方法 */
func (block *sessionControlblock) reduceRemainingDataLen(reducedDataLen int) {
	block.mutex.Lock()
	defer block.mutex.Unlock()
	block.remainingDataLen -= reducedDataLen
}

func (block *sessionControlblock) addRemainingDataLen(addedDataLen int) {
	block.mutex.Lock()
	defer block.mutex.Unlock()
	block.remainingDataLen += addedDataLen
}

func (block *sessionControlblock) getRemainingDataLen() int {
	block.mutex.Lock()
	defer block.mutex.Unlock()
	return block.remainingDataLen
}

/* 以下方法是对 pendingRequest 字段的操作方法 */
func (block *sessionControlblock) getPendingRequest() int {
	block.mutex.Lock()
	defer block.mutex.Unlock()
	return block.pendingRequest
}

func (block *sessionControlblock) addNewRequest() {
	block.mutex.Lock()
	defer block.mutex.Unlock()
	block.pendingRequest++
}

func (block *sessionControlblock) removeFinishedRequest() {
	block.mutex.Lock()
	defer block.mutex.Unlock()
	block.pendingRequest--
}
