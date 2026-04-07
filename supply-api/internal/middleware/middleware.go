package middleware

import (
	"log"
	"net/http"
	"runtime/debug"

	"lijiaoqiao/supply-api/internal/pkg/logging"
)

// Recovery 中间件 - 恢复 panic
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic recovered: %v\n%s", err, debug.Stack())
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Logging 中间件 - 请求日志（使用结构化JSON日志）
// P1-010修复: 使用结构化日志替代标准log
func Logging(next http.Handler, logger logging.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 从context获取追踪信息
		fields := make(map[string]interface{})
		fields["method"] = r.Method
		fields["path"] = r.URL.Path
		fields["query"] = r.URL.RawQuery

		// 尝试获取request_id
		if requestID := r.Header.Get("X-Request-Id"); requestID != "" {
			fields["request_id"] = requestID
		} else if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
			fields["request_id"] = requestID
		}

		// 尝试获取trace_id
		if tc, ok := GetTraceContext(r.Context()); ok {
			fields["trace_id"] = tc.TraceID
			fields["span_id"] = tc.SpanID
		}

		logger.Info("HTTP request", fields)
		next.ServeHTTP(w, r)
	})
}

// RequestID 中间件 - 请求追踪
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-Id")
		if requestID == "" {
			requestID = r.Header.Get("X-Request-ID")
		}
		if requestID != "" {
			w.Header().Set("X-Request-Id", requestID)
		}
		next.ServeHTTP(w, r)
	})
}
