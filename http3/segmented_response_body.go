package http3

import (
	"bytes"
	"io"
	"sync"
)

// newDataBlock 是对上层添加的数据及其起始位置的包装结构体
type newDataBlock struct {
	data        *[]byte
	startOffset int
}

// 对外提供的分段 response body 接口
// 调用者通过向该接口添加零散的数据分段就可以获得尽可能连续的数据分区
type segmentedResponseBody interface {
	io.ReadCloser

	// addData 方法提供添加新的数据分段的接口
	addData(data *[]byte, offset int)
}

// segmentedResponseBody 接口的实现类
type segmentedResponseBodyI struct {
	mutex sync.Mutex

	mainBuffer               bytes.Buffer     // 连续数据区
	maxContinuousBytesOffset int              // 连续数据区的最大连续字节的 offset
	dataMap                  *map[int]*[]byte // 临时保存数据分段 offset 和数据本身之间的关系
	dataSize                 int              // 保存的总数据量
	offset                   int              // 可以读 offset 以前的数据，可以写 offset 以后的数据

	newDataAddedChan *chan *newDataBlock // 加入新数据时向此 chan 发送信号以在主线程添加数据
	closeChan        *chan struct{}      // 需要关闭该 body 时向该 chan 发送数据
}

// newSegmentedResponseBody 返回一个新的 SegmentedResponseBody 实例
func newSegmentedResponseBody(size int) segmentedResponseBody {
	dataMap := make(map[int]*[]byte)
	newDataChan := make(chan *newDataBlock, 10)
	closeChan := make(chan struct{})
	body := segmentedResponseBodyI{
		mainBuffer: bytes.Buffer{},
		dataMap:    &dataMap,

		newDataAddedChan: &newDataChan,
		closeChan:        &closeChan,
	}
	// 在后台运行 body 主线程
	go body.run()
	return &body
}

// run 在后台运行数据整理程序
func (body *segmentedResponseBodyI) run() {
	for {
		select {
		case newData := <-*body.newDataAddedChan:
			// 有新的数据到达，在主线程整理数据
			body.addDataImpl(newData.data, newData.startOffset)
		case <-*body.closeChan:
			// 需要关闭 body 主线程，直接 return 即可
			return
		}
	}
}

// addData 方法负责添加新的数据分段
func (body *segmentedResponseBodyI) addData(data *[]byte, startOffset int) {
	// 在主线程实现添加数据的方法，以免阻塞上层应用
	*body.newDataAddedChan <- &newDataBlock{data, startOffset}
}

// addDataImpl 是向 body 中添加数据的具体实现方法
func (body *segmentedResponseBodyI) addDataImpl(data *[]byte, startOffset int) {
	body.mutex.Lock()
	defer body.mutex.Unlock()

	var newDataSegmentLen = len(*data) // 新增数据段的长度
	// 增加储存的数据总量
	body.dataSize += newDataSegmentLen

	/* 如果新增的数据段能够和 mianBuffer 中的数据相连接 */
	if startOffset == body.offset {
		// 如果写入的数据是 buffer 最前端的数据，则直接把写入的数据添加到 mainBuffer 中
		body.mainBuffer.Write(*data)
		// 设置新的 offset
		body.offset += newDataSegmentLen

		// 从被新添加的数据的结束位置开始，寻找下一个能够从后方连上的数据分段
		for {
			// 从 dataMap 临时空间中寻找是否还有能接的上的数据段
			dataSegment, ok := (*body.dataMap)[body.offset]
			if !ok {
				// 没有该位置的数据分段
				return
			}
			// 找打了下一个连续的数据分段，需要把它写进 mainBuffer 之中
			body.mainBuffer.Write(*dataSegment)
			delete(*body.dataMap, startOffset)
			body.offset += len(*dataSegment)
		}
	}

	/* 如果新增的数据段不能和 mianBuffer 中的数据相连接 */
	tmpStartOffset := startOffset + newDataSegmentLen // 临时使用的起始位置，初始值为新添加分段的终止位置
	// 把后面能连上的其他数据分段都拼到该分段上，并从 dataMap 中删除这些分段的 entry
	for {
		nextSegment, ok := (*body.dataMap)[tmpStartOffset]
		if !ok {
			break
		}
		nextSegmentLen := len(*nextSegment)
		*data = append(*data, *nextSegment...)
		delete(*body.dataMap, tmpStartOffset)
		tmpStartOffset += nextSegmentLen
	}
	// 这一段数据并不是与 buffer 相连续的部分，放到临时空间中存放
	(*body.dataMap)[startOffset] = data
}

// Read 方法对外提供读取内部连续数据区的接口
func (body *segmentedResponseBodyI) Read(buf []byte) (int, error) {
	body.mutex.Lock()
	defer body.mutex.Unlock()
	return body.mainBuffer.Read(buf)
}

// Close 方法负责关闭该示例相关的各种资源
func (body *segmentedResponseBodyI) Close() error {
	// 由于该示例中并无需要关闭的资源，故该方法直接返回 nil 错误
	// 发出关闭 body 主线程的信号
	*body.closeChan <- struct{}{}
	return nil
}
