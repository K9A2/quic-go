package http3

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/marten-seemann/qpack"
)

// 每个 client 下最多只能开 4 个 quic 连接，相当于最多同时使用 4 条连接处理同一个请求
const maxSessions = 4

// 默认块大小
const defaultBlockSize = 128 * 1024

// 队列名称
const documentQueueIndex = 1
const styleSheetQueueIndex = 2
const scriptQueueIndex = 3
const otherQueueIndex = 4

// 是调度器中原始 quicSession 的一个封装
type sessionControlblock struct {
	session *quic.Session // 对应的 quic 连接
	idle    bool          // 该连接是否处于空闲状态

	dataToFetch int // 还需要加载的字节数

	// 基于最新样本计算的信道参数
	rtt       float64 // 该连接的 rtt
	bandwdith float64 // 该连接的带宽
}

type requestControlBlock struct {
	isMainSession        bool // 是否为主请求
	bytesStartOffset     int  // 响应体开始位置，用于让主请求拼装为完成的请求
	bytesEndOffset       int  // 响应体结束位置，同上
	subRequestDispatched bool // 是否已经下发子请求

	url            string                        // 请求的 url，只在子请求是使用
	request        *http.Request                 // 对应的 http 请求
	requestDone    *chan struct{}                // 调度器完成该 http 请求时向该 chan 发送消息
	subRequestDone *chan *subRequestControlBlock // 子请求完成时向该 chan 发送消息
	// usingGizp      bool                          // 该请求是否使用 gzip

	response       *http.Response // 已经处理完成的 response，可以返回上层应用
	contentLength  int            // 响应体字节数
	unhandledError error          // 处理 response 过程中发生的错误
}

type subRequestControlBlock struct {
	data          []byte // 子请求所获取的数据
	contentLength int    // 子请求所获得的数据的字节数
	startOffset   int    // 子请求所获得的数据相对于主请求的开始字节数
	endOffset     int    // 子请求所获得的数据相对于主请求的终止字节数
}

type parallelRequestScheduler interface {
	addAndWait(*http.Request) (*http.Response, error)
	close() error
	run()
}

type parallelRequestSchedulerI struct {
	mutex sync.Mutex

	// 原 client 类定义的变量
	hostname         string // 此调度器负责的域
	tlsConfig        *tls.Config
	quicConfig       *quic.Config
	requestWriter    *requestWriter
	decoder          *qpack.Decoder
	roundTripperOpts *roundTripperOpts

	// session 管理部分
	openedSessions []*sessionControlblock // 已经打开的 quic 连接
	idleSession    int                    // 当前处于空闲状态的 quic 连接

	// request 管理部分
	documentQueue   []*requestControlBlock // html 文件请求队列
	styleSheetQueue []*requestControlBlock // css 文件请求队列
	scriptQueue     []*requestControlBlock // js 文件请求队列
	otherFileQueue  []*requestControlBlock // 其他文件请求队列

	mayExecuteNextRequest chan struct{}                 // 有任何可能实际执行下一请求时都向此 chan 中发送信号
	subRequestsChan       *chan *[]*requestControlBlock // 如果需要发送子请求，就把子请求的信息发送到这个 chan 中
}

func newParallelRequestScheduler(hostname string, tlsConfig *tls.Config, quicConfig *quic.Config,
	requestWriter *requestWriter, decoder *qpack.Decoder, roundTripperOpts *roundTripperOpts) parallelRequestScheduler {

	subRequestsChan := make(chan *[]*requestControlBlock)
	return &parallelRequestSchedulerI{
		// 初始化来自原 client 实例定义的变量
		hostname:         hostname,
		tlsConfig:        tlsConfig,
		quicConfig:       quicConfig,
		requestWriter:    requestWriter,
		decoder:          decoder,
		roundTripperOpts: roundTripperOpts,

		// 同一 domain 下最多只能打开 4 条 quic 连接
		openedSessions: make([]*sessionControlblock, 0, maxSessions),
		idleSession:    0,

		documentQueue:   make([]*requestControlBlock, 0),
		styleSheetQueue: make([]*requestControlBlock, 0),
		scriptQueue:     make([]*requestControlBlock, 0),
		otherFileQueue:  make([]*requestControlBlock, 0),

		mayExecuteNextRequest: make(chan struct{}),
		subRequestsChan:       &subRequestsChan,
	}
}

func (scheduler *parallelRequestSchedulerI) run() {
	for {
		select {
		case <-scheduler.mayExecuteNextRequest:
			// 执行主请求
			nextRequest, session := scheduler.mayExecute()
			if nextRequest != nil {
				go scheduler.mayDoRequestParallel(nextRequest, session)
			}
		case subRequests := <-*scheduler.subRequestsChan:
			// 执行子请求
			for _, subReq := range *subRequests {
				go scheduler.executeSubRequest(subReq)
			}
		}
	}
}

// popRequest 检查调度器队列中是否有需要处理的请求，并给出该就绪的请求和所在队列序号
func (scheduler *parallelRequestSchedulerI) popRequest() (*requestControlBlock, int) {
	var nextRequest *requestControlBlock
	var queueIndex int
	if len(scheduler.documentQueue) > 0 {
		nextRequest = scheduler.documentQueue[0]
		queueIndex = documentQueueIndex
	} else if len(scheduler.styleSheetQueue) > 0 {
		nextRequest = scheduler.styleSheetQueue[0]
		queueIndex = styleSheetQueueIndex
	} else if len(scheduler.scriptQueue) > 0 {
		nextRequest = scheduler.scriptQueue[0]
		queueIndex = scriptQueueIndex
	} else if len(scheduler.otherFileQueue) > 0 {
		nextRequest = scheduler.otherFileQueue[0]
		queueIndex = otherQueueIndex
	}
	return nextRequest, queueIndex
}

// removeFirst 移除指定队列的第一个元素
func (scheduler *parallelRequestSchedulerI) removeFirst(index int) {
	switch index {
	case documentQueueIndex:
		scheduler.documentQueue = scheduler.documentQueue[1:]
	case styleSheetQueueIndex:
		scheduler.styleSheetQueue = scheduler.styleSheetQueue[1:]
	case scriptQueueIndex:
		scheduler.scriptQueue = scheduler.scriptQueue[1:]
	case otherQueueIndex:
		scheduler.otherFileQueue = scheduler.otherFileQueue[1:]
	}
}

// getNewQuicSession 方法创建并返回一条新的 quicSession
func (scheduler *parallelRequestSchedulerI) getNewQuicSession() (*quic.Session, error) {
	// 建立一个新的 quicSession
	newSession, err := scheduler.dial()
	if err != nil {
		return nil, err
	}
	return newSession, nil
}

// getSession 获取调度器中可用的 quicSession
// 选取 session 的几个标准依次是：
// 1. session 空闲
func (scheduler *parallelRequestSchedulerI) getSession() *sessionControlblock {
	// 如果当前没有立刻可用的 session，就创建一个新的并作为结果返回
	if len(scheduler.openedSessions) <= 0 {
		newSession, err := scheduler.getNewQuicSession()
		if err != nil {
			// 出错
			fmt.Println(err.Error())
			return nil
		}
		scheduler.openedSessions = append(scheduler.openedSessions, &sessionControlblock{
			session: newSession, idle: true,
		})
		scheduler.idleSession++
		newSessionControlBlock := &sessionControlblock{
			session: newSession,
			idle:    false,
		}
		return newSessionControlBlock
	}

	// 已经有了现成的 session，那么就把在这些 session 中找一个空闲的
	for _, block := range scheduler.openedSessions {
		if block.idle {
			// 找到了一个处于空闲状态的 session
			return block
		}
	}

	// 有了现成的 session，但其中没有空闲的
	if len(scheduler.openedSessions) < 4 {
		// 还可以开新的 session
		newSession, err := scheduler.getNewQuicSession()
		if err != nil {
			// 出错
			fmt.Println(err.Error())
			return nil
		}
		scheduler.openedSessions = append(scheduler.openedSessions, &sessionControlblock{
			session: newSession, idle: true,
		})
		scheduler.idleSession++
		newSessionControlBlock := &sessionControlblock{
			session: newSession,
			idle:    false,
		}
		return newSessionControlBlock
	}

	// 没有空闲的 session，并且不能打开新的 session
	return nil
}

// mayExecute 方法负责在调度器队列中寻找可执行的下一请求
func (scheduler *parallelRequestSchedulerI) mayExecute() (*requestControlBlock, *sessionControlblock) {
	scheduler.mutex.Lock()
	defer scheduler.mutex.Unlock()

	// 获取可用的请求
	nextRequest, index := scheduler.popRequest()
	if nextRequest == nil {
		return nil, nil
	}
	// 获取可用的 quic 连接
	availableSession := scheduler.getSession()
	if availableSession == nil {
		return nil, nil
	}
	// 从调度器的待执行队列中删去即将执行的 requestControlBlock
	scheduler.removeFirst(index)
	// 把即将要使用的 session 标记为繁忙状态
	availableSession.idle = false
	return nextRequest, availableSession
}

// close 方法拆除所辖的全部 quicSession 并关闭此调度器
func (scheduler *parallelRequestSchedulerI) close() error {
	scheduler.mutex.Lock()
	defer scheduler.mutex.Unlock()
	var err error
	for _, block := range scheduler.openedSessions {
		session := *block.session
		err = session.Close()
		if err != nil {
			fmt.Println(err.Error())
			return err
		}
	}
	return nil
}

// setupH3Session 方法在传输的 quicSession 上初始化 H3 连接
func setupH3Session(quicSession *quic.Session) error {
	// 建立单向控制 stream
	controlStream, err := (*quicSession).OpenUniStream()
	if err != nil {
		return err
	}
	buf := &bytes.Buffer{}
	// write the type byte
	buf.Write([]byte{0x0})
	// send the SETTINGS frame
	(&settingsFrame{}).Write(buf)
	if _, err := controlStream.Write(buf.Bytes()); err != nil {
		return err
	}
	return nil
}

// dial 方法负责和目标服务器打开一条新的连接
func (scheduler *parallelRequestSchedulerI) dial() (*quic.Session, error) {
	var err error
	var quicSession quic.Session
	quicSession, err = dialAddr(scheduler.hostname, scheduler.tlsConfig, scheduler.quicConfig)
	if err != nil {
		return nil, err
	}

	go func() {
		// 在另外一个 go 程中打开控制 stream
		if err := setupH3Session(&quicSession); err != nil {
			fmt.Printf("Setting up session failed: %s\n", err)
			quicSession.CloseWithError(quic.ErrorCode(errorInternalError), "")
		}
	}()

	// 返回创建的新 quicSession
	return &quicSession, nil
}

func (scheduler *parallelRequestSchedulerI) addNewRequest(block *requestControlBlock) {
	scheduler.mutex.Lock()
	defer scheduler.mutex.Unlock()
	// 现在默认添加到 document 队列中
	// TODO: 按照请求类型添加到不同的队列中
	scheduler.documentQueue = append(scheduler.documentQueue, block)
	// 添加后立刻发出信号给调度器主 go 程
	scheduler.mayExecuteNextRequest <- struct{}{}
}

// addAndWait 方法负责把收到的请求添加到调度器队列中，并在调度器处理完成之后返回响应
func (scheduler *parallelRequestSchedulerI) addAndWait(req *http.Request) (*http.Response, error) {
	var requestDone = make(chan struct{}, 0)
	resBlock := requestControlBlock{
		isMainSession: true,
		request:       req,
		requestDone:   &requestDone,
	}
	scheduler.addNewRequest(&resBlock)

	for {
		select {
		case <-*resBlock.requestDone:
			fmt.Println("request done:", req.URL.Hostname()+req.URL.RequestURI())
			return resBlock.response, resBlock.unhandledError
		}
	}
}

// shoudUseParallelTransmission 负责根据调度器实际运行情况决定是否需要多开连接以
// 降低总传输时间。若需要打开多个子连接以并行传输时，返回值的第一项是主连接仍需要
// 接收的字节数，主连接在接受完指定的分段之后就可以断掉连接了。返回值的第二项是需要
// 新开的个子请求信息。
func (scheduler *parallelRequestSchedulerI) shoudUseParallelTransmission(
	url string, // 主请求的 url
	receivedBytesCount int, // 主请求已经接受的字节数
	remainingDataLen int, // 主请求仍需要接受的字节数
	mainSessionBandwidth float64, // 当前主连接的带宽
	mainSessionRTT float64, // 当前主连接的 RTT
	blockSize int, // 统一使用的块大小
	subRequestDone *chan *subRequestControlBlock, // 子请求完成时向该 chan 发送信息
) (int, *[]*requestControlBlock) { // 主请求仍需要接受的字节数，需要打开的子请求信息（如果需要）
	// 把剩余字节数转换为剩余分段数
	remainingBlocks := int(math.Ceil(float64(remainingDataLen) / float64(blockSize)))
	// 把在下一个 rtt 中主连接能收到的字节数转换为分段数
	blocksInflightNextRTT := int(math.Ceil(mainSessionBandwidth * mainSessionRTT / float64(blockSize)))
	// 可供拆分传输的块数目
	blocksToSplitted := remainingBlocks - blocksInflightNextRTT
	if blocksToSplitted <= 1 {
		// 只有一块时直接用 mainSession 处理
		return -1, nil
	}
	// 有必要在多个 session 上进行并行传输，计算需要发送的子请求数目
	// TODO: 在多请求并发是按照实际情况返回所需的 session 数目，这里是只处理一个请求的演示
	numSubRequests := int(math.Min(maxSessions-1, float64(blocksToSplitted)))
	// 然后新建 subRequests
	subRequests := make([]*requestControlBlock, 0)
	// 计算包括主请求在内的所有请求上，每个请求应当分担的数据量
	blocksPerSession := blocksToSplitted / (numSubRequests + 1)
	// 计算 mainSession 的终止字节位置
	mainSessionAdjustedEndOffset := (blocksInflightNextRTT + blocksPerSession) * blockSize

	// 由子请求负责的字节数
	remainingDataSubReqs := remainingDataLen - mainSessionAdjustedEndOffset
	// 子请求开始的字节位置
	startOffset := receivedBytesCount + mainSessionAdjustedEndOffset
	// 添加子请求，有可能出现部分请求字节数分配不均的情况。最多添加 4 个子请求
	for i := 0; i < maxSessions-1; i++ {
		if remainingDataSubReqs < 0 {
			break
		}
		// 最多添加 4 个子请求
		subReq := &requestControlBlock{
			url:              url,
			bytesStartOffset: startOffset,
			subRequestDone:   subRequestDone,
		}
		subRequests = append(subRequests, subReq)
		if len(subRequests) == 3 {
			// 把最后一个 session 的终止字节数调整为全部字节
			subRequests[len(subRequests)-1].bytesEndOffset = receivedBytesCount + remainingDataLen - 1
		} else {
			// 其余 session 的终止字节数设置为对应负责区块的终止字节数
			subRequests[len(subRequests)-1].bytesEndOffset = startOffset + blocksPerSession*blockSize - 1
		}
		remainingDataSubReqs -= (subReq.bytesEndOffset - subReq.bytesStartOffset)
		startOffset += blocksPerSession * blockSize
	}
	return mainSessionAdjustedEndOffset, &subRequests
}

// mayDoRequestParallel 方法负责实际发出请求并返回响应，视情况决定是否采用并行传输以降低下载时间
func (scheduler *parallelRequestSchedulerI) mayDoRequestParallel(
	reqBlock *requestControlBlock, sessBlock *sessionControlblock) {
	req := reqBlock.request
	mainSession := *sessBlock.session
	// 打开 quic stream，开始处理该 H3 请求
	str, err := mainSession.OpenStreamSync(context.Background())
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// Request Cancellation:
	// This go routine keeps running even after RoundTrip() returns.
	// It is shut down when the application is done processing the body.
	reqDone := make(chan struct{})
	go func() {
		select {
		case <-req.Context().Done():
			// 中途取消接受数据的方式
			str.CancelWrite(quic.ErrorCode(errorRequestCanceled))
			str.CancelRead(quic.ErrorCode(errorRequestCanceled))
		case <-reqDone:
		}
	}()

	rsp, rerr := scheduler.execute(req, str, mainSession, reqDone)
	if rerr.err != nil { // if any error occurred
		close(reqDone)
		if rerr.streamErr != 0 { // if it was a stream error
			str.CancelWrite(quic.ErrorCode(rerr.streamErr))
		}
		if rerr.connErr != 0 { // if it was a connection error
			var reason string
			if rerr.err != nil {
				reason = rerr.err.Error()
			}
			mainSession.CloseWithError(quic.ErrorCode(rerr.connErr), reason)
		}
	}
	defer rsp.Body.Close()

	// 首先实验使用 2 个连接并行下载数据
	var mainBuffer bytes.Buffer // 主 go 程的 buffer
	// 读取响应体总长度，确定需要复制的总数据量，在主 go 程处值为 [0-EOF]
	remainingDataLen, err := strconv.Atoi(rsp.Header.Get("Content-Length"))
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// 循环读取响应体中的全部数据
	subRequestDone := make(chan *subRequestControlBlock)
	mainRequestURL := fmt.Sprintf("%s://%s%s", req.URL.Scheme, req.URL.Hostname(), req.URL.RequestURI())
	for remainingDataLen > 0 {
		// 把数据写入到 mainBuffer 中
		written, bandwidth, err := fetch(remainingDataLen, &mainBuffer, rsp)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		remainingDataLen -= int(written)
		if !reqBlock.subRequestDispatched {
			// 尚未下发子请求时才会进入这段代码
			mainSessionAdjuestedEndOffset, subReqs :=
				scheduler.shoudUseParallelTransmission(
					mainRequestURL, mainBuffer.Len(), remainingDataLen, bandwidth,
					mainSession.GetConnectionRTT(), defaultBlockSize, &subRequestDone)
			if subReqs == nil {
				// 无需进行并行传输
				continue
			}
			// 把主请求标记为子请求已发送状态
			reqBlock.subRequestDispatched = true
			remainingDataLen = mainSessionAdjuestedEndOffset
			// 把需要开始的子请求发送到调度器
			*scheduler.subRequestsChan <- subReqs
		}
	}

	// 把主请求读取的数据添加到响应体中
	respBody := newSegmentedResponseBody()
	data := mainBuffer.Bytes()
	respBody.addData(&data, 0)
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// 如果主请求下发了子请求，那么也需要等待子请求完成并把所有数据添加到响应体中
	if reqBlock.subRequestDispatched {
		bytesTransferredBySubRequests, err := strconv.Atoi(rsp.Header.Get("Content-Length"))
		if err != nil {
			fmt.Println("ator error:", err.Error())
			return
		}
		bytesTransferredBySubRequests -= (mainBuffer.Len() + 1)
		for bytesTransferredBySubRequests > 0 {
			select {
			case subReq := <-subRequestDone:
				respBody.addData(&subReq.data, subReq.startOffset)
				bytesTransferredBySubRequests -= subReq.contentLength
			}
		}
	}
	// 把零散的数据区合并为统一的分段
	respBody.consolidate()

	// 最终返回该响应
	finalResponse := &http.Response{
		Proto:      "HTPP/3",
		ProtoMajor: 3,
		Header:     rsp.Header.Clone(),
		Body:       respBody,
	}
	reqBlock.response = finalResponse
	*reqBlock.requestDone <- struct{}{}
	return
}

// fetch 方法负责读取数据，并返回读取的字节数和带宽
func fetch(remainingDataLen int, buf *bytes.Buffer, rsp *http.Response) (int, float64, error) {
	var dataLenToCopy int
	// 确定本次需要从响应体复制出多少个字节的数据
	if remainingDataLen >= defaultBlockSize {
		// 响应体比默认块大小更大
		dataLenToCopy = defaultBlockSize
	} else {
		// 响应体比默认块大小更小
		dataLenToCopy = remainingDataLen
	}

	timeStart := time.Now()
	written, err := io.CopyN(buf, rsp.Body, int64(dataLenToCopy))
	timeEnd := time.Now()
	if err != nil {
		fmt.Println(err.Error())
		if err == io.EOF {
			return int(written), 0, nil
		}
	}
	timeConsumed := timeEnd.Sub(timeStart).Milliseconds()
	bandwidth := float64(written) / float64(timeConsumed)
	return int(written), bandwidth, nil
}

// execute 方法负责在给定的 quicStream 上执行单一的一个请求
func (scheduler *parallelRequestSchedulerI) execute(
	req *http.Request,
	str quic.Stream,
	quicSession quic.Session,
	reqDone chan struct{},
) (*http.Response, requestError) {
	// 是否使用 gzip 压缩
	var requestGzip bool
	if !scheduler.roundTripperOpts.DisableCompression && req.Method != "HEAD" &&
		req.Header.Get("Accept-Encoding") == "" && req.Header.Get("Range") == "" {
		requestGzip = true
	}

	// 发送请求
	if err := scheduler.requestWriter.WriteRequest(str, req, requestGzip); err != nil {
		return nil, newStreamError(errorInternalError, err)
	}

	// 开始接受对端返回的数据
	frame, err := parseNextFrame(str)
	if err != nil {
		return nil, newStreamError(errorFrameError, err)
	}
	// 确定第一帧是否为 H3 协议规定的 HEADER 帧
	hf, ok := frame.(*headersFrame)
	if !ok {
		return nil, newConnError(errorFrameUnexpected, errors.New("expected first frame to be a HEADERS frame"))
	}
	if hf.Length > scheduler.maxHeaderBytes() {
		return nil, newStreamError(errorFrameError, fmt.Errorf("HEADERS frame too large: %d bytes (max: %d)", hf.Length, scheduler.maxHeaderBytes()))
	}
	// 读取并解析 HEADER 帧
	headerBlock := make([]byte, hf.Length)
	if _, err := io.ReadFull(str, headerBlock); err != nil {
		return nil, newStreamError(errorRequestIncomplete, err)
	}
	// 调用 qpack 解码 HEADER 帧
	hfs, err := scheduler.decoder.DecodeFull(headerBlock)
	if err != nil {
		// TODO: use the right error code
		return nil, newConnError(errorGeneralProtocolError, err)
	}

	// 构造响应体
	res := &http.Response{
		Proto:      "HTTP/3",
		ProtoMajor: 3,
		Header:     http.Header{},
	}

	// 把从 HEADER 帧中解码到的信息复制到标准 HTTP 响应头中
	for _, hf := range hfs {
		switch hf.Name {
		case ":status":
			// 完成 status 字段的解析
			status, err := strconv.Atoi(hf.Value)
			if err != nil {
				return nil, newStreamError(errorGeneralProtocolError, errors.New("malformed non-numeric status pseudo header"))
			}
			res.StatusCode = status
			res.Status = hf.Value + " " + http.StatusText(status)
		default:
			res.Header.Add(hf.Name, hf.Value)
		}
	}

	// 新建一个空白响应体
	respBody := newResponseBody(str, reqDone, func() {
		quicSession.CloseWithError(quic.ErrorCode(errorFrameUnexpected), "")
	})
	// 根据是否需要 gzip 来实际构造响应体
	if requestGzip && res.Header.Get("Content-Encoding") == "gzip" {
		res.Header.Del("Content-Encoding")
		res.Header.Del("Content-Length")
		res.ContentLength = -1
		res.Body = newGzipReader(respBody)
		res.Uncompressed = true
	} else {
		res.Body = respBody
	}

	return res, requestError{}
}

// executeSubRequest 负责在执行子请求，并在子请求执行完成的时候向指定的 chan 中发送读取到的全部数据
func (scheduler *parallelRequestSchedulerI) executeSubRequest(reqBlock *requestControlBlock) {
	subRequest, err := http.NewRequest(http.MethodGet, reqBlock.url, nil)
	if err != nil {
		fmt.Println(err.Error())
	}
	subRequest.Header.Add(
		"Range", fmt.Sprintf("bytes=%d-%d", reqBlock.bytesStartOffset, reqBlock.bytesEndOffset))

	session := *scheduler.getSession().session
	// 打开 quic stream，开始处理该 H3 请求
	str, err := session.OpenStreamSync(context.Background())
	if err != nil {
		fmt.Println("executeSubRequest", err.Error())
		return
	}
	resp, err := scheduler.getResponse(subRequest, &str, &session)
	if err != nil {
		fmt.Println("executeSubRequest", err.Error())
		return
	}
	defer resp.Body.Close()
	data, err := ioutil.ReadAll(resp.Body)
	controlBlock := &subRequestControlBlock{
		data:          data,
		contentLength: len(data),
		startOffset:   reqBlock.bytesStartOffset,
		endOffset:     reqBlock.bytesEndOffset,
	}
	*reqBlock.subRequestDone <- controlBlock
}

// 指定的 session 和 stream 上获取执行请求并获取响应体
func (scheduler *parallelRequestSchedulerI) getResponse(req *http.Request, stream *quic.Stream, session *quic.Session) (*http.Response, error) {
	str := *stream
	sess := *session
	// Request Cancellation:
	// This go routine keeps running even after RoundTrip() returns.
	// It is shut down when the application is done processing the body.
	reqDone := make(chan struct{})
	go func() {
		select {
		case <-req.Context().Done():
			// 中途取消接受数据的方式
			str.CancelWrite(quic.ErrorCode(errorRequestCanceled))
			str.CancelRead(quic.ErrorCode(errorRequestCanceled))
		case <-reqDone:
		}
	}()

	rsp, rerr := scheduler.execute(req, str, sess, reqDone)
	if rerr.err != nil { // if any error occurred
		close(reqDone)
		if rerr.streamErr != 0 { // if it was a stream error
			str.CancelWrite(quic.ErrorCode(rerr.streamErr))
		}
		if rerr.connErr != 0 { // if it was a connection error
			var reason string
			if rerr.err != nil {
				reason = rerr.err.Error()
			}
			sess.CloseWithError(quic.ErrorCode(rerr.connErr), reason)
		}
	}
	return rsp, nil
}

// maxHeaderBytes 返回该 client 可以接受的最大请求头字节数
func (scheduler *parallelRequestSchedulerI) maxHeaderBytes() uint64 {
	if scheduler.roundTripperOpts.MaxHeaderBytes <= 0 {
		return defaultMaxResponseHeaderBytes
	}
	return uint64(scheduler.roundTripperOpts.MaxHeaderBytes)
}
