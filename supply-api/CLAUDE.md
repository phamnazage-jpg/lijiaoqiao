# Supply API - Claude Code 项目规范

## 项目概述

Supply API 是一个基于 Go 的微服务，提供供应链管理功能，包括账户管理、套餐管理、结算服务、收益服务和审计日志。

## 技术栈

- **语言**: Go 1.21+
- **数据库**: PostgreSQL 15+
- **缓存**: Redis
- **框架**: 标准库 + 自定义中间件
- **测试**: Go testing + testify

---

## 代码规范

### 1. 命名规范

#### 1.1 字段命名统一
**关键经验**: 跨模块字段必须保持命名一致，否则会导致类型转换错误。

| 规范 | 示例 | 说明 |
|------|------|------|
| IP来源字段 | `SourceIP` | 统一使用 `SourceIP`，禁止使用 `ClientIP` |
| 追踪ID字段 | `TraceID` | W3C Trace Context 标准 |
| 请求ID字段 | `RequestID` | HTTP 请求追踪 |
| 幂等键字段 | `IdempotencyKey` | 统一命名 |

#### 1.2 结构体命名
```
// ✅ 正确
type AuditEvent struct {
    SourceIP string `json:"source_ip"`
}

// ❌ 错误 - 与其他模块不一致
type AuditEvent struct {
    ClientIP string `json:"client_ip"`
}
```

### 2. 接口设计

#### 2.1 Store 接口必须包含版本控制
**关键经验**: 乐观锁是防止并发更新导致数据不一致的标准做法。

```go
// ✅ 正确 - 包含 expectedVersion 参数
type SettlementStore interface {
    Update(ctx context.Context, s *Settlement, expectedVersion int) error
}

// ❌ 错误 - 缺少版本控制
type SettlementStore interface {
    Update(ctx context.Context, s *Settlement) error
}
```

#### 2.2 领域服务接口与实现分离
```go
// 领域层定义接口
type SettlementStore interface {
    Create(ctx context.Context, s *Settlement) error
    GetByID(ctx context.Context, supplierID, id int64) (*Settlement, error)
    Update(ctx context.Context, s *Settlement, expectedVersion int) error
    List(ctx context.Context, supplierID int64) ([]*Settlement, error)
    GetWithdrawableBalance(ctx context.Context, supplierID int64) (float64, error)
}
```

### 3. 中间件设计

#### 3.1 Logging 中间件必须使用结构化日志
**关键经验**: 标准库 `log` 无法满足生产环境可观测性需求。

```go
// ✅ 正确 - 使用结构化日志接口
func Logging(next http.Handler, logger logging.Logger) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        fields := map[string]interface{}{
            "method": r.Method,
            "path":   r.URL.Path,
        }
        logger.Info("HTTP request", fields)
        next.ServeHTTP(w, r)
    })
}
```

#### 3.2 Tracing 中间件解析 W3C Trace Context
```go
// W3C Trace Context 标准 traceparent header 格式
// traceparent: 00-{trace-id}-{span-id}-{trace-flags}
func ParseTraceParent(traceParent string) (*TraceContext, error) {
    // 格式: 00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01
    // 长度: 55 字符
}
```

### 4. 健康检查设计

#### 4.1 统一使用 HealthHandler
**关键经验**: 避免重复实现导致的维护负担和不一致。

```go
// ✅ 正确 - 使用统一的 HealthHandler
healthHandler := httpapi.NewHealthHandlerWithDefaults(dbHealthCheck, redisHealthCheck)
mux.HandleFunc("/actuator/health", healthHandler.ServeHealth)

// ❌ 错误 - inline handler 导致代码重复
mux.HandleFunc("/actuator/health", handleHealthCheck(db, redisCache))
```

#### 4.2 健康检查端点路径
| 路径 | 说明 |
|------|------|
| `/actuator/health` | 综合健康检查 |
| `/actuator/health/live` | 存活探针 |
| `/actuator/health/ready` | 就绪探针 |

### 5. 审计日志设计

#### 5.1 事件字段规范
```go
type Event struct {
    EventID     string         `json:"event_id,omitempty"`
    TenantID    int64          `json:"tenant_id"`
    ObjectType  string         `json:"object_type"`   // e.g., "supply_settlement"
    ObjectID    int64          `json:"object_id"`
    Action      string         `json:"action"`         // e.g., "withdraw", "cancel"
    BeforeState map[string]any `json:"before_state,omitempty"`
    AfterState  map[string]any `json:"after_state,omitempty"`
    RequestID   string         `json:"request_id,omitempty"`
    ResultCode  string         `json:"result_code"`    // e.g., "OK", "SUP_SET_4001"
    SourceIP    string         `json:"source_ip,omitempty"`
    CreatedAt   time.Time      `json:"created_at"`
}
```

#### 5.2 敏感信息脱敏
审计日志必须脱敏处理：
- 手机号、邮箱
- 身份证号
- 银行账号
- 密码和密钥

### 6. 错误处理

#### 6.1 错误码格式
```
{SOURCE}_{CATEGORY}_{CODE}
例如: SUP_SET_4001
- SUP: 来源系统
- SET: 业务类别 (settlement)
- 4001: 具体错误码
```

#### 6.2 错误信息不泄露内部细节
```go
// ✅ 正确 - 用户友好错误信息
return errors.New("SUP_SET_4001: withdraw amount exceeds available balance")

// ❌ 错误 - 泄露内部实现
return errors.New("database connection failed: connection refused")
```

### 7. 数据库设计

#### 7.1 乐观锁实现
```sql
-- PostgreSQL 乐观锁
UPDATE settlements
SET status = $1, version = version + 1, updated_at = NOW()
WHERE id = $2 AND version = $3
RETURNING id;
-- 如果返回 0 行，说明版本冲突
```

#### 7.2 悲观锁实现
```sql
-- 扣减配额时使用悲观锁
UPDATE supply_packages
SET available_quota = available_quota - $1,
    sold_quota = sold_quota + $1
WHERE id = $2 AND user_id = $3 AND available_quota >= $1
RETURNING id;
```

---

## 测试规范

### 1. 测试文件命名
```
{package}_test.go              // 标准测试（默认）
{package}_integration_test.go   // 集成测试（需数据库）
{package}_e2e_test.go          // 端到端测试
```

### 2. Mock 接口而非具体实现
```go
// ✅ 正确 - Mock 接口
type mockSettlementStore struct {
    settlements map[int64]*Settlement
}

func (m *mockSettlementStore) GetByID(ctx context.Context, supplierID, id int64) (*Settlement, error) {
    if s, ok := m.settlements[id]; ok && s.SupplierID == supplierID {
        return s, nil
    }
    return nil, errors.New("not found")
}

// ❌ 错误 - Mock 具体类型
type mockRepo struct {
    repo *repository.SettlementRepository
}
```

### 3. Mock 审计存储签名（关键）
审计存储接口方法签名为：
```go
type AuditStore interface {
    Emit(ctx context.Context, event audit.Event) error  // 注意是 audit.Event 不是 interface{}
    Query(ctx context.Context, filter audit.EventFilter) ([]audit.Event, error)
    QueryWithTotal(ctx context.Context, filter audit.EventFilter) ([]audit.Event, int64, error)
    GetByID(ctx context.Context, eventID string) (audit.Event, error)
}
```

### 4. 测试覆盖率要求
| 模块 | 最低覆盖率 | 说明 |
|------|-----------|------|
| domain | 70% | 领域模型、状态机、业务规则 |
| audit/service | 80% | 审计服务、告警服务 |
| audit/handler | 75% | HTTP 处理器 |
| audit/model | 80% | 数据模型、验证 |
| audit/sanitizer | 80% | 敏感信息脱敏 |
| middleware | 80% | 认证、限流、幂等 |
| security | 80% | 安全相关 |
| iam | 70% | 身份认证授权 |

### 5. 测试运行命令
```bash
# 快速测试（跳过集成测试）
go test -short ./...

# 包含集成测试
go test -tags=integration ./...

# 详细输出
go test -v -cover ./internal/domain/...

# 检查覆盖率
go test -cover ./...
```

### 6. 测试命名规范
```
Test{Service}_{Method}_{Scenario}

示例：
- TestAccountService_Create_Success
- TestAccountService_Create_InvalidInput
- TestPackageService_Publish_ExpiredPackage
- TestSettlementService_Withdraw_ExceedsBalance
```

---

## Git 提交规范

### 1. 提交信息格式
```
{type}: {subject}

{body}

{footer}
```

### 2. Type 类型
- `fix`: 缺陷修复
- `feat`: 新功能
- `docs`: 文档更新
- `refactor`: 重构
- `test`: 测试相关
- `chore`: 构建/工具变更

### 3. 示例
```
fix: 修复结算更新时的乐观锁冲突

问题：并发更新结算状态时可能导致数据覆盖
解决：添加 expectedVersion 参数实现乐观锁

Fixes: SUP-1234
```

---

## 依赖管理

### 1. 使用 Go Modules
```bash
go mod init lijiaoqiao/supply-api
go mod tidy
```

### 2. 禁止依赖未验证的包
评估标准：
- 维护状态
- 下载量
- 安全漏洞历史

---

## 配置管理

### 1. 环境配置分离
```
config/
├── config.dev.yaml
├── config.staging.yaml
└── config.prod.yaml
```

### 2. 敏感配置通过环境变量
```yaml
database:
  password: ${DB_PASSWORD}
```

---

## 常见问题与解决方案

### Q1: 如何处理跨模块命名不一致？
**A**: 建立字段命名标准文档，所有模块遵循 W3C 和行业通用命名。

### Q2: 何时使用乐观锁 vs 悲观锁？
**A**:
- 乐观锁：读多写少，低冲突场景
- 悲观锁：高并发写，财务类敏感操作

### Q3: 如何避免中间件代码重复？
**A**: 使用统一的 Handler 模式，集中管理公共逻辑。

### Q4: 审计日志的性能影响如何控制？
**A**: 采样策略 + 异步写入 + 批量处理。

---

## 文件结构

```
supply-api/
├── cmd/supply-api/              # 主程序入口
├── internal/
│   ├── audit/                    # 审计日志模块
│   │   ├── model/              # 审计事件模型
│   │   ├── service/            # 审计服务
│   │   ├── handler/            # HTTP 处理器
│   │   ├── repository/         # 数据库仓储
│   │   ├── sanitizer/          # 敏感信息脱敏
│   │   └── events/             # 事件定义
│   ├── iam/                     # IAM 模块
│   ├── domain/                  # 领域模型
│   ├── middleware/               # HTTP 中间件
│   ├── repository/              # 通用数据仓储
│   ├── cache/                   # Redis 缓存
│   ├── config/                  # 配置管理
│   └── pkg/                     # 公共包
├── sql/postgresql/              # 数据库 DDL 脚本
└── docs/                        # 设计文档
```
