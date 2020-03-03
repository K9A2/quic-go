package http3

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
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
			log.Printf("header key = <%v>, value = <%v>", hf.Name, hf.Value)
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
