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
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/marten-seemann/qpack"
)

// 通知主线程触发下一个请求的剩余数据量比例阈值
const prestartDataLenRatio = 0.25

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
	documentQueue   *[]*requestControlBlock // html 文件请求队列
	styleSheetQueue *[]*requestControlBlock // css 文件请求队列
	scriptQueue     *[]*requestControlBlock // js 文件请求队列
	otherFileQueue  *[]*requestControlBlock // 其他文件请求队列

	mayExecuteNextRequest *chan struct{}                // 有任何可能实际执行下一请求时都向此 chan 中发送信号
	subRequestsChan       *chan *[]*requestControlBlock // 如果需要发送子请求，就把子请求的信息发送到这个 chan 中
	newSessionAddedChan   *chan struct{}                //有新的 session 被添加了就往该 chan 发送信号
}

// newParallelRequestScheduler 初始化并返回新生成的调度器 parallelRequestScheduler 实例
func newParallelRequestScheduler(info *clientInfo) requestScheduler {

	subRequestsChan := make(chan *[]*requestControlBlock, 10)
	// log.Printf("original subRequestChan address = <%v>", &subRequestsChan)
	mutex := sync.Mutex{}

	mayExecuteNextRequetChan := make(chan struct{}, 10)
	newSessionAddedChan := make(chan struct{})
	// log.Printf("original mayExecuteNextRequetChan address = <%v>", &mayExecuteNextRequetChan)

	documentQueue := make([]*requestControlBlock, 0)
	styleSheetQueue := make([]*requestControlBlock, 0)
	scriptQueue := make([]*requestControlBlock, 0)
	otherFileQueue := make([]*requestControlBlock, 0)

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

		documentQueue:   &documentQueue,
		styleSheetQueue: &styleSheetQueue,
		scriptQueue:     &scriptQueue,
		otherFileQueue:  &otherFileQueue,

		// mayExecuteNextRequest: make(chan struct{}),
		mayExecuteNextRequest: &mayExecuteNextRequetChan,
		subRequestsChan:       &subRequestsChan,
		newSessionAddedChan:   &newSessionAddedChan,
	}
}

// 调度器的主 go 程
func (scheduler *parallelRequestScheduler) run() {
	// 在后台起建立连接的线程
	go func() {
		// log.Printf("start adding new quic sessions")
		for i := 0; i < maxConcurrentSessions; i++ {
			go scheduler.addNewQuicSession()
		}
	}()

	// 处理来自各模块的事件
	for {
		select {
		case <-*scheduler.mayExecuteNextRequest:
			// log.Println("received mayExecuteNextRequest signal")
			// 执行主请求
			nextRequest, err := scheduler.mayExecute()
			if err == nil {
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
func (scheduler *parallelRequestScheduler) popRequest() (*requestControlBlock, int) {
	var nextRequest *requestControlBlock
	var queueIndex int
	if len(*scheduler.documentQueue) > 0 {
		nextRequest = (*scheduler.documentQueue)[0]
		queueIndex = documentQueueIndex
		// log.Printf("popRequest: document queue addr = <%p>", scheduler.documentQueue)
	} else if len(*scheduler.styleSheetQueue) > 0 {
		nextRequest = (*scheduler.styleSheetQueue)[0]
		queueIndex = styleSheetQueueIndex
	} else if len(*scheduler.scriptQueue) > 0 {
		nextRequest = (*scheduler.scriptQueue)[0]
		queueIndex = scriptQueueIndex
	} else if len(*scheduler.otherFileQueue) > 0 {
		nextRequest = (*scheduler.otherFileQueue)[0]
		queueIndex = otherQueueIndex
	}

	// if nextRequest != nil {
	// log.Printf("popRequest: url = <%v>, queue index = <%v>", nextRequest.request.URL.RequestURI(), queueIndex)
	// }
	return nextRequest, queueIndex
}

// removeFirst 移除指定队列的第一个元素
func (scheduler *parallelRequestScheduler) removeFirst(index int) {
	switch index {
	case documentQueueIndex:
		// log.Printf("removeFirst: url = <%v>, addr = <%v>", (*scheduler.documentQueue)[0].request.URL.RequestURI(), scheduler.documentQueue)
		*scheduler.documentQueue = (*scheduler.documentQueue)[1:]
		// log.Printf("removeFirst: document queue len = <%v>", len(*scheduler.documentQueue))
	case styleSheetQueueIndex:
		// log.Printf("removeFirst: url = <%v>", (*scheduler.styleSheetQueue)[0].request.URL.RequestURI())
		*scheduler.styleSheetQueue = (*scheduler.styleSheetQueue)[1:]
	case scriptQueueIndex:
		// log.Printf("removeFirst: url = <%v>", (*scheduler.scriptQueue)[0].request.URL.RequestURI())
		*scheduler.scriptQueue = (*scheduler.scriptQueue)[1:]
	case otherQueueIndex:
		// log.Printf("removeFirst: url = <%v>", (*scheduler.otherFileQueue)[0].request.URL.RequestURI())
		*scheduler.otherFileQueue = (*scheduler.otherFileQueue)[1:]
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

	scheduler.mutex.Lock()
	newSessionBlock := newSessionControlBlock(scheduler.maxSessionID, newSession, true)
	scheduler.openedSessions = append(scheduler.openedSessions, newSessionBlock)
	// log.Printf("add new quic session: id = <%v>", scheduler.maxSessionID)
	scheduler.maxSessionID++
	scheduler.mutex.Unlock()
	*scheduler.mayExecuteNextRequest <- struct{}{}
	return newSessionBlock, nil
}

// getSession 获取调度器中可用的 quicSession
func (scheduler *parallelRequestScheduler) getSession() (*sessionControlblock, error) {
	// if len(scheduler.openedSessions) < 1 {
	// 	// 现在还没有任何 session，需要等待后台建连线程提供至少一个 session
	// 	select {
	// 	case <-*scheduler.newSessionAddedChan:
	// 	}
	// }

	// 已经有了现成的 session，那么就把在这些 session 中找一个空闲的
	// log.Println("in getSession")
	for _, block := range scheduler.openedSessions {
		if block.dispatchable() {
			// 找到了一个处于空闲状态的 session
			// log.Printf("found an idle session: id = %v, len = %v", block.id, len(scheduler.openedSessions))
			return block, nil
		}
	}

	// log.Println("no available session")
	// 没有空闲的 session，并且不能打开新的 session
	return nil, errNoAvailableSession
}

// mayExecute 方法负责在调度器队列中寻找可执行的下一请求
func (scheduler *parallelRequestScheduler) mayExecute() (*requestControlBlock, error) {
	// log.Println("mayExecute: before lock")
	scheduler.mutex.Lock()
	defer scheduler.mutex.Unlock()

	// 获取可用的请求
	// log.Println("in mayExecute")
	// FIXME: 这里似乎会锁住整个调度器, 但是加了日志之后似乎重复不出来, 就这样放着先吧
	nextRequest, index := scheduler.popRequest()
	if nextRequest == nil {
		// log.Println("mayExecute: no available request")
		return nil, errNoAvailableRequest
	}
	// log.Printf("mayExecute: url = <%v>", nextRequest.request.URL.RequestURI())

	// 获取可用的 quic 连接
	availableSession, err := scheduler.getSession()
	if err != nil {
		// log.Println("mayExecute: no available session")
		return nil, errNoAvailableSession
	}
	availableSession.setBusy(nextRequest.request.URL.RequestURI())
	// log.Printf("mayExecute: url = <%v> session id = <%v>, pendingRequest = <%v>",
	// 	nextRequest.request.URL.RequestURI(), availableSession.id, availableSession.pendingRequest)

	// 从调度器的待执行队列中删去即将执行的 requestControlBlock
	// if len(*scheduler.documentQueue) > 0 {
	// 	log.Printf("scheduler queue before: %v", (*scheduler.documentQueue)[0].request.URL.RequestURI())
	// }
	scheduler.removeFirst(index)
	// if len(*scheduler.documentQueue) > 0 {
	// 	log.Printf("scheduler queue after: %v", (*scheduler.documentQueue)[0].request.URL.RequestURI())
	// }
	// 把即将要使用的 session 标记为繁忙状态
	// log.Printf("session = <%v> setBusy() to execute <%v>, value = <%v>, remainingDataLen = <%v>",
	// 	availableSession.id, nextRequest.request.URL.RequestURI(), availableSession.canDispatched, availableSession.getRemainingDataLen())
	// availableSession.pendingRequest++
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
	// log.Printf("add new request: url = <%v>", block.request.URL.RequestURI())
	if block.request.URL.RequestURI() == "/" {
		// 对于 "/" 和 "index.html" 的快速处理方法
		// log.Printf("adding to document queue: %v", block.request.URL.RequestURI())
		*scheduler.documentQueue = append(*scheduler.documentQueue, block)
	} else {
		// 对于其他的资源 url，则需要猜测 url 所对应的资源类别后加入合适的队列
		mimeType := mime.TypeByExtension(filepath.Ext(block.request.URL.RequestURI()))
		// 有确定的 mime type，则需要根据给出的 mime type 决定加入到哪一条队列中
		switch getQueueIndexByMimeType(mimeType) {
		case documentQueueIndex:
			// log.Printf("adding to document queue: %v", block.request.URL.RequestURI())
			*scheduler.documentQueue = append(*scheduler.documentQueue, block)
		case styleSheetQueueIndex:
			// log.Printf("adding to stylesheet queue: %v", block.request.URL.RequestURI())
			*scheduler.styleSheetQueue = append(*scheduler.styleSheetQueue, block)
		case scriptQueueIndex:
			// log.Printf("adding to script queue: %v", block.request.URL.RequestURI())
			*scheduler.scriptQueue = append(*scheduler.scriptQueue, block)
		default:
			// log.Printf("adding to other file queue: %v", block.request.URL.RequestURI())
			*scheduler.otherFileQueue = append(*scheduler.otherFileQueue, block)
		}
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
			if reqBlock.response == nil {
				// 无响应的请求以 404 作为错误码返回
				return getErrorResponse(req), nil
			}
			// 有响应的请求返回其本身的响应体
			return reqBlock.response, nil
		case <-*reqBlock.requestDone:
			// log.Printf("request done: %v", req.URL.Hostname()+req.URL.RequestURI())
			return reqBlock.response, reqBlock.unhandledError
		}
	}
}

// signalRequestDone 向调度器声明该请求已完成
func (scheduler *parallelRequestScheduler) signalRequestDone(reqBlock *requestControlBlock) {
	// log.Println("signalRequestDone: before lock")
	scheduler.mutex.Lock()
	// log.Println("signalRequestDone: lock acquired")

	if reqBlock.url != "" {
		reqBlock.designatedSession.setIdle(reqBlock.url)
	} else {
		reqBlock.designatedSession.setIdle(reqBlock.request.URL.RequestURI())
	}
	*reqBlock.requestDone <- struct{}{}
	// log.Printf("signalRequestDone: chan len = <%d>", len(*scheduler.mayExecuteNextRequest))
	*scheduler.mayExecuteNextRequest <- struct{}{}

	scheduler.mutex.Unlock()
	// log.Println("signalRequestDone: lock released")
}

// signalRequestError 向调度器声明该请求出现错误
func (scheduler *parallelRequestScheduler) signalRequestError(reqBlock *requestControlBlock) {
	scheduler.mutex.Lock()
	defer scheduler.mutex.Unlock()

	if reqBlock.url != "" {
		reqBlock.designatedSession.setIdle(reqBlock.url)
	} else {
		reqBlock.designatedSession.setIdle(reqBlock.request.URL.RequestURI())
	}
	*reqBlock.requestError <- struct{}{}
	*scheduler.mayExecuteNextRequest <- struct{}{}
}

// shouldUseParallelTransmission 负责根据调度器实际运行情况决定是否需要多开连接以
// 降低总传输时间。若需要打开多个子连接以并行传输时，返回值的第一项是主连接仍需要
// 接收的字节数，主连接在接受完指定的分段之后就可以断掉连接了。返回值的第二项是需要
// 新开的个子请求信息。
func (scheduler *parallelRequestScheduler) shouldUseParallelTransmission(
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

	// log.Printf("shouldUseParallelTransmission: url = <%v>, mainSessionBandwidth = <%v>", url, mainSessionBandwidth)
	// log.Printf("shouldUseParallelTransmission: url = <%v>, mainSessionRTT = <%v>", url, mainSessionRTT)

	// 把剩余字节数转换为剩余分段数
	remainingBlocks := int(math.Ceil(float64(remainingDataLen) / float64(blockSize)))
	// 把在下一个 rtt 中主连接能收到的字节数转换为分段数。
	blocksInflightNextRTT := int(math.Ceil(mainSessionBandwidth * mainSessionRTT / float64(blockSize)))
	// 剩余可供拆分传输的块数目
	blocksToSplitted := remainingBlocks - blocksInflightNextRTT

	// log.Printf("shouldUseParallelTransmission: url = <%v>, remainingBlocks = <%v>", url, remainingBlocks)
	// log.Printf("shouldUseParallelTransmission: url = <%v>, blocksToSplitted = <%v>", url, blocksToSplitted)
	// log.Printf("shouldUseParallelTransmission: url = <%v>, blocksInflightNextRTT = <%v>", url, blocksInflightNextRTT)

	if blocksToSplitted <= 1 {
		// 只有一块时直接用 mainSession 处理
		return -1, nil
	}

	// 如果只用主请求传输数据，则所需时间为
	remainingTimeMainSession := float64(blocksToSplitted) * float64(blockSize) / mainSessionBandwidth
	subReqeuestSessions := make([]*pseudoRequestControlBlock, 0)

	var timeSum float64
	// 复制已经打开的 session 信息
	// log.Println("in shouldUseParallelTransmission")
	scheduler.mutex.Lock()
	// log.Println("after lock")
	var bandwidth float64
	var rtt float64
	for _, block := range scheduler.openedSessions {
		if block.id == mainSessionID {
			// 跳过主请求所在的 session，即不在此 session 上传输主请求拆分出来的分段
			continue
		}

		/* 排除错误的样本 */
		bandwidth = block.getBandwidth()
		if bandwidth <= 0 {
			// log.Println("using mainsession bandwidth")
			bandwidth = mainSessionBandwidth
		}
		rtt = (*block.session).GetConnectionRTT()
		if rtt <= 0 {
			// log.Printf("using mainsession rtt")
			rtt = mainSessionRTT
		}

		// 添加伪请求控制块，避免修改原控制块
		// log.Printf("shouldUseParallelTransmission: bandwidth = <%v>, rtt = <%v>", bandwidth, rtt)
		timeToFinish := getTimeToFinish(block.getRemainingDataLen(), bandwidth, rtt, blocksToSplitted*blockSize)
		// log.Printf("shouldUseParallelTransmission: session = <%v>, timeToFinish = <%v>, remainingDataLen = <%v>, bandwidth = <%v>", block.id, timeToFinish, block.getRemainingDataLen(), block.getBandwidth())
		subReqeuestSessions = append(subReqeuestSessions, newPseudoRequestControlBlock(block, timeToFinish))
		timeSum += timeToFinish
	}
	scheduler.mutex.Unlock()
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
	// log.Printf("shouldUseParallelTransmission: url = <%v>, remainingTimeMainSession = <%v>", url, remainingTimeMainSession)
	// log.Printf("shouldUseParallelTransmission: url = <%v>, timeSum = <%v>", url, timeSum)
	mainSessionShare := 1 / (remainingTimeMainSession / timeSum)
	shareSum += mainSessionShare
	for _, pseudoReq := range subReqeuestSessions {
		pseudoReq.share = 1 / (pseudoReq.timeToFinish / timeSum)
		shareSum += pseudoReq.share
	}
	// 计算主请求以及所有子请求所应当承担的块数，最少为 1 块。块数为 0 的子请求将不会被发出。
	// log.Printf("shouldUseParallelTransmission: url = <%v>, mainSessionShare = <%v>", url, mainSessionShare)
	// log.Printf("shouldUseParallelTransmission: url = <%v>, shareSum = <%v>", url, shareSum)
	// log.Printf("shouldUseParallelTransmission: url = <%v>, blocksToSplitted = <%v>", url, blocksToSplitted)
	mainSessionBlocks := int(mainSessionShare / shareSum * float64(blocksToSplitted))
	if mainSessionBlocks >= blocksToSplitted {
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

	// log.Printf("shouldUseParallelTransmission: url = <%v>, mainSessionBlocks = <%v>", url, mainSessionBlocks)
	// log.Printf("shouldUseParallelTransmission: url = <%v>, mainSessionAdjustedEndOffset = <%v>", url, mainSessionAdjustedEndOffset)
	// log.Printf("shouldUseParallelTransmission: url = <%v>, remainingDataSubReqs = <%v>", url, remainingDataSubReqs)
	// log.Printf("shouldUseParallelTransmission: url = <%v>, startOffset = <%v>", url, startOffset)

	// 计算各子请求应当传输的数据块
	// TODO: 把数据的分布改成 raid 0 一样的条状分布
	// log.Printf("shouldUseParallelTransmission: len subRequestSession = <%v>", len(subReqeuestSessions))
	for _, block := range subReqeuestSessions {
		if remainingDataSubReqs < 0 {
			break
		}
		// 向下取整, 小于一块的数据数据量舍去不计
		numBlocks := int(math.Ceil(block.share / shareSum * float64(blocksToSplitted)))
		// log.Printf("shouldUseParallelTransmission: numBlocks = <%v>", numBlocks)
		if numBlocks < 1 {
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

		// if subReq.designatedSession != nil {
		// 	log.Printf("shouldUseParallelTransmission: url = <%v>, session = <%v>, start = <%v>, end = <%v>",
		// 		url, subReq.designatedSession.id, subReq.bytesStartOffset, subReq.bytesEndOffset)
		// } else {
		// 	log.Printf("shouldUseParallelTransmission: url = <%v>, session = <?>: start = <%v>, end = <%v>",
		// 		url, subReq.bytesStartOffset, subReq.bytesEndOffset)
		// }

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

// shouldSendPrestartSignal 根据剩余数据量和数据总量返回是否需要发出下一个请求
func shouldSendPrestartSignal(remainingDataLen int) bool {
	return remainingDataLen <= 2*defaultBlockSize
}

// mayDoRequestParallel 方法负责实际发出请求并返回响应，视情况决定是否采用并行传输以降低下载时间
func (scheduler *parallelRequestScheduler) mayDoRequestParallel(reqBlock *requestControlBlock) {
	log.Printf("mayDoRequestParallel: url = <%v>, session = <%v>, dispatchable = <%v>",
		reqBlock.request.URL.RequestURI(), reqBlock.designatedSession.id, reqBlock.designatedSession.dispatchable())

	req := reqBlock.request
	mainSession := *reqBlock.designatedSession.session
	// 打开 quic stream，开始处理该 H3 请求
	str, err := mainSession.OpenStreamSync(context.Background())
	if err != nil {
		log.Printf(err.Error())
		scheduler.signalRequestError(reqBlock)
		return
	}

	// 获取原始响应体
	rsp, err := scheduler.getResponse(req, &str, &mainSession)
	if err != nil {
		log.Printf("mayDoRequestParallel %v", err.Error())
		scheduler.signalRequestError(reqBlock)
		return
	}

	// 读取响应体总长度，确定需要复制的总数据量，在主 go 程处值为 [0-EOF]
	contentLength, err := strconv.Atoi(rsp.Header.Get("Content-Length"))
	if err != nil {
		// 该请求没有响应体
		log.Printf("url = <%v>, err = <%v>", req.URL.RequestURI(), err.Error())
		reqBlock.response = rsp
		scheduler.signalRequestError(reqBlock)
		return
	}
	remainingDataLen := contentLength
	reqBlock.setContentLength(contentLength)
	// log.Printf("mayDoRequestParallel: url = <%v>, contentLength = <%v>", req.URL.RequestURI(), contentLength)

	if contentLength < defaultBlockSize {
		reqBlock.response = rsp
		scheduler.signalRequestDone(reqBlock)
		log.Printf("using early stop: session = <%d>, url = <%v>", reqBlock.designatedSession.id, req.URL.RequestURI())
		return
	}

	// 让无需分段读取的请求立刻返回
	mainRequestURL := fmt.Sprintf("%s://%s%s", req.URL.Scheme, req.URL.Hostname(), req.URL.RequestURI())
	subRequestDone := make(chan *subRequestControlBlock, 4) // 如果使用子请求，则子请求在完成时需要向该 chan 发送信号
	if mainSession.GetConnectionRTT() > 0 && reqBlock.designatedSession.getBandwidth() > 0 {
		// 只有在收到了合法的样本之后才能做出正确的决策
		mainSessionRTT := mainSession.GetConnectionRTT()
		// log.Printf("mainSessionRTT = <%v>", mainSessionRTT)
		mainSessionAdjuestedEndOffset, subReqs := scheduler.shouldUseParallelTransmission(
			mainRequestURL,
			0,
			contentLength,
			reqBlock.designatedSession.getBandwidth(),
			mainSessionRTT,
			defaultBlockSize,
			&subRequestDone,
			reqBlock.designatedSession.id,
		)
		if subReqs == nil {
			// 无需分段传输，则直接返回现成的响应体
			reqBlock.response = rsp
			log.Printf("use no parallel request, session = <%v>, url = <%v>",
				reqBlock.designatedSession.id, reqBlock.request.URL.RequestURI())
			scheduler.signalRequestDone(reqBlock)
			return
		}

		/* 需要把该请求拆分称为多个子请求以并行处理 */
		remainingDataLen = mainSessionAdjuestedEndOffset // 调整主请求的终止字节数
		reqBlock.shoudUseParallelTransmission = false    // 避免再次进入子请求决策模块
		reqBlock.subRequestDispatched = true             // 标记为子请求已下发状态
		*scheduler.subRequestsChan <- subReqs            // 把需要开始的子请求发送到调度器
		log.Printf("use parallel request, subReq count = <%v>, url = <%v>",
			len(*subReqs), reqBlock.request.URL.RequestURI())
	}

	// 加上本次请求需要传输的数据量
	reqBlock.designatedSession.addRemainingDataLen(remainingDataLen)

	// 把主请求读取的数据添加到响应体中
	respBody := newSegmentedResponseBody(contentLength)
	// 返回包装后的响应
	finalResponse := &http.Response{
		Proto:      "HTPP/3",
		ProtoMajor: 3,
		StatusCode: rsp.StatusCode,
		Header:     rsp.Header.Clone(),
		Body:       respBody,
	}
	reqBlock.response = finalResponse
	// 通知上层响应体已准备好
	*reqBlock.requestDone <- struct{}{}
	// log.Printf("segmented response body returned")

	// 循环读取响应体中的全部数据
	// var mainBuffer bytes.Buffer // 主 go 程的 buffer
	// log.Printf("initial remainingDataLen: %v", remainingDataLen)
	var once sync.Once
	var offset int
	var readDataLen int
	message := EndTimeMessage{}
	for remainingDataLen > 0 {
		if shouldSendPrestartSignal(remainingDataLen) {
			once.Do(func() {
				// 还有一块的传输任务，可以通知调度器下发下一个请求了
				// log.Println("mainRequest: sending signale to next request")
				log.Printf("prestart signal sent: session = <%v>, url = <%v>", reqBlock.designatedSession.id, reqBlock.request.URL.RequestURI())
				reqBlock.designatedSession.setIdle(reqBlock.request.URL.RequestURI())
				*scheduler.mayExecuteNextRequest <- struct{}{}
			})
		}
		// 把数据写入到 mainBuffer 中
		// written, bandwidth, err := readData(remainingDataLen, &mainBuffer, rsp)
		// written, bandwidth, dataBuf, err := readDataBytes(rsp)
		written, bandwidth, err := copyToBuffer(&respBody, rsp)
		if err != nil {
			log.Printf(err.Error())
			scheduler.signalRequestError(reqBlock)
			return
		}
		// 实施添加数据
		// readBytes := mainBuffer.Next(written)
		readDataLen += written
		// log.Printf("session = <%v>, url = <%v>, readDataLen = <%v>", reqBlock.designatedSession.id, reqBlock.request.URL.RequestURI(), readDataLen)
		// respBody.addData(dataBuf, written, offset)
		// log.Println("after addData")

		offset += written
		// 调整本请求和所用 session 上需要传输的数据量
		remainingDataLen -= written
		reqBlock.designatedSession.reduceRemainingDataLen(written)
		reqBlock.designatedSession.setBandwidth(bandwidth)
		// log.Printf("move to next round: remainingDataLen = <%v>", remainingDataLen)

		/* 子请求决策模块 */
		if reqBlock.shoudUseParallelTransmission {
			// 尚未下发子请求时才会进入这段代码，以确定是否需要使用并行传输
			mainSessionRTT := mainSession.GetConnectionRTT()
			mainSessionAdjuestedEndOffset, subReqs :=
				scheduler.shouldUseParallelTransmission(
					mainRequestURL, readDataLen, remainingDataLen, bandwidth,
					mainSessionRTT, defaultBlockSize, &subRequestDone,
					reqBlock.designatedSession.id)
			if subReqs == nil {
				// 无需进行并行传输
				reqBlock.shoudUseParallelTransmission = false
				// log.Printf("use no parallel request, url = <%v>", mainRequestURL)
				continue
			}
			// 把主请求标记为子请求已发送状态
			reqBlock.subRequestDispatched = true
			remainingDataLen = mainSessionAdjuestedEndOffset
			// log.Printf("mainBuffer should have <%v> bytes", remainingDataLen+defaultBlockSize)
			// 设为 false 以免下次进入子请求决策模块
			reqBlock.shoudUseParallelTransmission = false
			// 把需要开始的子请求发送到调度器
			*scheduler.subRequestsChan <- subReqs
			log.Printf("use parallel request, subReq count = <%v>, url = <%v>", len(*subReqs), mainRequestURL)
		}
	}
	// 读取完指定数据段之后立刻关闭这条 stream
	rsp.Body.Close()
	log.Printf("main reqeuest finished: session = <%v>", reqBlock.request.URL.RequestURI())
	message.MainRequestFinishTime = time.Now()
	// 立刻把承载该请求的 session 改为可调度状态
	// TODO: 目前的实现为读取完数据之后再声明为可用，实际上要求在读取完成前一个 RTT 时即声明可用以避免网络空闲
	// TODO: 把向 scheduler 通知可以执行下一个请求的操作封装为一个方法，以免漏写
	// TODO: 整理出执行一个请求要满足的所有条件

	// data := mainBuffer.Bytes()
	// respBody.addData(&data, 0)
	// log.Printf("mainBuffer doest have <%v> bytes", len(data))
	// if err != nil {
	// 	log.Printf(err.Error())
	// 	reqBlock.designatedSession.setIdle()
	// 	*reqBlock.requestError <- struct{}{}
	// 	return
	// }

	// 如果主请求下发了子请求，那么也需要等待子请求完成并把所有数据添加到响应体中
	if reqBlock.subRequestDispatched {
		log.Printf("waiting for sub requests finish: url = <%v>, chan addr = <%p>", mainRequestURL, &subRequestDone)
		bytesTransferredBySubRequests := contentLength - offset + 1
		for bytesTransferredBySubRequests > 0 {
			select {
			case subReq := <-subRequestDone:
				message.SubRequestFinishTime = append(message.SubRequestFinishTime, time.Now())
				respBody.addData(&subReq.data, len(subReq.data), subReq.startOffset)
				bytesTransferredBySubRequests -= subReq.contentLength
				// log.Printf("bytesTransferredBySubRequests = <%v>", bytesTransferredBySubRequests)
			}
		}
	}
	log.Printf("all sub requests finished: url = <%v>", mainRequestURL)

	return
}

// execute 方法负责在给定的 quicStream 上执行单一的一个请求
func (scheduler *parallelRequestScheduler) execute(
	req *http.Request,
	str quic.Stream,
	quicSession quic.Session,
	reqDone chan struct{},
) (*http.Response, requestError) {
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
		// TODO: 确定有没有在 shouldUseParallelTransmission 中为 sub request 分配 session
		log.Println("get session for sub request")
		reqBlock.designatedSession, err = scheduler.getSession()
		if err != nil {
			log.Printf("error: %v", err.Error())
			*reqBlock.requestError <- struct{}{}
			reqBlock.designatedSession.setIdle(reqBlock.url)
			scheduler.mutex.Unlock()
			return
		}
		reqBlock.designatedSession.setBusy(reqBlock.url)
		scheduler.mutex.Unlock()
	} else {
		reqBlock.designatedSession.setBusy(reqBlock.url)
	}
	session := *reqBlock.designatedSession.session
	log.Printf("executing sub request <%v>, start = <%v>, end = <%v>, session = <%v>, pendingRequest = <%v>",
		reqBlock.url, reqBlock.bytesStartOffset, reqBlock.bytesEndOffset, reqBlock.designatedSession.id, reqBlock.designatedSession.pendingRequest)

	// 打开 quic stream，开始处理该 H3 请求
	str, err := session.OpenStreamSync(context.Background())
	if err != nil {
		log.Printf("executeSubRequest: %v", err.Error())
		*reqBlock.requestError <- struct{}{}
		reqBlock.designatedSession.setIdle(reqBlock.url)
		return
	}
	resp, err := scheduler.getResponse(subRequest, &str, &session)
	if err != nil {
		log.Printf("executeSubRequest: %v", err.Error())
		*reqBlock.requestError <- struct{}{}
		reqBlock.designatedSession.setIdle(reqBlock.url)
		return
	}

	// data, err := ioutil.ReadAll(resp.Body)
	dataBuffer := bytes.Buffer{}
	contentLength, err := strconv.Atoi(resp.Header.Get("Content-Length"))
	if err != nil {
		log.Printf("subReq error: url = <%v> , err = <%v>", reqBlock.url, err.Error())
		*reqBlock.requestError <- struct{}{}
		reqBlock.designatedSession.setIdle(reqBlock.url)
	}
	remainingDataLen := contentLength
	reqBlock.designatedSession.addRemainingDataLen(remainingDataLen)

	// log.Printf("data len for new request added: url = <%v>, dataLen = <%v>, newValue = <%v>", reqBlock.url, remainingDataLen, reqBlock.designatedSession.getRemainingDataLen())
	var once sync.Once
	for remainingDataLen > 0 {
		// TODO: 实现提前一个 RTT 发起请求的功能
		if shouldSendPrestartSignal(remainingDataLen) {
			// log.Printf("prestart: can send signal, remainingDataLen = <%d>, contentLength = <%d>", remainingDataLen, contentLength)
			once.Do(func() {
				// 发送信号给调度器以触发下一请求
				// log.Println("subRequest: sending signal to next request")
				log.Printf("prestart signal sent: session = <%v>, url = <%v>", reqBlock.designatedSession.id, reqBlock.url)
				reqBlock.designatedSession.setIdle(reqBlock.url)
				*scheduler.mayExecuteNextRequest <- struct{}{}
			})
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
	resp.Body.Close()

	controlBlock := &subRequestControlBlock{
		data:          dataBuffer.Bytes(),
		contentLength: dataBuffer.Len(),
		startOffset:   reqBlock.bytesStartOffset,
		endOffset:     reqBlock.bytesEndOffset,
	}

	log.Printf("sub request <%v> done, start = <%v>, end = <%v>, session = <%v>, chan addr = <%v>",
		reqBlock.url, reqBlock.bytesStartOffset, reqBlock.bytesEndOffset, reqBlock.designatedSession.id, reqBlock.subRequestDone)
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
