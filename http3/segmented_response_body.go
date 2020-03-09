package http3

import (
	"bytes"
	"io"
	"log"
	"sync"
)

// newDataBlock 是对上层添加的数据及其起始位置的包装结构体
type newDataBlock struct {
	data        *[]byte // 数据段指针
	written     int     // 数据段实有字节数
	startOffset int     // 数据段起始位置
}

// 对外提供的分段 response body 接口
// 调用者通过向该接口添加零散的数据分段就可以获得尽可能连续的数据分区
type segmentedResponseBody interface {
	io.ReadCloser
	io.Writer

	// addData 方法提供添加新的数据分段的接口
	addData(data *[]byte, written int, offset int)
}

// segmentedResponseBody 接口的实现类
type segmentedResponseBodyI struct {
	mutex sync.Mutex

	mainBuffer               bytes.Buffer     // 连续数据区
	maxContinuousBytesOffset int              // 连续数据区的最大连续字节的 offset
	dataMap                  *map[int]*[]byte // 临时保存数据分段 offset 和数据本身之间的关系
	dataSize                 int              // 保存的总数据量
	offset                   int              // 可以读 offset 以前的数据，可以写 offset 以后的数据
	readDataLen              int              // 被读取的字节数
	contentLength            int              // 全部数据长度, 在没有读完 contentLength 个字节之前, Read 方法不会返回 EOF 错误

	newDataAddedChan *chan *newDataBlock // 加入新数据时向此 chan 发送信号以在主线程添加数据
	closeChan        *chan struct{}      // 需要关闭该 body 时向该 chan 发送数据
	canReadChan      *chan struct{}      // 可以读取数据时向该 chan 发送消息
}

// newSegmentedResponseBody 返回一个新的 SegmentedResponseBody 实例
func newSegmentedResponseBody(contentLength int) segmentedResponseBody {
	dataMap := make(map[int]*[]byte)
	newDataChan := make(chan *newDataBlock, 10)
	closeChan := make(chan struct{})
	canReadChan := make(chan struct{}, 1000)
	// log.Printf("newSegmentedResponseBody: canReadChan addr = <%v>", &canReadChan)
	body := segmentedResponseBodyI{
		mainBuffer:    bytes.Buffer{},
		dataMap:       &dataMap,
		contentLength: contentLength,

		newDataAddedChan: &newDataChan,
		closeChan:        &closeChan,
		canReadChan:      &canReadChan,
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
			// log.Printf("addDataImpl: len = <%v>, offset = <%v>", newData.written, newData.startOffset)
			// 有新的数据到达，在主线程整理数据
			body.addDataImpl(newData.data, newData.written, newData.startOffset)
		case <-*body.closeChan:
			// 需要关闭 body 主线程，直接 return 即可
			// log.Printf("body close")
			return
		}
	}
}

// addData 方法负责添加新的数据分段
func (body *segmentedResponseBodyI) addData(data *[]byte, written int, startOffset int) {
	// log.Printf("addData: len = <%v>, offset = <%v>", len(*data), startOffset)
	// 在主线程实现添加数据的方法，以免阻塞上层应用
	*body.newDataAddedChan <- &newDataBlock{data, written, startOffset}
}

// addDataImpl 是向 body 中添加数据的具体实现方法
func (body *segmentedResponseBodyI) addDataImpl(data *[]byte, written int, startOffset int) {
	body.mutex.Lock()
	defer body.mutex.Unlock()

	// log.Printf("addDataImpl: offset = <%v>, len = <%v>, lock body", startOffset, written)
	// 增加储存的数据总量
	body.dataSize += written

	/* 如果新增的数据段能够和 mianBuffer 中的数据相连接 */
	if startOffset == body.offset {
		// 如果写入的数据是 buffer 最前端的数据，则直接把写入的数据添加到 mainBuffer 中
		// body.mainBuffer.Write((*data)[:written])
		// body.Write(*data)
		written, err := body.mainBuffer.Write(*data)
		if err != nil {
			// log.Printf("addDataImpl: error in adding data: err = <%v>", err.Error())
			// TODO: 暂时用 return 代替具体的错误处理操作
			return
		}
		*body.canReadChan <- struct{}{}
		// 设置新的 offset
		body.offset += written
		body.dataSize += written
		// log.Printf("addDataImpl: before consolidate loop: offset = <%v>, len = <%v>", startOffset, len(*data))

		// 从被新添加的数据的结束位置开始，寻找下一个能够从后方连上的数据分段
		for {
			// 从 dataMap 临时空间中寻找是否还有能接的上的数据段
			dataSegment, ok := (*body.dataMap)[body.offset]
			if !ok {
				log.Printf("addDataImpl: found no data at offset = <%v>", body.offset)
				// 没有该位置的数据分段
				break
			}
			// 找打了下一个连续的数据分段，需要把它写进 mainBuffer 之中
			// body.mainBuffer.Write(*dataSegment)
			// log.Printf("addDataImpl: found continuous data segment at offset = <%v>", body.offset)
			// body.Write(*dataSegment)
			written, err := body.mainBuffer.Write(*dataSegment)
			if err != nil {
				log.Printf("addDataImpl: error in adding data: err = <%v>", err.Error())
				return
			}
			*body.canReadChan <- struct{}{}
			delete(*body.dataMap, startOffset)
			body.offset += written
		}
		// 声明有新的数据可供读取
		// log.Printf("addDataImpl: wrapped canReadChan addr = <%v>", body.canReadChan)
		// log.Println("addDataImpl: before send signal")
		// *body.canReadChan <- struct{}{}
		// log.Printf("addDataImpl: after send to canReadChan : offset = <%v>, len = <%v>", startOffset, len(*data))
		// log.Println("addDataImpl: lock released")
		return
	}

	/* 如果新增的数据段不能和 mianBuffer 中的数据相连接 */
	// log.Printf("addDataImpl: add isolated data segment at offset = <%v>", startOffset)
	tmpStartOffset := startOffset + written // 临时使用的起始位置，初始值为新添加分段的终止位置
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
	// log.Println("addDataImpl: lock released")
}

// Write 向分段响应体写入数据
func (body *segmentedResponseBodyI) Write(buf []byte) (int, error) {
	body.mutex.Lock()
	defer body.mutex.Unlock()

	*body.canReadChan <- struct{}{}
	body.offset += len(buf)
	body.dataSize += len(buf)
	// log.Printf("Write: offset = <%v>, len = <%v>", body.offset, len(buf))
	return body.mainBuffer.Write(buf)
}

// Read 方法对外提供读取内部连续数据区的接口
func (body *segmentedResponseBodyI) Read(buf []byte) (int, error) {
	// log.Println("Read: before obtain early check lock")
	body.mutex.Lock()
	if body.readDataLen == body.contentLength {
		// 已经读完全部数据, 直接返回 EOF 让上层应用停止即可
		// log.Println("Read: early check lock released")
		body.mutex.Unlock()
		return 0, io.EOF
	}
	if len(*body.canReadChan) < 1 {
		// 还有数据可供读取, 但 chan 中没有足够的信号, 需要手动发一个信号给 chan 以触发读取过程
		// log.Println("Read: manually triggered read proces")
		*body.canReadChan <- struct{}{}
	}
	body.mutex.Unlock()
	// log.Println("Read: early check lock released")

	// log.Printf("Read: before select block, chan len = <%v>", len(*body.canReadChan))
skipReadOperation:
	select {
	case <-*body.canReadChan:
		// log.Printf("Read: prepare to read, current offset = <%v>", body.readDataLen)
		// 有新的数据可供读取
		body.mutex.Lock()
		// log.Println("Read: lock obtained")
		written, err := body.mainBuffer.Read(buf)
		body.readDataLen += written
		// if err != nil {
		// 	log.Printf("Read: application read <%v> bytes, readDataLen = <%v>, contentLength = <%v>, err = <%v>",
		// 		written, body.readDataLen, body.contentLength, err.Error())
		// } else {
		// 	log.Printf("Read: application read <%v> bytes, readDataLen = <%v>, contentLength = <%v>, no error",
		// 		written, body.readDataLen, body.contentLength)
		// }
		if written == 0 && err != nil {
			// 这里可能出现两种情况, 使得从 mainBuffer 中读到的数据长度为零, 并且错误为 EOF:
			// 1. 写进程跟不上读进程
			// 2. 真的读完全部数据了
			// log.Printf("Read: 0 len buffer: err = <%v>", err.Error())
			if body.readDataLen == body.contentLength {
				// 读完全部数据, 直接按照实际结果返回即可
				log.Printf("Read: all data read")
				return 0, err
			}
			// 只是写进程没跟上, 等写进程跟上就好了
			// log.Printf("Read: skip this read operation")
			body.mutex.Unlock()
			// log.Println("Read: lock released")
			goto skipReadOperation // buffer 中现在没有数据可供读取, 阻塞至下一次读取信号到来
		}
		body.mutex.Unlock()
		// log.Println("Read: lock released")
		return written, err
	}
}

// Close 方法负责关闭该示例相关的各种资源
func (body *segmentedResponseBodyI) Close() error {
	// 由于该示例中并无需要关闭的资源，故该方法直接返回 nil 错误
	// 发出关闭 body 主线程的信号
	*body.closeChan <- struct{}{}
	return nil
}
