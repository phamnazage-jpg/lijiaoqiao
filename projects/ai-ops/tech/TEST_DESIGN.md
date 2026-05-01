# AI-Ops 测试设计方案

> 版本：v1.0
> 日期：2026-04-27
> 状态：初稿
> 覆盖：AC-01 ~ AC-12、异常流程 F-01 ~ F-08、边缘流程 G ~ I

---

## 1. 测试策略

### 1.1 测试分层模型

```
┌─────────────────────────────────────────────────┐
│                   E2E Tests (黑盒)               │
│  场景：用户操作链路 + 系统集成验证                 │
│  工具：Go test + k6 / 自制 E2E runner           │
│  覆盖率目标：每个主流程 ≥ 1 条                    │
└─────────────────────────────────────────────────┘
                        ▲
┌─────────────────────────────────────────────────┐
│               Integration Tests (灰盒)             │
│  场景：Service 间协作、数据库读写、外部 API Mock   │
│  工具：Go test + testify + sqlmock + httptest   │
│  覆盖率门槛：service ≥ 80%, handler ≥ 80%        │
└─────────────────────────────────────────────────┘
                        ▲
┌─────────────────────────────────────────────────┐
│                Unit Tests (白盒)                 │
│  场景：单个函数/方法逻辑、边界条件、错误分支       │
│  工具：Go test + testify + gomock              │
│  覆盖率门槛：domain ≥ 70%                       │
└─────────────────────────────────────────────────┘
```

### 1.2 测试通过标准

| 维度 | 标准 |
|------|------|
| 覆盖率 | domain ≥ 70%, service/handler ≥ 80% |
| 主流程 | AC-01 ~ AC-12 全部有至少 1 条通过测试 |
| 异常流程 | F-01 ~ F-08 全部有至少 1 条验证测试 |
| 边缘流程 | G、H、I 全部有至少 1 条验证测试 |
| 告警噪声率 | 沙盒测试中误报率 ≤ 1%，超过则 CI 失败 |
| 自愈误触发 | 沙盒测试中 0 次误触发，否则 CI 失败 |

### 1.3 测试环境矩阵

| 环境 | 用途 | 数据特征 | 外部依赖 |
|------|------|---------|---------|
| **Local Dev** | 开发者快速验证 | Mock 数据 | Mock 所有外部服务 |
| **CI** | PR Merge 门禁 | Mock 数据 | Mock 所有外部服务 |
| **Sandbox** | 沙盒验证（自愈规则） | 生产数据脱敏副本 | Mock + 部分真实依赖 |
| **Staging** | 上线前全流程验证 | 生产数据脱敏副本 | 全真实依赖 |
| **Production** | 灰度上线 | 真实数据 | 全真实依赖 |

---

## 2. Mock 策略

### 2.1 外部依赖 Mock

| 依赖 | Mock 方案 | 工具 |
|------|---------|------|
| **Prometheus / 时序数据库** | 嵌入式 mock server，返回预置指标数据 | httptest + 自定义 mock |
| **gateway/internal/metrics** | Mock HTTP handler，返回 JSON 指标 | gock / httptest |
| **supply-api/ 供应商健康接口** | Mock 返回 200/401/429/500 | gock |
| **platform-token-runtime/ 运行时状态接口** | Mock 返回正常/异常状态 | gock |
| **通知渠道（Webhook/邮件/飞书）** | Mock server 接收并验证请求格式 | httptest |
| **PostgreSQL** | sqlmock 拦截 SQL，验证查询正确性 | github.com/DATA-DOG/go-sqlmock |
| **Redis** | miniredis 内存模拟 | github.com/alicebob/miniredis |

### 2.2 Mock 分层

```
Production 依赖:
  gateway metrics API ──→ supply-api 供应商接口 ──→ token-runtime 状态接口
         │                       │                        │
         ▼                       ▼                        ▼
Mock (CI/Local):         Mock (CI/Local):          Mock (CI/Local):
MetricsMockServer    →   SupplierMockServer    →   RuntimeMockServer
```

---

## 3. 测试用例矩阵（按 AC 编号）

### AC-01 实时监控看板

| 用例 ID | 描述 | 类型 | 覆盖条件 |
|---------|------|------|---------|
| TC-01-01 | 首页加载时间 <2s | Performance | Given 用户登录 When 访问首页 Then 响应时间 ≤2s |
| TC-01-02 | 首页显示 6 个指标 | Happy Path | Given 系统运行 When 首页加载 Then 显示 QPS/延迟/P99/错误率/供应商数/告警数 |
| TC-01-03 | 指标卡片 15s 内刷新 | Functional | Given 指标更新 When 数据推送 Then 15s 内页面刷新 |
| TC-01-04 | 无数据时看板展示"无数据" | Edge | Given 指标源断开 When 首页加载 Then 不显示过期数据 |

### AC-02 指标下钻

| 用例 ID | 描述 | 类型 | 覆盖条件 |
|---------|------|------|---------|
| TC-02-01 | 下钻显示 1 小时趋势图 | Happy Path | Given 点击指标卡片 When 下钻 Then 显示 60min 趋势 |
| TC-02-02 | 按 service/path/supplier 维度分割 | Functional | Given 趋势图 When 按 supplier 下钻 Then 正确分割 |
| TC-02-03 | 下钻查询 <3s | Performance | Given 大数据量 When 执行下钻 Then 响应 <3s |
| TC-02-04 | 无数据范围返回空图表 | Edge | Given 无数据 When 下钻 Then 显示空图表而非报错 |

### AC-03 告警规则配置

| 用例 ID | 描述 | 类型 | 覆盖条件 |
|---------|------|------|---------|
| TC-03-01 | 创建告警规则 | Happy Path | Given 登录管理员 When 创建规则 Then 规则保存成功 |
| TC-03-02 | 规则字段完整性校验 | Negative | Given 缺少必填字段 When 创建规则 Then 返回 400 |
| TC-03-03 | 规则变更 30s 内生效 | Functional | Given 规则已创建 When 修改阈值 Then 30s 后新规则生效 |
| TC-03-04 | 支持 50 条规则并发运行 | Load | Given 50 条规则 When 同时触发 Then 全部正确评估 |
| TC-03-05 | 规则编辑/禁用/删除 | Functional | Given 规则存在 When 编辑/禁用/删除 Then 状态正确变更 |

### AC-04 告警通知触达

| 用例 ID | 描述 | 类型 | 覆盖条件 |
|---------|------|------|---------|
| TC-04-01 | P0/P1 告警 30s 内通知 | Performance | Given P1 告警触发 When 通知发送 Then ≤30s 到达 |
| TC-04-02 | P2 告警 120s 内通知 | Performance | Given P2 告警触发 When 通知发送 Then ≤120s 到达 |
| TC-04-03 | 至少 2 种通知渠道 | Functional | Given 告警触发 When 发送 Then 飞书和邮件均收到 |
| TC-04-04 | 通知内容完整性 | Functional | Given 告警发送 Then 包含级别/规则名/时间/当前值/阈值/事件ID/链接 |
| TC-04-05 | Webhook 通知失败后自动切换 | Resilience | Given Webhook 发送失败 When 告警触发 Then 自动切换至邮件 |

### AC-05 告警聚合与抑制

| 用例 ID | 描述 | 类型 | 覆盖条件 |
|---------|------|------|---------|
| TC-05-01 | 1 分钟内 >20 条告警触发聚合 | Functional | Given 同一资源 1min 内触发 25 条 When 聚合 Then 生成 1 条集群告警 |
| TC-05-02 | 集群告警包含累计数量和规则列表 | Functional | Given 集群告警生成 Then 内容包含数量≥20 和规则列表 |
| TC-05-03 | 5 分钟抑制期内同一规则不重复通知 | Functional | Given 告警已发送 When 5min 内再次触发 Then 不重复通知 |
| TC-05-04 | 级别升级时抑制解除 | Functional | Given P2 告警抑制中 When 升级为 P1 Then 立即通知 |

### AC-06 自动自愈

| 用例 ID | 描述 | 类型 | 覆盖条件 |
|---------|------|------|---------|
| TC-06-01 | 自愈动作 60s 内完成 | Performance | Given 自愈规则触发 When 执行切换路由 Then ≤60s 完成含重试 |
| TC-06-02 | 自愈成功记录事件 | Happy Path | Given 自愈执行成功 When 完成 Then 事件记录 success |
| TC-06-03 | 自愈失败升级 P0 人工告警 | Functional | Given 自愈重试均失败 When 停止 Then 升级 P0 通知 |
| TC-06-04 | 无自愈规则时仅通知 | Functional | Given 告警无自愈配置 When 触发 Then 仅发送通知 |
| TC-06-05 | 沙盒模式：自愈不生效 | Resilience | Given 沙盒模式 When 自愈触发 Then 仅记录，不实际执行 |
| TC-06-06 | 自愈后 2min 评估是否解除 | Functional | Given 自愈执行 When 2min 后 Then 评估条件是否满足 |
| TC-06-07 | 自愈级联失败回退 | Functional | Given 自愈切换导致新故障 When 检测到 Then 回退并升级 |

### AC-07 配置审计日志

| 用例 ID | 描述 | 类型 | 覆盖条件 |
|---------|------|------|---------|
| TC-07-01 | 配置变更 1s 内生成审计记录 | Performance | Given 执行配置变更 When 完成 Then ≤1s 审计记录存在 |
| TC-07-02 | 审计字段完整性 | Functional | Given 审计记录 When 查询 Then 包含全部 10 个字段 |
| TC-07-03 | 审计日志不可篡改 | Security | Given 审计记录 When 尝试修改 Then 数据库层拒绝或被检测 |
| TC-07-04 | 审计日志 90 天保留 | Functional | Given 审计数据 91 天 When 查询 Then 91 天前记录不存在（新数据已清理） |
| TC-07-05 | 审计查询 <3s | Performance | Given 10000 条审计记录 When 按条件查询 Then <3s |

### AC-08 配置回滚

| 用例 ID | 描述 | 类型 | 覆盖条件 |
|---------|------|------|---------|
| TC-08-01 | 正常回滚 <60s | Performance | Given 审计记录存在 When 执行回滚 Then ≤60s 完成 |
| TC-08-02 | 回滚前显示子资源影响列表 | Functional | Given 回滚操作 When 执行前 Then 显示将被覆盖的子资源 |
| TC-08-03 | 回滚生成新审计记录 | Functional | Given 回滚执行 When 完成 Then 新审计记录关联原始 ID |
| TC-08-04 | 目标不存在时返回 AUDIT_ROLLBACK_TARGET_LOST | Negative | Given 目标已被删除 When 执行回滚 Then 返回错误码且不执行 |
| TC-08-05 | 回滚失败不静默 | Resilience | Given 回滚执行失败 When 完成 Then 返回错误码并通知 |

### AC-09 容量主板

| 用例 ID | 描述 | 类型 | 覆盖条件 |
|---------|------|------|---------|
| TC-09-01 | 显示 7 天趋势数据 | Functional | Given 容量主板 When 加载 Then 显示 7 天 Token/QPS/延迟趋势 |
| TC-09-02 | 负载等级标注（正常/警告/过载） | Functional | Given 负载数据 When 展示 Then 正确标注等级 |
| TC-09-03 | 预测触达上限时间 | Functional | Given 增长率数据 When 计算 Then 显示预测时间（仅供参考） |

### AC-10 日志/指标查询

| 用例 ID | 描述 | 类型 | 覆盖条件 |
|---------|------|------|---------|
| TC-10-01 | 按多维度筛选日志 | Functional | Given 查询条件 When 执行 Then 正确过滤 |
| TC-10-02 | 日志查询 <3s | Performance | Given 10000 条日志 When 查询 Then <3s |
| TC-10-03 | CSV 导出 10000 条 | Load | Given 查询结果 When 导出 Then 正确生成 CSV |
| TC-10-04 | 分页查询第 2 页 | Functional | Given 分页请求 When 获取第 2 页 Then 返回正确偏移 |

### AC-11 监控数据保存

| 用例 ID | 描述 | 类型 | 覆盖条件 |
|---------|------|------|---------|
| TC-11-01 | 原始数据保留 ≥7 天 | Functional | Given 8 天前数据 When 查询 Then 7 天内数据存在 |
| TC-11-02 | 分钟级聚合保留 ≥30 天 | Functional | Given 31 天前数据 When 查询 Then 31 天前不存在 |
| TC-11-03 | 小时级聚合保留 ≥90 天 | Functional | Given 91 天前数据 When 查询 Then 不存在 |

### AC-12 角色与权限

| 用例 ID | 描述 | 类型 | 覆盖条件 |
|---------|------|------|---------|
| TC-12-01 | 查看者只能读不可写 | Security | Given 查看者 When 尝试写操作 Then 返回 403 |
| TC-12-02 | 运维人员不可执行回滚 | Security | Given 运维人员 When 执行回滚 Then 返回 403 |
| TC-12-03 | 管理员可执行所有操作 | Functional | Given 管理员 When 执行任意操作 Then 成功 |

---

## 4. 异常流程测试（F-01 ~ F-08）

| 用例 ID | 异常场景 | 验证点 | 预期行为 |
|---------|---------|-------|---------|
| TF-01 | 自愈动作重试均失败 | P0 人工告警触发 | 10s 内重试 1 次，失败后立即升级 P0 电话/短信 |
| TF-02 | 通知渠道失效（Webhook 5xx） | 备用渠道切换 | 记录失败，使用邮件→飞书→短信 三次切换 |
| TF-03 | 回滚目标已不存在 | AUDIT_ROLLBACK_TARGET_LOST | 返回错误码，运营手动修复 |
| TF-04 | 指标采集器 5min 无数据 | 数据源丢失标识 | 控制台显示丢失标识，触发 P2 内部告警 |
| TF-05 | 审计日志存储满盘 | 降级不阻断业务 | 丢弃非关键字段或异步上报，业务操作继续 |
| TF-06 | 自愈形成级联故障 | 回退并升级 | 自动恢复上一步，升级人工告警，立即电话通知 |
| TF-07 | 监控数据库全面中断 | 只读/降级模式 | 控制台只读，告警引擎本地缓存继续运行 |
| TF-08 | 实时看板指标计算超时 | 显示上次结果 | 显示上次成功结果并标注时间戳 |

---

## 5. 灰度发布验证计划

### 5.1 各 Phase 验证内容

| Phase | 验证内容 | 通过标准 | 回归集 |
|-------|---------|---------|--------|
| **Phase 1** | 监控看板 + 日志查询 | AC-01, AC-02, AC-10, AC-11 全部通过 | 无历史功能 |
| **Phase 2** | 告警规则 + 通知渠道 | AC-03, AC-04, AC-05 全部通过 | Phase 1 全量 |
| **Phase 3** | 自愈引擎 + 审计回滚 | AC-06, AC-07, AC-08 全部通过 + 沙盒 10 次无误触发 | Phase 1+2 全量 |
| **Phase 4** | 容量主板 | AC-09 全部通过 | Phase 1+2+3 全量 |

### 5.2 灰度门禁检查项

每次 Phase 升级前必须全部通过：
- [ ] 所有 AC 测试用例 100% 通过
- [ ] 单元测试覆盖率达标（domain ≥70%, service ≥80%）
- [ ] 自愈沙盒验证 ≥10 次无误触发
- [ ] 回滚演练（至少 3 个资源类型）成功
- [ ] 性能基准测试通过（响应时间符合 AC 要求）
- [ ] 安全扫描通过（无高危漏洞）

---

## 6. 回归测试集

### 6.1 快速回归集（每次 PR）

```
TC-01-01, TC-01-02, TC-03-01, TC-03-03, TC-04-01, TC-07-01, TC-07-02, TC-12-01, TC-12-03
共 9 条，约 5-10 分钟
```

### 6.2 完整回归集（每次 Phase 升级）

```
TC-01-01 ~ TC-01-04
TC-02-01 ~ TC-02-04
TC-03-01 ~ TC-03-05
TC-04-01 ~ TC-04-05
TC-05-01 ~ TC-05-04
TC-06-01 ~ TC-06-07
TC-07-01 ~ TC-07-05
TC-08-01 ~ TC-08-05
TC-09-01 ~ TC-09-03
TC-10-01 ~ TC-10-04
TC-11-01 ~ TC-11-03
TC-12-01 ~ TC-12-03
TF-01 ~ TF-08
共 43 条，约 30-60 分钟
```

---

## 7. 技术栈与集成约束验证

### 7.1 统一技术栈与双运行模式验证

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TOPS-RUN-01 | 独立运行模式启动 | Happy Path | Given 独立 `config.yaml` 与独立数据库/Redis/时序库 When 启动 `cmd/ai-ops/main.go` Then `/actuator/health/ready` 返回 200，`/api/v1/ai-ops/*` 可访问 |
| TOPS-RUN-02 | 集成运行模式挂载 | Integration | Given gateway 或 supply-api 主进程加载 `IntegrationPlugin` When 启动 Then `/internal/ai-ops/*` 路由、后台 worker、健康检查挂载成功 |
| TOPS-RUN-03 | 配置分离加载 | Functional | Given 独立模式与集成模式分别启动 When 读取配置 Then 独立模式仅使用自身配置，集成模式正确合并主项目配置 |
| TOPS-RUN-04 | 数据库前缀隔离 | Structural | Given 执行迁移 When 检查 schema Then 仅创建 `ai_ops_` 前缀表 |

### 7.2 独立运行与集成运行验证

### 7.3 IntegrationPlugin 与模块挂载验证

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TOPS-PLG-01 | IntegrationPlugin 注册路由与健康检查 | Integration | Given 集成模式 When 插件注册 Then 监控、告警、日志、审计、健康检查路由挂载成功 |
| TOPS-PLG-02 | 模块开关生效 | Functional | Given `enabled_modules` 关闭某模块 When 启动 Then 对应路由/后台任务不注册，其他模块不受影响 |
| TOPS-PLG-03 | 集成模式共享资源 | Integration | Given 主进程注入共享 DB/Redis/logger/metrics client When 插件启动 Then 使用共享资源且不重复初始化冲突依赖 |

### 7.3 OpenAPI 契约验证

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TOPS-OAS-01 | OpenAPI 文档可访问 | Functional | Given 服务启动 When 请求 `/openapi.json` 或 `/docs` Then 返回 200 且包含监控、告警、自愈、审计、日志查询接口 |
| TOPS-OAS-02 | 路由与 OpenAPI 一致 | Contract | Given 导出的 OpenAPI 文档 When 对照 HTTP 路由 Then 请求/响应/错误码与实现一致，无缺失公开接口 |
| TOPS-OAS-03 | 集成前缀可配置 | Contract | Given 集成模式配置内部前缀 When 导出文档 Then 文档反映 `/internal/ai-ops/` 前缀或明确区分外部/内部暴露面 |

### 7.4 NewAPI / Sub2API 适配层验证

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TOPS-ADP-01 | `/metrics` 采集适配 | Contract | Given NewAPI/Sub2API 通过 Prometheus scrape 拉取指标 When 调用 `/metrics` Then 指标命名、label、采样频率满足契约 |
| TOPS-ADP-02 | 告警回调适配 | Integration | Given 外部系统配置 Webhook 回调 When 告警触发 Then 回调内容完整、签名正确、失败可重试 |
| TOPS-ADP-03 | 自愈脚本调用外部管理 API | Integration | Given 自愈动作触发程序化脚本 When 通过适配层调用 NewAPI/Sub2API Then 鉴权、错误码映射、回退逻辑符合设计 |

---

## 8. 发布门禁与阶段结论

### 8.1 发布门禁检查表

以下门禁项全部通过前，不得进入生产交付：

- [ ] 独立运行与集成运行模式均完成启动验证，路由、worker、健康检查真实挂载
- [ ] `BuildServer` / `BuildRuntime` 中条件能力已显式接入，而非仅存在定义
- [ ] OpenAPI、`/metrics`、Webhook、管理 API 的鉴权与字段边界合同测试通过
- [ ] 自愈动作均完成沙盒验证、快照记录与回滚演练
- [ ] 审计日志保证先写审计再执行业务，高风险操作审计失败即拒绝
- [ ] viewer / operator / admin 三类角色权限矩阵验证通过
- [ ] 告警洪泛、自愈误触发、时序库中断、通知渠道失效四类高风险回归全部通过
- [ ] 至少一条真实故障检测 → 告警 → 通知/回滚链路完成端到端验证

### 8.2 阶段门控结论

**当前结论：REQUEST_CHANGES**

**进入开发/实现前必须补齐：**
- 将 HLD 中的威胁建模点全部下沉为可执行测试与阻断项。
- 为“定义 → 装配 → 调用 → 入口”四层链路补充 QA 检查要求，重点覆盖自愈、告警、审计、权限。
- 分别给出独立模式与集成模式的最小验证命令、预期输出与失败判定。

**阻断条件：**
- 高风险动作没有沙盒/回滚闭环。
- 审计不能证明先写后执行业务。
- 关键能力只存在接口声明，未真实接入运行主链路。

---

## 9. 性能测试

### 9.1 性能基准

| 指标 | 目标值 | 压测方法 |
|------|-------|---------|
| 首页加载 | <2s (P99) | k6 并发 50 用户 |
| 告警触发到通知 | P0/P1 <30s, P2 <120s | 单次告警触发计时 |
| 下钻查询 | <3s (P99) | k6 并发 20 用户 |
| 审计查询 | <3s (P99) | 10000 条数据下查询 |
| 配置回滚 | <60s (P99) | 单次回滚计时 |
| 支持并发告警规则 | ≥50 条同时评估 | 并发注入 50 条告警数据 |

---

## 10. 安全测试

| 测试项 | 方法 | 验证点 |
|-------|------|-------|
| 权限越界 | 使用低权限 Token 尝试高权限操作 | 返回 403 |
| 审计日志篡改 | 尝试 UPDATE/DELETE 审计表 | 操作被拒绝或被检测 |
| SQL 注入 | 输入 `' OR 1=1 --` 等 | 参数化查询无注入 |
| 告警信息泄露 | 跨用户查询告警 | 无数据泄露 |
| 高风险变更未二次确认 | 提交影响 90% 流量的变更 | 变更被标记待确认 |
