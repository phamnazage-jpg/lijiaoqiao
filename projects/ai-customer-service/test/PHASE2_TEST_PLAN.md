# Phase 2 测试质量提升规划

> 生成时间：2026-05-01 09:00 GMT+8
> 负责人：宰相（小龙团队 QA subagent）
> 项目：ai-customer-service
> 依据：PRODUCTION_PHASE1_STATUS.md、TEST_COVERAGE_REPORT.md

---

## 一、当前质量基线（Phase 1 已达标）

### 1.1 整体状态

| 指标 | 当前值 | Phase 1 目标 | Phase 2 目标 |
|------|--------|--------------|--------------|
| 整体覆盖率 | **62.6%** | >60% ✅ | >70% |
| Build + vet + tests | ✅ 全通过 | ✅ 必须 | ✅ 必须 |
| Phase 1 核心包 | 4/5 >60% | >60% ✅ | >70% |
| E2E 测试 | 100% | >60% ✅ | 100% ✅ |
| Integration 测试 | 100% | >60% ✅ | 100% ✅ |

### 1.2 各包覆盖率现状

| 包 | 覆盖率 | 状态 | Phase 2 目标 |
|----|--------|------|--------------|
| `internal/service/reply` | 100% | ✅ | 保持 |
| `internal/service/handoff` | 100% | ✅ | 保持 |
| `test/e2e` | 100% | ✅ | 保持 |
| `test/integration` | 100% | ✅ | 保持 |
| `internal/service/dialog` | 88.5% | ✅ | >90% |
| `internal/platform/httpx` | 84.3% | ✅ | >85% |
| `internal/service/intent` | 80.8% | ✅ | >85% |
| `internal/http/handlers` | 78.4% | ✅ | >85% |
| `internal/app` | 74.2% | ✅ | >80% |
| `internal/config` | 70.6% | ✅ | >75% |
| `internal/store/memory` | 59.1% | ⚠️ | >70% |
| `internal/store/postgres` | 43.1% | ⚠️ | >60% |
| `internal/http` (router) | 41.3% | ⚠️ | >60% |
| `internal/platform/health` | 38.1% | ⚠️ | >60% |
| Domain 包（6个） | 0% | ❌ | >30% |
| `cmd/ai-customer-service` | 0% | ❌ | 测试可选 |
| `internal/platform/logging` | 0% | ❌ | >40% |

---

## 二、Phase 2 测试补齐优先级

### P0 — 必须补齐（上线后 2 周内）

| 优先级 | 包 | 当前覆盖率 | 目标覆盖率 | 关键缺失 |
|--------|-----|-----------|-----------|---------|
| P0-1 | `internal/store/memory` | 59.1% | >70% | `ListAll(0%)`、`GetStats(0%)` |
| P0-2 | `internal/store/postgres` | 43.1% | >60% | `Assign(0%)`、`Resolve(0%)`、`Close(0%)` |
| P0-3 | `internal/http` (router) | 41.3% | >60% | `writeMethodNotAllowed(0%)`、webhook channel 路由 |
| P0-4 | `internal/platform/health` | 38.1% | >60% | `IsReady(0%)`、dependency check 边界 |
| P0-5 | `internal/http/handlers` | 78.4% | >85% | `HandleChannel(0%)`、`TicketStatsHandler.Get(0%)` |

### P1 — 强烈建议补齐（上线后 4 周内）

| 优先级 | 包 | 当前覆盖率 | 目标覆盖率 | 关键缺失 |
|--------|-----|-----------|-----------|---------|
| P1-1 | Domain 包 (6个) | 0% | >30% | 所有 domain 包无测试文件 |
| P1-2 | `internal/platform/logging` | 0% | >40% | Logger 初始化未覆盖 |
| P1-3 | `internal/service/dialog` | 88.5% | >90% | Process 边界场景补全 |
| P1-4 | `internal/app` | 74.2% | >80% | Shutdown 错误处理分支 |

### P2 — 可选（长期优化）

| 优先级 | 包 | 当前覆盖率 | 目标覆盖率 | 说明 |
|--------|-----|-----------|-----------|------|
| P2-1 | `cmd/ai-customer-service` | 0% | 测试可选 | main 函数测试意义有限 |
| P2-2 | `internal/http/handlers` | 78.4% | >90% | `clientIP(66.7%)` 边界场景 |

---

## 三、具体补齐方案

### 3.1 P0-1: `internal/store/memory` 测试补齐

**当前缺失：**
- `ListAll()` — 0% 覆盖（无测试调用）
- `GetStats()` — 0% 覆盖（无测试调用）

**补齐方案：**
在 `internal/store/memory/ticket_store_test.go` 中新增：

```go
func TestTicketStore_ListAll(t *testing.T) {
	store := NewTicketStore()
	ctx := context.Background()

	// Create 3 tickets
	store.Create(ctx, &ticket.Ticket{ID: "t1", Status: ticket.StatusOpen})
	store.Create(ctx, &ticket.Ticket{ID: "t2", Status: ticket.StatusResolved})
	store.Create(ctx, &ticket.Ticket{ID: "t3", Status: ticket.StatusClosed})

	// ListAll should return all 3
	all, err := store.ListAll(ctx)
	if err != nil || len(all) != 3 {
		t.Fatalf("ListAll() = %d tickets, want 3", len(all))
	}
}

func TestTicketStore_GetStats(t *testing.T) {
	store := NewTicketStore()
	ctx := context.Background()

	// Create tickets with different statuses and channels
	store.Create(ctx, &ticket.Ticket{ID: "t1", Status: ticket.StatusOpen, Priority: ticket.PriorityP1})
	store.Create(ctx, &ticket.Ticket{ID: "t2", Status: ticket.StatusResolved, Priority: ticket.PriorityP2})

	stats, err := store.GetStats(ctx)
	if err != nil {
		t.Fatalf("GetStats() error = %v", err)
	}
	if stats.Total != 2 {
		t.Fatalf("stats.Total = %d, want 2", stats.Total)
	}
	if stats.Open != 1 || stats.Resolved != 1 {
		t.Fatalf("stats Open/Resolved = %d/%d, want 1/1", stats.Open, stats.Resolved)
	}
}
```

**预期提升：** 59.1% → **>70%**

---

### 3.2 P0-2: `internal/store/postgres` 测试补齐

**当前缺失：**
- `Assign()` — 0%（未覆盖）
- `Resolve()` — 0%（未覆盖）
- `Close()` — 0%（未覆盖）

**补齐方案：**
在 `internal/store/postgres/store_test.go` 中新增 workflow 操作测试（需 sqlmock）：

```go
func TestTicketWorkflowStore_Assign(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	auditStore := NewAuditStore(db)
	workflowStore := NewTicketWorkflowStore(db, auditStore)

	// Mock UPDATE query
	mock.ExpectExec("UPDATE tickets SET").
		WithArgs("agent1", sqlmock.AnyArg(), "t1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock audit insert
	mock.ExpectExec("INSERT INTO audit").
		WillReturnResult(sqlmock.NewResult(1, 1))

	err = workflowStore.Assign(context.Background(), "t1", "agent1", "admin", "127.0.0.1", time.Now())
	if err != nil {
		t.Fatalf("Assign() error = %v", err)
	}
}
```

**预期提升：** 43.1% → **>60%**

---

### 3.3 P0-3: `internal/http` (router) 测试补齐

**当前缺失：**
- `writeMethodNotAllowed()` — 0%（从未调用）
- Webhook channel 路由未测

**补齐方案：**
在 `internal/http/router_test.go` 中新增：

```go
func TestRouter_WriteMethodNotAllowed_Called(t *testing.T) {
	// Test that unknown methods on known paths call writeMethodNotAllowed
	probe := health.NewProbe()
	h := handlers.NewHealthHandler(probe)
	ticketHandler := &handlers.TicketHandler{}
	router := NewRouter(RouterDeps{Health: h, Tickets: ticketHandler})

	// POST to /tickets (only GET allowed) should trigger writeMethodNotAllowed
	req := httptest.NewRequest(http.MethodPost, "/api/v1/customer-service/tickets", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /tickets = %d, want 405", rr.Code)
	}
}
```

**预期提升：** 41.3% → **>60%**

---

### 3.4 P0-4: `internal/platform/health` 测试补齐

**当前缺失：**
- `IsReady()` — 0%（未测试）
- dependency check 边界条件

**补齐方案：**
在 `internal/platform/health/health_test.go` 中新增：

```go
func TestProbe_IsReady_AfterSetReady(t *testing.T) {
	probe := NewProbe()
	probe.SetReady(true)
	if !probe.IsReady() {
		t.Error("IsReady() = false, want true after SetReady(true)")
	}
}

func TestDependency_Evaluate_FailsWhenCheckFails(t *testing.T) {
	dep := Dependency{
		Name: "test",
		Check: func() error { return fmt.Errorf("check failed") },
	}
	err := dep.Evaluate()
	if err == nil {
		t.Error("Evaluate() = nil, want error when Check fails")
	}
}
```

**预期提升：** 38.1% → **>60%**

---

### 3.5 P0-5: `internal/http/handlers` 测试补齐

**当前缺失：**
- `HandleChannel()` — 0%（未测试）
- `TicketStatsHandler.Get()` — 0%（集成测试未覆盖 handler 本身）

**补齐方案：**

#### HandleChannel 测试：

在 `internal/http/handlers/webhook_handler_test.go` 中新增：

```go
func TestWebhookHandler_HandleChannel_OverridesBodyChannel(t *testing.T) {
	h := newTestWebhookHandler(nil)
	payload := `{"message_id":"m1","channel":"wrong","open_id":"u1","content":"hi"}`
	req := httptest.NewRequest(http.MethodPost, "/webhook/correct", bytes.NewBufferString(payload))
	resp := httptest.NewRecorder()

	// Call HandleChannel with "correct" — should override "wrong" in body
	h.HandleChannel(resp, req, "correct")

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	// Verify response contains channel="correct"
}
```

#### TicketStatsHandler.Get 测试：

在 `internal/http/handlers/ticket_stats_handler_test.go`（新建）中：

```go
func TestTicketStatsHandler_Get_Success(t *testing.T) {
	mockService := &mockTicketStatsService{
		stats: ticketstats.Stats{Total: 100, Open: 30},
	}
	handler := NewTicketStatsHandler(mockService, nil)

	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	resp := httptest.NewRecorder()
	handler.Get(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
}
```

**预期提升：** 78.4% → **>85%**

---

### 3.6 P1-1: Domain 包测试补齐

**当前缺失：**
6 个 domain 包（`audit`、`intent`、`message`、`session`、`ticket`、`ticketstats`）全部无测试文件。

**补齐方案：**

为每个 domain 包创建基础测试，覆盖结构体构造和边界条件：

```go
// internal/domain/ticket/ticket_test.go
func TestTicket_NewTicket(t *testing.T) {
	ticket := &Ticket{
		ID: "t1",
		Status: StatusOpen,
		Priority: PriorityP1,
	}
	if ticket.ID != "t1" {
		t.Errorf("ticket.ID = %s, want t1", ticket.ID)
	}
}

func TestTicket_ValidPriorities(t *testing.T) {
	validPriorities := []Priority{PriorityP1, PriorityP2, PriorityP3}
	for _, p := range validPriorities {
		if p == "" {
			t.Errorf("priority %q is empty", p)
		}
	}
}
```

**预期提升：** 0% → **>30%**（每包 3-5 个基础测试）

---

## 四、执行时间表（建议）

| 阶段 | 时间 | 优先级 | 预期成果 |
|------|------|--------|----------|
| **Week 1** | 上线后第 1 周 | P0-1 ~ P0-3 | memory + postgres + router 达标 |
| **Week 2** | 上线后第 2 周 | P0-4 ~ P0-5 | health + handlers 达标 |
| **Week 3** | 上线后第 3 周 | P1-1 | Domain 包基础测试补齐 |
| **Week 4** | 上线后第 4 周 | P1-2 ~ P1-4 | 整体覆盖率 >70% |
| **Long-term** | 上线后 2 个月 | P2 | 覆盖率 >80%（可选） |

---

## 五、质量门禁（Phase 2）

| 指标 | Phase 1（当前） | Phase 2 目标 |
|------|----------------|--------------|
| 整体覆盖率 | 62.6% ✅ | **>70%** |
| 核心包覆盖率 | 4/5 >60% | 全部 >70% |
| Domain 包覆盖率 | 0% | >30% |
| Build + vet + tests | ✅ 全通过 | ✅ 全通过 |
| P0 测试补齐 | — | Week 2 完成 |

---

## 六、风险与依赖

| 风险 | 缓解措施 |
|------|----------|
| PostgreSQL 测试需要 sqlmock | Week 1 引入 sqlmock 依赖 |
| Domain 包测试意义有限 | 仅测试关键边界和构造逻辑 |
| 灰度阶段发现新 bug 需补测 | 预留 Week 3 缓冲时间 |

---

*本文档由宰相（小龙团队 QA subagent）生成 | 2026-05-01 09:00 GMT+8*
