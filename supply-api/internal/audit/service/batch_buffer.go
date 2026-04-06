package service

import (
	"context"
	"sync"
	"time"

	"lijiaoqiao/supply-api/internal/audit/model"
)

// BatchBufferConfig 批量缓冲区配置
type BatchBufferConfig struct {
	BatchSize     int           // 批量大小（默认50）
	FlushInterval time.Duration // 刷新间隔（默认5ms）
	BufferSize    int           // 通道缓冲大小（默认1000）
}

// DefaultBatchBufferConfig 默认配置
var DefaultBatchBufferConfig = BatchBufferConfig{
	BatchSize:     50,
	FlushInterval: 5 * time.Millisecond,
	BufferSize:    1000,
}

// BatchBuffer 批量写入缓冲区
// 设计目标：50条/批或5ms刷新间隔，支持5K-8K TPS
type BatchBuffer struct {
	config  BatchBufferConfig
	eventCh chan *model.AuditEvent
	buffer  []*model.AuditEvent
	mu      sync.Mutex
	closed  bool

	flushTick *time.Ticker
	stopCh    chan struct{}
	doneCh    chan struct{}

	// FlushHandler 处理批量刷新回调
	FlushHandler func(events []*model.AuditEvent) error
}

// NewBatchBuffer 创建批量缓冲区
func NewBatchBuffer(batchSize int, flushInterval time.Duration) *BatchBuffer {
	config := DefaultBatchBufferConfig
	if batchSize > 0 {
		config.BatchSize = batchSize
	}
	if flushInterval > 0 {
		config.FlushInterval = flushInterval
	}

	return &BatchBuffer{
		config:    config,
		eventCh:   make(chan *model.AuditEvent, config.BufferSize),
		buffer:    make([]*model.AuditEvent, 0, batchSize),
		flushTick: time.NewTicker(config.FlushInterval),
		stopCh:    make(chan struct{}),
		doneCh:    make(chan struct{}),
	}
}

// Start 启动批量缓冲处理
func (b *BatchBuffer) Start(ctx context.Context) error {
	go b.run()
	return nil
}

// run 后台处理循环
func (b *BatchBuffer) run() {
	defer close(b.doneCh)

	for {
		select {
		case <-b.stopCh:
			// 停止信号：处理剩余缓冲
			b.flush()
			return
		case event := <-b.eventCh:
			b.addEvent(event)
		case <-b.flushTick.C:
			b.flush()
		}
	}
}

// addEvent 添加事件到缓冲
func (b *BatchBuffer) addEvent(event *model.AuditEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buffer = append(b.buffer, event)

	// 达到批量大小立即刷新
	if len(b.buffer) >= b.config.BatchSize {
		b.doFlushLocked()
	}
}

// flush 刷新缓冲（带锁）- 也会处理eventCh中的待处理事件
func (b *BatchBuffer) flush() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// 处理eventCh中已有的事件
	for {
		select {
		case event := <-b.eventCh:
			b.buffer = append(b.buffer, event)
		default:
			goto done
		}
	}
done:
	b.doFlushLocked()
}

// doFlushLocked 执行刷新（ caller 必须持锁）
func (b *BatchBuffer) doFlushLocked() {
	if len(b.buffer) == 0 {
		return
	}

	// 复制缓冲数据
	events := make([]*model.AuditEvent, len(b.buffer))
	copy(events, b.buffer)

	// 清空缓冲
	b.buffer = b.buffer[:0]

	// 调用处理函数（如果已设置）
	if b.FlushHandler != nil {
		if err := b.FlushHandler(events); err != nil {
			// TODO: 错误处理 - 记录日志、重试等
			// 当前简化处理：仅记录
		}
	}
}

// Add 添加审计事件
func (b *BatchBuffer) Add(event *model.AuditEvent) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.closed {
		return ErrBufferClosed
	}

	select {
	case b.eventCh <- event:
		return nil
	default:
		// 通道满，添加到缓冲
		b.buffer = append(b.buffer, event)
		if len(b.buffer) >= b.config.BatchSize {
			b.doFlushLocked()
		}
		return nil
	}
}

// FlushNow 立即刷新
func (b *BatchBuffer) FlushNow() error {
	b.flush()
	return nil
}

// Close 关闭缓冲区
func (b *BatchBuffer) Close() error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil
	}
	b.closed = true
	b.mu.Unlock()

	close(b.stopCh)
	<-b.doneCh
	b.flushTick.Stop()
	close(b.eventCh)

	return nil
}

// SetFlushHandler 设置刷新处理器
func (b *BatchBuffer) SetFlushHandler(handler func(events []*model.AuditEvent) error) {
	b.FlushHandler = handler
}

// 错误定义
var (
	ErrBufferClosed        = &BatchBufferError{"buffer is closed"}
	ErrMissingFlushHandler = &BatchBufferError{"flush handler not set"}
)

// BatchBufferError 批量缓冲错误
type BatchBufferError struct {
	msg string
}

func (e *BatchBufferError) Error() string {
	return e.msg
}
