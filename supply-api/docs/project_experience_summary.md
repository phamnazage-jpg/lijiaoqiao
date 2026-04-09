# Supply API 项目经验总结

> 本文档总结项目实施过程中的关键经验教训

## 一、设计阶段常见问题

### 1.1 跨文档命名不一致

**问题描述**：
在代码审查中发现多处字段命名不一致，如 `ClientIP` vs `SourceIP`，导致类型转换错误。

**受影响的文件**：
- `auth.go` 使用 `ClientIP`
- `audit_event.go` 使用 `SourceIP`

**修复方案**：
统一使用 `SourceIP`，更新所有引用。

**经验教训**：
- 建立跨模块字段命名标准文档
- Code Review 时重点检查命名一致性
- 使用 linter 检测不一致的字段名

### 1.2 接口定义与实现不匹配

**问题描述**：
领域层定义的 Store 接口缺少乐观锁参数，但实现层已支持。

**示例**：
```go
// 接口定义（缺少版本控制）
type SettlementStore interface {
    Update(ctx context.Context, s *Settlement) error
}

// 实现（已支持乐观锁）
func (r *SettlementRepository) Update(ctx context.Context, pkg *Settlement, expectedVersion int) error
```

**修复方案**：
同步更新接口定义，添加 `expectedVersion` 参数。

**经验教训**：
- 接口定义必须与实现保持同步
- 大型重构前先梳理接口依赖
- 使用接口适配器模式桥接新旧实现

### 1.3 缓存与吊销机制矛盾

**问题描述**：
Token 缓存在有效期内无法及时吊销。

**修复方案**：
- 缓存 TTL 设置较短（10秒）
- 吊销时主动失效缓存
- 后端状态变更触发缓存刷新

**经验教训**：
- 缓存策略必须考虑吊销场景
- 主动失效优于被动过期

---

## 二、代码实现常见问题

### 2.1 重复代码

**问题描述**：
`main.go` 中存在与 `healthcheck.go` 重复的健康检查处理函数。

**修复前**：
```go
// main.go 中的 inline handler
mux.HandleFunc("/actuator/health", handleHealthCheck(db, redisCache))
mux.HandleFunc("/actuator/health/live", handleLiveness)
mux.HandleFunc("/actuator/health/ready", handleReadiness)

// healthcheck.go 中已有的完整实现
type HealthHandler struct {
    healthChecker  *DefaultHealthChecker
    readinessChecks []HealthChecker
    livenessChecks []HealthChecker
}
```

**修复后**：
```go
// 统一使用 HealthHandler
healthHandler := httpapi.NewHealthHandlerWithDefaults(dbHealthCheck, redisHealthCheck)
mux.HandleFunc("/actuator/health", healthHandler.ServeHealth)
```

**经验教训**：
- 优先使用已有的通用组件
- 避免在 main.go 中直接实现业务逻辑
- 定期清理不再使用的 inline handlers

### 2.2 结构化日志缺失

**问题描述**：
Logging 中间件使用标准库 `log.Printf` 而非结构化日志。

**修复前**：
```go
func Logging(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        log.Printf("%s %s", r.Method, r.URL.Path)  // 非结构化
        next.ServeHTTP(w, r)
    })
}
```

**修复后**：
```go
func Logging(next http.Handler, logger logging.Logger) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        fields := map[string]interface{}{
            "method": r.Method,
            "path":   r.URL.Path,
            "trace_id": tc.TraceID,
        }
        logger.Info("HTTP request", fields)
        next.ServeHTTP(w, r)
    })
}
```

**经验教训**：
- 生产环境必须使用结构化日志
- 日志需包含：timestamp, level, trace_id, request_id, 业务字段
- 结构化日志便于查询和分析

### 2.3 未使用的导入和函数

**问题描述**：
代码变更后遗留未使用的导入和函数定义。

**示例**：
删除 inline handler 后未删除 `encoding/json` 导入。

**经验教训**：
- 使用 `go vet` 和 IDE 检查未使用的导入
- 删除废弃代码而非注释
- 代码重构后立即清理相关引用

---

## 三、数据库设计问题

### 3.1 字段映射错误

**问题描述**：
Package Repository 中 `SupplierID` 重复映射到 `supply_account_id` 和 `user_id`。

**修复前**：
```go
pkg.SupplierID, pkg.SupplierID, pkg.Platform, pkg.Model,  // 错误：SupplierID 出现两次
```

**修复后**：
```go
pkg.SupplierID, pkg.AccountID, pkg.Platform, pkg.Model,  // 正确映射
```

**经验教训**：
- SQL 参数绑定时仔细核对字段顺序
- 使用结构体标签明确映射关系
- 编写数据库相关的单元测试

### 3.2 乐观锁与悲观锁选择

**使用场景**：

| 场景 | 锁策略 | 说明 |
|------|--------|------|
| 结算状态更新 | 乐观锁 | 低频操作，冲突概率低 |
| 配额扣减 | 悲观锁 | 高并发，需要保证原子性 |
| 账户余额 | 悲观锁 | 财务敏感操作 |

**经验教训**：
- 根据业务场景选择合适的锁策略
- 乐观锁需处理 `ErrConcurrencyConflict` 错误
- 悲观锁需考虑锁超时和死锁

---

## 四、中间件设计问题

### 4.1 Tracing 中间件缺失

**问题描述**：
未实现 W3C Trace Context 标准，无法进行分布式追踪。

**修复方案**：
```go
// 解析 traceparent header
func ParseTraceParent(traceParent string) (*TraceContext, error) {
    // 格式: 00-{trace-id}-{span-id}-{trace-flags}
    // 长度: 55 字符
    traceID := traceParent[3:35]
    spanID := traceParent[36:52]
}

// 注入到 context
func TracingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        traceParent := r.Header.Get("traceparent")
        // 解析并注入 context
    })
}
```

**经验教训**：
- 微服务必须实现分布式追踪
- 遵循 W3C Trace Context 标准
- trace_id 需要贯穿所有日志

### 4.2 TimeoutMiddleware 并发安全

**问题描述**：
超时中间件实现存在死锁和竞态条件，导致测试不稳定。

**错误实现（死锁）**：
```go
// 错误：主 goroutine 获取锁后等待 handler goroutine
mu.Lock()
go func() {
    next.ServeHTTP(wrapped, r)  // wrapped.WriteHeader() 尝试获取同一个锁
    mu.Unlock()  // 死锁！
}()
select {
case <-done:
    return
case <-time.After(timeout):
    mu.Lock()  // 再次尝试获取锁 - 死锁！
    // ...
}
```

**错误实现（竞态）**：
```go
// 错误：handler 和超时同时写入 ResponseWriter
go func() {
    next.ServeHTTP(w, r)  // 写入 200
    close(handlerDone)
}()

select {
case <-handlerDone:
    return
case <-time.After(timeout):
    // handler 可能同时写入，造成竞态
    http.Error(w, "timeout", 504)
}
```

**正确实现**：
```go
func WithTimeoutMiddleware(next http.Handler, timeout time.Duration) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var mu sync.Mutex
        responseSent := false

        handlerDone := make(chan struct{})

        go func() {
            next.ServeHTTP(w, r)
            close(handlerDone)
        }()

        select {
        case <-handlerDone:
            return
        case <-time.After(timeout):
            mu.Lock()
            if !responseSent {
                responseSent = true
                mu.Unlock()
                w.Header().Set("X-Timeout", "true")
                http.Error(w, fmt.Sprintf("middleware timeout after %v", timeout), http.StatusGatewayTimeout)
                return
            }
            mu.Unlock()
            return
        }
    })
}
```

**经验教训**：
- 中间件的锁设计必须清晰：主 goroutine 和 handler goroutine 不能同时持有锁
- 使用 `sync.Once` 或互斥锁 + 标志位确保响应只发送一次
- 超时设置必须足够长（建议 >100ms），避免在 race 检测下不稳定
- 基准测试和单元测试的超时设置需要合理匹配
- 测试覆盖率不等于测试质量：需要真正验证并发场景

---

## 五、测试问题

### 5.1 Mock 对象未正确覆盖所有方法

**问题描述**：
`captureLogger` 仅覆盖了 `log()` 方法，但测试调用的是 `Info()`、`Debug()` 等方法。

**修复**：
```go
type captureLogger struct {
    *jsonLogger
}

func (l *captureLogger) Info(msg string, fields ...map[string]interface{}) {
    var f map[string]interface{}
    if len(fields) > 0 {
        f = fields[0]
    }
    l.log(LogLevelInfo, msg, f)
}
// 类似覆盖 Debug, Warn, Error, Fatal
```

**经验教训**：
- Go 嵌入式方法调用解析到被嵌入类型
- Mock 对象必须覆盖所有公共方法
- 编写测试后实际运行验证

---

## 六、项目管理问题

### 6.1 过期文件清理

**问题描述**：
Git 仓库中遗留大量已删除但未清理的报告文件。

**修复命令**：
```bash
git rm $(git status --short | grep "^ D " | sed 's/^ D //')
```

**经验教训**：
- 定期清理已删除文件的 git 跟踪状态
- 报告文件使用归档目录而非版本控制
- CI/CD 流程自动清理过期文件

### 6.2 文档与代码不同步

**问题描述**：
代码变更后相关设计文档未同步更新。

**经验教训**：
- 文档更新纳入代码变更流程
- 使用文档即代码（Docs as Code）实践
- 自动化文档生成

---

## 七、关键设计决策记录

### 7.1 JWT Token 格式
- 算法：HS256（内部服务）/ RS256（跨服务）
- Claims：subject_id, role, scope, tenant_id, iat, exp

### 7.2 审计事件采样策略
- 成功率：1% 采样
- 失败率：100% 采样

### 7.3 健康检查路径
- `/actuator/health` - 综合健康
- `/actuator/health/live` - 存活探针
- `/actuator/health/ready` - 就绪探针

---

## 八、测试验证经验（2026-04-09）

### 8.1 集成测试环境配置

**Unix Socket vs TCP 连接**：
```bash
# Unix socket（开发环境推荐）
export SUPPLY_API_DB_HOST="/var/run/postgresql"
dsn = "postgres://user:password@/dbname?host=/var/run/postgresql&sslmode=disable"

# TCP 连接（生产环境）
dsn = "postgres://user:password@localhost:5432/dbname?sslmode=disable"
```

**常见错误**：
- `password authentication failed` - 检查 pg_hba.conf 或使用 Unix socket
- `database does not exist` - DSN 路径解析错误
- `server error (FATAL)` - 主机名解析问题

### 8.2 测试覆盖率要求

| 模块 | 当前覆盖率 | 最低要求 | 优秀 |
|------|-----------|---------|------|
| audit/events | 97.6% | 80% | 95%+ |
| audit/handler | 79.6% | 75% | 85%+ |
| audit/model | 93.8% | 80% | 90%+ |
| audit/sanitizer | 84.3% | 80% | 90%+ |
| audit/service | 83.0% | 80% | 85%+ |
| security | 88.8% | 80% | 90%+ |
| domain | 61.2% | 70% | 80%+ |
| middleware | 53.9% | 70% | 80%+ |

### 8.3 性能基准测试

**运行方式**：
```bash
go test -tags=slow -bench=. -benchmem -run=^$ ./internal/benchmark/...
```

**参考性能数据**：
| 操作 | 性能 | Allocation |
|------|------|-----------|
| AccountService_Create | 678.7 ns/op | 601 B/op, 5 allocs |
| AccountService_Verify | 3.6 ns/op | 0 B/op, 0 allocs |
| PackageService_CreateDraft | 508.8 ns/op | 462 B/op, 1 allocs |
| SettlementService_Withdraw | 625.7 ns/op | 463 B/op, 2 allocs |
| ConcurrentAccountAccess | 3.5 ns/op | 0 B/op, 0 allocs |
| LoggingMiddleware | 1.8 μs/op | 5.4 KB/op, 18 allocs |
| TracingMiddleware | 1.9 μs/op | 5.7 KB/op, 19 allocs |

### 8.4 E2E 测试常见问题

**编译错误**：
```go
// 错误：导入但未使用
import (
    "context"                    // ❌ 未使用
    "github.com/stretchr/testify/assert"  // ❌ 未使用
)

// 修复
import (
    _ "context"  // 使用空白导入或删除
)
```

**变量声明未使用**：
```go
// 错误
ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
defer cancel()

// 修复
_, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
defer cancel()
_ = ctx  // 如果确实需要 ctx
```

### 8.5 数据库表验证

**验证步骤**：
1. 连接数据库：`psql` 或 `pg_isready`
2. 列出所有表：`SELECT table_name FROM information_schema.tables WHERE table_schema = 'public'`
3. 验证字段：`SELECT column_name FROM information_schema.columns WHERE table_name = 'xxx'`
4. 验证索引：`SELECT indexname FROM pg_indexes WHERE tablename = 'xxx'`

**核心表结构验证通过**：
- `supply_accounts` - 包含 `version` 字段（乐观锁）
- `supply_packages` - 包含 `available_quota`、`version` 字段
- `supply_settlements` - 支持 `FOR UPDATE SKIP LOCKED`、`NOWAIT`

### 8.6 中间件鲁棒性验证

**TimeoutMiddleware 并发问题**：
- 主 goroutine 和 handler goroutine 不能同时持有锁
- 使用 `sync.Once` 或互斥锁 + 标志位确保响应只发送一次
- 超时设置必须足够长（建议 >100ms）

**性能测试注意**：
- 基准测试在 short mode 下会被跳过
- `testing.Short()` 返回 true 时不运行基准测试
- 使用 `-short=false` 覆盖默认行为

---

## 九、改进建议

### 9.1 短期改进
1. [x] 补充集成测试（43个测试已通过）
2. [x] 修复 E2E 测试编译错误
3. [x] 建立基准测试套件
4. [ ] 完善 API 文档（OpenAPI/Swagger）
5. [ ] 补充 middleware 模块测试覆盖率（当前 53.9%）

### 9.2 中期改进
1. [ ] 实现数据库连接池监控
2. [ ] 添加 Redis 缓存命中率指标
3. [ ] 完善错误码体系文档
4. [ ] 补充 domain 模块测试覆盖率（当前 61.2%）

### 9.3 长期改进
1. [ ] 迁移到 gRPC
2. [ ] 实现服务网格
3. [ ] 添加 A/B 测试框架
