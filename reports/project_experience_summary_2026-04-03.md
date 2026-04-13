# 项目经验总结

> 日期：2026-04-03
> 项目：supply-api 审计系统优化
> 总结人：AI助手

---

## 一、关键经验教训

### 1.1 接口设计：添加新方法时必须同步更新所有实现

**问题**：`AuditStoreInterface` 添加 `EmitBatch` 方法后，所有实现（包括 mock）都需要同步更新。

**教训**：
- 添加接口方法时，立即在所有实现类中实现
- Mock 对象必须与接口保持同步
- 接口变更应作为整体变更（atomic change）

**代码规范**：
```go
// ✅ 正确做法：接口变更时同步更新所有实现
type AuditStoreInterface interface {
    Emit(ctx context.Context, event *AuditEvent) error
    EmitBatch(ctx context.Context, events []*AuditEvent) error  // 新方法
    Query(ctx context.Context, filter *EventFilter) ([]*AuditEvent, int64, error)
}

// ✅ Mock 实现也同步添加
func (m *mockAuditStore) EmitBatch(ctx context.Context, events []*AuditEvent) error {
    for _, event := range events {
        if err := m.Emit(ctx, event); err != nil {
            return err
        }
    }
    return nil
}
```

---

### 1.2 并发安全：锁获取模式必须一致

**问题**：`InMemoryAuditStore.Emit` 持有写锁后调用 `cleanupOldEvents`，而 `cleanupOldEvents` 内部尝试获取锁导致死锁。

**教训**：
- 在持有锁的方法内部调用其他方法时，确保使用 `*Locked` 后缀的内部版本
- 不要在持锁方法中调用获取同一把锁的方法
- 使用 `defer unlock` 确保锁一定释放

**代码规范**：
```go
// ✅ 正确做法：分离公共方法和锁内方法
func (s *InMemoryAuditStore) Emit(ctx context.Context, event *AuditEvent) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    // 直接调用锁内版本
    if len(s.events) >= MaxEvents {
        s.cleanupOldEventsLocked(MaxEvents / 10)  // 不再获取锁
    }
    // ...
    return nil
}

func (s *InMemoryAuditStore) cleanupOldEvents(removeCount int) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.cleanupOldEventsLocked(removeCount)  // 先获取锁再调用
}

func (s *InMemoryAuditStore) cleanupOldEventsLocked(removeCount int) {
    // caller 已持有锁，直接操作
}
```

---

### 1.3 TDD 流程：先写测试再实现

**经验**：
- 先创建测试文件（会编译失败）
- 实现功能使测试通过
- 测试覆盖关键路径

**流程**：
```bash
# 1. 创建测试（会编译失败）
$ go test ./...  # FAIL: undefined: NewBatchBuffer

# 2. 实现功能
$ go test ./...  # PASS

# 3. 验证所有测试
$ go test -v ./...  # 全绿
```

---

### 1.4 SQL 列名一致性：设计文档与代码必须对齐

**问题**：`before_data` vs `before_state` 列名不一致

**教训**：
- 设计文档中的 SQL DDL 必须与代码中的 SQL 语句一致
- 修改列名时同步修改所有引用位置
- 使用 `grep` 验证无遗漏

**验证命令**：
```bash
# 检查错误列名是否已清除
$ grep -n 'before_data\|after_data' internal/audit/repository/audit_repository.go

# 确认正确列名
$ grep -n 'before_state\|after_state' internal/audit/repository/audit_repository.go
```

---

### 1.5 设计文档维护：设计变更后立即更新文档

**问题**：实现了 BatchBuffer 但设计文档未及时更新 API 端点

**教训**：
- 代码实现与设计文档必须同步更新
- 实现新功能后立即更新设计文档的 API 部分
- 定期检查设计文档与实现的差距

**检查清单**：
- [ ] 新增 API 是否已在设计文档中定义
- [ ] 新增字段是否已在 schema 文档中记录
- [ ] 新增组件是否已在架构图中标注

---

## 二、代码规范要点

### 2.1 Go 接口设计

| 规范 | 说明 |
|------|------|
| 接口方法数量 | 尽量小（5个以内），避免过度设计 |
| 接口粒度 | 按职责分离，不要一个巨接口 |
| 兼容性 | 添加方法时考虑向后兼容 |
| Mock | 每个接口都有对应的 mock 实现用于测试 |

### 2.2 并发安全

| 规范 | 说明 |
|------|------|
| 锁粒度 | 只保护必要数据，最小化锁范围 |
| 死锁预防 | 统一锁获取顺序，避免嵌套获取同一把锁 |
| 读写锁 | 读多写少用 `RWMutex` |
| channel | 用于解耦和异步，非同步保护 |

### 2.3 批量处理

| 规范 | 说明 |
|------|------|
| 批大小 | 根据业务场景设置（审计日志：50条/批） |
| 超时刷新 | 防止低流量时数据延迟（5ms刷新间隔） |
| 半同步 | 低TPS同步，高TPS批量 |
| 优雅关闭 | 关闭时处理完缓冲数据 |

---

## 三、易犯错问题清单

### 3.1 P0 阻塞问题（必须修复）

| # | 问题 | 预防措施 |
|---|------|----------|
| 1 | SQL 列名不一致 | 设计文档 DDL 与代码 SQL 必须一致 |
| 2 | 批量写入未实现 | 按 Phase 计划实现性能优化 |
| 3 | 路由注册两次 | 注册逻辑只执行一次 |
| 4 | 接口实现不完整 | 接口变更时同步更新所有实现 |

### 3.2 P1 高优先级问题

| # | 问题 | 预防措施 |
|---|------|----------|
| 1 | M-014/015/016 过滤条件不一致 | 使用模型字段而非 EventName 前缀 |
| 2 | 分页 total 不准确 | 使用 Query 返回的 total |
| 3 | 内存泄漏 | 注意锁内操作避免死锁 |

### 3.3 代码异味

| # | 问题 | 建议 |
|---|------|------|
| 1 | 硬编码值 | 使用常量替代 |
| 2 | 魔法数字 | 定义有意义的常量 |
| 3 | TODO | 定期清理，转化为 Issue |

---

## 四、项目进度追踪

### 4.1 P0 问题修复状态

| 编号 | 问题 | 状态 | 完成日期 |
|------|------|------|----------|
| P0-01 | SQL列名一致性 | ✅ 已完成 | 2026-04-03 |
| P0-02 | BatchBuffer实现 | ✅ 已完成 | 2026-04-03 |
| P0-03 | 分区表 | ⏳ 设计文档已更新为暂不分 |
| P0-04 | CI/CD Gate脚本 | ⏳ 设计文档已更新 |

### 4.2 P1 问题修复状态

| 编号 | 问题 | 状态 | 完成日期 |
|------|------|------|----------|
| P1-01 | M-014过滤条件 | ✅ 已完成 | 2026-04-03 |
| P1-02 | M-015判断逻辑 | ✅ 已完成 | 2026-04-03 |
| P1-03 | M-016分母定义 | ✅ 已完成 | 2026-04-03 |
| P1-04 | 内存过滤 | ⚠️ 待优化 | - |
| P1-05 | 分页total | ✅ 已完成 | 2026-04-03 |
| P1-06 | JSONB索引 | ⚠️ 待实现 | - |

### 4.3 新增功能

| 功能 | 状态 | 完成日期 |
|------|------|----------|
| 指标API (M-013~M-016) | ✅ 已实现 | 2026-04-03 |
| 批量写入API | ✅ 已实现 | 2026-04-03 |
| EmitBatch方法 | ✅ 已实现 | 2026-04-03 |

---

## 五、后续工作

### 5.1 立即行动

| # | 任务 | 优先级 |
|---|------|--------|
| 1 | 补充单元测试覆盖 | 高 |
| 2 | 更新设计文档 API 部分 | 高 |
| 3 | 清理废弃代码标记 | 中 |

### 5.2 按需实施

| # | 任务 | 说明 |
|---|------|------|
| 1 | 告警API | 业务确认后实现 |
| 2 | 单事件GET API | `GET /api/v1/audit/events/{event_id}` |
| 3 | 冗余布尔字段 | 性能测试后决定 |

---

## 六、测试命令备忘

```bash
# 运行所有测试
go test ./...

# 运行特定包测试
go test -v ./internal/audit/service/...

# 运行 Batch 相关测试
go test -v -run TestBatch ./internal/audit/service/...

# 编译检查
go build ./...

# 检查列名问题
grep -n 'before_data\|after_data' internal/audit/repository/audit_repository.go
```

---

**总结日期**：2026-04-03
**下次更新**：下次重大变更后
