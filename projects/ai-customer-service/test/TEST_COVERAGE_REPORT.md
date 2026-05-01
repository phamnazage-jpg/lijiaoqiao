# 测试覆盖率报告

> 生成时间：2026-04-30 21:52 GMT+8
> 工具：`go test -cover`
> 项目：ai-customer-service

---

## 1. 各包当前覆盖率

| 包 | 覆盖率 | 达标 | 备注 |
|----|--------|------|------|
| `internal/service/intent` | **80.8%** | ✅ | Phase 1 核心 |
| `internal/service/handoff` | **75.0%** | ✅ | Phase 1 核心 |
| `internal/config` | **70.6%** | ✅ | Phase 1 核心 |
| `internal/http/handlers` | **65.7%** | ✅ | Phase 1 核心 |
| `test/integration` | 53.1% | ⚠️ | 接近目标 |
| `test/e2e` | 32.7% | ⚠️ | 需提升 |
| `internal/service/dialog` | 49.2% | ⚠️ | 接近目标 |
| `internal/app` | 17.4% | ❌ | 待补齐 |
| `internal/store/memory` | 0.0% | ❌ | 无测试文件 |
| `internal/store/postgres` | 1.6% | ❌ | Phase 2 范围 |
| `internal/http` | 0.0% | ❌ | 路由器未覆盖 |
| `internal/platform/httpx` | 0.0% | ❌ | 中间件未覆盖 |
| `internal/platform/health` | 0.0% | ❌ | 健康检查未覆盖 |
| `internal/platform/logging` | 0.0% | ❌ | 日志未覆盖 |
| `internal/domain/error/cserrors` | 0.0% | ❌ | 错误码未覆盖 |
| Domain 包（6 个） | 0.0% | ❌ | 无测试文件 |
| `cmd/ai-customer-service` | 0.0% | ❌ | main 未覆盖 |

**整体覆盖率：47.0%**

---

## 2. 覆盖率目标

### Phase 1 上线目标（>60%）

必须达标的包：

| 包 | 当前覆盖率 | 目标 | 差距 |
|----|-----------|------|------|
| `internal/http/handlers` | 65.7% | >60% | ✅ 已达标 |
| `internal/config` | 70.6% | >60% | ✅ 已达标 |
| `internal/service/handoff` | 75.0% | >60% | ✅ 已达标 |
| `internal/service/intent` | 80.8% | >60% | ✅ 已达标 |
| `internal/service/dialog` | 49.2% | >60% | ⚠️ 差 10.8% |
| `internal/app` | 17.4% | >60% | ❌ 差 42.6% |
| `test/integration` | 53.1% | >60% | ⚠️ 差 6.9% |
| `test/e2e` | 32.7% | >60% | ❌ 差 27.3% |

### Phase 2 目标（>40%）

| 包 | 当前覆盖率 | 目标 |
|----|-----------|------|
| `internal/store/postgres` | 1.6% | >40% |
| `internal/store/memory` | 0.0% | >40% |
| `internal/platform/httpx` | 0.0% | >40% |
| `internal/http` | 0.0% | >40% |
| Domain 包（6 个） | 0.0% | >30% |

---

## 3. 缺失测试的包列表

### P0 — 必须补齐（上线阻断）

| 包 | 当前覆盖率 | 关键缺失函数 |
|----|-----------|-------------|
| `internal/app` | 17.4% | `app.New`（60%）未充分测试，`Shutdown` 未覆盖 |
| `test/e2e` | 32.7% | 编译失败（app.go undefined: ticket/ticketListerStore） |
| `internal/service/dialog` | 49.2% | `Process`（78.4%）未达 100%，边界场景缺失 |

### P1 — 上线后补齐

| 包 | 当前覆盖率 | 说明 |
|----|-----------|------|
| `internal/store/postgres` | 1.6% | Phase 2 范围，postgres 驱动未 mock |
| `internal/store/memory` | 0.0% | 全部 store 方法未覆盖 |
| `internal/platform/httpx` | 0.0% | `NewRateLimiter`（60%），滑动窗口逻辑未验证 |
| `internal/platform/health` | 0.0% | 健康检查探针未覆盖 |
| `internal/http` | 0.0% | `NewRouter`（27.8%），中间件注册路径缺失 |
| `internal/platform/logging` | 0.0% | Logger 初始化未覆盖 |
| `internal/domain/error/cserrors` | 0.0% | `ErrorMsg`（31.4%），错误码路径未覆盖 |
| Domain 包（6 个） | 0.0% | `audit/ticketstats/ticket/intent/message/session` 全部无测试文件 |

---

## 4. 测试策略说明

### 4.1 当前测试分层

```
e2e 层：test/e2e/         ← 全链路集成（依赖 app.New 编译修复）
integration 层：test/integration/  ← AC-02 矩阵 + 端到端场景
handler 层：internal/http/handlers/ ← HTTP 接口单元测试
service 层：internal/service/       ← 业务逻辑单元测试
config 层：internal/config/         ← 配置解析测试
store 层：internal/store/           ← 数据访问测试（memory/postgres）
```

### 4.2 Phase 1 补齐策略

**优先补齐（P0）**：
1. `internal/service/dialog/service_test.go` — 补 `Process` 未覆盖分支，提升至 >60%
2. `test/e2e/` — 等待 TechLead 修复 app.go 编译问题后，补充覆盖率
3. `internal/app/app_test.go` — 覆盖 `New` 和 `Shutdown` 方法

**补齐方式**：
- 使用 table-driven test 覆盖分支路径
- `dialog.Process` 补充边界 case（intent=nil、session=nil、LLM 超时）
- `app.New` mock 所有依赖后验证初始化逻辑

### 4.3 Phase 2 补齐策略

**分阶段**：
1. **第一阶段**：覆盖率 >30% — 覆盖核心 public 方法
2. **第二阶段**：覆盖率 >40% — 覆盖错误路径和边界条件

**重点包**：
- `internal/store/postgres` — 使用 sqlmock 隔离数据库依赖
- `internal/platform/httpx` — 单元测试滑动窗口算法
- `internal/http/router.go` — 路由注册 + 404/405 路径测试

---

## 5. 函数级覆盖率详情

### 关键函数覆盖率

| 函数 | 包 | 覆盖率 | 状态 |
|------|-----|--------|------|
| `Process` | `internal/service/dialog/service.go:60` | 78.4% | ⚠️ 接近目标 |
| `New` | `internal/app/app.go:39` | 60.0% | ✅ 达标 |
| `List` | `internal/http/handlers/ticket_handler.go:32` | 0.0% | ❌ 未覆盖 |
| `Get` | `internal/http/handlers/ticket_stats_handler.go:29` | 0.0% | ❌ 未覆盖 |
| `NewTicketStatsHandler` | `internal/http/handlers/ticket_stats_handler.go:24` | 0.0% | ❌ 未覆盖 |
| `WithRateLimit` | `internal/platform/httpx/limits.go:90` | 100.0% | ✅ 已覆盖 |
| `Allow` | `internal/platform/httpx/limits.go:50` | 100.0% | ✅ 已覆盖 |
| `NewRateLimiter` | `internal/platform/httpx/limits.go:34` | 60.0% | ⚠️ 待提升 |

---

## 6. 下一步行动

| 优先级 | 行动项 | 负责人 | 目标覆盖率 |
|--------|--------|--------|-----------|
| P0 | 修复 `app.go` 编译错误 | TechLead | e2e 可运行 |
| P0 | 补齐 `dialog/service_test.go` | QA | >60% |
| P0 | 补齐 `app/app_test.go` | QA | >40% |
| P1 | 补齐 `store/memory/*_test.go` | QA | >40% |
| P1 | 补齐 `platform/httpx/limits_test.go` | QA | >60% |
| P2 | 补齐 `store/postgres/*_test.go` | QA | >40% |

---

*报告生成：宰相 | 2026-04-30 21:52 GMT+8*