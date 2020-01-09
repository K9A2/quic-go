package quic

import (
  "mime"
  "path/filepath"
  "strings"
  "sync"

  "github.com/google/logger"
)

// StreamControlBlock 是 MemoryStorage 中存放的 stream 控制块
type StreamControlBlock struct {
  StreamID StreamID
  URL      string
  Active   bool
  Data     *[]byte
  Replaced bool
}

// NewStreamControlBlock 返回一个按照给定值构造的 StreamControlBlock 变量
func NewStreamControlBlock(id StreamID, url string, active bool, data *[]byte, replaced bool) *StreamControlBlock {
  return &StreamControlBlock{
    StreamID: id,
    URL:      url,
    Active:   active,
    Data:     data,
    Replaced: replaced,
  }
}

// GetMimeType 根据资源路径返回对应的 MIME 类型
func GetMimeType(fileName string) string {
  // 获取资源的扩展名
  fileExt := filepath.Ext(fileName)
  var mtype string

  // 以下扩展名的资源无法从系统库中获取 MIME 类型，需要在本函数中指定 MIME 类型
  switch fileExt {
  case "":
    mtype = "text/html"
  case ".woff2":
    mtype = "font"
  }
  if mtype != "" {
    return mtype
  }

  // 不是已知的无法从系统库中获取到 MIME 类型的资源，则需要尝试从系统库中获取资源
  mtype = mime.TypeByExtension(filepath.Ext(fileName))
  if mtype == "" {
    // 无法从系统库中查找到该资源对应的 MIME 类型
    mtype = "unknown"
    logger.Infof("warning: unknown mimetype for url = %v\n", fileName)
  } else {
    // 针对可以从系统库中获取到 MIME 类型的资源，检查系统库给的 MIME 类型中是否包含
    // 类似于 “text/css; charset=utf-8” 的字符集信息
    semicolonIndex := strings.Index(mtype, ";")
    if semicolonIndex > 0 {
      // 移除字符集信息
      mtype = mtype[:semicolonIndex]
    }
  }
  return mtype
}

// MemoryStorage 在内存中存放所有静态文件的控制字段和数据字段
type MemoryStorage struct {
  mutex sync.Mutex

  // 约定的传输顺序
  ManagedStreams []*StreamControlBlock
  // 保存 url 到 managed 类型 stream 在 sequence 数组中的位置
  URLToManagedStreamIndex map[string]int
  // 保存 StreamID 到 managed 类型 stream 在 sequence 数组中的位置
  IDToManagedStreamIndex map[StreamID]int
  // 已安排顺序的活跃 stream 数目
  ActiveManagedStreamCount int

  // 没有指定顺序的 stream 采用 RR 算法交替发送数据
	UnmanagedStreams []StreamID
}

// 全局唯一的 memory storage 实例
var storage = MemoryStorage{
  ManagedStreams:          make([]*StreamControlBlock, 0),
  URLToManagedStreamIndex: make(map[string]int),
  IDToManagedStreamIndex:  make(map[StreamID]int),
  UnmanagedStreams:        make([]StreamID, 0),
}

// InitMemoryStorage 负责在内存中加载指定的资源文件
func InitMemoryStorage() {
  /* 把所有 managed 的资源 url 全部读出并压入 memory storage 中的传输队列中 */
  storage.ManagedStreams = append(storage.ManagedStreams,
    NewStreamControlBlock(-1, "", false, nil, false)) // 先把控制 stream 压入队列
  storage.URLToManagedStreamIndex[""] = 0 // 此 stream 位于队伍首位，以最高优先级传输
  logger.Infof("url = <%v> added to index <%v>", "", 0)
  // 加载其余的 url 队列中
  for index, url := range youtubeList {
    storage.ManagedStreams =
      append(storage.ManagedStreams, NewStreamControlBlock(-1, url, false, nil, false))
    storage.URLToManagedStreamIndex[url] = index + 1
    logger.Infof("url = <%v> added to index <%v>", url, index+1)
  }
  logger.Infof("memory storage inited")
}

// GetMemoryStorage 返回指向 memory storage 全局变量的指针
func GetMemoryStorage() *MemoryStorage {
  return &storage
}

// ContainsByURL 检查给出的 url 是否有收录在 MemoryStorage 中
// func (ms *MemoryStorage) ContainsByURL(key string) bool {
//   defer ms.mutex.Unlock()
//   ms.mutex.Lock()
//   _, ok := ms.URLToManagedStreamIndex[key]
//   return ok
// }

// ContainsByID 检查给出的 StreamID 是否有收录在 MemoryStorage 中
// func (ms *MemoryStorage) ContainsByID(key StreamID) bool {
//   defer ms.mutex.Unlock()
//   ms.mutex.Lock()
//   _, ok := ms.IDToManagedStreamIndex[key]
//   return ok
// }

// ActivateByURL 激活 storage 实例中以 url 为 key 的 stream
// func (ms *MemoryStorage) ActivateByURL(url string) {
//   defer ms.mutex.Unlock()
//   ms.mutex.Lock()
//   index, ok := ms.URLToManagedStreamIndex[url]
//   if ok {
//     logger.Infof("error in activating non-exists stream by url <%v>", url)
//     return
//   }
//   ms.ManagedStreams[index].Active = true
// }

// GetByURL 以 url 为 key 查找资源对应的 stream control block
// func (ms *MemoryStorage) GetByURL(url string) *StreamControlBlock {
//   defer ms.mutex.Unlock()
//   ms.mutex.Lock()
//   val, ok := ms.urlToBlockMap[url]
//   if !ok {
//     logger.Errorf("error: loading non-existent value by key <%v>\n", url)
//     return nil
//   }
//   logger.Infof("cache hit, key = <%v>, len = <%v>", url, len(*val.Data))
//   return val
// }

// GetByID 通过 StreamID 来在 storage 实例中查找对应的 StreamControlBlock
// func (ms *MemoryStorage) GetByID(id StreamID) *StreamControlBlock {
//   defer ms.mutex.Unlock()
//   ms.mutex.Lock()
//   val, ok := ms.idToBlockMap[id]
//   if !ok {
//     logger.Errorf("error: loading non-existent value by key <%v>", id)
//     return nil
//   }
//   return val
// }

// MarkAsReplaced 把 url 指定的资源的替换标志位设为 true
// func (ms *MemoryStorage) MarkAsReplaced(url string) bool {
//   defer ms.mutex.Unlock()
//   ms.mutex.Lock()
//   val, ok := ms.urlToBlockMap[url]
//   if ok {
//     val.Replaced = true
//     logger.Infof("replace bit for url <%v> set to true", url)
//     return ok
//   }
//   return ok
// }

// BindURLAndStreamID 在 storage 实例中绑定 url 和 对应的 StreamID
func (ms *MemoryStorage) BindURLAndStreamID(url string, id StreamID) {
  defer ms.mutex.Unlock()
  ms.mutex.Lock()
  index, ok := ms.URLToManagedStreamIndex[url]
  if !ok {
		// 不会向 IDToManagedStreamIndex 中添加 unmanaged stream 的条目
    logger.Infof("binding an unmanaged stream with url <%v> to id <%v>", url, id)
    return
  }
  _, ok = ms.IDToManagedStreamIndex[id]
  if ok {
    logger.Errorf("given id already exists, id = <%v>, url = <%v>", id, url)
    return
  }
	ms.IDToManagedStreamIndex[id] = index
	// 绑定 StreamID 和 URL 的关系
	ms.ManagedStreams[index].StreamID = id
  logger.Infof("binding stream id <%v> to url <%v>", id, url)
}
