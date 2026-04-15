> 归档状态：已归档
> 归档批次：ARCHIVE-HISTORY-2026-04-14-A01
> 归档标识：AHR-20260414-019
> 当前用途：历史快照，仅供追溯；不得作为现行门禁、发布决议或实现状态事实源。
> 当前事实源：`docs/plans/2026-04-14-repo-integrity-refactor-plan.md`、`reports/archive/gate_verification/` 与仍在维护的签署/门禁文档。

# 路由策略模板设计评审报告

> 评审日期：2026-04-02
> 评审文档：`docs/routing_strategy_template_design_v1_2026-04-02.md`
> 评审基线：PRD v1、Router Core Takeover计划、技术架构设计

---

## 评审结论

**CONDITIONAL GO**

设计文档整体质量良好，完整覆盖了P0/P1需求并与Router Core架构对齐。但存在若干需要在实施前明确的细节问题：

1. **严重**：评分模型权重与技术架构不一致（延迟40%/可用性30%/成本20%/质量10% vs 文档中未明确锁定）
2. **中等**：缺少A/B测试和灰度发布支持
3. **中等**：Fallback与Ratelimit集成逻辑需要与现有ratelimit模块确认兼容性
4. **低**：M-008 route_mark_coverage指标采集依赖RouterEngine字段，需确保全路径覆盖

---

## 1. PRD P0/P1需求覆盖

| 需求项 | 覆盖状态 | 实现说明 | 备注 |
|--------|----------|----------|------|
| P0: 多provider负载与fallback | **完全覆盖** | 第四章详细设计了多级Fallback架构，支持Tier1/Tier2层级和多种触发条件 | ✅ |
| P0: 请求重试与错误可见 | **完全覆盖** | FallbackConfig中MaxRetries/RetryIntervalMs配置；RoutingDecision包含完整审计字段 | ✅ |
| P1: 路由策略模板（按场景） | **完全覆盖** | 策略类型枚举完整（cost_based/quality_first/latency_first/model_specific/composite）；支持YAML配置化；通过applicable_models/providers实现场景匹配 | ✅ |
| P1: 多维度决策 | **完全覆盖** | CostAwareBalancedParams支持成本/质量/延迟三维度权衡；ScoringModel提供归一化评分机制 | ✅ |

**评审意见**：
- P0需求完全满足，Fallback机制设计比技术架构更完善（增加了触发条件、层级概念）
- P1需求完整实现，策略模板类型丰富且配置化完整
- 建议在实施阶段确认Fallback与现有ratelimit模块的集成方式

---

## 2. M-006/M-007/M-008指标对齐

| 指标 | 指标定义 | 对齐状态 | 设计支持度 | 实现说明 |
|------|----------|----------|------------|----------|
| **M-006** | overall_takeover_pct >= 60% | **对齐** | 高 | `RoutingDecision.RouterEngine`字段标记"router_core"；`RoutingMetrics.RecordDecision()`按router_engine统计；`UpdateTakeoverRate()`更新overallRate |
| **M-007** | cn_takeover_pct = 100% | **对齐** | 高 | cn_provider策略模板（第757-787行）配置国内供应商优先，`default_provider: "cn_primary"`，Fallback至Tier2国际供应商 |
| **M-008** | route_mark_coverage_pct >= 99.9% | **部分对齐** | 中 | `RecordTakeoverMark()`方法存在，但依赖RouterEngine字段全路径覆盖；需验证所有路由路径是否均设置此字段 |

**关键风险**：
- **M-008风险**：route_mark_coverage需要确保100%的请求都带有router_engine标记。文档中`RecordTakeoverMark`仅在E2E测试示例中调用，需确保生产代码中所有路由决策路径都调用此方法。

---

## 3. 与Router Core一致性

### 3.1 架构一致性

| 检查项 | 状态 | 问题描述 | 建议 |
|--------|------|----------|------|
| RouterService模块设计 | ✅ 一致 | 文档中`RoutingEngine`对应技术架构的RouterService | 无 |
| Provider Adapter模式 | ✅ 一致 | ProviderInfo/ProviderAdapter接口与adapter.Registry设计一致 | 无 |
| 多维度评分机制 | ⚠️ **权重不一致** | 技术架构：延迟40%/可用性30%/成本20%/质量10%；文档ScoringModel未锁定权重，由StrategyParams传入 | **需明确**：是否将技术架构的固定权重作为默认值？或允许策略模板覆盖？ |

### 3.2 评分模型权重对比

| 维度 | 技术架构权重 | 文档实现 | 一致性 |
|------|-------------|----------|--------|
| 延迟 | 40% | LatencyWeight（未指定默认值） | ⚠️ 不一致 |
| 可用性 | 30% | AvailabilityScore | ⚠️ 未在ScoringModel中体现 |
| 成本 | 20% | CostWeight | ⚠️ 不一致 |
| 质量 | 10% | QualityWeight | ⚠️ 不一致 |

**结论**：技术架构定义的是`calculateScore`函数的**参考权重**，而文档中`ScoringModel`是**可配置权重**模型。两者设计思路不同（固定 vs 可配置），建议：
1. 在策略模板中明确定义默认权重
2. 不同策略模板允许覆盖权重但需说明适用场景

### 3.3 Fallback机制一致性

| 检查项 | 状态 | 说明 |
|--------|------|------|
| Failover决策 | ✅ 一致 | 文档Tier/FallbackTrigger机制完整 |
| 重试策略 | ✅ 一致 | MaxRetries/RetryIntervalMs配置完整 |
| 流式边界保护 | ⚠️ **未覆盖** | 技术架构中提到Stream Guard Layer，文档未明确流式请求的Fallback行为差异 |


---

## 4. 一致性问题清单

| 严重度 | 问题 | 影响 | 建议修复 |
|--------|------|------|----------|
| **高** | 评分权重未锁定 | 不同策略模板可能产生不同的路由结果，与技术架构预期不符 | 在`StrategyParams`或`ScoreWeights`中定义默认权重值，并在策略模板YAML示例中明确标注 |
| **高** | M-008 route_mark_coverage采集路径不完整 | 可能导致指标不达标 | 确保`RoutingEngine.SelectProvider()`和所有Fallback路径都调用`RecordTakeoverMark()` |
| **中** | 缺少A/B测试支持 | 无法验证策略效果 | 增加ABStrategyTemplate类型，支持流量分组实验 |
| **中** | Fallback与Ratelimit集成需确认 | 文档`FallbackRateLimiter`是新设计，与现有`ratelimit.TokenBucketLimiter`关系需明确 | 确认Fallback请求是否复用主限流配额，还是使用独立配额 |
| **中** | 灰度发布支持缺失 | 无法灰度验证策略效果 | 增加策略灰度配置（如percentage/rolling_update） |
| **低** | 流式请求Fallback行为未定义 | 流式请求在部分响应后失败的处理逻辑不明确 | 在FallbackTrigger中增加`stream_interruption`触发条件 |

---

## 5. 与现有代码结构一致性

### 5.1 目录结构一致性

| 检查项 | 文档设计 | 现有代码 | 一致性 |
|--------|----------|----------|--------|
| 路由目录 | `gateway/internal/router/` | `gateway/internal/router/router.go` | ✅ 一致 |
| Adapter目录 | `gateway/internal/adapter/` | `gateway/internal/adapter/adapter.go` | ✅ 一致 |
| Middleware集成 | `RoutingRateLimitMiddleware` | `gateway/internal/ratelimit/ratelimit.go` | ✅ 结构一致，需确认集成方式 |
| Alert集成 | `RoutingAlerter` | `gateway/internal/alert/alert.go` | ✅ 结构一致 |

### 5.2 接口兼容性

| 接口 | 文档定义 | 现有接口 | 兼容性 |
|------|----------|----------|--------|
| Router.SelectProvider | `(ctx, model) -> (ProviderAdapter, error)` | `Router.SelectProvider(ctx, model)` | ✅ 兼容 |
| Router.GetFallbackProviders | `(ctx, model) -> ([]ProviderAdapter, error)` | `Router.GetFallbackProviders(ctx, model)` | ✅ 兼容 |
| Router.RecordResult | `(ctx, provider, success, latencyMs)` | 未在文档中直接对应，但MetricsCollector覆盖 | ⚠️ 建议统一为MetricsCollector方式 |

**评审意见**：文档设计的`RoutingEngine`是新组件，与现有`Router`接口并存的设计合理，可渐进式迁移。

---

## 6. 可测试性评估

| 测试项 | 可测试性 | 测试方法 | 备注 |
|--------|----------|----------|------|
| 评分模型量化 | ✅ 高 | `TestScoringModel_CalculateScore`单元测试 | 权重可配置，测试场景丰富 |
| 策略切换验证 | ✅ 高 | YAML配置动态加载+策略匹配逻辑测试 | `TestStrategyMatchOrder` |
| Fallback层级执行 | ✅ 高 | `TestFallbackStrategy_TierExecution` | 已提供测试示例 |
| M-006/M-007指标采集 | ✅ 中 | E2E测试`TestRoutingEngine_E2E_WithTakeoverMetrics` | 需确保全路径覆盖 |
| M-008 route_mark_coverage | ⚠️ 中 | 依赖100%路径覆盖 | 需增加集成测试验证 |

---

## 7. 行业最佳实践

| 实践项 | 状态 | 说明 |
|--------|------|------|
| 策略配置热更新 | ✅ 已支持 | `StrategyLoader.WatchChanges()`使用fsnotify监控配置文件变更 |
| A/B测试支持 | ❌ 不支持 | 缺少流量分组和实验配置 |
| 灰度发布支持 | ❌ 不支持 | 缺少canary/percentage配置 |
| 配置版本管理 | ⚠️ 未提及 | 建议增加策略配置版本和回滚机制 |
| 策略优先级冲突处理 | ✅ 已覆盖 | `StrategyMatchOrder`配置解决 |

---

## 8. 改进建议

### 8.1 高优先级修复项

1. **明确评分权重默认值**
   ```go
   // 建议在ScoreWeights中定义默认值
   const DefaultScoreWeights = ScoreWeights{
       CostWeight:        0.2,  // 20%
       QualityWeight:     0.1,  // 10%
       LatencyWeight:     0.4,  // 40%
       AvailabilityWeight: 0.3, // 30%
   }
   ```

2. **完善M-008指标采集**
   - 确保`RoutingEngine.SelectProvider()`和`handleFallback()`路径都调用`RecordTakeoverMark()`
   - 增加集成测试覆盖全路径

### 8.2 中优先级增强项

1. **增加ABStrategyTemplate**
   ```go
   type ABStrategyTemplate struct {
       RoutingStrategyTemplate
       ControlGroupID string
       ExperimentGroupID string
       TrafficSplit int // 0-100
   }
   ```

2. **完善流式Fallback逻辑**
   - 在`FallbackTrigger`中增加`stream_interruption`触发条件
   - 定义流式部分响应后的降级行为

3. **增加策略灰度配置**
   ```yaml
   strategy:
     id: "cn_provider"
     rollout:
       enabled: true
       percentage: 10  # 初始10%流量
       max_percentage: 100
       increment: 10  # 每次增加10%
       interval: 24h
   ```

### 8.3 低优先级优化项

1. 增加配置版本管理和回滚机制
2. 增加策略效果分析指标（成本节省率、延迟改善率）
3. 提供策略模拟器工具支持离线验证

---

## 9. 最终结论

### 评审结果：CONDITIONAL GO

| 维度 | 评分 | 说明 |
|------|------|------|
| PRD P0/P1覆盖 | 9/10 | 完全覆盖，Fallback设计优秀 |
| M-006/M-007/M-008对齐 | 8/10 | 整体对齐，M-008有覆盖风险 |
| Router Core一致性 | 7/10 | 架构一致，评分权重需明确 |
| 代码结构一致性 | 9/10 | 目录结构一致，接口兼容 |
| 可测试性 | 8/10 | 测试设计完整，覆盖率高 |
| 行业最佳实践 | 6/10 | 缺少A/B测试和灰度发布支持 |

**通过条件**：
1. 明确评分模型默认权重（建议与技术架构一致：延迟40%/可用性30%/成本20%/质量10%）
2. 完善M-008 route_mark_coverage全路径采集逻辑
3. 补充A/B测试和灰度发布支持设计

**备注**：本设计文档整体质量良好，核心路由逻辑和Fallback机制设计完善。建议在实施前与Router Core团队确认评分权重默认值，并补充M-008的全路径覆盖验证方案。

---

## 附录：评审检查清单

- [x] PRD P0需求覆盖检查
- [x] PRD P1需求覆盖检查
- [x] M-006指标对齐检查
- [x] M-007指标对齐检查
- [x] M-008指标对齐检查
- [x] Router Core架构一致性检查
- [x] 评分模型权重一致性检查
- [x] Fallback机制一致性检查
- [x] 代码目录结构一致性检查
- [x] 接口兼容性检查
- [x] 可测试性评估
- [x] 行业最佳实践评估
- [x] 改进建议输出

---

**评审人**：Claude Code
**评审日期**：2026-04-02
**文档版本**：v1.0
