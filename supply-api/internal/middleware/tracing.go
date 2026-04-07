package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
)

// ==================== P1-006 分布式追踪集成 ====================

// W3C Trace Context 标准实现
// 参考: https://www.w3.org/TR/trace-context/

// TraceContext Trace上下文
type TraceContext struct {
	TraceID   string // 追踪ID (32字符十六进制)
	SpanID    string // Span ID (16字符十六进制)
	TraceFlags string // 追踪标志 (01 = sampled)
}

// W3C Trace Context Header格式
// traceparent: 00-{trace-id}-{span-id}-{trace-flags}
// 例如: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01

const (
	// TraceContextVersion 追踪上下文版本
	TraceContextVersion = "00"
	// TraceFlagSampled 采样标志
	TraceFlagSampled = "01"
	// TraceFlagNotSampled 未采样标志
	TraceFlagNotSampled = "00"
)

// TraceContextKey Trace上下文在context中的key
type traceContextKey struct{}

// WithTraceContext 在context中设置追踪上下文
func WithTraceContext(ctx context.Context, tc *TraceContext) context.Context {
	return context.WithValue(ctx, traceContextKey{}, tc)
}

// GetTraceContext 从context获取追踪上下文
func GetTraceContext(ctx context.Context) (*TraceContext, bool) {
	if tc, ok := ctx.Value(traceContextKey{}).(*TraceContext); ok {
		return tc, true
	}
	return nil, false
}

// ParseTraceParent 解析traceparent header
func ParseTraceParent(traceParent string) (*TraceContext, error) {
	if traceParent == "" {
		return nil, fmt.Errorf("traceparent header is empty")
	}

	// 格式: 00-{trace-id}-{span-id}-{trace-flags}
	// 长度检查
	if len(traceParent) < 55 { // 00- + 32 + - + 16 + - + 02
		return nil, fmt.Errorf("invalid traceparent format")
	}

	// 检查版本
	version := traceParent[0:2]
	if version != TraceContextVersion {
		return nil, fmt.Errorf("unsupported trace context version: %s", version)
	}

	// 提取各部分
	// 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
	// 0123456789012345678901234567890123456789012345678901234
	// 0         1         2         3         4         5
	traceID := traceParent[3:35]
	spanID := traceParent[36:52]
	traceFlags := traceParent[53:55]

	// 验证trace-id长度 (必须是32字符)
	if len(traceID) != 32 {
		return nil, fmt.Errorf("invalid trace-id length: %d", len(traceID))
	}

	// 验证span-id长度 (必须是16字符)
	if len(spanID) != 16 {
		return nil, fmt.Errorf("invalid span-id length: %d", len(spanID))
	}

	// 验证trace-flags
	if traceFlags != TraceFlagSampled && traceFlags != TraceFlagNotSampled {
		return nil, fmt.Errorf("invalid trace-flags: %s", traceFlags)
	}

	return &TraceContext{
		TraceID:   traceID,
		SpanID:    spanID,
		TraceFlags: traceFlags,
	}, nil
}

// FormatTraceParent 格式化traceparent header
func (tc *TraceContext) FormatTraceParent() string {
	return fmt.Sprintf("%s-%s-%s-%s", TraceContextVersion, tc.TraceID, tc.SpanID, tc.TraceFlags)
}

// GenerateTraceID 生成新的TraceID
func GenerateTraceID() string {
	// 简化实现：使用随机16字节 = 32字符十六进制
	return generateRandomHex(32)
}

// GenerateSpanID 生成新的SpanID
func GenerateSpanID() string {
	// 简化实现：使用随机8字节 = 16字符十六进制
	return generateRandomHex(16)
}

// NewTraceContext 创建新的Trace上下文
func NewTraceContext() *TraceContext {
	return &TraceContext{
		TraceID:   GenerateTraceID(),
		SpanID:    GenerateSpanID(),
		TraceFlags: TraceFlagSampled,
	}
}

// NewChildSpanContext 创建子Span上下文
func (tc *TraceContext) NewChildSpanContext() *TraceContext {
	return &TraceContext{
		TraceID:   tc.TraceID,
		SpanID:    GenerateSpanID(),
		TraceFlags: tc.TraceFlags,
	}
}

// IsSampled 是否采样
func (tc *TraceContext) IsSampled() bool {
	return tc.TraceFlags == TraceFlagSampled
}

// TraceIDAndSpanID 生成用于日志的格式
func (tc *TraceContext) LogFields() map[string]string {
	return map[string]string{
		"trace_id": tc.TraceID,
		"span_id":  tc.SpanID,
	}
}

// generateRandomHex 生成密码学安全的随机十六进制字符串
func generateRandomHex(length int) string {
	// length/2 因为hex编码后长度翻倍
	bytes := make([]byte, (length+1)/2)
	if _, err := rand.Read(bytes); err != nil {
		// 不应该发生，但如果发生使用确定性降级
		for i := range bytes {
			bytes[i] = byte(i * 7 % 256)
		}
	}
	return hex.EncodeToString(bytes)[:length]
}

// TracingMiddleware HTTP追踪中间件
// P1-006修复：解析traceparent header并注入到context
func TracingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		traceParent := r.Header.Get("traceparent")

		var tc *TraceContext
		if traceParent != "" {
			// 解析传入的traceparent
			parsed, err := ParseTraceParent(traceParent)
			if err == nil {
				tc = parsed
			}
		}

		if tc == nil {
			// 如果没有有效的traceparent，生成新的
			tc = NewTraceContext()
		}

		// 将trace context注入到request context
		ctx := WithTraceContext(r.Context(), tc)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}
