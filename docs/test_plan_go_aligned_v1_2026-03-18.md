# Go 主测试链路对齐方案

- 版本：v1.0
- 日期：2026-03-18
- 目标：将测试体系与主技术栈（Go + PostgreSQL）对齐，替代 Python 工程骨架为主的测试设计。

---

## 1. 适用范围

1. 适用于网关主链路：路由、鉴权、计费、适配器、风控、审计。
2. 覆盖阶段：S0-S2（优先保障 S1/S2 的上线与替换门禁）。
3. 本文档作为测试实施主方案，历史 Python 示例仅保留参考。

---

## 2. 测试金字塔（Go版）

| 层级 | 占比 | 工具 | 目标 |
|---|---:|---|---|
| 单元测试 | 70% | `go test` + `testing` + `testify` | 逻辑正确性、异常分支、边界条件 |
| 集成测试 | 20% | `go test` + `testcontainers-go` + `httptest` | DB/Redis/网关链路联通与一致性 |
| E2E/门禁测试 | 10% | `playwright` + `k6` + 契约回归脚本 | 用户旅程、性能门禁、兼容门禁 |

覆盖率目标：
1. 核心包总覆盖率 >= 80%。
2. Router/Billing/Adapter 覆盖率 >= 85%。
3. 关键门禁用例（S2 Gate）通过率 = 100%。

---

## 3. 测试目录与命名规范

```text
立交桥/
  gateway/
    internal/
    pkg/
    tests/
      unit/
        router/
        billing/
        auth/
      integration/
        api/
        db/
        adapter/
      contract/
        compat/
      e2e/
        user_journey/
      performance/
        k6/
```

命名规则：
1. 单元测试文件：`*_test.go`。
2. 集成测试标签：`//go:build integration`。
3. 门禁测试标签：`//go:build gate`。

---

## 4. 工具链基线（Go）

| 能力 | 工具 | 说明 |
|---|---|---|
| 单元测试 | `testing`, `testify/require` | 断言与失败信息可读性 |
| Mock | `gomock` 或 `testify/mock` | 仅在外部依赖边界处使用 |
| HTTP测试 | `httptest` | Handler/中间件测试 |
| DB集成 | `testcontainers-go` + PostgreSQL 15 | 与生产数据库方言一致 |
| Redis集成 | `testcontainers-go` + Redis 7 | 限流/并发门控验证 |
| 覆盖率 | `go test -coverprofile` | CI 门禁 |
| 性能 | `k6` | P95/P99 与错误率门禁 |
| 前端E2E | `playwright` | 注册、Key、调用、账单旅程 |

---

## 5. 关键测试套件（必须落地）

### 5.1 Router Core 套件

1. 主路径端点归一：`/responses` -> `/v1/responses`。
2. 路由决策正确性：模型映射、租户策略、fallback。
3. 接管率标记：`router_engine` 写入一致性。

### 5.2 Billing 套件

1. 幂等扣费：重复 `request_id` 不重复扣费。
2. 冲突检测：`billing_conflict_rate_pct` 监测与告警。
3. 对账一致性：usage 与 billing 差异 <= 0.1%。

### 5.3 兼容契约套件

1. Schema Gate：请求/响应字段与类型。
2. Behavior Gate：stream/no-replay/错误码语义。
3. Performance Gate：P95/P99/5xx/账务指标。

### 5.4 安全套件

1. query key 外拒内转边界。
2. subapi 内网隔离与 mTLS。
3. RLS/租户越权访问防护。

---

## 6. Go 测试示例（最小可执行）

```go
package billing_test

import (
    "testing"
    "github.com/stretchr/testify/require"
)

func TestCharge_IdempotentByRequestID(t *testing.T) {
    svc := newTestBillingService(t)

    reqID := "req-123"
    err1 := svc.Charge("u1", reqID, 100)
    err2 := svc.Charge("u1", reqID, 100)

    require.NoError(t, err1)
    require.NoError(t, err2)

    cnt, err := svc.CountTransactionsByRequestID(reqID)
    require.NoError(t, err)
    require.Equal(t, 1, cnt)
}
```

---

## 7. CI 门禁流水线（Go版）

```yaml
name: go-test-pipeline
on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

jobs:
  unit:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.21.x'
      - run: go test ./... -coverprofile=coverage.out
      - run: go tool cover -func=coverage.out

  integration:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.21.x'
      - run: go test -tags=integration ./tests/integration/...

  gate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.21.x'
      - run: go test -tags=gate ./tests/contract/...
      - run: ./scripts/gate/perf_gate_check.sh
```

---

## 8. 历史 Python 测试迁移映射

| 历史做法 | 对齐后做法 |
|---|---|
| `pytest` 单测 | `go test` + `testify` |
| Python `AsyncClient` API 测试 | Go `httptest` + 集成容器 |
| sqlite 内存库 | PostgreSQL 容器（与生产一致） |
| Python 契约脚本 | Go 契约测试 + CI gate 标签 |

---

## 9. 实施排期（首周）

1. D1-D2：迁移 Router/Billing 单测到 Go 主链路。
2. D3：补齐 integration（PostgreSQL/Redis）。
3. D4：接入 gate 标签与性能门禁脚本。
4. D5：提交覆盖率与门禁报告。

