# Supply API 测试方案 v1.2

## 1. 概述

本文档定义了 Supply API 项目的系统性测试策略，确保代码质量、可维护性和快速迭代能力。
遵循 Google Testing Blog 和 Atlassian Testing Guide 行业最佳实践。

---

## 2. 测试金字塔（标准三层）

### 2.1 金字塔结构

```
         ┌─────────────┐
         │    E2E     │  ← 5-10% (Playwright)
        ┌─────────────┐
        │ Integration │  ← 15-20% (Store, DB, 真实依赖)
       ┌───────────────┐
       │    Unit     │  ← 70-80% (业务逻辑、领域模型)
      └───────────────┘
```

### 2.2 各层定义

| 层级 | 目标占比 | 定义 | Build Tag | 速度目标 |
|------|----------|------|-----------|----------|
| E2E | 5-10% | 关键业务流程端到端验证 | `//go:build e2e` | < 1s |
| Integration | 15-20% | Store、Repository、DB 集成 | `//go:build integration` | < 100ms |
| Unit | 70-80% | 业务逻辑、领域模型、组件 | (默认) | < 10ms |

**注意**: 移除非标准的 "Component" 层，其包含在 Unit 层中。

---

## 3. 测试组织结构

### 3.1 文件命名规范

```
{package}_test.go                # 单元测试（默认）
{package}_integration_test.go   # 集成测试（需数据库）
{package}_e2e_test.go          # E2E 测试（需完整环境）
{package}_slow_test.go         # 慢速测试（默认跳过）
```

### 3.2 Build Tag 使用

```go
//go:build unit
// +build unit

package domain_test  // 单元测试

//go:build integration
// +build integration

package repository_test  // 集成测试

//go:build e2e
// +build e2e

package e2e_test  // E2E 测试

//go:build slow
// +build slow

package slow_test  // 慢速测试（CI中默认跳过）
```

### 3.3 测试包结构

```
internal/
├── domain/                    # 领域模型
│   ├── account.go
│   ├── account_test.go       # 账号单元测试
│   ├── package.go
│   ├── package_test.go      # 套餐单元测试
│   └── invariants_test.go   # 不变量测试
│
├── testutil/                # 测试工具包（新增）
│   ├── factory/              # 测试数据工厂
│   │   ├── account.go
│   │   ├── package.go
│   │   └── settlement.go
│   ├── mock/                # 统一Mock
│   │   └── mocks.go
│   └── assert/              # 自定义断言
│       └── assertions.go
│
├── middleware/              # HTTP中间件
│   ├── auth.go
│   └── auth_test.go        # 认证测试
```

---

## 4. 测试数据管理

### 4.1 测试数据工厂（新增）

```go
// internal/testutil/factory/account.go

type AccountFactory struct {
    supplierID  int64
    provider    Provider
    accountType AccountType
    credential  string
    riskAck     bool
}

func NewAccountFactory() *AccountFactory {
    return &AccountFactory{
        supplierID:  1001,
        provider:    ProviderOpenAI,
        accountType: AccountTypeAPIKey,
        credential:  "sk-test-key",
        riskAck:     true,
    }
}

func (f *AccountFactory) WithSupplierID(id int64) *AccountFactory {
    f.supplierID = id
    return f
}

func (f *AccountFactory) WithProvider(p Provider) *AccountFactory {
    f.provider = p
    return f
}

func (f *AccountFactory) Build() *CreateAccountRequest {
    return &CreateAccountRequest{
        SupplierID:  f.supplierID,
        Provider:    f.provider,
        AccountType: f.accountType,
        Credential:  f.credential,
        RiskAck:     f.riskAck,
    }
}

// 使用示例
func TestAccountService_Create(t *testing.T) {
    factory := NewAccountFactory()

    // 正常场景
    req := factory.Build()

    // 边界场景
    invalidReq := factory.
        WithCredential("").
        Build()
}
```

### 4.2 固定测试数据

```go
func TestAccountService_Create(t *testing.T) {
    store := newMockAccountStore()

    req := &CreateAccountRequest{
        SupplierID:  1001,
        Provider:    ProviderOpenAI,
        AccountType: AccountTypeAPIKey,
        Credential:  "sk-test-key",
        RiskAck:     true,
    }

    account, err := store.Create(context.Background(), req)
    // ...
}
```

### 4.3 边界值测试

```go
tests := []struct {
    name  string
    input float64
    want  bool
}{
    {"zero", 0.0, true},
    {"positive", 100.0, true},
    {"negative", -1.0, false},
    {"very large", 1e10, true},
}
```

---

## 5. 单元测试规范

### 5.1 测试结构 (AAA模式)

```go
func TestXXX_Scenario(t *testing.T) {
    // Arrange - 准备测试数据
    store := newMockStore()
    svc := NewService(store)

    // Act - 执行被测操作
    result, err := svc.DoSomething(ctx, req)

    // Assert - 验证结果
    assert.NoError(t, err)
    assert.Equal(t, expected, result)
}
```

### 5.2 Mock 接口而非具体实现

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

### 5.3 Mock 审计存储正确姿势

```go
type AuditStore interface {
    Emit(ctx context.Context, event audit.Event) error
    Query(ctx context.Context, filter audit.EventFilter) ([]audit.Event, error)
    QueryWithTotal(ctx context.Context, filter audit.EventFilter) ([]audit.Event, int64, error)
    GetByID(ctx context.Context, eventID string) (audit.Event, error)
}

// ✅ 正确 - 使用具体类型
func (m *mockAuditStore) Emit(ctx context.Context, event audit.Event) error {
    return nil
}

// ✅ 错误模拟 - 返回错误
func (m *mockFailingAuditStore) Emit(ctx context.Context, event audit.Event) error {
    return errors.New("audit emit failed")
}
```

### 5.4 表驱动测试

```go
func TestSettlementStatus_Transitions(t *testing.T) {
    tests := []struct {
        name     string
        from     SettlementStatus
        to       SettlementStatus
        expected bool
    }{
        {"pending to processing", SettlementStatusPending, SettlementStatusProcessing, true},
        {"pending to completed", SettlementStatusPending, SettlementStatusCompleted, false},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := ValidateStateTransition(tt.from, tt.to)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

---

## 6. 集成测试规范

### 6.1 Build Tag 隔离

```go
//go:build integration
// +build integration

package repository_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestIntegrationSettlementRepository(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    // 需要真实的 PostgreSQL 或使用 sqlmock
}
```

### 6.2 使用 Test Database

```go
//go:build integration
// +build integration

func TestIntegrationSettlementRepository(t *testing.T) {
    // 选项1: 使用 sqlmock
    db, mock, _ := sqlmock.New()
    defer db.Close()

    // 选项2: 使用轻量级测试数据库
    // 推荐: github.com/testcontainers/testcontainers-go
}
```

### 6.3 运行命令

```bash
# 只运行单元测试（默认）
go test ./...

# 包含集成测试
go test -tags=integration ./...

# 排除集成测试（快速模式）
go test -short ./...

# 运行慢速测试
go test -tags=slow ./...
```

---

## 7. 覆盖率要求

### 7.1 模块覆盖率目标

| 模块 | 最低覆盖率 | 当前 | 状态 |
|------|-----------|------|------|
| domain | 70% | 71.2% | ✅ |
| middleware | 80% | 80.4% | ✅ |
| audit/handler | 75% | 79.6% | ✅ |
| audit/service | 80% | 83.0% | ✅ |
| audit/model | 80% | 93.8% | ✅ |
| audit/sanitizer | 80% | 84.3% | ✅ |
| security | 80% | 88.8% | ✅ |
| iam | 70% | 93.2% | ✅ |

### 7.2 覆盖率检查命令

```bash
# ✅ 推荐：单独验证关键模块（显示真实覆盖率）
go test -cover ./internal/domain/...      # → 71.2%
go test -cover ./internal/middleware/...  # → 80.4%

# ⚠️ 联合运行（覆盖率数值会被稀释）
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

### 7.3 覆盖率未达标处理

1. 分析未覆盖代码路径
2. 添加针对性测试用例
3. 确认覆盖率达到目标
4. **禁止强行凑覆盖率而编写无意义测试**

---

## 8. 测试命名规范

### 8.1 函数命名

```
Test{Service}_{Method}_{Scenario}

示例：
- TestAccountService_Create_Success
- TestAccountService_Create_InvalidInput
- TestPackageService_Publish_ExpiredPackage
- TestSettlementService_Withdraw_ExceedsBalance
```

### 8.2 子测试命名

```go
func TestAccountService_Activate(t *testing.T) {
    tests := []struct {
        name       string
        setup      func() *Account
        supplierID int64
        wantErr    bool
    }{
        {
            name:       "activate pending account success",
            setup:      func() *Account { /* ... */ },
            supplierID: 1001,
            wantErr:    false,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // ...
        })
    }
}
```

---

## 9. 并发与竞态测试

### 9.1 启用 Race 检测

```bash
# 运行所有测试并检测竞态条件
go test -race ./...

# 详细输出
go test -race -v ./internal/domain/...
```

### 9.2 并发安全测试示例

```go
func TestConcurrentAccountAccess(t *testing.T) {
    store := newMockAccountStore()
    var wg sync.WaitGroup

    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            _, err := store.GetByID(context.Background(), 1001, 1)
            assert.NoError(t, err)
        }(i)
    }

    wg.Wait()
}
```

### 9.2 中间件并发测试要点

**中间件的并发安全问题通常体现在**：
- ResponseWriter 的并发写入
- 共享状态的竞争访问
- 超时与正常响应的冲突

**TimeoutMiddleware 正确测试模式**：

```go
// ✅ 正确：超时设置足够长（>100ms），确保正常完成
func TestWithTimeoutMiddleware_NormalCompletion(t *testing.T) {
    handler := WithTimeoutMiddleware(nextHandler, 100*time.Millisecond)

    req := httptest.NewRequest("GET", "/", nil)
    w := httptest.NewRecorder()

    handler.ServeHTTP(w, req)

    if w.Code != http.StatusOK {
        t.Errorf("expected status 200, got %d", w.Code)
    }
}

// ✅ 正确：使用 WaitGroup 确保 handler 完成后再检查
func TestWithTimeoutMiddleware_Concurrent(t *testing.T) {
    var wg sync.WaitGroup
    wg.Add(1)

    go func() {
        defer wg.Done()
        handler.ServeHTTP(w, req)
    }()

    wg.Wait()
    // 现在可以安全检查结果
}
```

**⚠️ 常见错误**：

```go
// ❌ 错误：超时设置过短（<10ms）导致 race 检测下不稳定
timeoutHandler := WithTimeoutMiddleware(handler, 1*time.Millisecond)

// ❌ 错误：测试并发写入 ResponseRecorder 但不等待完成
go func() {
    handler.ServeHTTP(w, req)  // 可能还在执行
}()
time.Sleep(10 * time.Millisecond)
assert.Equal(t, 200, w.Code)  // w 可能尚未写入

// ❌ 错误：假设 select 会优先选择已关闭的 channel
// 当 handlerDone 和 timeout 同时就绪时，行为是未定义的
```

### 9.3 Race 检测必须通过

所有并发测试必须在 race 模式下通过：

```bash
# 必须验证
go test -race ./internal/middleware/...

# 基准测试也需要 race 检测
go test -race -bench=. ./internal/benchmark/...
```

---

## 10. 性能回归测试

### 10.1 执行时间监控

```go
//go:build slow
// +build slow

func TestPerformance_SettlementQuery(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping performance test")
    }

    start := time.Now()

    // 执行查询
    result, err := svc.Query(ctx, req)

    elapsed := time.Since(start)

    // 断言在可接受范围内
    assert.NoError(t, err)
    assert.True(t, elapsed < 100*time.Millisecond,
        "Query took %v, expected < 100ms", elapsed)
}
```

---

## 11. 测试运行策略

### 11.1 本地开发

```bash
# 快速测试（跳过慢速和集成测试）
go test -short ./...

# 完整测试（含集成测试）
go test -tags=integration, slow ./...

# 竞态检测
go test -race ./...

# 只测试修改的包
go test ./internal/domain/...

# 详细输出
go test -v -cover ./internal/domain/...
```

### 11.2 CI/CD

```yaml
# .github/workflows/test.yml
name: Test
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'

      - name: Run unit tests
        run: go test -short -race -coverprofile=coverage.out ./...

      - name: Run integration tests
        run: go test -tags=integration -race -coverprofile=coverage.out ./...

      - name: Run slow tests
        run: go test -tags=slow ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          files: ./coverage.out
```

---

## 12. 常见问题处理

### 12.1 测试依赖外部服务

```go
// ✅ 使用 Mock
func TestSettlementService(t *testing.T) {
    store := newMockSettlementStore()
    svc := NewSettlementService(store, nil, nil)
}
```

### 12.2 时间相关测试

```go
// 使用依赖注入
type SettlementService struct {
    store SettlementStore
    clock Clock  // 注入时间依赖
}
```

### 12.3 Flaky 测试处理

```go
// ❌ 错误 - 在测试中重试
func TestNetworkCall(t *testing.T) {
    for i := 0; i < 3; i++ {
        if err := attempt(); err == nil {
            return
        }
    }
}

// ✅ 正确 - 标记为已知问题并使用超时
func TestNetworkCall(t *testing.T) {
    if os.Getenv("CI") == "" {
        t.Skip("Skipping flaky test outside CI")
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    err := callWithRetry(ctx, endpoint)
    assert.NoError(t, err)
}
```

---

## 13. 测试检查清单

新代码合并前：

- [ ] 所有单元测试通过 (`go test ./...`)
- [ ] 覆盖率达标（无下降）
- [ ] Race 检测通过 (`go test -race ./...`)
- [ ] 无 `TODO` 或 `FIXME` 遗留测试
- [ ] Mock 使用正确接口签名
- [ ] 测试名称符合规范
- [ ] 表驱动测试覆盖边界情况
- [ ] 集成测试在 CI 中正常运行
- [ ] 性能测试在慢速测试套件中

---

## 14. 下一步行动计划

### ✅ 已完成

1. Domain 模块覆盖率提升 (40.7% → 71.2%)
2. Middleware 模块覆盖率提升 (52.7% → 80.4%)
3. Audit handler 模块覆盖率提升 (75% → 79.6%)

### P1 - 创建测试工具包

1. **testutil/factory** - 测试数据工厂
2. **testutil/mock** - 统一Mock库
3. **testutil/assert** - 自定义断言

### P2 - 完善集成测试

1. Repository 模块集成测试骨架
2. Settlement Store 集成测试

### P3 - 补充测试类型

1. E2E 测试骨架
2. 性能回归测试

---

## 15. 参考资料

- [Google Testing Blog](https://testing.googleblog.com/)
- [Atlassian Testing Guide](https://www.atlassian.com/continuous-delivery/software-testing)
- [Go Testing](https://pkg.go.dev/testing)
- [testify](https://github.com/stretchr/testify)
- [testcontainers-go](https://github.com/testcontainers/testcontainers-go)
- [Go Race Detector](https://go.dev/blog/race-detector)
- [Advanced Testing in Go](https://google.github.io/aip/214)
