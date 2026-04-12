# 日志标准化重构方案

## 问题分析

### 当前状态
- 已实现结构化日志接口 (`internal/pkg/logging/logger.go`)
- 支持 JSON 格式输出，带 trace_id、span_id、request_id 等标准字段
- 内置敏感字段脱敏

### 问题
大量代码仍使用标准库 `log`：
```go
log.Printf("[AUDIT_ERROR] failed to emit audit event: %v", err)
```

**缺陷：**
1. 非结构化，难以解析和搜索
2. 缺少 trace_id，无法关联请求链路
3. 缺少标准化字段，grep 难度大
4. 无法输出到统一日志收集系统

## 优化方案

### 1. 创建 log.Logger 适配器

将标准库 `log` 重定向到结构化日志：

```go
// logadapter 包：将标准 log 重定向到结构化日志
package logadapter

import (
    "log"
    "os"
    "sync"
)

var (
    defaultLogger Logger
    mu            sync.Mutex
)

// SetLogger 设置全局日志实例
func SetLogger(l Logger) {
    mu.Lock()
    defer mu.Unlock()
    defaultLogger = l
}

// Printf 实现 log.Logger 接口
func (l *stdLoggerAdapter) Printf(format string, v ...interface{}) {
    if defaultLogger != nil {
        defaultLogger.Info(fmt.Sprintf(format, v...), nil)
    }
}

// init 拦截标准库 log
func init() {
    log.SetOutput(&stdLoggerAdapter{})
    log.SetFlags(0) // 禁用标准库时间戳
}
```

### 2. 使用 slog 作为后端（推荐）

Go 1.21+ 内置 `log/slog`，直接使用：

```go
import "log/slog"

func main() {
    // 初始化 slog JSON handler
    handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    })
    slog.SetDefault(slog.New(handler))

    // 使用 slog
    slog.Info("server started", "addr", ":8080")
    slog.Error("connection failed", "err", err, "peer", "10.0.0.1")
}
```

### 3. 统一字段命名

| 字段 | 标准化名称 | 说明 |
|------|-----------|------|
| 租户ID | `tenant_id` | |
| 用户ID | `user_id` | |
| 请求ID | `request_id` | HTTP 请求追踪 |
| Trace ID | `trace_id` | W3C Trace Context |
| 操作类型 | `operation` | 如 `withdraw`, `create` |
| 耗时 | `duration_ms` | 毫秒 |
| 状态码 | `status_code` | HTTP 状态码 |
| 错误码 | `error_code` | 业务错误码 |

### 4. 实施步骤

#### Phase 1: 基础设施
1. 使用 `log/slog` 替代自定义实现
2. 配置 JSON handler 输出到 stdout
3. 设置默认字段（service_name, version）

#### Phase 2: HTTP 层改造
1. 中间件注入 trace_id、request_id
2. Request/Response 日志
3. 慢请求日志

#### Phase 3: Domain 层改造
1. 业务操作日志（withdraw, create）
2. 错误日志包含 error_code
3. 审计事件日志

#### Phase 4: 清理
1. 移除所有 `log.Printf`
2. 添加 log level 配置
3. 支持日志采样

### 5. 日志格式规范

**JSON 日志条目：**
```json
{
  "time": "2026-04-13T10:15:30.123Z",
  "level": "INFO",
  "service": "supply-api",
  "trace_id": "abc123",
  "request_id": "req-456",
  "tenant_id": 1001,
  "operation": "withdraw",
  "duration_ms": 45,
  "status_code": 200,
  "message": "withdraw completed"
}
```

**错误日志：**
```json
{
  "time": "2026-04-13T10:15:30.123Z",
  "level": "ERROR",
  "service": "supply-api",
  "error_code": "SUP_SET_4001",
  "error": "withdraw amount exceeds available balance",
  "tenant_id": 1001,
  "operation": "withdraw",
  "stack_trace": "..."
}
```

### 6. 迁移检查清单

- [ ] 移除 `log.Printf` 替换为结构化日志
- [ ] 所有日志包含 `trace_id`（从 context 提取）
- [ ] 错误日志包含 `error_code`
- [ ] 敏感字段自动脱敏
- [ ] 配置 log level 支持动态调整
- [ ] Benchmark 测试日志性能影响 < 5%

## 推荐方案

**使用 Go 1.21+ 内置 `log/slog`**，无需引入第三方依赖。

```go
// 推荐配置
handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level:     slog.LevelInfo,
    AddSource: false,
    ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
        if a.Key == slog.TimeKey {
            a.Value = slog.StringValue(a.Value.Time().Format(time.RFC3339Nano))
        }
        return a
    },
})
slog.SetDefault(slog.New(handler))
```
