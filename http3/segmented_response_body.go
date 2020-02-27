package http3

import (
	"bytes"
	"fmt"
	"io"
	"sync"
)

// 对外提供的分段 response body 接口
// 调用者通过向该接口添加零散的数据分段就可以获得尽可能连续的数据分区
type segmentedResponseBody interface {
	io.ReadCloser

	// addData 方法提供添加新的数据分段的接口
	addData(data *[]byte, offset int)
	// consolidate 方法负责把零散的数据分段拼合称为连续的数据分段
	consolidate()
}

// segmentedResponseBody 接口的实现类
type segmentedResponseBodyI struct {
	mutex sync.Mutex

	buf                      bytes.Buffer     // 连续数据区
	maxContinuousBytesOffset int              // 连续数据区的最大连续字节的 offset
	dataMap                  *map[int]*[]byte // 保存数据分段 offset 和数据本身之间的关系
	dataSize                 int              // 保存的总数据量
}

// newSegmentedResponseBody 返回一个新的 SegmentedResponseBody 实例
func newSegmentedResponseBody() segmentedResponseBody {
	dataMap := make(map[int]*[]byte)
	return &segmentedResponseBodyI{
		buf:     bytes.Buffer{},
		dataMap: &dataMap,
	}
}

// addData 方法负责添加新的数据分段
func (body *segmentedResponseBodyI) addData(data *[]byte, startOffset int) {
	body.mutex.Lock()
	defer body.mutex.Unlock()
	(*body.dataMap)[startOffset] = data
	body.dataSize += len(*data)
}

// consolidate 方法把 dataMap 中分散的各个数据分段合并后写入到 buffer 中
func (body *segmentedResponseBodyI) consolidate() {
	body.mutex.Lock()
	defer body.mutex.Unlock()

	data := []byte{}
	consolidatedDataSize := 0
	for consolidatedDataSize < body.dataSize {
		segment, ok := (*body.dataMap)[consolidatedDataSize]
		if !ok {
			fmt.Println("error, key not found", consolidatedDataSize)
		}
		data = append(data, *segment...)
		delete(*body.dataMap, consolidatedDataSize)
		consolidatedDataSize += len(*segment)
	}
	body.buf.Write(data)
}

// Read 方法对外提供读取内部连续数据区的接口
func (body *segmentedResponseBodyI) Read(buf []byte) (int, error) {
	body.mutex.Lock()
	defer body.mutex.Unlock()
	return body.buf.Read(buf)
}

// Close 方法负责关闭该示例相关的各种资源
func (body *segmentedResponseBodyI) Close() error {
	// 由于该示例中并无需要关闭的资源，故该方法直接返回 nil 错误
	return nil
}
