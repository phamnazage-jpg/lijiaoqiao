# Supply API 测试方案 v1.0

## 1. 概述

本文档定义了 Supply API 项目的系统性测试策略，确保代码质量、可维护性和快速迭代能力。

### 1.1 测试金字塔

```
                    ┌─────────────┐
                    │    E2E     │  ← 少量关键路径验证
                   ┌─────────────┐
                   │  Integration│  ← API、DB、消息队列集成
                  ┌───────────────┐
                  │    Unit      │  ← 大量快速反馈
                 ┌─────────────────┐
                 │   Component     │  ← 单组件内部逻辑
                ┌───────────────────┐
```

### 1.2 测试类型定义

| 类型 | 目标 | 工具 | 速度 |
|------|------|------|------|
| 单元测试 | 业务逻辑、领域模型 | Go testing + testify | < 1ms |
| 集成测试 | Store、Repository、DB | go:build integration | < 100ms |
| E2E测试 | 关键业务流程 | Playwright | < 1s |

---

## 2. 测试组织结构

### 2.1 文件命名规范

```
{package}_test.go              // 单元测试（默认）
{package}_integration_test.go   // 集成测试（需数据库）
{package}_e2e_test.go          // 端到端测试
```

### 2.2 测试包结构

```
internal/
├── domain/                    # 领域模型
│   ├── account.go            # 账号领域逻辑
│   ├── account_test.go       # 账号单元测试
│   ├── package.go            # 套餐领域逻辑
│   ├── package_test.go       # 套餐单元测试
│   └── invariants_test.go    # 不变量测试
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
│   ├── auth_test.go         # 认证测试
│   ├── ratelimit.go
│   └── ratelimit_test.go    # 限流测试
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

审计存储使用 `audit.AuditStore` 接口，方法签名为：

```go
type AuditStore interface {
    Emit(ctx context.Context, event audit.Event) error
    Query(ctx context.Context, filter audit.EventFilter) ([]audit.Event, error)
    QueryWithTotal(ctx context.Context, filter audit.EventFilter) ([]audit.Event, int64, error)
    GetByID(ctx context.Context, eventID string) (audit.Event, error)
}
```

**错误示例：**
```go
// ❌ 错误 - 使用 interface{}
func (m *mockAuditStore) Emit(ctx context.Context, event interface{}) error {
    return nil
}
```

**正确示例：**
```go
// ✅ 正确 - 使用具体类型
func (m *mockAuditStore) Emit(ctx context.Context, event audit.Event) error {
    return nil
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
        // ...
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

## 4. 集成测试规范

### 4.1 使用 build tag 隔离

```go
//go:build integration
// +build integration

package repository_test

import (
    "testing"
    "github.com/stretchr/testify/assert"
    "lijiaoqiao/supply-api/internal/repository"
)

// IntegrationTestSettlementRepository 需要真实的 PostgreSQL
func TestIntegrationSettlementRepository(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test in short mode")
    }
    // ...
}
```

### 4.2 运行集成测试

```bash
# 只运行单元测试（默认）
go test ./...

# 包含集成测试
go test -tags=integration ./...

# 排除集成测试
go test -short ./...
```

---

## 5. 覆盖率要求

### 5.1 模块覆盖率目标

| 模块 | 最低覆盖率 | 当前覆盖率 | 状态 |
|------|-----------|-----------|------|
| domain | 70% | 71.2% | ✅ |
| middleware | 80% | 80.4% | ✅ |
| audit/service | 80% | 83.0% | ✅ |
| audit/handler | 75% | 79.6% | ✅ |
| audit/model | 80% | 93.8% | ✅ |
| audit/sanitizer | 80% | 84.3% | ✅ |
| security | 80% | 88.8% | ✅ |
| iam | 70% | 93.2% | ✅ |

### 5.2 覆盖率检查命令

```bash
# 检查单个模块
go test -cover ./internal/domain/...

# 生成覆盖率报告
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# 检查覆盖率达标情况
go test -cover ./... 2>&1 | grep -E "(coverage|FAIL)"
```

### 5.3 覆盖率未达标处理

1. 分析未覆盖代码路径
2. 添加针对性测试用例
3. 确认覆盖率达到目标
4. 禁止强行凑覆盖率而编写无意义测试

---

## 6. 测试数据管理

### 6.1 固定测试数据

```go
func TestAccountService_Create(t *testing.T) {
    store := newMockAccountStore()

    req := &CreateAccountRequest{
        SupplierID:  1001,           // 固定供应商ID
        Provider:    ProviderOpenAI, // 固定提供商
        AccountType: AccountTypeAPIKey,
        Credential:  "sk-test-key", // 测试用密钥
        RiskAck:     true,
    }

    account, err := store.Create(context.Background(), req)
    // ...
}
```

### 6.2 边界值测试

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

## 7. 测试命名规范

### 7.1 函数命名

```
Test{UnitOfWork}_{Scenario}_{ExpectedResult}

示例：
- TestAccountService_Create_Success
- TestAccountService_Create_InvalidInput
- TestPackageService_Publish_ExpiredPackage
- TestSettlementService_Withdraw_ExceedsBalance
```

### 7.2 测试文件内子测试

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

## 8. 测试运行策略

### 8.1 本地开发

```bash
# 快速测试（跳过慢速测试）
go test -short ./...

# 完整测试（含集成测试）
go test -tags=integration ./...

# 只测试修改的包
go test ./internal/domain/...

# 详细输出
go test -v ./internal/domain/...
```

### 8.2 CI/CD

```yaml
# .github/workflows/test.yml
name: Test
on: [push, pull_request]

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Go
        uses: actions/setup-go@v4
        with:
          go-version: '1.21'

      - name: Run unit tests
        run: go test -short -coverprofile=coverage.out ./...

      - name: Run integration tests
        run: go test -tags=integration -coverprofile=coverage.out ./...

      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage.out
```

---

## 9. 常见问题处理

### 9.1 测试依赖外部服务

**问题**: 数据库、Redis、消息队列不可用

**解决**: 使用 Mock 替代真实依赖

```go
// ❌ 依赖真实存储
func TestSettlementService(t *testing.T) {
    repo, _ := NewPostgresRepository(db) // 需要真实DB
}

// ✅ 使用 Mock
func TestSettlementService(t *testing.T) {
    store := newMockSettlementStore() // 无外部依赖
    svc := NewSettlementService(store, nil, nil)
}
```

### 9.2 时间相关测试

**问题**: `time.Now()` 导致测试不确定

**解决**: 使用依赖注入或时间模拟

```go
// 通过参数注入时间或使用 clock 接口
type SettlementService struct {
    store  SettlementStore
    clock  Clock  // 注入时间依赖
}

func (s *SettlementService) Withdraw(ctx context.Context, supplierID int64, req *WithdrawRequest) (*Settlement, error) {
    now := s.clock.Now()
    // 使用 now 而非 time.Now()
}
```

### 9.3 并发测试

**问题**: 竞态条件难以复现

**解决**: 使用 `race` 检测器

```bash
go test -race ./...
```

---

## 10. 测试检查清单

新代码合并前：

- [ ] 所有单元测试通过
- [ ] 覆盖率达标（无下降）
- [ ] 无 `TODO` 或 `FIXME` 遗留测试
- [ ] Mock 使用正确接口签名
- [ ] 测试名称符合规范
- [ ] 表驱动测试覆盖边界情况
- [ ] 集成测试在 CI 中正常运行

---

## 11. 参考资料

- [Go Testing](https://pkg.go.dev/testing)
- [testify](https://github.com/stretchr/testify)
- [Advanced Testing in Go](https://google.github.io/aip/214)
