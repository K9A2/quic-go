package http3

import (
	"bytes"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/lucas-clemente/quic-go"
	"github.com/marten-seemann/qpack"
)

// 队列名称
const documentQueueIndex = 1
const styleSheetQueueIndex = 2
const scriptQueueIndex = 3
const otherQueueIndex = 4

// 各队列对应的文件的 mime 类型关键字
const documentQueueFileType = "html"
const styleSheetQueueFileType = "css"
const scriptQueueFileType = "javascript"

// 错误定义
var errHostNotConnected = errors.New("can not connect to given host")    // 连不上对端主机
var errNoAvailableSession = errors.New("no available session")           // 找不到指定的
var errCanNotExecuteRequest = errors.New("can not execute this request") // 无法执行此请求
var errNoAvailableRequest = errors.New("no available request")           // 队列中没有待处理的下一请求

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

// getTimeToFinish 函数计算在当前情况下，包含请求就绪在内的传输完所有数据需要的时间
func getTimeToFinish(remainingDataLen int, bandwidth float64, rtt float64, newDataLen int) float64 {
	timeToAvailable := float64(remainingDataLen) / bandwidth
	if timeToAvailable < rtt {
		timeToAvailable = rtt
	}
	timeToFinishLoadNewData := float64(newDataLen) / bandwidth
	return timeToAvailable + timeToFinishLoadNewData
}

// copyToBuffer 把 HTTP 响应体中的数据复制到 buffer 中
func copyToBuffer(buffer *segmentedBufferControlBlock, rsp *http.Response, blockSize int64, remainingDataLen int) (int, float64, error) {
	if int64(remainingDataLen) < blockSize {
		blockSize = int64(remainingDataLen)
	}
	timeStart := time.Now()
	written, err := io.CopyN(buffer, rsp.Body, blockSize)
	timeEnd := time.Now()
	timeConsumed := timeEnd.Sub(timeStart).Microseconds()
	bandwidth := float64(written) / float64(timeConsumed) * 1000000.0

	if err != nil {
		if err == io.EOF {
			// 暂时没有数据可供复制
			return int(written), bandwidth, nil
		}
		// 出错
		return 0, 0, err
	}
	// 正常返回
	return int(written), bandwidth, nil
}

// readDataBytes 从响应体中读取数据并返回读取到的数据
// 返回值中包括实际读取的数据长度, 读取期间的平均带宽, 以及读取期间发生的错误 (没有
// 错误则为 nil). 参数如下:
// rsp: 作为数据来源的 HTTP 响应
func readDataBytes(rsp *http.Response) (int, float64, *[]byte, error) {
	buf := make([]byte, 4*1024) // 存放数据用的缓冲区
	timeStart := time.Now()
	written, err := rsp.Body.Read(buf) // 实际读出的字节数会保存在 written 中
	timeEnd := time.Now()
	timeConsumed := timeEnd.Sub(timeStart).Microseconds()
	bandwidth := float64(written) / float64(timeConsumed) * 1000000.0

	if err != nil {
		if err == io.EOF {
			// log.Println("readDataBytes: read all data")
			// 读完了全部数据
			return int(written), bandwidth, &buf, nil
		}
		// 出现错误
		// log.Printf("readDataBytes: err = <%v>", err.Error())
		return 0, 0, nil, err
	}
	// log.Printf("radDataBytes: return normal, written = <%v>, bandwidth = <%v>", written, bandwidth)
	// 正常返回
	return int(written), bandwidth, &buf, nil
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
	// timeConsumed := timeEnd.Sub(timeStart).Milliseconds()
	timeConsumed := timeEnd.Sub(timeStart).Microseconds()
	// 带宽的单位为 Bps，所以需要通过乘 1000 来抵消掉用毫秒作为计时单位的 1000 倍缩小
	// 带宽的单位为 Bps，所以需要通过乘 1000000 来抵消掉用微秒作为计时单位的 1000000 倍缩小
	bandwidth := float64(written) / float64(timeConsumed) * 1000000.0
	// log.Printf("readData: written = <%v>, time = <%v>", written, timeConsumed)
	return int(written), bandwidth, nil
}

func maxHeaderBytes(optionMaxHeaderBytes int64) uint64 {
	if optionMaxHeaderBytes <= 0 {
		return defaultMaxResponseHeaderBytes
	}
	return uint64(optionMaxHeaderBytes)
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

// getErrorResponse 方法返回一个 404 Not Found 错误体
func getErrorResponse(req *http.Request) *http.Response {
	return &http.Response{
		Proto:      "HTPP/3",
		ProtoMajor: 3,
		StatusCode: http.StatusNotFound,
		Header:     req.Header.Clone(),
	}
}

// dial 方法按照给定的参数向对端拨号，并返回双方的 quicSession
func dial(hostname string, tlsConfig *tls.Config, quicConfig *quic.Config) (*quic.Session, error) {
	quicSession, err := dialAddr(hostname, tlsConfig, quicConfig)
	if err != nil {
		return nil, err
	}

	go func() {
		if err := setupH3Session(&quicSession); err != nil {
			log.Printf("Setting up session failed: %v", err.Error())
			quicSession.CloseWithError(quic.ErrorCode(errorInternalError), "")
		}
	}()

	return &quicSession, nil
}

// isusingGzip 方法返回该连接是否使用 GZIP 压缩
func isUsingGzip(compressionDisabled bool, method string, acceptEncoding string, requestRange string) bool {
	// 是否使用 gzip 压缩
	if !compressionDisabled && method != "HEAD" && acceptEncoding == "" && requestRange == "" {
		return true
	}
	return false
}

// getResponse 从给定的连接中获取响应体
func getResponse(
	req *http.Request,
	usingGzip bool,
	str *quic.Stream,
	sess *quic.Session,
	requestWriter *requestWriter,
	maxHeaderBytes uint64,
	decoder *qpack.Decoder,
	reqDone chan struct{},
) (*http.Response, requestError) {
	if err := requestWriter.WriteRequest(*str, req, usingGzip); err != nil {
		log.Printf("write request error: %v", err.Error())
		return nil, newStreamError(errorInternalError, err)
	}

	// 开始接受对端返回的数据
	frame, err := parseNextFrame(*str)
	if err != nil {
		log.Printf("parse next frame error: %v", err.Error())
		return nil, newStreamError(errorFrameError, err)
	}
	// 确定第一帧是否为 H3 协议规定的 HEADER 帧
	hf, ok := frame.(*headersFrame)
	if !ok {
		log.Println("unexpected first frame")
		return nil, newConnError(errorFrameUnexpected, errors.New("expected first frame to be a HEADERS frame"))
	}
	// TODO: 用普通方法代替 maxHeaderBytes
	if hf.Length > maxHeaderBytes {
		log.Println("header frame too large")
		return nil, newStreamError(errorFrameError,
			fmt.Errorf("HEADERS frame too large: %d bytes (max: %d)", hf.Length, maxHeaderBytes))
	}
	// 读取并解析 HEADER 帧
	headerBlock := make([]byte, hf.Length)
	if _, err := io.ReadFull(*str, headerBlock); err != nil {
		log.Println("read head block error")
		return nil, newStreamError(errorRequestIncomplete, err)
	}
	// 调用 qpack 解码 HEADER 帧
	hfs, err := decoder.DecodeFull(headerBlock)
	if err != nil {
		log.Println("decode header error")
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
	respBody := newResponseBody(*str, reqDone, func() {
		(*sess).CloseWithError(quic.ErrorCode(errorFrameUnexpected), "")
	})
	// 根据是否需要 gzip 来实际构造响应体
	if usingGzip && res.Header.Get("Content-Encoding") == "gzip" {
		res.Header.Del("Content-Encoding")
		res.Header.Del("Content-Length")
		res.ContentLength = -1
		res.Body = newGzipReader(respBody)
		res.Uncompressed = true
		log.Println("init with a gzip reader")
	} else {
		res.Body = respBody
	}

	return res, requestError{}
}

// CollectorAddr 为数据收集器的默认监听地址, 本机 UDP 8888 端口
const CollectorAddr = "127.0.0.1:8888"

// EndTimeMessage 记录了主请求以及子请求结束时间
type EndTimeMessage struct {
	Key                   int64       // 消息的 key, 为 HTTP/3 客户端被创建的时间
	MainRequestFinishTime time.Time   // 主请求结束时间
	SubRequestFinishTime  []time.Time // 子请求结束时间
}

// StatisticsCollector 负责收集运行数据
type StatisticsCollector struct {
}

// NewStatisticsCollector 创建数据收集器实例并返回其指针
func NewStatisticsCollector() *StatisticsCollector {
	return &StatisticsCollector{}
}

// Run 运行收集器主线程
func (collector *StatisticsCollector) Run() {
	listener, err := net.ListenPacket("udp", CollectorAddr)
	if err != nil {
		log.Printf("collector: error in listening: %v", err.Error())
		return
	}

	log.Printf("collector: running at <%v>", listener.LocalAddr().String())

	for {
		buf := make([]byte, 1024)
		written, remoteAddr, err := listener.ReadFrom(buf)
		if err != nil {
			log.Printf("collector: error in accepting conn: %v", err.Error())
			return
		}
		log.Printf("collector: read <%v> from <%v>", written, remoteAddr)
		go handlePacket(&buf)
	}
}

// handlePacket 处理到来的 UDP 数据包
func handlePacket(buf *[]byte) {
	receivedBuf := bytes.NewBuffer(*buf)
	var receivedMessage EndTimeMessage
	if err := binary.Read(receivedBuf, binary.LittleEndian, &receivedMessage); err != nil {
		log.Printf("handleConn: error in parsing message from udp conn")
	}
	log.Printf("received message: main request = <%v>, sub requests = <%v>",
		receivedMessage.MainRequestFinishTime, receivedMessage.SubRequestFinishTime)
}

// sendToStatisticCollector 发送数据到 collector
func sendToStatisticCollector(message *EndTimeMessage) {
	sendBuf := bytes.Buffer{}
	if err := binary.Write(&sendBuf, binary.LittleEndian, message); err != nil {
		log.Printf("send message error: %v", err.Error())
		return
	}
	conn, err := net.Dial("udp", CollectorAddr)
	if err != nil {
		log.Printf("error in dialing to collector: %v", err.Error())
		return
	}
	conn.Write(sendBuf.Bytes())
}

const (
	// KB 是 1KB 数据的长度
	KB = 1024
)

// computeBlockSize 根据给定的带宽和 RTT 数据计算合适的块大小
func computeBlockSize(bandwidth, rtt float64) int64 {
	rttDataLen := int(bandwidth * rtt) // 一个 RTT 内能够读取的数据
	if rttDataLen < 16*KB {
		return 16 * KB
	} else if rttDataLen < 32*KB {
		return defaultBlockSize // i.e., 32KB
	} else if rttDataLen < 64*KB {
		return 64 * KB
	} else if rttDataLen < 128*KB {
		return 128 * KB
	} else if rttDataLen < 256*KB {
		return 128 * KB
	} else {
		return 256 * KB
	}
}
