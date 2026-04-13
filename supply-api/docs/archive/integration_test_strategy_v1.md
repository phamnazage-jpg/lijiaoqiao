# 集成测试策略 v1.0

> **文档版本**: v1.0
> **创建日期**: 2026-04-07
> **问题**: P1-012 供应侧技术设计定义了失败注入测试，但缺少跨域集成测试策略

---

## 1. 测试金字塔

```
                    ┌─────────────┐
                    │    E2E      │ 少量，验证关键链路
                    │   Tests     │ 5-10个
                   ─┴─────────────┴─
                  ┌─────────────────┐
                  │  Integration    │ 跨域测试，验证模块间交互
                  │    Tests        │ 20-30个
                 ─┴─────────────────┴─
                ┌───────────────────────┐
                │     Unit Tests       │ 大量，快速反馈
                │                       │ 200+个
               ─┴───────────────────────┴─
```

---

## 2. 跨域集成测试范围

### 2.1 域间依赖关系

```
IAM ──────► Auth ──────► Supply
 │              │              │
 ▼              ▼              ▼
Core ◄──────── Audit ◄──── Billing
```

| 源域 | 目标域 | 依赖类型 | 测试用例数 |
|------|--------|----------|------------|
| IAM | Auth | Token验证 | 5 |
| Supply | Billing | 结算触发 | 8 |
| Supply | Audit | 审计记录 | 5 |
| Supply | Core | 账户操作 | 10 |
| IAM | Audit | 权限变更审计 | 5 |

### 2.2 核心集成测试场景

| 场景ID | 场景描述 | 涉及域 | 优先级 |
|--------|----------|--------|--------|
| IT-001 | 用户登录→Token签发→API调用 | IAM→Auth→Core | P0 |
| IT-002 | 创建供应账户→发布套餐→下单 | Supply→Core | P0 |
| IT-003 | 下单→使用API→生成账单 | Supply→Billing | P0 |
| IT-004 | 结算申请→处理→完成通知 | Supply→Billing→Audit | P1 |
| IT-005 | 权限变更→Token吊销→验证拒绝 | IAM→Auth→Audit | P1 |
| IT-006 | 批量调价→部分失败→补偿处理 | Supply→Audit | P1 |

---

## 3. 集成测试环境

### 3.1 环境配置

| 环境 | 用途 | 数据库 | 缓存 | 特点 |
|------|------|--------|------|------|
| local | 本地开发 | PostgreSQL (Docker) | Redis (Docker) | 快速迭代 |
| integration | CI/CD集成 | PostgreSQL + Redis | 独立实例 | 隔离 |
| staging | 预发布 | 生产镜像 | 生产镜像 | 接近生产 |

### 3.2 测试数据库初始化

```sql
-- integration_test_setup.sql
-- 测试前初始化测试数据

BEGIN;

-- 清理现有测试数据
TRUNCATE TABLE iam_users CASCADE;
TRUNCATE TABLE supply_accounts CASCADE;
TRUNCATE TABLE supply_packages CASCADE;
TRUNCATE TABLE supply_orders CASCADE;
TRUNCATE TABLE billing_ledger_entries CASCADE;

-- 插入测试用户
INSERT INTO iam_users (id, username, email, role) VALUES
    (1, 'test_admin', 'admin@test.com', 'admin'),
    (2, 'test_operator', 'operator@test.com', 'operator'),
    (3, 'test_viewer', 'viewer@test.com', 'viewer');

-- 插入测试账户
INSERT INTO supply_accounts (id, user_id, platform, status, risk_level) VALUES
    (1, 1, 'openai', 'active', 'low'),
    (2, 2, 'anthropic', 'active', 'normal');

-- 插入测试套餐
INSERT INTO supply_packages (id, supply_account_id, user_id, platform, model,
    total_quota, available_quota, status) VALUES
    (1, 1, 1, 'openai', 'gpt-4', 1000000, 800000, 'active'),
    (2, 2, 2, 'anthropic', 'claude-3', 500000, 500000, 'active');

COMMIT;
```

---

## 4. 集成测试用例

### 4.1 IAM → Auth 集成测试

```go
// TestIT001_TokenLifecycle IAM到Auth集成测试
func TestIT001_TokenLifecycle(t *testing.T) {
    // 1. 创建用户
    user := &iam.User{Username: "test_user", Email: "test@example.com"}
    err := iamService.CreateUser(ctx, user)
    require.NoError(t, err)

    // 2. 签发Token
    token, err := authService.IssueToken(ctx, user.ID, []string{"supply:accounts:read"})
    require.NoError(t, err)
    assert.NotEmpty(t, token.AccessToken)

    // 3. 验证Token
    claims, err := authService.ValidateToken(ctx, token.AccessToken)
    require.NoError(t, err)
    assert.Equal(t, user.ID, claims.UserID)

    // 4. 吊销Token
    err = authService.RevokeToken(ctx, token.AccessToken)
    require.NoError(t, err)

    // 5. 验证Token已失效
    _, err = authService.ValidateToken(ctx, token.AccessToken)
    assert.Error(t, err)
}
```

### 4.2 Supply → Billing 集成测试

```go
// TestIT002_SettlementFlow Supply到Billing集成测试
func TestIT002_SettlementFlow(t *testing.T) {
    // 1. 创建供应商账户
    account := &supply.Account{
        UserID:   testUserID,
        Platform: "openai",
        Status:   "active",
    }
    err := supplyRepo.Create(ctx, account)
    require.NoError(t, err)

    // 2. 创建套餐
    pkg := &supply.Package{
        AccountID:    account.ID,
        TotalQuota:   1000000,
        AvailableQuota: 1000000,
        Status:       "active",
    }
    err = supplyRepo.CreatePackage(ctx, pkg)
    require.NoError(t, err)

    // 3. 生成使用记录
    usage := &supply.UsageRecord{
        AccountID:  account.ID,
        Tokens:     5000,
        Cost:       0.15,
        Status:     "completed",
    }
    err = supplyRepo.CreateUsageRecord(ctx, usage)
    require.NoError(t, err)

    // 4. 验证收益记录生成
    earnings, err := billingRepo.GetEarningsByAccount(ctx, account.ID)
    require.NoError(t, err)
    assert.NotEmpty(t, earnings)
    assert.Equal(t, usage.Cost, earnings[0].Amount)

    // 5. 创建结算单
    settlement, err := billingService.CreateSettlement(ctx, account.ID)
    require.NoError(t, err)
    assert.Equal(t, "pending", settlement.Status)

    // 6. 处理结算
    err = billingService.ProcessSettlement(ctx, settlement.ID)
    require.NoError(t, err)

    // 7. 验证结算完成
    settlement, err = billingRepo.GetSettlement(ctx, settlement.ID)
    require.NoError(t, err)
    assert.Equal(t, "completed", settlement.Status)
}
```

### 4.3 跨域失败注入测试

```go
// TestIT003_FailureInjection 跨域失败注入测试
func TestIT003_FailureInjection(t *testing.T) {
    tests := []struct {
        name          string
        injectFailure func()
        verify        func(*testing.T)
    }{
        {
            name: "数据库连接失败时收益计算",
            injectFailure: func() {
                // 模拟数据库连接失败
                mockDB.SetFailure(true)
                defer mockDB.SetFailure(false)
            },
            verify: func(t *testing.T) {
                // 验证使用记录仍然可以创建
                _, err := supplyService.RecordUsage(ctx, usage)
                assert.Error(t, err)
                // 验证重试机制
            },
        },
        {
            name: "Redis不可用时Token验证",
            injectFailure: func() {
                mockRedis.SetAvailable(false)
                defer mockRedis.SetAvailable(true)
            },
            verify: func(t *testing.T) {
                // 验证降级到数据库验证
                claims, err := authService.ValidateToken(ctx, token)
                assert.NoError(t, err)
            },
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            tt.injectFailure()
            tt.verify(t)
        })
    }
}
```

---

## 5. 集成测试夹具

### 5.1 测试夹具定义

```go
// IntegrationTestFixture 集成测试夹具
type IntegrationTestFixture struct {
    DB       *pgxpool.Pool
    Redis    *redis.Client
    IAM      *iam.Service
    Auth     *auth.Service
    Supply   *supply.Service
    Billing  *billing.Service
    Audit    *audit.Service
    Teardown func()
}

// NewIntegrationTestFixture 创建集成测试夹具
func NewIntegrationTestFixture(t *testing.T) *IntegrationTestFixture {
    // 启动测试数据库
    db := startTestDatabase(t)
    redis := startTestRedis(t)

    fixture := &IntegrationTestFixture{
        DB:    db,
        Redis: redis,
        IAM:   iam.NewService(db),
        Auth:  auth.NewService(db, redis),
        // ... 其他服务初始化
    }

    fixture.Teardown = func() {
        db.Close()
        redis.Close()
    }

    return fixture
}
```

### 5.2 测试数据生成

```go
// TestDataGenerator 测试数据生成器
type TestDataGenerator struct {
    fixture *IntegrationTestFixture
}

func (g *TestDataGenerator) CreateTestUser(t *testing.T) *iam.User {
    user := &iam.User{
        Username: random.String(10),
        Email:    random.Email(),
        Role:     "operator",
    }
    err := g.fixture.IAM.CreateUser(context.Background(), user)
    require.NoError(t, err)
    return user
}

func (g *TestDataGenerator) CreateTestAccount(t *testing.T, userID int64) *supply.Account {
    account := &supply.Account{
        UserID:   userID,
        Platform: "openai",
        Status:   "active",
    }
    err := g.fixture.Supply.CreateAccount(context.Background(), account)
    require.NoError(t, err)
    return account
}
```

---

## 6. CI/CD 集成

### 6.1 GitHub Actions 配置

```yaml
# .github/workflows/integration-test.yml
name: Integration Tests

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  integration-test:
    runs-on: ubuntu-latest

    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_DB: supply_test
          POSTGRES_USER: test_user
          POSTGRES_PASSWORD: test_pass
        ports:
          - 5432:5432
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

      redis:
        image: redis:7
        ports:
          - 6379:6379

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'

      - name: Run Integration Tests
        run: |
          go test -tags=integration -v ./tests/integration/...
        env:
          DATABASE_URL: postgres://test_user:test_pass@localhost:5432/supply_test
          REDIS_URL: redis://localhost:6379

      - name: Upload Coverage
        uses: codecov/codecov-action@v3
        with:
          files: ./coverage/integration.out
```

---

## 7. 测试覆盖率目标

| 测试类型 | 覆盖率目标 | 说明 |
|----------|------------|------|
| 单元测试 | > 80% | 每个模块独立测试 |
| 集成测试 | > 60% | 域间交互测试 |
| E2E测试 | > 40% | 关键用户旅程 |

---

## 8. 执行计划

### 8.1 迭代计划

| 迭代 | 内容 | 用时 |
|------|------|------|
| Sprint 1 | 搭建测试框架、配置CI | 1周 |
| Sprint 2 | IAM/Auth集成测试 | 1周 |
| Sprint 3 | Supply/Billing集成测试 | 1周 |
| Sprint 4 | 失败注入测试、覆盖率优化 | 1周 |

### 8.2 测试执行频率

| 测试类型 | 执行频率 | 执行时间 |
|----------|----------|----------|
| 单元测试 | 每次PR | < 2分钟 |
| 集成测试 | 每次PR合并 | < 10分钟 |
| E2E测试 | 每日构建 | < 30分钟 |
| 回归测试 | 每次发布 | < 1小时 |

---

> **维护记录**:
> - v1.0 (2026-04-07): 初始版本
