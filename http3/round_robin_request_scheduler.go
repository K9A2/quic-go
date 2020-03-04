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

const maxParallelStreams = 4

// roundRobinRequestScheduler 是轮询调度器的定义
type roundRobinRequestScheduler struct {
	sync.Mutex

	hostname         string
	tlsConfig        *tls.Config
	quicConfig       *quic.Config
	requestWriter    *requestWriter
	decoder          *qpack.Decoder
	roundTripperOpts *roundTripperOpts

	openedSession    []*sessionControlblock // 保存所有打开的 quicSession
	nextSessionIndex int                    // 当前使用的 quicSession 下标

	mayExecuteNextRequest *chan struct{}
	requestQueue          []*requestControlBlock
	pendingRequests       int
	maxSessionID          int
}

// newRoundRobinRequestScheduler 按照给定的信息构造一个新的轮询调度器并返回其指针
func newRoundRobinRequestScheduler(info *clientInfo) *roundRobinRequestScheduler {
	mayExecuteNextRequestChan := make(chan struct{}, 10)
	return &roundRobinRequestScheduler{
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

// run 运行调度器主线程
func (scheduler *roundRobinRequestScheduler) run() {
	for {
		select {
		case <-*scheduler.mayExecuteNextRequest:
			// 视情况决定是否执行下一请求
			scheduler.mayExecute()
		}
	}
}

// close 拆除所有 quic 连接并关闭调度器
func (scheduler *roundRobinRequestScheduler) close() error {
	scheduler.Lock()
	defer scheduler.Unlock()
	for _, sessionBlock := range scheduler.openedSession {
		if err := (*sessionBlock.session).Close(); err != nil {
			log.Printf("error in closing session: id = <%v>, err = <%v>",
				sessionBlock.id, err.Error())
			return err
		}
	}
	return nil
}

// addNewRequest 向调度器实例中添加请求控制块
func (scheduler *roundRobinRequestScheduler) addNewRequest(reqBlock *requestControlBlock) {
	scheduler.requestQueue = append(scheduler.requestQueue, reqBlock)
	*scheduler.mayExecuteNextRequest <- struct{}{}
}

// addAndWait 把请求添加到调度器内部队列中，由调度器在适当时候执行
func (scheduler *roundRobinRequestScheduler) addAndWait(req *http.Request) (*http.Response, error) {
	scheduler.Lock()
	defer scheduler.Unlock()

	var requestDone = make(chan struct{}, 10)
	var requestError = make(chan struct{}, 10)
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

// getSession 方法返回当前可用的 quicSession
func (scheduler *roundRobinRequestScheduler) getSession() (*sessionControlblock, error) {
	if len(scheduler.openedSession) < maxSession {
		// 一直创建新的 session
		newSession, err := dial(scheduler.hostname, scheduler.tlsConfig, scheduler.quicConfig)
		if err != nil {
			log.Printf("error in creating new quic session: %v", err.Error())
			return nil, err
		}
		newSessionBlock := newSessionControlBlock(scheduler.maxSessionID, newSession, true)
		scheduler.openedSession = append(scheduler.openedSession, newSessionBlock)
		scheduler.nextSessionIndex = (scheduler.nextSessionIndex + 1) % maxSession
		log.Printf("using initial quic session: id = <%v>, nextSessionIndex = <%v>", newSessionBlock.id, scheduler.nextSessionIndex)
		return newSessionBlock, nil
	}

	// 已经创建了最大数量的 quic session，需要在已有的 quic session 中轮询
	nextSession := scheduler.openedSession[scheduler.nextSessionIndex]
	// 轮转到下一 quic session
	scheduler.nextSessionIndex = (scheduler.nextSessionIndex + 1) % maxSession
	log.Printf("using existing quic session: id = <%v>, nextSessionIndex = <%v>", nextSession.id, scheduler.nextSessionIndex)
	return nextSession, nil
}

// mayExecute 视情况决定是否执行下一请求
func (scheduler *roundRobinRequestScheduler) mayExecute() {
	if scheduler.pendingRequests >= maxParallelStreams || len(scheduler.requestQueue) < 1 {
		// 当前正在执行的请求数超过限制，或者队列中没有可供执行的请求
		return
	}

	// 获取下一需要执行的请求以及执行该请求的 quic session
	nextRequest := scheduler.requestQueue[0]
	nextSession, err := scheduler.getSession()
	if err != nil {
		log.Printf("error in getting next session: %v", err.Error())
		return
	}
	nextRequest.designatedSession = nextSession

	// 在新的 go 程中执行该请求
	go scheduler.execute(nextRequest)
}

// execute 方法在给定的 session 上执行该请求
func (scheduler *roundRobinRequestScheduler) execute(reqBlock *requestControlBlock) {
	log.Printf("session = <%v>, executing request = <%v>", reqBlock.designatedSession.id, reqBlock.request.URL.RequestURI())

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
			str.CancelWrite(quic.ErrorCode(errorRequestCanceled))
			str.CancelRead(quic.ErrorCode(errorRequestCanceled))
		case <-reqDone:
		}
	}()

	usingGzip := isUsingGzip(scheduler.roundTripperOpts.DisableCompression,
		req.Method, req.Header.Get("accept-encoding"), req.Header.Get("range"))
	resp, reqErr := getResponse(req, usingGzip, &str, &quicSession,
		scheduler.requestWriter, maxHeaderBytes(scheduler.roundTripperOpts.MaxHeaderBytes),
		scheduler.decoder, reqDone)
	if reqErr.err != nil {
		close(reqDone)
		if reqErr.streamErr != 0 {
			str.CancelWrite(quic.ErrorCode(reqErr.streamErr))
		}
		if reqErr.connErr != 0 {
			var reason string
			if reqErr.err != nil {
				reason = reqErr.err.Error()
			}
			quicSession.CloseWithError(quic.ErrorCode(reqErr.connErr), reason)
		}
	}

	reqBlock.response = resp
	*reqBlock.requestDone <- struct{}{}
	scheduler.Lock()
	scheduler.pendingRequests--
	scheduler.Unlock()
}
