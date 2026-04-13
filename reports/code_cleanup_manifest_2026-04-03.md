# 审计系统代码清理清单

> 日期：2026-04-03
> 目标：基于审核报告 P0/P1 问题，制定代码修复计划
> 依据：`audit_system_production_readiness_review_2026-04-03.md`

---

## 一、需要修改的代码文件

### 1.1 P0 阻塞问题（必须修复后才能上线）

| 编号 | 文件 | 问题 | 修复操作 |
|------|------|------|----------|
| P0-01 | `internal/audit/repository/audit_repository.go` | SQL列名不一致（`before_data/after_data` vs `before_state/after_state`） | 修改INSERT和SELECT SQL，将 `before_data` 改为 `before_state`，`after_data` 改为 `after_state` |
| P0-02 | `internal/audit/service/` | 批量写入未实现，10000 TPS目标无法达到 | ✅ 已实现 `BatchBuffer`（50条/批，5ms刷新），测试通过 |
| P0-03 | `internal/audit/repository/` | 分区表未实现 | 实现按月分区的 `audit_events` 表（设计文档已更新为暂不分区的简化方案） |
| P0-04 | `reports/gates/` | CI/CD Gate脚本缺陷（bc依赖、缺少重试和超时） | 已在设计文档9.1节更新为 `awk` 方案，需同步更新脚本 |

**P0-01 详细修复（audit_repository.go）**：
```sql
-- 第96行 INSERT SQL
-- 修改前：
before_data, after_data,
-- 修改后：
before_state, after_state,

-- 第226行 SELECT SQL
-- 修改前：
before_data, after_data,
-- 修改后：
before_state, after_state,
```

---

### 1.2 P1 高优先级问题

| 编号 | 文件 | 问题 | 修复操作 |
|------|------|------|----------|
| P1-01 | `internal/audit/service/metrics_service.go` | M-014过滤条件 | ✅ 已修复，使用 `event_category + event_sub_category` |
| P1-02 | `internal/audit/service/metrics_service.go` | M-015判断逻辑 | ✅ 已修复，使用 `target_direct = TRUE` |
| P1-03 | `internal/audit/service/metrics_service.go` | M-016分子分母定义 | ✅ 已确认按设计文档实现 |
| P1-04 | `internal/audit/service/metrics_service.go` | M-013~M-016内存过滤 | ⚠️ 待优化（当前在内存中过滤，SQL WHERE过滤需要更大改动） |
| P1-05 | `internal/httpapi/supply_api.go` | 分页total不准确 | ✅ 已修复，使用 `QueryWithTotal` 返回的真实total |
| P1-06 | `internal/httpapi/supply_api.go` | JSONB表达式索引 | ⚠️ 待实现（需配合数据库schema更新） |

---

## 二、需要实现的新代码

### 2.1 BatchBuffer 批量写入（Phase 2）

**文件**：`internal/audit/service/batch_buffer.go`（新建）

**功能**：
- 缓冲50条记录或5ms刷新间隔（以先到者为准）
- 支持半同步和全异步模式
- 线程安全，使用channel

**接口设计**：
```go
type BatchBuffer struct {
    events    chan *AuditEvent
    flushTick *time.Ticker
    batchSize int
    timeout   time.Duration
}

func (b *BatchBuffer) Start(ctx context.Context) error
func (b *BatchBuffer) Add(event *AuditEvent) error
func (b *BatchBuffer) Flush() error
func (b *BatchBuffer) Close() error
```

---

### 2.2 异步写入队列（Phase 3）

**文件**：`internal/audit/service/async_writer.go`（新建）

**功能**：
- 使用Go Channel作为内存队列
- 后台goroutine消费并批量写入
- 无Kafka依赖

**接口设计**：
```go
type AsyncWriter struct {
    eventCh  chan *AuditEvent
    buffer   *BatchBuffer
    workerCh chan struct{}
}

func (w *AsyncWriter) Start(ctx context.Context)
func (w *AsyncWriter) Emit(event *AuditEvent)
func (w *AsyncWriter) Stop()
```

---

## 三、需要清理/移除的代码

### 3.1 废弃的内存存储（替换为DB-backed）

| 文件 | 操作 | 原因 |
|------|------|------|
| `internal/audit/service/audit_service_memory.go` | 保留但标记废弃 | 生产环境使用DatabaseAuditService |
| `internal/audit/memory_audit_store.go` | 保留但标记废弃 | 仅用于开发模式fallback |

---

## 四、接口统一计划

### 4.1 审计接口统一（方案A：渐进式适配）

**目标**：统一 `audit.AuditStore`（旧） 和 `AuditStoreInterface`（新）

**步骤**：
1. 在 `AuditStore` 接口添加 `BatchEmit` 方法
2. 创建 `AuditStoreAdapter` 适配旧实现
3. 逐步将调用方迁移到新接口

**涉及文件**：
- `internal/audit/audit.go`（定义AuditStore接口）
- `internal/audit/service/audit_service_db.go`（实现AuditStoreInterface）
- `cmd/supply-api/main.go`（使用适配器连接）

---

## 五、修复优先级矩阵

```
P0 (阻断上线 - 修复后方可发布)
├── P0-01: SQL列名一致性
├── P0-02: BatchBuffer实现
└── P0-04: CI/CD Gate脚本

P1 (本周完成 - 影响性能和正确性)
├── P1-01: M-014过滤条件修复
├── P1-02: M-015判断逻辑修复
├── P1-03: M-016分母定义修复
├── P1-05: 分页total修复
└── P1-06: JSONB索引优化

P2 (按需 - 改进建议)
├── 实现异步写入队列（Phase 3）
└── 实现分区表（数据量超1000万时）
```

---

## 六、验证检查清单

修复完成后验证：

- [ ] SQL列名一致性：`before_state` / `after_state`
- [ ] 批量写入：50条/批，5ms刷新
- [ ] M-014过滤：`event_category='CRED' AND event_sub_category='INGRESS'`
- [ ] M-015过滤：`target_direct = TRUE`
- [ ] M-016分母：严格区分 AUTH-QUERY-KEY
- [ ] 分页total：使用Query返回的total
- [ ] CI/CD脚本：使用awk，无bc依赖

---

**清理执行状态**：P0-01已完成（2026-04-03）
**预计工作量**：P0（2天）+ P1（3天）+ P2（按需）
