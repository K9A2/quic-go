package http3

import (
	"bytes"
	"io"
	"log"
	"sync"
)

// segmentedBufferControlBlock 是有序 buffer 队列中的对象
type segmentedBufferControlBlock struct {
	sync.Mutex

	sessionBlockID int
	start          int
	end            int
	buffer         *bytes.Buffer
	readDataLen    int

	dataSize int
	readSize int
}

// Read 以同步方式从 segmentedBufferControlBlock 中的缓冲区读取数据
func (body *segmentedBufferControlBlock) Read(p []byte) (int, error) {
	body.Lock()
	defer body.Unlock()

	read, err := body.buffer.Read(p)
	if err != nil {
		if err != io.EOF {
			log.Printf("Read: error in reading buffer = <%p>, err = <%s>", body.buffer, err.Error())
		}
	}
	body.readSize += read
	// log.Printf("Read: read <%d> bytes from buffer <%p>, buffer.readSize = <%d>", read, body.buffer, body.readSize)
	return read, err
}

// Write 以同步方式向 segmentedBufferControlBlock 中的缓冲区写入数据
func (body *segmentedBufferControlBlock) Write(p []byte) (int, error) {
	body.Lock()
	defer body.Unlock()
	written, err := body.buffer.Write(p)
	if err != nil {
		log.Printf("Write: error in writting buffer = <%p>, err = <%s>", body.buffer, err.Error())
	}
	body.dataSize += written
	// log.Printf("Write: write <%d> bytes to buffer <%p>, buffer.dataSize = <%d>", written, body.buffer, body.dataSize)
	return written, err
}

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
	// registerSegmentedBuffer 向分段响应体绑定缓冲区和对应的 sessionBlockID
	registerSegmentedBuffer(*segmentedBufferControlBlock)
	// setBufferBound 以原缓冲器的起始字节数为键设置对应的缓冲区的上下限
	setBufferBound(int, int, int, int)
	// signaleDataArrival 方法通知读线程开始工作
	signaleDataArrival()
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

	bufferList              []*segmentedBufferControlBlock // buffer 控制块队列
	currentBufferBlockIndex int                            // 当前正在读 bufferList 中哪一块 buffer
}

// newSegmentedResponseBody 返回一个新的 SegmentedResponseBody 实例
func newSegmentedResponseBody(contentLength int) segmentedResponseBody {
	dataMap := make(map[int]*[]byte)
	newDataChan := make(chan *newDataBlock, 10)
	closeChan := make(chan struct{})
	canReadChan := make(chan struct{}, 10)
	body := segmentedResponseBodyI{
		mainBuffer:    bytes.Buffer{},
		dataMap:       &dataMap,
		contentLength: contentLength,

		newDataAddedChan: &newDataChan,
		closeChan:        &closeChan,
		canReadChan:      &canReadChan,

		bufferList: make([]*segmentedBufferControlBlock, 0),
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
			body.addDataImpl(newData.data, newData.written, newData.startOffset)
		case <-*body.closeChan:
			// 需要关闭 body 主线程，直接 return 即可
			return
		}
	}
}

// addData 方法负责添加新的数据分段
func (body *segmentedResponseBodyI) addData(data *[]byte, written int, startOffset int) {
	// 在主线程实现添加数据的方法，以免阻塞上层应用
	*body.newDataAddedChan <- &newDataBlock{data, written, startOffset}
}

// addDataImpl 是向 body 中添加数据的具体实现方法
func (body *segmentedResponseBodyI) addDataImpl(data *[]byte, written int, startOffset int) {
	body.mutex.Lock()
	defer body.mutex.Unlock()

	// 增加储存的数据总量
	body.dataSize += written
	/* 如果新增的数据段能够和 mainBuffer 中的数据相连接 */
	if startOffset == body.offset {
		// 如果写入的数据是 buffer 最前端的数据，则直接把写入的数据添加到 mainBuffer 中
		written, err := body.mainBuffer.Write(*data)
		if err != nil {
			// TODO: 暂时用 return 代替具体的错误处理操作
			return
		}
		*body.canReadChan <- struct{}{}
		// 设置新的 offset
		body.offset += written
		body.dataSize += written

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
			written, err := body.mainBuffer.Write(*dataSegment)
			if err != nil {
				log.Printf("addDataImpl: error in adding data: err = <%v>", err.Error())
				return
			}
			// 声明有新的数据可供读取
			*body.canReadChan <- struct{}{}
			delete(*body.dataMap, startOffset)
			body.offset += written
		}
		return
	}

	/* 如果新增的数据段不能和 mainBuffer 中的数据相连接 */
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
}

// Write 向分段响应体写入数据
func (body *segmentedResponseBodyI) Write(buf []byte) (int, error) {
	body.mutex.Lock()
	defer body.mutex.Unlock()

	*body.canReadChan <- struct{}{}
	body.offset += len(buf)
	body.dataSize += len(buf)
	return body.mainBuffer.Write(buf)
}

// Read 方法对外提供读取内部连续数据区的接口
func (body *segmentedResponseBodyI) Read(buf []byte) (int, error) {
	body.mutex.Lock()
	if body.readDataLen == body.contentLength {
		// 已经读完全部数据, 直接返回 EOF 让上层应用停止即可
		body.mutex.Unlock()
		return 0, io.EOF
	}
	if len(*body.canReadChan) < 1 {
		// 还有数据可供读取, 但 chan 中没有足够的信号, 需要手动发一个信号给 chan 以触发读取过程
		*body.canReadChan <- struct{}{}
	}
	body.mutex.Unlock()

skipReadOperation:
	select {
	case <-*body.canReadChan:
		// 有新的数据可供读取
		body.mutex.Lock()
		targetBuffer := body.bufferList[body.currentBufferBlockIndex]
		written, err := targetBuffer.Read(buf)
		body.readDataLen += written
		targetBuffer.readDataLen += written

		// log.Printf("Read: original data: written = <%d>, err = <%v>, buffer addr = <%p>", written, err, targetBuffer.buffer)

		// TODO: 需要重做这一段的控制逻辑
		if err != nil && err != io.EOF {
			// 出现了其他错误，由上层处理
			log.Printf("Read: unexpected error = <%s>", err.Error())
			return written, err
		}

		if err != nil {
			// 从这里开始，下面各项的 err 都是 EOF 错误。如果上层收到 EOF 错误，则
			// 会认为所有数据已经被读完，并终止读取操作。因此，在没读完的时候需要屏蔽
			// 掉 buffer 返回 EOF 错误。这里可能出现以下几种情况，使得读到 EOF 错误：
			// 1. 写进程跟不上读进程
			// 2. 读完当前 buffer 的数据
			// 3. 真的读完全部数据了
			if targetBuffer.readDataLen < (targetBuffer.end - targetBuffer.start + 1) {
				// 写线程跟不上读线程，表现是本段 buffer 的读取字节数小于应读字节数
				// log.Println("Read: read thread too fast")
				body.mutex.Unlock()
				if written != 0 {
					// 已经读到了一部分字节，需要手工除掉返回的 EOF 错误
					return written, nil
				}
				// 一个字节都没读到，则进行下一回合，等待写线程写入数据之后再行读取
				goto skipReadOperation
			}

			// 从这里开始，下面就是这一个 buffer 的全部数据都已经被读完，有两种处理方式
			// 1. 对于仅读完本 buffer 的数据而尚未读完请求所声明的全部数据，则切换到下一个 buffer，
			//    如果已经读到部分字节，则返回这些已经读到的字节；否则阻塞至有新数据到达
			// 2. 对于读完请求声明的全数据，则我们返回 (0, EOF) 即可。
			if body.readDataLen == body.contentLength {
				// 读完全部数据
				// log.Printf("Read: all response body data read, readDataLen = <%d>, contentLength = <%d>", body.readDataLen, body.contentLength)
				body.mutex.Unlock()
				if written != 0 {
					// 读到最后一块数据
					return written, io.EOF
				}
				// 上一次操作刚好读完全部数据，使得本次操作没有读到任何数据
				return 0, io.EOF
			}

			// FIXME: 切换到下一个 buffer 的条件有问题
			// log.Printf("Read: ---- switch to next buffer ----")
			// log.Printf("Read: targetBuffer: readDataLen = <%d>, contentLength = <%d>, addr = <%p>",
			// targetBuffer.readDataLen, targetBuffer.end-targetBuffer.start+1, targetBuffer.buffer)
			// log.Printf("Read: body.readDataLen = <%v>, body.contentLength = <%v>", body.readDataLen, body.contentLength)
			// 没有读到任何数据就遇到 EOF 了
			// log.Printf("Read: move to next buffer")
			body.currentBufferBlockIndex++
			// log.Printf("Read: buffer all read: start = <%d>, end = <%d>", targetBuffer.start, targetBuffer.end)
			// log.Printf("Read: next buffer: start = <%d>, end = <%d>",
			// body.bufferList[body.currentBufferBlockIndex].start, body.bufferList[body.currentBufferBlockIndex].end)
			// log.Printf("Read: ---- switch to next buffer ----")
			// 发送一个信号以触发下一次读操作
			*body.canReadChan <- struct{}{}
			body.mutex.Unlock()
			goto skipReadOperation
		}
		// log.Printf("Read: application readDataLen = <%d>, body.readDataLen = <%v>, buffer.readDataLen = <%v>",
		// 	written, body.readDataLen, targetBuffer.readDataLen)
		body.mutex.Unlock()
		// 正常返回读到的字节数和 nil 错误
		return written, err
	}
}

// registerSegmentedBuffer 向分段响应体绑定缓冲区和对应的 session
func (body *segmentedResponseBodyI) registerSegmentedBuffer(bufferBlock *segmentedBufferControlBlock) {
	body.mutex.Lock()
	defer body.mutex.Unlock()

	// log.Printf("register: session = <%d>, start = <%d>, end = <%d>, buf addr = <%p>, buf len = <%d>",
	// bufferBlock.sessionBlockID, bufferBlock.start, bufferBlock.end, bufferBlock.buffer, bufferBlock.buffer.Len())
	/* 对这一块 buffer 创建对应的控制块并添加到 buffer 控制块队列中的适当位置上 */
	body.bufferList = append(body.bufferList, bufferBlock)
	// log.Printf("register: body.bufferList addr = <%p>, len = <%d>", &body.bufferList, len(body.bufferList))
	// 发送可读信号，但不保证一定可以读到数据
	*body.canReadChan <- struct{}{}
}

// setBufferBound 以 sessionBlockID 为键设置对应的 buffer 的字节流起始位置
func (body *segmentedResponseBodyI) setBufferBound(oldStart int, oldEnd int, newStart int, newEnd int) {
	body.mutex.Lock()
	defer body.mutex.Unlock()

	for _, bufferBlock := range body.bufferList {
		if bufferBlock.start == oldStart && bufferBlock.end == oldEnd {
			// log.Printf("setBufferBound: hit, oldStart = <%d>, oldEnd = <%d>, newStart = <%d>, newEnd = <%d>",
			// 	oldStart, oldEnd, newStart, newEnd)
			// 找到了目标 buffer 控制块
			bufferBlock.start = newStart
			bufferBlock.end = newEnd
		}
	}
}

// signaleDataArrival 方法通知读线程可以读数据
func (body *segmentedResponseBodyI) signaleDataArrival() {
	body.mutex.Lock()
	defer body.mutex.Unlock()
	if len(*body.canReadChan) < 1 {
		*body.canReadChan <- struct{}{}
	}
}

// Close 方法负责关闭该示例相关的各种资源
func (body *segmentedResponseBodyI) Close() error {
	// 由于该示例中并无需要关闭的资源，故该方法直接返回 nil 错误
	// 发出关闭 body 主线程的信号
	*body.closeChan <- struct{}{}
	return nil
}
