# 审计日志增强设计评审报告

> 评审日期：2026-04-02
> 设计文档：docs/audit_log_enhancement_design_v1_2026-04-02.md
> 评审结论：CONDITIONAL GO（需修复高严重度问题）

---

## 评审结论

**CONDITIONAL GO**

设计文档整体架构合理，事件分类体系完整，M-013~M-016指标映射清晰。但存在若干高严重度一致性问题需要修复后才能进入开发阶段。

---

## 1. M-013~M-016指标覆盖

| 指标ID | 指标名称 | 覆盖状态 | 实现说明 | 问题 |
|--------|----------|----------|----------|------|
| M-013 | supplier_credential_exposure_events = 0 | 部分覆盖 | 凭证暴露检测器设计完整，事件分类正确 | 缺少与XR-001 invariant_violation的关联 |
| M-014 | platform_credential_ingress_coverage_pct = 100% | 有疑问 | SQL计算逻辑存在，与M-016关系需澄清 | M-014和M-016存在逻辑边界模糊 |
| M-015 | direct_supplier_call_by_consumer_events = 0 | 已覆盖 | target_direct字段设计完整 | 跨域检测机制未详细说明 |
| M-016 | query_key_external_reject_rate_pct = 100% | 已覆盖 | AUTH-QUERY-KEY/AUTH-QUERY-REJECT事件设计完整 | 与M-014的指标边界需澄清 |

**关键疑问**：M-014要求"覆盖率100%"（所有入站都是platform_token），而M-016要求"拒绝率100%"（所有query key被拒绝）。如果query key请求存在并被拒绝，该事件如何计入M-014的覆盖率？

---

## 2. 与XR-001/TOK-002一致性

| 检查项 | 状态 | 问题描述 |
|--------|------|----------|
| XR-001: request_id/idempotency_key/operator_id/object_id/result_code字段 | 通过 | 审计事件包含所有必需字段 |
| XR-001: invariant_violation事件必须写入 | **不通过** | 设计中未定义invariant_violation事件类型，SECURITY大类为空 |
| XR-001: 幂等语义（首次成功/重放同参/重放异参/处理中） | **部分通过** | idempotency_key字段存在，但API响应未定义409/202语义 |
| TOK-002: 4个事件（token.authn.success/fail, token.authz.denied, token.query_key.rejected） | **部分通过** | 事件拆分合理，但token.query_key.rejected对应的事件名称不一致 |
| TOK-002: 最小字段集（event_id, request_id, token_id, subject_id, route, result_code, client_ip, created_at） | 通过 | 设计包含所有最小字段，token_id/subject_id标记为可空 |
| 数据库跨域模型: audit_events表设计 | 通过 | 与database_domain_model_and_governance_v1一致 |

---

## 3. 一致性问题清单

### 3.1 高严重度（Must Fix）

| # | 严重度 | 问题 | 依据 | 建议修复 |
|---|--------|------|------|----------|
| 1 | **High** | invariant_violation事件未定义 | XR-001明确要求"所有不变量失败必须写入审计事件 invariant_violation，并携带 rule_code"，但设计的事件分类（3.1~3.5节）中没有此事件，SECURITY大类为空 | 在事件分类体系中增加`INVARIANT_VIOLATION`事件子类（建议挂在SECURITY大类下），并定义`invariant_rule`字段的填充规则 |
| 2 | **High** | M-014与M-016指标边界模糊 | M-014要求"平台凭证入站覆盖率=100%"，M-016要求"query key拒绝率=100%"。如果query key请求被拒绝，该事件如何影响M-014的计算？设计未明确两个指标的边界和相互关系 | 在设计文档中明确：M-014的分母是"经平台凭证校验的入站请求"（不含被拒绝的无效请求），M-016的分母是"检测到的所有query key请求"（含被拒绝的） |
| 3 | **High** | API幂等性返回语义不完整 | POST /api/v1/audit/events支持X-Idempotency-Key header，但API响应未定义409冲突（重放异参）和202处理中语义，与XR-001的幂等协议不一致 | 在API响应中增加409和202状态码定义，说明触发条件和返回体 |

### 3.2 中严重度（Should Fix）

| # | 严重度 | 问题 | 依据 | 建议修复 |
|---|--------|------|------|----------|
| 4 | **Medium** | 事件命名与TOK-002不完全对齐 | TOK-002使用`token.query_key.rejected`，设计使用`AUTH-QUERY-REJECT`，语义相同但命名风格不一致 | 统一事件命名规范，或在映射表中说明等价关系 |
| 5 | **Medium** | 错误码规范缺失 | 设计定义了结果码格式（12.2节），但未与现有错误码体系（如SUP_*、AUTH_*、SEC_*）进行对齐验证 | 增加错误码对照表，说明与现有体系的映射关系 |
| 6 | **Medium** | M-015直连检测机制未详细说明 | 设计有target_direct字段，但"跨域调用检测"的实现机制未描述 | 在设计文档中补充M-015的检测点说明 |

### 3.3 低严重度（Nice to Fix）

| # | 严重度 | 问题 | 依据 | 建议修复 |
|---|--------|------|------|----------|
| 7 | **Low** | 性能目标未与现有系统基线对比 | 设计目标（<10ms写入、<500ms查询）未说明对比基准 | 补充与现有gateway/supply-api的性能基线对比 |
| 8 | **Low** | 分区表实现语法可能有兼容性问题 | PostgreSQL分区表语法（5.1节）可能在低版本PG上不兼容 | 说明最低PG版本要求，或调整语法 |

---

## 4. 改进建议

### 4.1 紧急修复（进入开发前必须完成）

1. **补充invariant_violation事件定义**
   ```go
   // 建议在事件分类中增加
   const (
       CategorySECURITY = "SECURITY"
       SubCategoryInvariantViolation = "INVARIANT_VIOLATION"
   )

   // 审计事件增加字段
   type AuditEvent struct {
       // ... 现有字段 ...
       InvariantRule string `json:"invariant_rule,omitempty"` // 触发的不变量规则编码
   }
   ```

2. **澄清M-014与M-016的指标边界**
   - 明确M-014的分母：credential_type = 'platform_token'的入站请求（经过平台凭证校验的请求）
   - 明确M-016的分母：event_name LIKE 'AUTH-QUERY%'的所有请求（含被拒绝的）
   - 两者互不影响，因为query key请求在通过平台认证前不会进入M-014的计数范围

3. **补充API幂等性响应语义**
   ```json
   // 409 重放异参
   {
     "error": {
       "code": "IDEMPOTENCY_PAYLOAD_MISMATCH",
       "message": "Idempotency key reused with different payload"
     }
   }

   // 202 处理中
   {
     "status": "processing",
     "retry_after_ms": 1000
   }
   ```

### 4.2 建议增强

1. **增加事件名称映射表**：说明设计中的事件名称与TOK-002/XR-001中定义的事件名称的映射关系

2. **补充错误码对照表**：说明与现有错误码体系（SUP_*、AUTH_*、SEC_*）的映射

3. **完善M-015检测机制说明**：补充跨域调用检测的技术实现方案

4. **增加脱敏规则版本管理**：脱敏规则（12.3节）应支持版本化和灰度发布

---

## 5. 最终结论

### 5.1 评审结果

**CONDITIONAL GO** - 设计文档在架构层面基本合格，但存在3个高严重度一致性问题，必须在进入开发阶段前修复。

### 5.2 阻塞项

| # | 阻塞项 | 修复标准 |
|---|--------|----------|
| 1 | invariant_violation事件未定义 | 在事件分类体系中明确定义，并说明触发时机和填充规则 |
| 2 | M-014与M-016边界模糊 | 在设计文档中明确两个指标的计算边界和相互关系 |
| 3 | API幂等性响应不完整 | 定义409/202状态码的触发条件和返回体 |

### 5.3 后续行动

1. **设计作者**：根据上述问题清单修订设计文档
2. **评审通过条件**：3个高严重度问题全部修复后，视为CONDITIONAL GO，可进入开发阶段
3. **预计修复时间**：1-2天

---

## 附录：评审对比基线

| 基线文档 | 版本 | 关键引用 |
|----------|------|----------|
| PRD v1 | v1.0 (2026-03-25) | P1需求：审计日志（策略与key变更）；关键规则：策略变更必须可审计 |
| XR-001 | v1.1 (2026-03-27) | 审计字段：request_id/idempotency_key/operator_id/object_id/result_code；必须写入invariant_violation |
| TOK-002 | v1.0 (2026-03-29) | 4个Token审计事件；最小字段集：event_id/request_id/token_id/subject_id/route/result_code/client_ip/created_at |
| 数据库跨域模型 | v1.0 (2026-03-27) | Audit域：audit_events表；索引策略覆盖高频查询 |
| Daily Review | 2026-04-03 | M-013~M-016均标记为"待staging验证"，说明设计阶段已完成mock实现 |

---

**报告生成时间**：2026-04-02
**评审人**：Claude Code
**文档版本**：v1.0
