package quic

import (
	// "github.com/google/logger"
)

// FileTypeScheduler 是基于文件类型和文件大小的调度器
type FileTypeScheduler struct {
	// 指向 MemoryStorage 实例的指针
	storage *MemoryStorage
}

// NewFileTypeScheduler 根据加载的配置文件构造一个新的文件类型调度器
func NewFileTypeScheduler() *FileTypeScheduler {
	return &FileTypeScheduler{
		storage: GetMemoryStorage(),
	}
}

// ActiveStreamCount 返回调度器所管理的活跃 stream 数目
func (fts *FileTypeScheduler) ActiveStreamCount() int {
	return fts.storage.ActiveManagedStreamCount + len(fts.storage.UnmanagedStreams)
}

// Empty 返回该调度器中是否有活跃 stream
func (fts *FileTypeScheduler) Empty() bool {
	return fts.ActiveStreamCount() == 0
}

// findAndSetActive 把该队列中具有指定 id 和 url 的 stream 设为指定状态，即活跃和非活跃
func findStreamAndSetStatus(id StreamID, url string, status bool, queue *[]*StreamControlBlock) {
	for _, val := range *queue {
		if val.URL == url {
			val.StreamID = id
			val.Active = status
			return
		}
	}
}

// AddActiveStream 把具有指定 url 的 stream 设为活跃状态。此 stream 被设为活跃
// 状态之后可被 framer 封装数据
func (fts *FileTypeScheduler) AddActiveStream(id StreamID) {
	defer fts.storage.mutex.Unlock()
	fts.storage.mutex.Lock()

	// 先根据 id 确定是否为 managed 的 stream
	index, ok := fts.storage.IDToManagedStreamIndex[id]
	if ok {
		fts.storage.ManagedStreams[index].Active = true
		fts.storage.ActiveManagedStreamCount++
		// logger.Infof("activated managed stream <%v>", id)
		return
	}

	// 把该 StreamID 添加到 RR 发送队列中
	fts.storage.UnmanagedStreams = append(fts.storage.UnmanagedStreams, id)
	// logger.Infof("activated unmanaged stream <%v>", id)
}

// RemoveIdleStream 把具有指定 id 和 url 的 stream 设为空闲状态，调度器不会向 framer
// 提供此类 stream
func (fts *FileTypeScheduler) RemoveIdleStream(id StreamID) {
	defer fts.storage.mutex.Unlock()
	fts.storage.mutex.Lock()

	// 根据 id 确定是否为 managed 的 stream
	index, ok := fts.storage.IDToManagedStreamIndex[id]
	if ok {
		fts.storage.ManagedStreams[index].Active = false
		fts.storage.ActiveManagedStreamCount--
		// logger.Infof("remove managed idle stream <%v>", id)
		return
	}

	first := fts.storage.UnmanagedStreams[0]
	if first == id {
		fts.storage.UnmanagedStreams = fts.storage.UnmanagedStreams[1:]
		// logger.Infof("remove unmanaged idle stream <%v>", id)
		return
	}
	// logger.Infof("error: first <%v>, id <%v>", first, id)
}

// findFirstActiveStream 查找队列中第一条为 active 状态的 stream
func findFirstActiveStream(queue *[]*StreamControlBlock) (StreamID, bool) {
	for _, val := range *queue {
		if val.Active {
			return val.StreamID, true
		}
	}
	return -1, false
}

// PopNextActiveStream 给出下一条活跃 stream 的 stream ID，由 framer 从此 stream 中
// 封装数据。没有 unmanaged 的 stream 的时候才轮到 managed 的 stream 传输
func (fts *FileTypeScheduler) PopNextActiveStream() StreamID {
	defer fts.storage.mutex.Unlock()
	fts.storage.mutex.Lock()

	if fts.storage.ActiveManagedStreamCount > 0 {
		result, ok := findFirstActiveStream(&fts.storage.ManagedStreams)
		if ok {
			return result
		}
		// logger.Infof("error: active managed stream declared but not found")
		return -1
	}

	if len(fts.storage.UnmanagedStreams) > 0 {
		return fts.storage.UnmanagedStreams[0]
	}
	return -1
}

// findNilStream 从给定的队列中查找具有指定 StreamID 的 stream 的数组下标
func findNilStream(id StreamID, queue *[]*StreamControlBlock) (int, bool) {
	for index, val := range *queue {
		if val.StreamID == id {
			return index, true
		}
	}
	return -1, false
}

// RemoveNilStream 从调度器中移除现值已为 nil 的 stream
func (fts *FileTypeScheduler) RemoveNilStream(id StreamID) {
	defer fts.storage.mutex.Unlock()
	fts.storage.mutex.Lock()

	// 确定是否为 managed 的 stream
	index, ok := fts.storage.IDToManagedStreamIndex[id]
	if ok {
		fts.storage.ManagedStreams[index].Active = false
		fts.storage.ActiveManagedStreamCount--
		// logger.Infof("removed nil managed stream <%v>", id)
		return
	}

	if len(fts.storage.UnmanagedStreams) > 0 {
		first := fts.storage.UnmanagedStreams[0]
		if first == id {
			fts.storage.UnmanagedStreams = fts.storage.UnmanagedStreams[1:]
			// logger.Infof("removed nil unmanaged stream <%v>", id)
			return
		}
		// logger.Infof("error: try to remove an non-exists stream <%v>", id)
	}
}
