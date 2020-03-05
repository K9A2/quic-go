package http3

import (
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"

	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/internal/utils"
	"github.com/marten-seemann/qpack"
)

const defaultUserAgent = "quic-go HTTP/3"
const defaultMaxResponseHeaderBytes = 10 * 1 << 20 // 10 MB

var defaultQuicConfig = &quic.Config{
	MaxIncomingStreams: -1, // don't allow the server to create bidirectional streams
	// KeepAlive:          true,
}

var dialAddr = quic.DialAddr

type roundTripperOpts struct {
	DisableCompression bool
	MaxHeaderBytes     int64
}

// client 是对外暴露的 h3 client 接口
type client interface {
	// 实现 HTTP/3 所必须的两个接口
	http.RoundTripper
	io.Closer
}

// client is a HTTP3 client doing requests
// 负责具体实现 HTTP/3 Client 接口。由此接口负责实现
type clientI struct {
	dialOnce     sync.Once
	dialer       func(network, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.Session, error)
	handshakeErr error

	requestWriter *requestWriter

	hostname string

	logger utils.Logger

	scheduler requestScheduler // 调度器实例
}

func newClient(
	hostname string,
	tlsConf *tls.Config,
	opts *roundTripperOpts,
	quicConfig *quic.Config,
	dialer func(network, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.Session, error),
) client {
	if tlsConf == nil {
		tlsConf = &tls.Config{}
	} else {
		tlsConf = tlsConf.Clone()
	}
	// Replace existing ALPNs by H3
	tlsConf.NextProtos = []string{nextProtoH3}
	if quicConfig == nil {
		quicConfig = defaultQuicConfig
	}
	quicConfig.MaxIncomingStreams = -1 // don't allow any bidirectional streams
	logger := utils.DefaultLogger.WithPrefix("h3 client")

	newClient := &clientI{
		hostname: authorityAddr("https", hostname),
		dialer:   dialer,
		logger:   logger,
	}

	info := &clientInfo{
		hostname:         authorityAddr("https", hostname),
		tlsConfig:        tlsConf,
		quicConfig:       quicConfig,
		requestWriter:    newRequestWriter(logger),
		decoder:          qpack.NewDecoder(func(hf qpack.HeaderField) {}),
		roundTripperOpts: opts,
	}

	// 初始化调度器实例
	// newClient.scheduler = newParallelRequestScheduler(authorityAddr("https", hostname), tlsConf, quicConfig,
	// 	newRequestWriter(logger), qpack.NewDecoder(func(hf qpack.HeaderField) {}), opts)
	newClient.scheduler = newRequestScheduler(parallelRequestSchedulerName, info)
	// 在别的 go 程中运行调度器实例
	go newClient.scheduler.run()

	return newClient
}

// 目前的实现是关闭同一 client 下打开的所有 quicSession
func (c *clientI) Close() error {
	return c.scheduler.close()
}

// RoundTrip executes a request and returns a response
// 在这里把一整个大请求视情况拆分成多个小请求进行并行传输
// req 是上层来的一整个请求，返回的 response 也是以一整个返回的形式向上层提供
// 上层并不感知到下发的请求已在此处被拆分成多个请求以在多条信道上并行传输
func (c *clientI) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != "https" {
		return nil, errors.New("http3: unsupported scheme")
	}
	if authorityAddr("https", hostnameFromRequest(req)) != c.hostname {
		return nil, fmt.Errorf("http3 client BUG: RoundTrip called for the wrong client (expected %s, got %s)", c.hostname, req.Host)
	}

	resp, err := c.scheduler.addAndWait(req)
	if err != nil {
		fmt.Println(err.Error())
		return nil, err
	}

	return resp, err
}
