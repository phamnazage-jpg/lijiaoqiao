# Supply API 测试方案 v1.1

## 1. 概述

本文档定义了 Supply API 项目的系统性测试策略，确保代码质量、可维护性和快速迭代能力。

### 1.1 测试金字塔

```
                        ┌─────────────┐
                        │    E2E     │  ← 关键业务流程验证
                       ┌─────────────┐
                       │  Integration│  ← API、DB、消息队列
                      ┌───────────────┐
                      │    Unit      │  ← 业务逻辑、领域模型
                     ┌─────────────────┐
                     │   Component    │  ← 单组件内部逻辑
                    └──────────────────┘
```

| 层级 | 目标 | 工具 | 速度 | Build Tag |
|------|------|------|------|-----------|
| E2E | 关键业务流程 | Playwright | < 1s | `//go:build e2e` |
| Integration | Store、Repository、DB | go:build integration | < 100ms | `//go:build integration` |
| Unit | 业务逻辑、领域模型 | Go testing + testify | < 1ms | (默认) |
| Component | 单组件内部逻辑 | Go testing | < 1ms | (默认) |

**重要**: Middleware 模块当前覆盖率 52.7%，**未达标**（目标 80%），需优先改进。

---

## 2. 测试组织结构

### 2.1 文件命名规范

```
{package}_test.go              # 单元测试（默认，无 build tag）
{package}_integration_test.go   # 集成测试（需数据库）
{package}_e2e_test.go          # E2E 测试（需完整环境）
```

### 2.2 Build Tag 使用

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
```

### 2.3 测试包结构

```
internal/
├── domain/                    # 领域模型
│   ├── account.go            # 账号领域逻辑
│   ├── account_test.go       # 账号单元测试
│   ├── package.go           # 套餐领域逻辑
│   ├── package_test.go      # 套餐单元测试
│   └── invariants_test.go   # 不变量测试
│
├── audit/                    # 审计模块
│   ├── service/
│   │   ├── audit_service.go
│   │   └── audit_service_test.go
│   └── handler/
│       ├── audit_handler.go
│       └── audit_handler_test.go
│
├── middleware/              # HTTP中间件
│   ├── auth.go
│   ├── auth_test.go        # 认证测试
│   ├── ratelimit.go
│   └── ratelimit_test.go   # 限流测试
```

---

## 3. 单元测试规范

### 3.1 测试结构 (AAA模式)

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

### 3.2 Mock 接口而非具体实现

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

### 3.3 Mock 审计存储正确姿势

审计存储使用 `audit.AuditStore` 接口：

```go
type AuditStore interface {
    Emit(ctx context.Context, event audit.Event) error
    Query(ctx context.Context, filter audit.EventFilter) ([]audit.Event, error)
    QueryWithTotal(ctx context.Context, filter audit.EventFilter) ([]audit.Event, int64, error)
    GetByID(ctx context.Context, eventID string) (audit.Event, error)
}
```

**成功场景：**
```go
func (m *mockAuditStore) Emit(ctx context.Context, event audit.Event) error {
    return nil  // 成功时不记录
}
```

**错误场景（关键）：**
```go
func (m *mockFailingAuditStore) Emit(ctx context.Context, event audit.Event) error {
    return errors.New("audit emit failed")
}
```

### 3.4 表驱动测试

适用于多场景测试：

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

## 4. 测试数据管理

### 4.1 测试 Setup/Teardown

```go
func TestAccountService(t *testing.T) {
    store := newMockAccountStore()

    t.Cleanup(func() {
        // 清理测试数据（如果需要）
    })

    // 测试逻辑...
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

## 5. 集成测试规范

### 5.1 Build Tag 隔离

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
    // 需要真实的 PostgreSQL
}
```

### 5.2 运行命令

```bash
# 只运行单元测试（默认）
go test ./...

# 包含集成测试
go test -tags=integration ./...

# 排除集成测试
go test -short ./...

# 运行特定 tag
go test -tags=unit ./internal/domain/...
```

---

## 6. 覆盖率要求

### 6.1 模块覆盖率目标

| 模块 | 最低覆盖率 | 当前覆盖率 | 状态 | 优先级 |
|------|-----------|-----------|------|--------|
| domain | 70% | 71.2% | ✅ | - |
| **middleware** | **80%** | **52.7%** | 🔴 | **P0** |
| audit/handler | 75% | 79.6% | ✅ | - |
| audit/service | 80% | 83.0% | ✅ | - |
| audit/model | 80% | 93.8% | ✅ | - |
| audit/sanitizer | 80% | 84.3% | ✅ | - |
| security | 80% | 88.8% | ✅ | - |
| iam | 70% | 93.2% | ✅ | - |

**⚠️ 关键问题**: Middleware 模块覆盖率 52.7%，与目标差距 -27.3%，需优先改进。

### 6.2 覆盖率检查命令

**重要**: Go test 在运行 `go test ./...` 时会进行覆盖率聚合，可能导致某些模块显示的覆盖率低于单独运行时的值。

```bash
# ✅ 推荐：单独验证关键模块（显示真实覆盖率）
go test -cover ./internal/domain/...      # → 71.2%
go test -cover ./internal/middleware/... # → 80.4%
go test -cover ./internal/audit/handler/...
go test -cover ./internal/audit/service/...

# ⚠️ 联合运行（覆盖率数值会被稀释，不反映真实情况）
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# 检查覆盖率达标情况（使用单独运行）
go test -cover ./internal/domain/... 2>&1 | grep "coverage"
```

### 6.3 覆盖率未达标处理

1. 分析未覆盖代码路径
2. 添加针对性测试用例
3. 确认覆盖率达到目标
4. **禁止强行凑覆盖率而编写无意义测试**

---

## 7. 测试命名规范

### 7.1 函数命名

```
Test{Service}_{Method}_{Scenario}

示例：
- TestAccountService_Create_Success
- TestAccountService_Create_InvalidInput
- TestPackageService_Publish_ExpiredPackage
- TestSettlementService_Withdraw_ExceedsBalance
```

### 7.2 子测试命名

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
        {
            name:       "activate non-existent fails",
            setup:      func() *Account { return nil },
            supplierID: 9999,
            wantErr:    true,
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

## 8. 并发与竞态测试

### 8.1 启用 Race 检测

```bash
# 运行所有测试并检测竞态条件
go test -race ./...

# 详细输出
go test -race -v ./internal/domain/...
```

### 8.2 并发安全测试示例

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

---

## 9. 测试运行策略

### 9.1 本地开发

```bash
# 快速测试（跳过慢速测试）
go test -short ./...

# 完整测试（含集成测试）
go test -tags=integration ./...

# 竞态检测
go test -race ./...

# 只测试修改的包
go test ./internal/domain/...

# 详细输出
go test -v -cover ./internal/domain/...
```

### 9.2 CI/CD

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

      - name: Check Coverage
        run: |
          go test -cover ./... > coverage.txt
          cat coverage.txt
          # 检查关键模块覆盖率
          grep "middleware" coverage.txt

      - name: Upload coverage
        uses: codecov/codecov-action@v4
        with:
          files: ./coverage.out
```

---

## 10. 常见问题处理

### 10.1 测试依赖外部服务

```go
// ❌ 依赖真实存储
func TestSettlementService(t *testing.T) {
    repo, _ := NewPostgresRepository(db)
}

// ✅ 使用 Mock
func TestSettlementService(t *testing.T) {
    store := newMockSettlementStore()
    svc := NewSettlementService(store, nil, nil)
}
```

### 10.2 时间相关测试

```go
// 使用依赖注入
type SettlementService struct {
    store  SettlementStore
    clock  Clock  // 注入时间依赖
}

func (s *SettlementService) Withdraw(ctx context.Context, supplierID int64, req *WithdrawRequest) (*Settlement, error) {
    now := s.clock.Now()  // 使用注入的时间
}
```

### 10.3 Flaky 测试处理

```go
func TestNetworkCall(t *testing.T) {
    // 重试机制
    var lastErr error
    for i := 0; i < 3; i++ {
        if err := attempt(); err == nil {
            return
        }
        lastErr = err
        time.Sleep(10 * time.Millisecond)
    }
    t.Fatalf("failed after retries: %v", lastErr)
}
```

---

## 11. 测试检查清单

新代码合并前：

- [ ] 所有单元测试通过 (`go test ./...`)
- [ ] 覆盖率达标（无下降）
- [ ] Race 检测通过 (`go test -race ./...`)
- [ ] 无 `TODO` 或 `FIXME` 遗留测试
- [ ] Mock 使用正确接口签名
- [ ] 测试名称符合规范
- [ ] 表驱动测试覆盖边界情况
- [ ] 集成测试在 CI 中正常运行
- [ ] **Middleware 模块覆盖率优先改进**（当前 52.7% → 目标 80%）

---

## 12. 下一步行动计划

### ✅ 已完成

1. **Domain 模块覆盖率提升** (40.7% → 71.2%) ✅
2. **Middleware 模块覆盖率提升** (52.7% → 80.4%) ✅
3. **Audit handler 模块覆盖率提升** (75% → 79.6%) ✅

### P1 - 高优先级
1. Repository 模块覆盖率提升（1.3% → 30%）
2. settlement.go 方法覆盖（部分方法 0%）

### P2 - 中优先级
3. IAM handler/service 测试补充
4. HTTP API handler 测试补充
5. E2E 测试骨架

---

## 13. 参考资料

- [Go Testing](https://pkg.go.dev/testing)
- [testify](https://github.com/stretchr/testify)
- [Go Race Detector](https://go.dev/blog/race-detector)
- [Advanced Testing in Go](https://google.github.io/aip/214)
