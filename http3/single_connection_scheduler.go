package http3

import (
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"sync"

	"github.com/lucas-clemente/quic-go"
	"github.com/marten-seemann/qpack"
)

// 最大并发 stream 数
const maxConcurrentStreams = 20

// 本调度器最多只允许使用 1 条 quicSession
const maxSession = 1

// singleConnectionScheduler 提供在同一连接上复用多个请求的能力
type singleConnectionScheduler struct {
	mutex *sync.Mutex

	/* 这些是原 http3 client 类型的变量 */
	hostname         string
	tlsConfig        *tls.Config
	quicConfig       *quic.Config
	requestWriter    *requestWriter
	decoder          *qpack.Decoder
	roundTripperOpts *roundTripperOpts

	openedSession         []*sessionControlblock // 已经打开的 session，最多打开一条 session
	mayExecuteNextRequest *chan struct{}         // 可能可以发送下一请求时向此 chan 发送消息
	requestQueue          []*requestControlBlock // 请求队列
	pendingRequests       int                    // 正在处理的请求
	maxSessionID          int                    // 最新一条 quicSession 的 ID
}

// newSingleConnectionScheduler 方法按照 info 中指定的信息
// 构造一个新的 newSingleConnectionScheduler 实例并返回其指针
func newSingleConnectionScheduler(info *clientInfo) *singleConnectionScheduler {
	mayExecuteNextRequestChan := make(chan struct{}, 10)
	return &singleConnectionScheduler{
		mutex: &sync.Mutex{},

		hostname:         info.hostname,
		tlsConfig:        info.tlsConfig,
		quicConfig:       info.quicConfig,
		requestWriter:    info.requestWriter,
		decoder:          info.decoder,
		roundTripperOpts: info.roundTripperOpts,

		openedSession:         make([]*sessionControlblock, 0),
		mayExecuteNextRequest: &mayExecuteNextRequestChan,
		requestQueue:          make([]*requestControlBlock, 0),
		maxSessionID:          0,
	}
}

// run 方法运行调度器主线程
func (scheduler *singleConnectionScheduler) run() {
	for {
		select {
		case <-*scheduler.mayExecuteNextRequest:
			// 视情况决定是否需要执行下一请求
			scheduler.mayExecute()
		}
	}
}

// close 方法关闭调度器并拆除所管理的 quic 连接
func (scheduler *singleConnectionScheduler) close() error {
	scheduler.mutex.Lock()
	defer scheduler.mutex.Unlock()
	var err error
	for _, block := range scheduler.openedSession {
		session := *block.session
		err = session.Close()
		if err != nil {
			log.Println(err.Error())
			return err
		}
	}
	return nil
}

// addNewRequest 方法向调度器中的请求队列添加一个新的请求控制块，并向调度器主线程发送信号
func (scheduler *singleConnectionScheduler) addNewRequest(reqBlock *requestControlBlock) {
	scheduler.mutex.Lock()
	defer scheduler.mutex.Unlock()
	scheduler.requestQueue = append(scheduler.requestQueue, reqBlock)
	*scheduler.mayExecuteNextRequest <- struct{}{}
}

// addAndWait 方法向调度器内部队列中添加一个新的请求并在该请求结束之后返回其相应以及错误（如果有）
func (scheduler *singleConnectionScheduler) addAndWait(req *http.Request) (*http.Response, error) {
	var requestDone = make(chan struct{}, 0)
	var requestError = make(chan struct{}, 0)
	reqBlock := requestControlBlock{
		request:      req,
		requestDone:  &requestDone,
		requestError: &requestError,
	}
	scheduler.addNewRequest(&reqBlock)

	for {
		select {
		case <-*reqBlock.requestDone:
			return reqBlock.response, reqBlock.unhandledError
		case <-*reqBlock.requestError:
			return getErrorResponse(req), nil
		}
	}
}

// popRequest 方法返回调度器队列中的第一个请求。如果队列中没有请求，则返回 nil。
func (scheduler *singleConnectionScheduler) popRequest() *requestControlBlock {
	if len(scheduler.requestQueue) > 0 {
		nextRequest := scheduler.requestQueue[0]
		return nextRequest
	}
	return nil
}

// mayExecute 方法视情况从决定是否调度器中执行下一请求
func (scheduler *singleConnectionScheduler) mayExecute() {
	scheduler.mutex.Lock()
	defer scheduler.mutex.Unlock()

	// 并发请求数有最大限制
	if scheduler.pendingRequests < maxConcurrentSessions {
		nextRequest := scheduler.popRequest()
		if nextRequest == nil {
			// 队列中没有请求可供执行
			return
		}
		sessionBlock := scheduler.getSession()
		if sessionBlock == nil {
			// 无法获取所需的 quicSession，当做是服务器错误返回
			*nextRequest.requestError <- struct{}{}
			return
		}

		nextRequest.designatedSession = sessionBlock
		// 请求和 quicSession 都获取到之后就可以开始处理请求了
		scheduler.pendingRequests++
		go scheduler.execute(nextRequest)
	}
}

// getSession 方法返回调度器中可用的 quicSession，如果没有，就会创建新的
func (scheduler *singleConnectionScheduler) getSession() *sessionControlblock {
	if len(scheduler.openedSession) > 0 {
		// 唯一可用的 session 已打开
		return scheduler.openedSession[0]
	}
	// 还没有打开唯一的一条 quicSession，需要立刻打开
	newSession, err := dial(scheduler.hostname, scheduler.tlsConfig, scheduler.quicConfig)
	if err != nil {
		return nil
	}
	return newSessionControlBlock(scheduler.maxSessionID, newSession, true)
}

// execute 方法实际执行给定的请求
func (scheduler *singleConnectionScheduler) execute(reqBlock *requestControlBlock) {
	req := reqBlock.request
	quicSession := *reqBlock.designatedSession.session

	str, err := quicSession.OpenStreamSync(context.Background())
	if err != nil {
		log.Printf(err.Error())
		*reqBlock.requestError <- struct{}{}
		return
	}

	reqDone := make(chan struct{})
	go func() {
		select {
		case <-req.Context().Done():
			// 出错，需要手动终止对应的 quicStream
			str.CancelWrite(quic.ErrorCode(errorRequestCanceled))
			str.CancelRead(quic.ErrorCode(errorRequestCanceled))
		case <-reqDone:
			// 请求结束，那什么也不做，让这个 go 程自然退出
		}
	}()

	usingGzip := isUsingGzip(scheduler.roundTripperOpts.DisableCompression,
		req.Method, req.Header.Get("accept-encoding"), req.Header.Get("range"))
	resp, requestErr := getResponse(req, usingGzip, &str, &quicSession,
		scheduler.requestWriter, maxHeaderBytes(scheduler.roundTripperOpts.MaxHeaderBytes),
		scheduler.decoder, reqDone)
	if requestErr.err != nil {
		close(reqDone)
		if requestErr.streamErr != 0 {
			str.CancelWrite(quic.ErrorCode(requestErr.streamErr))
		}
		if requestErr.connErr != 0 {
			var reason string
			if requestErr.err != nil {
				reason = requestErr.err.Error()
			}
			quicSession.CloseWithError(quic.ErrorCode(requestErr.connErr), reason)
		}
	}
	// 向调用者发送信号
	reqBlock.response = resp
	*reqBlock.requestDone <- struct{}{}
	scheduler.mutex.Lock()
	scheduler.pendingRequests--
	scheduler.mutex.Unlock()
}
