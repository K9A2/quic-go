package http3

import (
	"crypto/tls"
	"log"
	"net/http"

	"github.com/lucas-clemente/quic-go"
	"github.com/marten-seemann/qpack"
)

// 每个 client 下最多只能开 4 个 quic 连接，相当于最多同时使用 4 条连接处理同一个请求
const maxConcurrentSessions = 8

// 默认块大小
const defaultBlockSize = 32 * 1024

// 调度器名称定义
const roundRobinRequestSchedulerName = "round-robin-request-scheduler"
const parallelRequestSchedulerName = "parallel-request-scheduler"
const singleConnectionRequestSchedulerName = "single-connection-request-scheduler"

// clientInfo 是供 http3 client 传入自身信息的接口体
type clientInfo struct {
	hostname         string
	tlsConfig        *tls.Config
	quicConfig       *quic.Config
	requestWriter    *requestWriter
	decoder          *qpack.Decoder
	roundTripperOpts *roundTripperOpts
}

// requestScheduler 是请求调度器的对外接口
type requestScheduler interface {
	addAndWait(*http.Request) (*http.Response, error) // 把收到的请求添加到
	close() error                                     // 拆除调度器实例

	run() // 运行调度器实例主线程
}

// newRequestScheduler 是根据给定调度器类型生成对应调度器实例的工厂方法
func newRequestScheduler(schedulerType string, info *clientInfo) requestScheduler {
	switch schedulerType {
	case roundRobinRequestSchedulerName:
		return newRoundRobinRequestScheduler(info)
	case parallelRequestSchedulerName:
		return newParallelRequestScheduler(info)
	case singleConnectionRequestSchedulerName:
		return newSingleConnectionScheduler(info)
	default:
		log.Printf("undefined request scheduler type: <%v>", schedulerType)
		return nil
	}
}
