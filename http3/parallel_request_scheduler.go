package http3

import (
	"bytes"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/lucas-clemente/quic-go"
	"github.com/marten-seemann/qpack"
)

type parallelRequestScheduler struct {
	mutex *sync.Mutex

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

	mayExecuteNextRequest *chan struct{}                // 有任何可能实际执行下一请求时都向此 chan 中发送信号
	subRequestsChan       *chan *[]*requestControlBlock // 如果需要发送子请求，就把子请求的信息发送到这个 chan 中
}

// newParallelRequestScheduler 初始化并返回新生成的调度器 parallelRequestScheduler 实例
func newParallelRequestScheduler(info *clientInfo) requestScheduler {

	subRequestsChan := make(chan *[]*requestControlBlock, 10)
	// log.Printf("original subRequestChan address = <%v>", &subRequestsChan)
	mutex := sync.Mutex{}

	mayExecuteNextRequetChan := make(chan struct{}, 10)
	// log.Printf("original mayExecuteNextRequetChan address = <%v>", &mayExecuteNextRequetChan)
	return &parallelRequestScheduler{
		mutex: &mutex,

		maxSessionID: 1,

		// 初始化来自原 client 实例定义的变量
		hostname:         info.hostname,
		tlsConfig:        info.tlsConfig,
		quicConfig:       info.quicConfig,
		requestWriter:    info.requestWriter,
		decoder:          info.decoder,
		roundTripperOpts: info.roundTripperOpts,

		// 同一 domain 下最多只能打开 4 条 quic 连接
		openedSessions: make([]*sessionControlblock, 0, maxConcurrentSessions),
		idleSession:    0,

		documentQueue:   make([]*requestControlBlock, 0),
		styleSheetQueue: make([]*requestControlBlock, 0),
		scriptQueue:     make([]*requestControlBlock, 0),
		otherFileQueue:  make([]*requestControlBlock, 0),

		// mayExecuteNextRequest: make(chan struct{}),
		mayExecuteNextRequest: &mayExecuteNextRequetChan,
		subRequestsChan:       &subRequestsChan,
	}
}

// 调度器的主 go 程
func (scheduler *parallelRequestScheduler) run() {
	for {
		select {
		case <-*scheduler.mayExecuteNextRequest:
			// 执行主请求
			nextRequest, err := scheduler.mayExecute()
			if err == nil {
				go scheduler.mayDoRequestParallel(nextRequest)
			}
			// log.Println("case 1 finished")
		case subRequests := <-*scheduler.subRequestsChan:
			// FIXME: 会受到长度为 0 的数据，go 环境 panic：index out of range [0] with length 0
			// log.Printf("sub request received: url = <%v>, num = <%v>", (*subRequests)[0].url, len(*subRequests))
			// 执行子请求
			for _, subReq := range *subRequests {
				go scheduler.executeSubRequest(subReq)
			}
			// log.Println("case 2 finished")
		}
	}
}

// popRequest 检查调度器队列中是否有需要处理的请求，并给出该就绪的请求和所在队列序号
func (scheduler *parallelRequestScheduler) popRequest() (*requestControlBlock, int) {
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
func (scheduler *parallelRequestScheduler) removeFirst(index int) {
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
func (scheduler *parallelRequestScheduler) getNewQuicSession() (*quic.Session, error) {
	// 建立一个新的 quicSession
	newSession, err := dial(scheduler.hostname, scheduler.tlsConfig, scheduler.quicConfig)
	if err != nil {
		return nil, err
	}
	return newSession, nil
}

// addNewQuicSession 向调度器添加一条新的 quicSession，并返回对应的控制块
func (scheduler *parallelRequestScheduler) addNewQuicSession() (*sessionControlblock, error) {
	newSession, err := scheduler.getNewQuicSession()
	if err != nil {
		// 出错，可能是 404 等错误
		return nil, errHostNotConnected
	}
	newSessionBlock := newSessionControlBlock(scheduler.maxSessionID, newSession, true)
	scheduler.openedSessions = append(scheduler.openedSessions, newSessionBlock)
	scheduler.maxSessionID++
	return newSessionBlock, nil
}

// getSession 获取调度器中可用的 quicSession
func (scheduler *parallelRequestScheduler) getSession() (*sessionControlblock, error) {
	// 如果当前没有立刻可用的 session，就创建一个新的并作为结果返回
	if len(scheduler.openedSessions) <= 0 {
		newSessionControlBlock, err := scheduler.addNewQuicSession()
		if err != nil {
			return nil, errHostNotConnected
		}
		newSessionControlBlock.setBusy()
		// log.Printf("return initial session: id = %v, len = %v", newSessionControlBlock.id, len(scheduler.openedSessions))
		return newSessionControlBlock, nil
	}

	// 已经有了现成的 session，那么就把在这些 session 中找一个空闲的
	for _, block := range scheduler.openedSessions {
		if block.dispatchable() {
			// 找到了一个处于空闲状态的 session
			// log.Printf("found an idle session: id = %v, len = %v", block.id, len(scheduler.openedSessions))
			block.setBusy()
			return block, nil
		}
	}

	// 有了现成的 session，但其中没有空闲的。在仍有空位的情况下打开新的 session。
	if len(scheduler.openedSessions) < maxConcurrentSessions {
		newSessionControlBlock, err := scheduler.addNewQuicSession()
		if err != nil {
			return nil, errHostNotConnected
		}
		newSessionControlBlock.setBusy()
		// log.Printf("create and return a new session: id = %v, len = %v", newSessionControlBlock.id, len(scheduler.openedSessions))
		return newSessionControlBlock, nil
	}

	// 没有空闲的 session，并且不能打开新的 session
	return nil, errNoAvailableSession
}

// mayExecute 方法负责在调度器队列中寻找可执行的下一请求
func (scheduler *parallelRequestScheduler) mayExecute() (*requestControlBlock, error) {
	scheduler.mutex.Lock()
	defer scheduler.mutex.Unlock()

	// 获取可用的请求
	nextRequest, index := scheduler.popRequest()
	if nextRequest == nil {
		return nil, errNoAvailableRequest
	}
	// 获取可用的 quic 连接
	availableSession, err := scheduler.getSession()
	if err != nil {
		return nil, errNoAvailableSession
	}
	// 从调度器的待执行队列中删去即将执行的 requestControlBlock
	scheduler.removeFirst(index)
	// 把即将要使用的 session 标记为繁忙状态
	// log.Printf("session = <%v> setBusy() to execute <%v>, value = <%v>, remainingDataLen = <%v>", availableSession.id, nextRequest.request.URL.RequestURI(), availableSession.canDispatched, availableSession.getRemainingDataLen())
	availableSession.pendingRequest++
	nextRequest.designatedSession = availableSession
	return nextRequest, nil
}

// close 方法拆除所辖的全部 quicSession 并关闭此调度器
func (scheduler *parallelRequestScheduler) close() error {
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

func (scheduler *parallelRequestScheduler) addNewRequest(block *requestControlBlock) {
	scheduler.mutex.Lock()
	defer scheduler.mutex.Unlock()
	if block.request.URL.RequestURI() == "/" {
		// log.Printf("adding to document queue: %v", block.request.URL.RequestURI())
		scheduler.documentQueue = append(scheduler.documentQueue, block)
	}
	mimeType := mime.TypeByExtension(filepath.Ext(block.request.URL.RequestURI()))
	// 有确定的 mime type，则需要根据给出的 mime type 决定加入到哪一条队列中`
	switch getQueueIndexByMimeType(mimeType) {
	case documentQueueIndex:
		// log.Printf("adding to document queue: %v", block.request.URL.RequestURI())
		scheduler.documentQueue = append(scheduler.documentQueue, block)
	case styleSheetQueueIndex:
		// log.Printf("adding to stylesheet queue: %v", block.request.URL.RequestURI())
		scheduler.styleSheetQueue = append(scheduler.styleSheetQueue, block)
	case scriptQueueIndex:
		// log.Printf("adding to script queue: %v", block.request.URL.RequestURI())
		scheduler.scriptQueue = append(scheduler.scriptQueue, block)
	default:
		// log.Printf("adding to other file queue: %v", block.request.URL.RequestURI())
		scheduler.otherFileQueue = append(scheduler.otherFileQueue, block)
	}
	// 添加后立刻发出信号给调度器主 go 程，由后者负责确定在何时发出该请求
	*scheduler.mayExecuteNextRequest <- struct{}{}
}

// addAndWait 方法负责把收到的请求添加到调度器队列中，并在调度器处理完成之后返回响应
func (scheduler *parallelRequestScheduler) addAndWait(req *http.Request) (*http.Response, error) {
	var requestDone = make(chan struct{}, 0)
	var requestError = make(chan struct{}, 0)
	reqBlock := requestControlBlock{
		isMainSession:                true,
		request:                      req,
		requestDone:                  &requestDone,
		requestError:                 &requestError,
		shoudUseParallelTransmission: true, // 把该字段设为 true 可以让该请求至少进行一次并行传输决策
	}
	scheduler.addNewRequest(&reqBlock)

	for {
		select {
		case <-*reqBlock.requestError:
			// 请求错误统一按照 404 Not Found 处理
			return getErrorResponse(req), nil
		case <-*reqBlock.requestDone:
			// log.Printf("request done: %v", req.URL.Hostname()+req.URL.RequestURI())
			return reqBlock.response, reqBlock.unhandledError
		}
	}
}

// shoudUseParallelTransmission 负责根据调度器实际运行情况决定是否需要多开连接以
// 降低总传输时间。若需要打开多个子连接以并行传输时，返回值的第一项是主连接仍需要
// 接收的字节数，主连接在接受完指定的分段之后就可以断掉连接了。返回值的第二项是需要
// 新开的个子请求信息。
func (scheduler *parallelRequestScheduler) shoudUseParallelTransmission(
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

	// FIXME: 存在 start 小于 end 的问题
	// FIXME: 上层下发的 remainingDataLen 没有及时更新
	// log.Printf("shoudUseParallelTransmission: url = <%v>, blocksToSplitted = <%v>", url, blocksToSplitted)

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
		timeToFinish := getTimeToFinish(block.getRemainingDataLen(), block.getBandwidth(), block.rtt, blocksToSplitted*blockSize)
		// log.Printf("shoudUseParallelTransmission: session = <%v>, timeToFinish = <%v>, remainingDataLen = <%v>, bandwidth = <%v>", block.id, timeToFinish, block.getRemainingDataLen(), block.getBandwidth())
		subReqeuestSessions = append(subReqeuestSessions, newPseudoRequestControlBlock(block, timeToFinish))
		timeSum += timeToFinish
	}
	// 假设该请求仍需传输的数据需要全部 4 条 session 进行传输
	// 之后，我们将会把没有用上的伪控制块删掉
	for len(subReqeuestSessions) < maxConcurrentSessions-1 {
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
	mainSessionBlocks := int(mainSessionShare / shareSum * float64(blocksToSplitted) * 0.9)
	if mainSessionBlocks >= blocksToSplitted {
		// 给与主请求 1.5 倍的数据量
		mainSessionBlocks = blocksToSplitted
	}
	// 计算 mainSession 的终止字节位置
	mainSessionAdjustedEndOffset := mainSessionBlocks * blockSize
	// 由子请求负责的字节数
	remainingDataSubReqs := remainingDataLen - mainSessionAdjustedEndOffset
	// 然后新建 subRequests
	subRequests := make([]*requestControlBlock, 0)
	// 子请求开始的字节位置
	startOffset := receivedBytesCount + mainSessionAdjustedEndOffset

	// log.Printf("shoudUseParallelTransmission: url = <%v>, mainSessionBlocks = <%v>", url, mainSessionBlocks)
	// log.Printf("shoudUseParallelTransmission: url = <%v>, mainSessionAdjustedEndOffset = <%v>", url, mainSessionAdjustedEndOffset)
	// log.Printf("shoudUseParallelTransmission: url = <%v>, remainingDataSubReqs = <%v>", url, remainingDataSubReqs)
	// log.Printf("shoudUseParallelTransmission: url = <%v>, startOffset = <%v>", url, startOffset)

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

		// log.Printf("shoudUseParallelTransmission: start = <%v>, end = <%v>", subReq.bytesStartOffset, subReq.bytesEndOffset)

		remainingDataSubReqs -= (subReq.bytesEndOffset - subReq.bytesStartOffset)
		startOffset += numBlocks * blockSize
	}
	if len(subRequests)-1 > 0 {
		// 把最后一个子请求的终止字节数调整为全部字节
		subRequests[len(subRequests)-1].bytesEndOffset = receivedBytesCount + remainingDataLen - 1
		// log.Printf("shoudUseParallelTransmission: adjusted end = <%v>", subRequests[len(subRequests)-1].bytesEndOffset)
	}

	return mainSessionAdjustedEndOffset, &subRequests
}

// mayDoRequestParallel 方法负责实际发出请求并返回响应，视情况决定是否采用并行传输以降低下载时间
func (scheduler *parallelRequestScheduler) mayDoRequestParallel(reqBlock *requestControlBlock) {
	req := reqBlock.request
	mainSession := *reqBlock.designatedSession.session
	// 打开 quic stream，开始处理该 H3 请求
	str, err := mainSession.OpenStreamSync(context.Background())
	if err != nil {
		log.Printf(err.Error())
		return
	}

	rsp, err := scheduler.getResponse(req, &str, &mainSession)
	if err != nil {
		log.Printf("mayDoRequestParallel %v", err.Error())
		return
	}

	// 首先实验使用 2 个连接并行下载数据
	var mainBuffer bytes.Buffer // 主 go 程的 buffer
	// 读取响应体总长度，确定需要复制的总数据量，在主 go 程处值为 [0-EOF]
	remainingDataLen, err := strconv.Atoi(rsp.Header.Get("Content-Length"))
	if err != nil {
		log.Printf(err.Error())
		return
	}
	// log.Printf("content-len = <%v>, url = <%v>", remainingDataLen, req.URL.RequestURI())

	// 加上本次请求需要传输的数据量
	// reqBlock.designatedSession.remainingDataLen += remainingDataLen
	reqBlock.designatedSession.addRemainingDataLen(remainingDataLen)
	// log.Printf("data len for new request added: url = <%v>, dataLen = <%v>, newValue = <%v>", req.URL.RequestURI(), remainingDataLen, reqBlock.designatedSession.getRemainingDataLen())

	// 循环读取响应体中的全部数据
	subRequestDone := make(chan *subRequestControlBlock, 4)
	mainRequestURL := fmt.Sprintf("%s://%s%s", req.URL.Scheme, req.URL.Hostname(), req.URL.RequestURI())
	for remainingDataLen > 0 {
		// TODO: 实现提前一个 RTT 发起请求的功能
		if remainingDataLen <= defaultBlockSize {
			// 还有一块的传输任务，可以通知调度器下发下一个请求了
			// log.Printf("before send to: mayExecuteNextRequest: chan len = <%v>, address = <%p>", len(*scheduler.mayExecuteNextRequest), scheduler.mayExecuteNextRequest)
			*scheduler.mayExecuteNextRequest <- struct{}{}
			// log.Printf("after send to: mayExecuteNextRequest: chan len = <%v>", len(*scheduler.mayExecuteNextRequest))
		}
		// 把数据写入到 mainBuffer 中
		// log.Printf("session = <%v>, url = <%v>, beforeRead", reqBlock.designatedSession.id, reqBlock.request.URL.RequestURI())
		written, bandwidth, err := readData(remainingDataLen, &mainBuffer, rsp)
		if err != nil {
			log.Printf(err.Error())
			return
		}
		// log.Printf("session = <%v>, url = <%v>, readDataLen = <%v>", reqBlock.designatedSession.id, reqBlock.request.URL.RequestURI(), written)

		// 调整本请求和所用 session 上需要传输的数据量
		remainingDataLen -= written
		// reqBlock.designatedSession.remainingDataLen -= written
		reqBlock.designatedSession.reduceRemainingDataLen(written)
		// reqBlock.designatedSession.bandwdith = bandwidth
		reqBlock.designatedSession.setBandwidth(bandwidth)
		// log.Printf("set bandwidth: session = <%v>, bandwidth = <%v>, remainingDataLen = <%v>", reqBlock.designatedSession.id, bandwidth, remainingDataLen)
		if remainingDataLen <= 0 {
			// early stop 功能，避免小于块大小的文件进入子请求决策模块
			// log.Printf("early stop: url = <%v>", reqBlock.request.URL.RequestURI())
			break
		}

		/* 子请求决策模块 */
		if reqBlock.shoudUseParallelTransmission {
			// 尚未下发子请求时才会进入这段代码，以确定是否需要使用并行传输
			// log.Printf("before decision, url = <%v>", req.URL.RequestURI())
			mainSessionAdjuestedEndOffset, subReqs :=
				scheduler.shoudUseParallelTransmission(
					mainRequestURL, mainBuffer.Len(), remainingDataLen, bandwidth,
					mainSession.GetConnectionRTT(), defaultBlockSize, &subRequestDone,
					reqBlock.designatedSession.id)
			if subReqs == nil {
				// log.Printf("no parallel is required for url = <%v>", req.URL.RequestURI())
				// 无需进行并行传输
				reqBlock.shoudUseParallelTransmission = false
				// log.Printf("shoudlUseParallelTransmission set to false: url = <%v>, remainingDataLen = <%v>", req.URL.RequestURI(), remainingDataLen)
				continue
			}
			// log.Printf("use <%v> sub request to transfer main request = <%v>", len(*subReqs), reqBlock.request.URL.RequestURI())
			// 把主请求标记为子请求已发送状态
			reqBlock.subRequestDispatched = true
			remainingDataLen = mainSessionAdjuestedEndOffset
			// 设为 false 以免下次进入子请求决策模块
			reqBlock.shoudUseParallelTransmission = false
			// 把需要开始的子请求发送到调度器
			// log.Printf("subRequest unsent: url = <%v>, len = <%v>", req.URL.RequestURI(), len(*subReqs))
			// log.Printf("wrapped subRequestsChan address = <%v>, chan len = <%v>", scheduler.subRequestsChan, len(*scheduler.subRequestsChan))
			*scheduler.subRequestsChan <- subReqs
			// log.Printf("subRequest sent: url = <%v>, len = <%v>", req.URL.RequestURI(), len(*subReqs))
		}
	}
	// log.Printf("main request done: session = <%v> idle, url = <%v>, readDataLen = <%v>", reqBlock.designatedSession.id, reqBlock.request.URL.RequestURI(), mainBuffer.Len())
	// 读取完指定数据段之后立刻关闭这条 stream
	rsp.Body.Close()
	// 立刻把承载该请求的 session 改为可调度状态
	// TODO: 目前的实现为读取完数据之后再声明为可用，实际上要求在读取完成前一个 RTT 时即声明可用以避免网络空闲
	reqBlock.designatedSession.setIdle()

	// 把主请求读取的数据添加到响应体中
	respBody := newSegmentedResponseBody()
	data := mainBuffer.Bytes()
	respBody.addData(&data, 0)
	if err != nil {
		log.Printf(err.Error())
		return
	}

	// 如果主请求下发了子请求，那么也需要等待子请求完成并把所有数据添加到响应体中
	if reqBlock.subRequestDispatched {
		bytesTransferredBySubRequests, err := strconv.Atoi(rsp.Header.Get("Content-Length"))
		if err != nil {
			log.Printf("ator error: %v", err.Error())
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

// execute 方法负责在给定的 quicStream 上执行单一的一个请求
func (scheduler *parallelRequestScheduler) execute(
	req *http.Request,
	str quic.Stream,
	quicSession quic.Session,
	reqDone chan struct{},
) (*http.Response, requestError) {
	// // 是否使用 gzip 压缩
	// var requestGzip bool
	// if !scheduler.roundTripperOpts.DisableCompression && req.Method != "HEAD" &&
	// 	req.header.get("accept-encoding") == "" && req.Header.Get("Range") == "" {
	// 	requestGzip = true
	// }
	requestGzip := isUsingGzip(scheduler.roundTripperOpts.DisableCompression,
		req.Method, req.Header.Get("accept-encoding"), req.Header.Get("Range"))

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
	// TODO: 用普通方法代替 maxHeaderBytes
	if hf.Length > maxHeaderBytes(scheduler.roundTripperOpts.MaxHeaderBytes) {
		return nil, newStreamError(errorFrameError, fmt.Errorf("HEADERS frame too large: %d bytes (max: %d)", hf.Length, maxHeaderBytes(scheduler.roundTripperOpts.MaxHeaderBytes)))
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
func (scheduler *parallelRequestScheduler) executeSubRequest(reqBlock *requestControlBlock) {
	subRequest, err := http.NewRequest(http.MethodGet, reqBlock.url, nil)
	if err != nil {
		log.Printf(err.Error())
	}

	subRequest.Header.Add(
		"Range", fmt.Sprintf("bytes=%d-%d", reqBlock.bytesStartOffset, reqBlock.bytesEndOffset))

	if reqBlock.designatedSession == nil {
		scheduler.mutex.Lock()
		// 调度器没有为该请求分配 session，需要在执行的时候现开一条新的 session
		reqBlock.designatedSession, err = scheduler.getSession()
		scheduler.mutex.Unlock()
		if err != nil {
			log.Printf("error: %v", err.Error())
			*reqBlock.requestError <- struct{}{}
			return
		}
	}
	session := *reqBlock.designatedSession.session
	// log.Printf("executing sub request <%v>, start = <%v>, end = <%v>, session = <%v>", reqBlock.url, reqBlock.bytesStartOffset, reqBlock.bytesEndOffset, reqBlock.designatedSession.id)

	// 打开 quic stream，开始处理该 H3 请求
	str, err := session.OpenStreamSync(context.Background())
	if err != nil {
		log.Printf("executeSubRequest: %v", err.Error())
		return
	}
	resp, err := scheduler.getResponse(subRequest, &str, &session)
	if err != nil {
		log.Printf("executeSubRequest: %v", err.Error())
		return
	}
	// data, err := ioutil.ReadAll(resp.Body)
	dataBuffer := bytes.Buffer{}
	remainingDataLen, err := strconv.Atoi(resp.Header.Get("Content-Length"))
	if err != nil {
		log.Printf("subReq error: %v", err.Error())
	}
	reqBlock.designatedSession.addRemainingDataLen(remainingDataLen)

	// log.Printf("data len for new request added: url = <%v>, dataLen = <%v>, newValue = <%v>", reqBlock.url, remainingDataLen, reqBlock.designatedSession.getRemainingDataLen())
	for remainingDataLen > 0 {
		// TODO: 实现提前一个 RTT 发起请求的功能
		if remainingDataLen <= defaultBlockSize {
			// 发送信号给调度器以触发下一请求
			*scheduler.mayExecuteNextRequest <- struct{}{}
		}
		// 把数据写入到 mainBuffer 中
		// log.Printf("session = <%v>, url = <%v>, beforeRead", reqBlock.designatedSession.id, reqBlock.url)
		written, bandwidth, err := readData(remainingDataLen, &dataBuffer, resp)
		if err != nil {
			log.Printf(err.Error())
			return
		}
		// log.Printf("session = <%v>, url = <%v>, readDataLen = <%v>", reqBlock.designatedSession.id, reqBlock.url, written)

		// 调整本请求和所用 session 上需要传输的数据量
		remainingDataLen -= written
		// reqBlock.designatedSession.remainingDataLen -= written
		reqBlock.designatedSession.reduceRemainingDataLen(written)
		// 更新读取本分段的平均带宽
		// reqBlock.designatedSession.bandwdith = bandwidth
		reqBlock.designatedSession.setBandwidth(bandwidth)
		// log.Printf("set bandwidth: session = <%v>, bandwidth = <%v>, remainingDataLen = <%v>", reqBlock.designatedSession.id, bandwidth, remainingDataLen)
	}
	// 把该 session 标记为可用状态
	// reqBlock.designatedSession.canDispatched = true
	// log.Printf("subReq = <%v>, session = <%v> idle, readDataLen = <%v>, start = <%v>, end = <%v>", reqBlock.url, reqBlock.designatedSession.id, dataBuffer.Len(), reqBlock.bytesStartOffset, reqBlock.bytesEndOffset)
	reqBlock.designatedSession.setIdle()
	resp.Body.Close()

	controlBlock := &subRequestControlBlock{
		data:          dataBuffer.Bytes(),
		contentLength: dataBuffer.Len(),
		startOffset:   reqBlock.bytesStartOffset,
		endOffset:     reqBlock.bytesEndOffset,
	}

	// log.Printf("sub request <%v> done, start = <%v>, end = <%v>, session = <%v>", reqBlock.url, reqBlock.bytesStartOffset, reqBlock.bytesEndOffset, reqBlock.designatedSession.id)
	*reqBlock.subRequestDone <- controlBlock
}

// 指定的 session 和 stream 上获取执行请求并获取响应体
func (scheduler *parallelRequestScheduler) getResponse(req *http.Request, stream *quic.Stream, session *quic.Session) (*http.Response, error) {
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
