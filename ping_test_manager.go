package quic

import (
	"sync"
	"time"

	"github.com/lucas-clemente/quic-go/internal/protocol"
	"github.com/lucas-clemente/quic-go/internal/wire"
)

const (
	// 移动平均值的默认样本数目，采用最近十个样本的移动平均值，作为输出 RTT 的数值
	defaultdefaultMeasurementInterval = 50

	// 移动平均线的默认样本数目
	defaultMovingAverageSamples = 10
)

// ptm 实例定义，该类有以下几个主要作用：
// 1. 在后台运行，不停地向服务器发送 ping 帧，发送间隔为最新的 RTT 的移动平均值
//    在没有收到有效 RTT 样本之前，采用默认的 50ms 作为发送间隔
// 2. 通过 GetRTT() 方法向调用者提供以毫秒为单位的 RTT 值
type pingTestManager struct {
	Mutex sync.Mutex

	PingPacketNumbers []protocol.PacketNumber

	signalQueue         []struct{}
	newRTTSampleChan    chan int64
	toSendNextPingFrame chan struct{} // 让 ptm 主线程发送下一个 ping 帧

	// 记录 ping 帧所在的数据包编号及其发送时间的 map
	packetNumberMap map[protocol.PacketNumber]time.Time

	rttSamples         []float64 // 采用定长的环形缓冲区计算延迟的移动平均值
	averageRTT         float64   // rtt 样本的 10 次移动平均值，单位为毫秒，浮点数
	nextSamplePosition int       // 环形缓冲区中下一可用空闲位置

	sum         float64 // 计算移动 RTT 的移动平均值过程中用来计算所有样本之和
	usedSamples int8    // 计算移动 RTT 的移动平均值过程中用来计算使用了多少个样本

	parentSession *session // ptm 实例所属的 session 实例，用于下发 ping 帧
}

// 初始化并返回一个新的 ptm 实例
func newPingTestManager(parentSession *session) *pingTestManager {
	return &pingTestManager{
		packetNumberMap: make(map[protocol.PacketNumber]time.Time),

		PingPacketNumbers: make([]protocol.PacketNumber, 0, 10),

		signalQueue:         make([]struct{}, 0),
		newRTTSampleChan:    make(chan int64, 10),
		toSendNextPingFrame: make(chan struct{}, 1),
		rttSamples:          make([]float64, defaultMovingAverageSamples, defaultMovingAverageSamples),

		// 添加父 session 指针
		parentSession: parentSession,
	}
}

// AddPingPacketSignal 方法向 signalQueue 中添加一个新的信号
func (ptm *pingTestManager) AddPingPacketSignal() {
	ptm.Mutex.Lock()
	ptm.signalQueue = append(ptm.signalQueue, struct{}{})
	ptm.Mutex.Unlock()
}

// ConsumePingPacketSignal 方法移除 signalQueue 中的第一个信号
func (ptm *pingTestManager) ConsumePingPacketSignal() {
	ptm.signalQueue = ptm.signalQueue[1:]
}

// PingPacketAvailable 方法返回是否有 ping 包需要处理
func (ptm *pingTestManager) PingPacketAvailable() bool {
	ptm.Mutex.Lock()
	defer ptm.Mutex.Unlock()
	return len(ptm.signalQueue) > 0
}

// ptm 实例的主线程
func (ptm *pingTestManager) run() {
	for {
		select {
		case rttSample := <-ptm.newRTTSampleChan:
			// 立刻更新 rtt 的 10 次移动平均值
			ptm.updateAverageRTT(rttSample)
			ptm.toSendNextPingFrame <- struct{}{}
		case <-ptm.toSendNextPingFrame:
			// 立刻准备发送下一个 ping 帧
			ptm.scheduleNextPingFrame()
		}
	}
}

// 发送下一 ping 帧
func (ptm *pingTestManager) scheduleNextPingFrame() {
	ptm.parentSession.framer.QueueControlFrame(&wire.PingFrame{})
	ptm.parentSession.scheduleSending()
}

// updateAverageRTT 更新 ptm 实例中该连接的 RTT 样本的移动平均值
// rttSample: 新的 rtt 样本，单位为 us
func (ptm *pingTestManager) updateAverageRTT(rttSample int64) {
	ptm.Mutex.Lock()
	defer ptm.Mutex.Unlock()

	// 需要先转换为秒之后才能放入队列
	ptm.rttSamples[ptm.nextSamplePosition] = float64(rttSample) / 1000.0 / 1000.0
	// 移动到下一空位
	ptm.nextSamplePosition = (ptm.nextSamplePosition + 1) % defaultMovingAverageSamples
	ptm.sum = 0
	ptm.usedSamples = 0
	for i := 0; i < defaultMovingAverageSamples; i++ {
		if ptm.rttSamples[i] == 0 {
			break
		}
		ptm.sum += ptm.rttSamples[i]
		ptm.usedSamples++
	}
	ptm.averageRTT = float64(ptm.sum) / float64(ptm.usedSamples)
}

// GetRTT 方法返回 ptm 实例计算并缓存的 RTT 10 次移动平均值
func (ptm *pingTestManager) GetRTT() float64 {
	ptm.Mutex.Lock()
	defer ptm.Mutex.Unlock()
	return ptm.averageRTT
}
