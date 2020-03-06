package http3

import (
	"net/http"
)

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

type subRequestControlBlock struct {
	data          []byte // 子请求所获取的数据
	contentLength int    // 子请求所获得的数据的字节数
	startOffset   int    // 子请求所获得的数据相对于主请求的开始字节数
	endOffset     int    // 子请求所获得的数据相对于主请求的终止字节数
}

type requestControlBlock struct {
	isMainSession                bool // 是否为主请求
	bytesStartOffset             int  // 响应体开始位置，用于让主请求拼装为完成的请求
	bytesEndOffset               int  // 响应体结束位置，同上
	shoudUseParallelTransmission bool // 是否需要使用并行传输
	subRequestDispatched         bool // 是否已经下发子请求

	url            string                        // 请求的 url，只在子请求是使用
	request        *http.Request                 // 对应的 http 请求
	requestDone    *chan struct{}                // 调度器完成该 http 请求时向该 chan 发送消息
	subRequestDone *chan *subRequestControlBlock // 子请求完成时向该 chan 发送消息
	requestError   *chan struct{}                // 出现任何错误时向该 chan 发送信息

	response       *http.Response // 已经处理完成的 response，可以返回上层应用
	contentLength  int            // 响应体字节数
	unhandledError error          // 处理 response 过程中发生的错误

	designatedSession *sessionControlblock // 调度器指定用来承载该请求的 session
}

func (block *requestControlBlock) setContentLength(contentLength int) {
	block.contentLength = contentLength
}
