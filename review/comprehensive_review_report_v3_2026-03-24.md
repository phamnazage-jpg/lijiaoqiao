# 立交桥项目全面 Review 报告（2026-03-24）

- 报告版本：v3.0
- 评审日期：2026-03-24
- 评审范围：`/home/long/project/立交桥` 下 `docs/`、`review/` 及交付物可验证性
- 评审方法：文档一致性审查、门禁可执行性审查、合规与安全交叉审查

---

## 1. 执行结论

**结论：NO-GO（当前不可进入实施/发布）**

阻断原因不是单点缺陷，而是“业务模型、合规红线、门禁证据”三者同时不一致。  
尤其是以下新增硬约束未被吸收：

> **短期内，用户分享 token 只能分享给平台，不能直接分享给其他用户。**

当前主方案仍为“用户A供给 -> 平台 -> 用户B购买/拿 Key 直用”的双边市场模型，与该约束冲突。

---

## 2. 关键发现（按严重级别）

## 2.1 P0-01：短期策略与核心业务模型冲突（直接阻断）

### 现象

多个核心文档把“用户对用户的 token/配额交易”作为当前主路径，不是“仅对平台共享”。

### 证据

- [`docs/llm_gateway_subapi_evolution_plan_v4_1_2026-03-18.md:223`](/home/long/project/立交桥/docs/llm_gateway_subapi_evolution_plan_v4_1_2026-03-18.md:223) 定义 `需求方（Consumer）`。
- [`docs/supply_side_product_design_v1_2026-03-18.md:33`](/home/long/project/立交桥/docs/supply_side_product_design_v1_2026-03-18.md:33) 明确“供应方 -> 平台 -> 需求方”。
- [`docs/supply_side_product_design_v1_2026-03-18.md:159`](/home/long/project/立交桥/docs/supply_side_product_design_v1_2026-03-18.md:159) 需求方“浏览套餐/下单购买”。
- [`docs/supply_detailed_design_v1_2026-03-18.md:688`](/home/long/project/立交桥/docs/supply_detailed_design_v1_2026-03-18.md:688) 需求方“获取 API Key”。
- [`docs/business_model_profitability_design_v1_2026-03-18.md:15`](/home/long/project/立交桥/docs/business_model_profitability_design_v1_2026-03-18.md:15) 明确“加价出售给需求方”。

### 影响

1. 与短期经营策略冲突，执行后会造成产品、运营、风控目标错配。
2. token 责任边界不清，用户争议和赔付责任上升。
3. 后续法务条款与用户协议难以闭环。

### 处理建议（必须先做）

1. 将短期模型改为 **B2P（用户仅向平台共享）**，去除 C2C 交易语义。
2. 从 S0/S1 设计中下线“需求方选购套餐、需求方拿供应方来源 Key”流程。
3. 短期只保留“平台统一计费与平台统一调用凭证”。

---

## 2.2 P0-02：ToS 红线与商业主张自相矛盾（直接阻断）

### 现象

ToS 文档定义“账号共享禁令/转售禁令”为红线，但商业文档同时把“统购统销、加价转售”作为核心模式。

### 证据

- [`docs/tos_compliance_engine_design_v1_2026-03-18.md:107`](/home/long/project/立交桥/docs/tos_compliance_engine_design_v1_2026-03-18.md:107) `R001 账号共享禁令`。
- [`docs/tos_compliance_engine_design_v1_2026-03-18.md:108`](/home/long/project/立交桥/docs/tos_compliance_engine_design_v1_2026-03-18.md:108) `R002 转售禁令`。
- [`docs/business_model_profitability_design_v1_2026-03-18.md:26`](/home/long/project/立交桥/docs/business_model_profitability_design_v1_2026-03-18.md:26) “核心模式：统购统销（买断->加价出售）”。
- [`docs/supply_side_product_design_v1_2026-03-18.md:129`](/home/long/project/立交桥/docs/supply_side_product_design_v1_2026-03-18.md:129) 出售价公式明确加价。

### 影响

1. 规则引擎会阻断业务主流程，或业务主流程会违反规则引擎，二者必有其一失效。
2. 运营与法务无法建立统一执法口径。
3. GO/NO-GO 判定失真。

### 处理建议（必须先做）

1. 按供应商维度拆分可做/不可做模型，不再统一“统购统销”。
2. 未获法务书面放行前，禁止把“加价转售”写入默认商业主路径。
3. 将 ToS 红线映射到门禁表中的硬 Gate（见 P1-03）。

---

## 2.3 P0-03：法务结论与证据链未闭环，却出现“已拍板/可推进”表述（直接阻断）

### 现象

法务文档仍是“待确认”，但主基线与历史报告中已出现“已拍板/修复完成”语义。

### 证据

- [`docs/tos_legal_communication_plan_v1_2026-03-18.md:197`](/home/long/project/立交桥/docs/tos_legal_communication_plan_v1_2026-03-18.md:197) 至 [`docs/tos_legal_communication_plan_v1_2026-03-18.md:202`](/home/long/project/立交桥/docs/tos_legal_communication_plan_v1_2026-03-18.md:202) 多项法务事项未勾选。
- [`docs/tos_legal_communication_plan_v1_2026-03-18.md:216`](/home/long/project/立交桥/docs/tos_legal_communication_plan_v1_2026-03-18.md:216) 文档状态是“沟通准备材料”。
- [`docs/llm_gateway_subapi_evolution_plan_v4_1_2026-03-18.md:409`](/home/long/project/立交桥/docs/llm_gateway_subapi_evolution_plan_v4_1_2026-03-18.md:409) 对 S4 关键项标记“已拍板”。
- [`review/final_decision_2026-03-31.md:1`](/home/long/project/立交桥/review/final_decision_2026-03-31.md:1) 到 [`review/final_decision_2026-03-31.md:5`](/home/long/project/立交桥/review/final_decision_2026-03-31.md:5) 仍为空模板态。

### 影响

1. 决策链与证据链脱节，治理可信度不足。
2. 管理层看到“已修复/可推进”时存在误判风险。

### 处理建议（必须先做）

1. 把所有“已拍板”状态改为“待法务签署后生效”。
2. 引入法务签署文件路径作为发布硬门禁。
3. 复审报告只允许引用已存在证据，不允许前置写“预审结论”代替事实。

---

## 2.4 P1-01：关键参数与时间线存在多版本口径，SSOT 失效

### 现象与证据

1. **采购折扣系数冲突**：
   - [`docs/llm_gateway_subapi_evolution_plan_v4_1_2026-03-18.md:396`](/home/long/project/立交桥/docs/llm_gateway_subapi_evolution_plan_v4_1_2026-03-18.md:396) = 60%
   - [`docs/supply_side_product_design_v1_2026-03-18.md:135`](/home/long/project/立交桥/docs/supply_side_product_design_v1_2026-03-18.md:135) = 85%

2. **毛利率目标冲突**：
   - [`docs/llm_gateway_subapi_evolution_plan_v4_1_2026-03-18.md:397`](/home/long/project/立交桥/docs/llm_gateway_subapi_evolution_plan_v4_1_2026-03-18.md:397) = 15-50%
   - [`docs/supply_side_product_design_v1_2026-03-18.md:136`](/home/long/project/立交桥/docs/supply_side_product_design_v1_2026-03-18.md:136) = 15-30%

3. **S0 周期冲突**：
   - [`docs/llm_gateway_subapi_evolution_plan_v4_1_2026-03-18.md:110`](/home/long/project/立交桥/docs/llm_gateway_subapi_evolution_plan_v4_1_2026-03-18.md:110) = 12周（至 2026-06-08）
   - [`docs/supply_side_product_design_v1_2026-03-18.md:280`](/home/long/project/立交桥/docs/supply_side_product_design_v1_2026-03-18.md:280) = 至 2026-04-30

### 影响

1. 商业测算、排期、人力分配无法统一。
2. 验收时“同项多口径”会导致争议。

### 建议

1. 将 `supply_side_product_design` 标记为“历史草稿/仅参考”或同步修正到 SSOT。
2. 以 `acceptance_gate_single_source` + `v4.1` 作为唯一定义来源。

---

## 2.5 P1-02：主技术栈宣称 PostgreSQL，但多处 SQL 仍为 MySQL 方言

### 证据

- [`docs/supply_detailed_design_v1_2026-03-18.md:193`](/home/long/project/立交桥/docs/supply_detailed_design_v1_2026-03-18.md:193) `AUTO_INCREMENT`
- [`docs/supply_detailed_design_v1_2026-03-18.md:231`](/home/long/project/立交桥/docs/supply_detailed_design_v1_2026-03-18.md:231) `ON UPDATE CURRENT_TIMESTAMP`
- [`docs/technical_architecture_design_v1_2026-03-18.md:821`](/home/long/project/立交桥/docs/technical_architecture_design_v1_2026-03-18.md:821) `ON UPDATE CURRENT_TIMESTAMP`
- [`docs/security_api_key_vulnerability_analysis_v1_2026-03-18.md:245`](/home/long/project/立交桥/docs/security_api_key_vulnerability_analysis_v1_2026-03-18.md:245) `AUTO_INCREMENT`

### 影响

1. 文档驱动开发会直接生成不可执行 DDL。
2. 安全与账务演练无法在目标数据库复现。

### 建议

1. 建立 `sql/postgres/` 唯一 DDL 仓，并在文档引用该目录。
2. CI 增加 SQL 方言 lint（禁止 MySQL 关键字进入 PostgreSQL 文档）。

---

## 2.6 P1-03：安全方案标准未统一（MD5 旧方案与 HMAC 新方案并存）

### 证据

- [`docs/security_api_key_vulnerability_analysis_v1_2026-03-18.md:176`](/home/long/project/立交桥/docs/security_api_key_vulnerability_analysis_v1_2026-03-18.md:176) 使用 `MD5` 截断做 user_hash。
- [`docs/security_api_key_vulnerability_analysis_v1_2026-03-18.md:188`](/home/long/project/立交桥/docs/security_api_key_vulnerability_analysis_v1_2026-03-18.md:188) `MD5` 校验和。
- [`docs/security_solution_v1_2026-03-18.md:398`](/home/long/project/立交桥/docs/security_solution_v1_2026-03-18.md:398) 升级为 `HMAC-SHA256`。

### 影响

1. 开发团队不清楚该实现哪个版本，易引入弱实现。
2. 安全评审结论不可复现。

### 建议

1. 明确“已废弃”文档并打上禁用标记。
2. 安全标准集中到一个 `security_ssot.md`，其余文档仅引用。

---

## 2.7 P1-04：门禁与交付物可验证性不足（大量证据路径不存在）

### 证据

- 任务单引用了 `tests/`、`evidence/`、`docs/compat/` 等产物路径：
  - [`docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md:50`](/home/long/project/立交桥/docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md:50)
  - [`docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md:148`](/home/long/project/立交桥/docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md:148)
- 当前仓库未发现这些目录：
  - [`/home/long/project/立交桥`](/home/long/project/立交桥) 仅含 `docs/`、`review/`、`llm-gateway-competitors/`
  - `tests/`、`evidence/` 不存在

### 影响

1. 无法对“已完成/可发布”进行客观验收。
2. 现阶段更像“方案库”，不是“可执行工程仓”。

### 建议

1. 拆分“规划仓”与“实施仓”，或在当前仓新增真实工程目录。
2. 所有 Gate 必须附可复跑证据脚本和执行结果。

---

## 3. 本次审查对“短期 token 只可分享给平台”的专项结论

### 3.1 结论

**当前文档基线不满足该约束。**

### 3.2 必做改造（短期策略修订）

1. 将“需求方购买供应方套餐”改为“需求方购买平台标准化服务套餐”。
2. 去除“需求方获取 API Key（来源于供应侧共享）”语义，统一为平台颁发的租户级调用凭证。
3. 所有“供应方收益分账”先降级为“平台采购结算”，不对终端用户暴露供应方身份与份额。
4. 验收门禁新增硬指标：
   - `direct_user_token_share_count = 0`
   - `consumer_receives_supplier_credential = false`

---

## 4. 建议整改顺序（72小时内）

1. **24小时内**：冻结并修订 SSOT（v4.1 + 验收门禁），删除/降级冲突条目。
2. **48小时内**：输出法务签署版“可做/不可做矩阵”并回填 ToS 引擎规则。
3. **72小时内**：补齐至少一套可执行证据（DDL校验、门禁脚本、回归记录）后再发起 GO 评审。

---

## 5. 复审准入条件（建议）

满足以下全部条件后，才建议从 `NO-GO` 转为 `CONDITIONAL GO`：

1. 短期策略已统一为“仅对平台共享”并完成全量文档替换。
2. 法务确认项全部闭环并有签署证据。
3. 关键口径（折扣、毛利、周期、SQL方言）单一来源且无冲突。
4. 至少一个端到端 Gate 具备可执行证据链（脚本 + 日志 + 报告）。

---

## 6. 评审范围说明

1. 本次未对 `llm-gateway-competitors/` 中第三方开源项目做深度代码审计（规模过大且非当前自研基线）。
2. 本次结论基于项目仓内可见文档和可验证产物；对外部系统状态不做推断。
