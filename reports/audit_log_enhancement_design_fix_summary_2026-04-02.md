> 归档状态：已归档
> 归档批次：ARCHIVE-HISTORY-2026-04-14-A01
> 归档标识：AHP-20260414-002
> 当前用途：历史快照，仅供追溯；不得作为现行门禁、发布决议或实现状态事实源。
> 当前事实源：`docs/plans/2026-04-14-repo-integrity-refactor-plan.md`、`reports/archive/gate_verification/` 与仍在维护的签署/门禁文档。

# 审计日志增强设计文档修复报告

> 修复日期：2026-04-02
> 原文档：`docs/audit_log_enhancement_design_v1_2026-04-02.md`
> 评审报告：`reports/review/audit_log_enhancement_design_review_2026-04-02.md`

---

## 修复概述

根据评审报告，共修复6个问题（3个高严重度 + 3个中严重度），修复后设计与TOK-002/XR-001/合规能力包保持一致。

---

## 修复清单

### 高严重度问题（Must Fix）

#### 1. invariant_violation事件未定义 [FIXED]

**问题描述**：XR-001明确要求"所有不变量失败必须写入审计事件invariant_violation"，但设计中SECURITY大类为空。

**修复内容**：
- 在3.6节新增SECURITY事件子类
- 添加`INVARIANT-VIOLATION`子类（直接关联M-013）
- 增加`INVARIANT-VIOLATION`事件详细定义，包含6个不变量规则：
  - INV-PKG-001：供应方资质过期
  - INV-PKG-002：供应方余额为负
  - INV-PKG-003：售价不得低于保护价
  - INV-SET-001：`processing/completed`不可撤销
  - INV-SET-002：提现金额不得超过可提现余额
  - INV-SET-003：结算单金额与余额流水必须平衡

**修复位置**：文档第142-161行

---

#### 2. M-014与M-016指标边界模糊 [FIXED]

**问题描述**：M-014要求"覆盖率=100%"，M-016要求"拒绝率=100%"。如果query key请求被拒绝，该事件如何影响M-014计算？

**修复内容**：
- 在8.2节M-014下新增"M-014与M-016边界说明"小节
- 明确M-014分母定义：经平台凭证校验的入站请求（`credential_type = 'platform_token'`），不含被拒绝的无效请求
- 明确M-016分母定义：检测到的所有query key请求（含被拒绝的）
- 说明两者互不影响的原因

**示例说明**：
- 80个platform_token请求 + 20个query key请求（被拒绝）
- M-014 = 80/80 = 100%（分母只计算platform_token请求）
- M-016 = 20/20 = 100%（分母计算所有query key请求）

**修复位置**：文档第961-973行

---

#### 3. API幂等性响应语义不完整 [FIXED]

**问题描述**：POST /api/v1/audit/events支持X-Idempotency-Key，但未定义409冲突和202处理中的响应语义。

**修复内容**：
- 在6.1节新增"幂等性响应语义"小节
- 定义4种状态码场景：
  - 201：首次成功
  - 202：处理中
  - 409：重放异参（幂等键已使用但payload不同）
  - 200：重放同参（幂等键已使用且payload相同）
- 提供每种场景的响应体示例

**修复位置**：文档第537-549行

---

### 中严重度问题（Should Fix）

#### 4. 事件命名与TOK-002不完全对齐 [FIXED]

**问题描述**：TOK-002使用`token.query_key.rejected`，设计使用`AUTH-QUERY-REJECT`，语义相同但命名风格不一致。

**修复内容**：
- 在12.1.1节新增"事件名称与TOK-002对齐映射"小节
- 建立5个事件的等价映射关系：
  - AUTH-TOKEN-OK <-> token.authn.success
  - AUTH-TOKEN-FAIL <-> token.authn.fail
  - AUTH-SCOPE-DENY <-> token.authz.denied
  - AUTH-QUERY-REJECT <-> token.query_key.rejected
  - AUTH-QUERY-KEY（仅审计记录）
- 说明两种命名风格的适用场景

**修复位置**：文档第1305-1318行

---

#### 5. 错误码规范缺失 [FIXED]

**问题描述**：未与现有错误码体系（SUP_*/AUTH_*/SEC_*）进行对齐验证。

**修复内容**：
- 在12.2.1节新增"错误码体系对照表"
- 对齐TOK-002错误码：AUTH_MISSING_BEARER、AUTH_INVALID_TOKEN、AUTH_TOKEN_INACTIVE、AUTH_SCOPE_DENIED、QUERY_KEY_NOT_ALLOWED
- 对齐XR-001错误码：SEC_CRED_EXPOSED、SEC_DIRECT_BYPASS、SEC_INV_PKG_*、SEC_INV_SET_*
- 对齐供应侧错误码：SUP_PKG_*、SUP_SET_*
- 明确每个错误码对应的审计事件

**修复位置**：文档第1337-1349行

---

#### 6. M-015直连检测机制未详细说明 [FIXED]

**问题描述**：target_direct字段存在但"跨域调用检测"的实现机制未描述。

**修复内容**：
- 在8.3节新增"M-015直连检测机制详细设计"小节
- 详细说明4种检测方法：
  - IP/域名白名单比对
  - 上游API模式匹配
  - DNS解析监控
  - 连接来源检测
- 提供检测流程图（M015-FLOW-01）
- 定义target_direct字段填充规则表

**修复位置**：文档第1000-1045行

---

## 验证清单

- [x] 与XR-001 invariant_violation要求一致
- [x] 与TOK-002事件命名对齐
- [x] 与合规能力包M-015检测机制一致
- [x] M-014/M-016边界明确且互不干扰
- [x] API幂等性响应语义完整
- [x] 错误码与现有体系对齐

---

## 修复后的文档版本

- 文档路径：`/home/long/project/立交桥/docs/audit_log_enhancement_design_v1_2026-04-02.md`
- 修复日期：2026-04-02
- 状态：已根据评审意见修复所有高严重度和中严重度问题

---

**报告生成时间**：2026-04-02
**修复执行人**：Claude Code
