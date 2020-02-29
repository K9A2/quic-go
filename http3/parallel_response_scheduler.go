package http3

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/marten-seemann/qpack"
)

// 每个 client 下最多只能开 4 个 quic 连接，相当于最多同时使用 4 条连接处理同一个请求
const maxSessions = 4

// 默认块大小
const defaultBlockSize = 32 * 1024

// 队列名称
const documentQueueIndex = 1
const styleSheetQueueIndex = 2
const scriptQueueIndex = 3
const otherQueueIndex = 4

const documentQueueFileType = "html"
const styleSheetQueueFileType = "css"
const scriptQueueFileType = "javascript"

// getQueueIndexByMimeType 根据给出的 mimeType 返回这个资源应当加入的队列序号
func getQueueIndexByMimeType(mimeType string) int {
	if strings.Contains(mimeType, documentQueueFileType) {
		return documentQueueIndex
	} else if strings.Contains(mimeType, styleSheetQueueFileType) {
		return styleSheetQueueIndex
	} else if strings.Contains(mimeType, scriptQueueFileType) {
		return scriptQueueIndex
	}
	return otherQueueIndex
}

// 是调度器中原始 quicSession 的一个封装
type sessionControlblock struct {
	id int

	session       *quic.Session // 对应的 quic 连接
	canDispatched bool          // 该连接是否能够被调度器用于承载其他请求

	dataToFetch int // 还需要加载的字节数

	// 基于最新样本计算的信道参数
	rtt       float64 // 该连接的 rtt
	bandwdith float64 // 该连接的带宽

	pendingRequest   int // 该 session 上承载的请求数目
	remainingDataLen int // 该 session 上仍需加载的数据量
}

// pseudoRequestControlBlock 用来在正式创建请求之前确定该请求应当负责的数据范围
type pseudoRequestControlBlock struct {
	// 使用此 session 加载主请求所有剩余数据所需要的时间
	timeToFinish float64
	numBlocks    int // 应当由子请求负责传输的数据块数目

	sessionBlock *sessionControlblock
	share        float64 // 应当负担的数据量占总数据量的比例
}

func newPseudoRequestControlBlock(
	sessionBlock *sessionControlblock,
	timeToFinish float64,
) *pseudoRequestControlBlock {
	return &pseudoRequestControlBlock{
		timeToFinish: timeToFinish, sessionBlock: sessionBlock,
	}
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

	response       *http.Response // 已经处理完成的 response，可以返回上层应用
	contentLength  int            // 响应体字节数
	unhandledError error          // 处理 response 过程中发生的错误

	designatedSession *sessionControlblock // 调度器指定用来承载该请求的 session
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

	maxSessionID int

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
		maxSessionID: 1,

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

// 调度器的主 go 程
func (scheduler *parallelRequestSchedulerI) run() {
	for {
		select {
		case <-scheduler.mayExecuteNextRequest:
			// 执行主请求
			nextRequest := scheduler.mayExecute()
			if nextRequest != nil {
				go scheduler.mayDoRequestParallel(nextRequest)
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

// addNewQuicSession 向调度器添加一条新的 quicSession，并返回对应的控制块
func (scheduler *parallelRequestSchedulerI) addNewQuicSession() *sessionControlblock {
	newSession, err := scheduler.getNewQuicSession()
	if err != nil {
		// 出错
		fmt.Println(err.Error())
		return nil
	}
	newSessionControlBlock := &sessionControlblock{
		id:            scheduler.maxSessionID,
		session:       newSession,
		canDispatched: true, // 初始状态为可调度状态
	}
	scheduler.openedSessions = append(scheduler.openedSessions, newSessionControlBlock)
	scheduler.maxSessionID++
	return newSessionControlBlock
}

// getSession 获取调度器中可用的 quicSession
func (scheduler *parallelRequestSchedulerI) getSession() *sessionControlblock {
	// 如果当前没有立刻可用的 session，就创建一个新的并作为结果返回
	if len(scheduler.openedSessions) <= 0 {
		// fmt.Println("return initial session")
		return scheduler.addNewQuicSession()
	}

	// 已经有了现成的 session，那么就把在这些 session 中找一个空闲的
	for _, block := range scheduler.openedSessions {
		if block.canDispatched {
			// 找到了一个处于空闲状态的 session
			// fmt.Println("found an idle session", len(scheduler.openedSessions))
			return block
		}
	}

	// 有了现成的 session，但其中没有空闲的。在仍有空位的情况下打开新的 session。
	if len(scheduler.openedSessions) < 4 {
		// fmt.Println("no idle session found, create new session")
		return scheduler.addNewQuicSession()
	}

	// 没有空闲的 session，并且不能打开新的 session
	return nil
}

// mayExecute 方法负责在调度器队列中寻找可执行的下一请求
func (scheduler *parallelRequestSchedulerI) mayExecute() *requestControlBlock {
	scheduler.mutex.Lock()
	defer scheduler.mutex.Unlock()

	// 获取可用的请求
	nextRequest, index := scheduler.popRequest()
	if nextRequest == nil {
		return nil
	}
	// 获取可用的 quic 连接
	availableSession := scheduler.getSession()
	if availableSession == nil {
		return nil
	}
	// 从调度器的待执行队列中删去即将执行的 requestControlBlock
	scheduler.removeFirst(index)
	// 把即将要使用的 session 标记为繁忙状态
	availableSession.canDispatched = false
	availableSession.pendingRequest++
	nextRequest.designatedSession = availableSession
	return nextRequest
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
	if block.request.URL.RequestURI() == "/" {
		fmt.Println("adding to document queue", block.request.URL.RequestURI())
		scheduler.documentQueue = append(scheduler.documentQueue, block)
	}
	mimeType := mime.TypeByExtension(filepath.Ext(block.request.URL.RequestURI()))
	// 有确定的 mime type，则需要根据给出的 mime type 决定加入到哪一条队列中`
	switch getQueueIndexByMimeType(mimeType) {
	case documentQueueIndex:
		fmt.Println("adding to document queue", block.request.URL.RequestURI())
		scheduler.documentQueue = append(scheduler.documentQueue, block)
	case styleSheetQueueIndex:
		fmt.Println("adding to stylesheet queue", block.request.URL.RequestURI())
		scheduler.styleSheetQueue = append(scheduler.styleSheetQueue, block)
	case scriptQueueIndex:
		fmt.Println("adding to script queue", block.request.URL.RequestURI())
		scheduler.scriptQueue = append(scheduler.scriptQueue, block)
	default:
		fmt.Println("adding to other file queue", block.request.URL.RequestURI())
		scheduler.otherFileQueue = append(scheduler.otherFileQueue, block)
	}
	// 添加后立刻发出信号给调度器主 go 程，由后者负责确定在何时发出该请求
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

// getTimeToFinish 函数计算在当前情况下，包含请求就绪在内的传输完所有数据需要的时间
func getTimeToFinish(remainingDataLen int, bandwidth float64, rtt float64, newDataLen int) float64 {
	timeToAvailable := float64(remainingDataLen) / bandwidth
	if timeToAvailable < rtt {
		timeToAvailable = rtt
	}
	timeToFinishLoadNewData := float64(newDataLen) / bandwidth
	return timeToAvailable + timeToFinishLoadNewData
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
	mainSessionID int, // 主请求所在 session 的 id
) (int, *[]*requestControlBlock) { // 主请求仍需要接受的字节数，需要打开的子请求信息（如果需要）
	if remainingDataLen < blockSize {
		// 少量数据直接使用主连接处理
		return -1, nil
	}

	// 把剩余字节数转换为剩余分段数
	remainingBlocks := int(math.Ceil(float64(remainingDataLen) / float64(blockSize)))
	// 把在下一个 rtt 中主连接能收到的字节数转换为分段数。rtt 的单位为毫秒。
	blocksInflightNextRTT := int(math.Ceil(mainSessionBandwidth * mainSessionRTT * 0.001 / float64(blockSize)))
	// 剩余可供拆分传输的块数目
	blocksToSplitted := remainingBlocks - blocksInflightNextRTT
	if blocksToSplitted <= 1 {
		// 只有一块时直接用 mainSession 处理
		return -1, nil
	}

	// 如果只用主请求传输数据，则所需时间为
	remainingTimeMainSession := float64(blocksToSplitted) * float64(blockSize) / mainSessionBandwidth
	subReqeuestSessions := make([]*pseudoRequestControlBlock, 0)

	var timeSum float64
	// 复制已经打开的 session 信息
	for _, block := range scheduler.openedSessions {
		if block.id == mainSessionID {
			// 跳过主请求所在的 session，即不在此 session 上传输主请求拆分出来的分段
			continue
		}
		// 添加伪请求控制块，避免修改原控制块
		timeToFinish := getTimeToFinish(block.remainingDataLen, block.bandwdith, block.rtt, blocksToSplitted*blockSize)
		subReqeuestSessions = append(subReqeuestSessions, newPseudoRequestControlBlock(block, timeToFinish))
		timeSum += timeToFinish
	}
	// 假设该请求仍需传输的数据需要全部 4 条 session 进行传输
	// 之后，我们将会把没有用上的伪控制块删掉
	for len(subReqeuestSessions) < maxSessions-1 {
		// 添加假设即将打开的新 sessin 的控制块。由于我们并不知道即将创建的新
		// session 的信道参数是什么，我们用 mainSession 的数据做为估计值。
		timeToFinish := getTimeToFinish(0, mainSessionBandwidth, mainSessionRTT*0.001, blocksToSplitted*blockSize)
		subReqeuestSessions = append(subReqeuestSessions, newPseudoRequestControlBlock(nil, timeToFinish))
		timeSum += timeToFinish
	}

	// 计算主请求以及所有子请求所应当承担的数据量的比例
	var shareSum float64
	mainSessionShare := 1 / (remainingTimeMainSession / timeSum)
	shareSum += mainSessionShare
	for _, pseudoReq := range subReqeuestSessions {
		pseudoReq.share = 1 / (pseudoReq.timeToFinish / timeSum)
		shareSum += pseudoReq.share
	}
	// 计算主请求以及所有子请求所应当承担的块数，最少为 1 块。块数为 0 的子请求将不会被发出。
	mainSessionBlocks := int(mainSessionShare / shareSum * float64(blocksToSplitted))
	// 计算 mainSession 的终止字节位置
	mainSessionAdjustedEndOffset := mainSessionBlocks * blockSize
	// 由子请求负责的字节数
	remainingDataSubReqs := remainingDataLen - mainSessionAdjustedEndOffset
	// 然后新建 subRequests
	subRequests := make([]*requestControlBlock, 0)
	// 子请求开始的字节位置
	startOffset := receivedBytesCount + mainSessionAdjustedEndOffset

	// 计算各子请求应当传输的数据块
	// TODO: 把数据的分布改成 raid 0 一样的条状分布
	for _, block := range subReqeuestSessions {
		if remainingDataSubReqs < 0 {
			break
		}
		numBlocks := int(math.Ceil(block.share / shareSum * float64(blocksToSplitted)))
		if numBlocks == 1 {
			// 该子请求将传送少于等于一块的数据。这一点数据由上一子请求传送，以减少一个请求。
			break
		}
		subReq := &requestControlBlock{
			url:              url,
			bytesStartOffset: startOffset,
			bytesEndOffset:   startOffset + numBlocks*blockSize - 1,
			subRequestDone:   subRequestDone,
			// 如果 designatedSession 为 nil，则调度器在执行该请求时需要打开一条新的 quicSession
			designatedSession: block.sessionBlock,
		}
		// 最多添加 4 个子请求
		subRequests = append(subRequests, subReq)
		remainingDataSubReqs -= (subReq.bytesEndOffset - subReq.bytesStartOffset)
		startOffset += numBlocks * blockSize
	}
	if len(subRequests)-1 > 0 {
		// 把最后一个子请求的终止字节数调整为全部字节
		subRequests[len(subRequests)-1].bytesEndOffset = receivedBytesCount + remainingDataLen - 1
	}

	return mainSessionAdjustedEndOffset, &subRequests
}

// mayDoRequestParallel 方法负责实际发出请求并返回响应，视情况决定是否采用并行传输以降低下载时间
func (scheduler *parallelRequestSchedulerI) mayDoRequestParallel(
	reqBlock *requestControlBlock) {
	req := reqBlock.request
	mainSession := *reqBlock.designatedSession.session
	// 打开 quic stream，开始处理该 H3 请求
	str, err := mainSession.OpenStreamSync(context.Background())
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	rsp, err := scheduler.getResponse(req, &str, &mainSession)
	if err != nil {
		fmt.Println("mayDoRequestParallel", err.Error())
		return
	}

	// 首先实验使用 2 个连接并行下载数据
	var mainBuffer bytes.Buffer // 主 go 程的 buffer
	// 读取响应体总长度，确定需要复制的总数据量，在主 go 程处值为 [0-EOF]
	remainingDataLen, err := strconv.Atoi(rsp.Header.Get("Content-Length"))
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	// 加上本次请求需要传输的数据量
	reqBlock.designatedSession.remainingDataLen += remainingDataLen

	// 循环读取响应体中的全部数据
	subRequestDone := make(chan *subRequestControlBlock)
	mainRequestURL := fmt.Sprintf("%s://%s%s", req.URL.Scheme, req.URL.Hostname(), req.URL.RequestURI())
	for remainingDataLen > 0 {
		// TODO: 实现提前一个 RTT 发起请求的功能
		if remainingDataLen <= defaultBlockSize {
			// 还有一块的传输任务，可以通知调度器下发下一个请求了
			scheduler.mayExecuteNextRequest <- struct{}{}
		}
		// 把数据写入到 mainBuffer 中
		written, bandwidth, err := readData(remainingDataLen, &mainBuffer, rsp)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		// 调整本请求和所用 session 上需要传输的数据量
		remainingDataLen -= written
		reqBlock.designatedSession.remainingDataLen -= written
		reqBlock.designatedSession.bandwdith = bandwidth
		if remainingDataLen <= 0 {
			// early stop 功能，避免小于块大小的文件进入子请求决策模块
			break
		}

		/* 子请求决策模块 */
		if !reqBlock.subRequestDispatched {
			// 尚未下发子请求时才会进入这段代码，以确定是否需要使用并行传输
			mainSessionAdjuestedEndOffset, subReqs :=
				scheduler.shoudUseParallelTransmission(
					mainRequestURL, mainBuffer.Len(), remainingDataLen, bandwidth,
					mainSession.GetConnectionRTT(), defaultBlockSize, &subRequestDone,
					reqBlock.designatedSession.id)
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
	// 读取完指定数据段之后立刻关闭这条 stream
	rsp.Body.Close()
	// 立刻把承载该请求的 session 改为可调度状态
	// TODO: 目前的实现为读取完数据之后再声明为可用，实际上要求在读取完成前一个 RTT 时即声明可用以避免网络空闲
	reqBlock.designatedSession.canDispatched = true

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
		StatusCode: rsp.StatusCode,
		Header:     rsp.Header.Clone(),
		Body:       respBody,
	}
	reqBlock.response = finalResponse
	*reqBlock.requestDone <- struct{}{}
	return
}

// readData 方法负责读取数据，并返回读取的字节数和带宽
func readData(remainingDataLen int, buf *bytes.Buffer, rsp *http.Response) (int, float64, error) {
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
		if err == io.EOF {
			return int(written), 0, nil
		}
	}
	timeConsumed := timeEnd.Sub(timeStart).Milliseconds()
	// 带宽的单位为 Bps，所以需要通过乘 1000 来抵消掉用毫秒作为计时单位的 1000 倍缩小
	bandwidth := float64(written) / float64(timeConsumed) * 1000.0
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

	if reqBlock.designatedSession == nil {
		// 调度器没有为该请求分配 session，需要在执行的时候现开一条新的 session
		reqBlock.designatedSession = scheduler.getSession()
	}
	session := *reqBlock.designatedSession.session

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
	// data, err := ioutil.ReadAll(resp.Body)
	dataBuffer := bytes.Buffer{}
	remainingDataLen, err := strconv.Atoi(resp.Header.Get("Content-Length"))
	if err != nil {
		fmt.Println("subReq error", err.Error())
	}
	for remainingDataLen > 0 {
		// TODO: 实现提前一个 RTT 发起请求的功能
		if remainingDataLen <= defaultBlockSize {
			// 发送信号给调度器以触发下一请求
			scheduler.mayExecuteNextRequest <- struct{}{}
		}
		// 把数据写入到 mainBuffer 中
		written, bandwidth, err := readData(remainingDataLen, &dataBuffer, resp)
		if err != nil {
			fmt.Println(err.Error())
			// fmt.Println("fuck")
			return
		}

		// 调整本请求和所用 session 上需要传输的数据量
		remainingDataLen -= written
		reqBlock.designatedSession.remainingDataLen -= written
		// 更新读取本分段的平均带宽
		reqBlock.designatedSession.bandwdith = bandwidth
	}

	resp.Body.Close()

	controlBlock := &subRequestControlBlock{
		data:          dataBuffer.Bytes(),
		contentLength: dataBuffer.Len(),
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
