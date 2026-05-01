# Supply Intelligence 测试设计方案

> 版本：v1.0
> 日期：2026-04-27
> 状态：初稿
> 覆盖：AC-01 ~ AC-12、异常/边缘流程 FP-01 ~ FP-10、场景 S1~S4

---

## 1. 测试策略

### 1.1 测试分层模型

```
┌─────────────────────────────────────────────────┐
│                   E2E Tests (黑盒)               │
│  场景：从探针调度到状态变更、从发现到上架全链路   │
│  工具：Go test + httptest + 自制 E2E runner     │
└─────────────────────────────────────────────────┘
                        ▲
┌─────────────────────────────────────────────────┐
│             Integration Tests (灰盒)             │
│  场景：Service 间协作、异步任务队列、外部 API Mock│
│  工具：Go test + testify + sqlmock + gock       │
│  覆盖率门槛：service ≥ 80%, handler ≥ 80%       │
└─────────────────────────────────────────────────┘
                        ▲
┌─────────────────────────────────────────────────┐
│                Unit Tests (白盒)                 │
│  场景：状态机逻辑、探针评估、风险评分计算          │
│  工具：Go test + testify + gomock              │
│  覆盖率门槛：domain ≥ 70%                       │
└─────────────────────────────────────────────────┘
```

### 1.2 测试通过标准

| 维度 | 标准 |
|------|------|
| 覆盖率 | domain ≥ 70%, service/handler ≥ 80% |
| 模块 A（探针） | AC-01 ~ AC-03 全部通过 |
| 模块 B（发现） | AC-04 ~ AC-05 全部通过 |
| 模块 C（准入测试） | AC-06 ~ AC-07 全部通过 |
| 模块 D（自动注册） | AC-08 ~ AC-09 全部通过 |
| 模块 E（工作台） | AC-10 ~ AC-12 全部通过 |
| 异常/边缘流程 | FP-01 ~ FP-10 全部有验证测试 |
| 误报率 | 7 天连续运行 false positive ≤ 1% |

### 1.3 外部依赖 Mock

| 依赖 | Mock 方案 | 工具 |
|------|---------|------|
| **供应商 API（探针目标）** | Mock server 返回 200/401/403/429/500 | gock |
| **供应商模型列表 API** | Mock 返回 JSON 模型列表 | gock |
| **供应商注册接口** | Mock 返回注册成功/400/500 | gock |
| **SMS/邮件网关** | Mock server 接收验证码 | httptest |
| **KMS 服务** | Mock 加密/解密逻辑 | 接口层 Mock |
| **Job Scheduler（Temporal）** | 使用 Temporal test suite | temporalio/test-sdk |
| **supply-api 数据库** | sqlmock 拦截读写 | go-sqlmock |

---

## 2. 模块 A 测试用例（供应商品质探针）

### AC-01 探针覆盖度

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TA-01-01 | 15 分钟内探针覆盖率 ≥99% | Functional | Given 100 条 active/suspended 账号 When 15min 后统计 Then ≥99 条被探针 |
| TA-01-02 | suspended 账号同等探针 | Functional | Given suspended 账号 When 探针执行 Then 同样被覆盖 |
| TA-01-03 | 暂停探针账号不被覆盖 | Edge | Given 账号设置 pause_probe=true When 探针执行 Then 该账号被跳过 |

### AC-02 状态变更正确性

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TA-02-01 | active → suspended（1次401） | Happy Path | Given active 账号 When 连续 1 次返回 401 Then 60s 内状态变为 suspended |
| TA-02-02 | suspended → disabled（连续3次401） | Happy Path | Given suspended 账号 When 连续 3 次返回 401 Then 60s 内状态变为 disabled |
| TA-02-03 | 429 单次不改变状态 | Edge | Given active 账号 When 返回 429 一次 Then 15min 内状态保持 active |
| TA-02-04 | 指数退避重试逻辑 | Functional | Given 返回 429 When 探针执行 Then 按 1→2→4min 退避重试 |
| TA-02-05 | 状态机不允许 active→disabled 直变 | Edge | Given active 账号 When 连续 3 次失败 Then 不会直接变为 disabled（必须先 suspended） |
| TA-02-06 | 手动暂停账号状态不自动变更 | Edge | Given 账号 pause_probe=true When 供应商返回异常 Then 状态不变 |

### AC-03 误报率

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TA-03-01 | 7 天误报率 ≤1% | Long Run | Given 100 条正常账号 When 连续运行 7 天 Then 误变更次数 ≤7 |
| TA-03-02 | 探针与手动操作并发 | Concurrency | Given 手动修改状态的同时 When 探针执行 Then 乐观锁冲突处理正确 |

---

## 3. 模块 B 测试用例（全网模型发现）

### AC-04 新模型发现延迟

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TB-04-01 | 新模型在 2 扫描周期内被发现 | Functional | Given 供应商新增 model_id When 扫描执行 Then 2h 内 model_candidates 出现 discovered 记录 |
| TB-04-02 | 模型比对去重正确 | Functional | Given 已存在的 active model When 全网扫描 Then 不会重复创建 candidate |
| TB-04-03 | 模型下架告警触发 | Functional | Given active package 对应的 model_id 从供应商列表消失 When 2 扫描周期后 Then 运营工作台出现下架告警 |

### AC-05 已下架模型告警

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TB-05-01 | 下架模型不自动变更 package 状态 | Edge | Given model_id 消失 When 扫描执行 Then package 状态保持 active，生成告警 |
| TB-05-02 | 分页获取完整模型列表 | Functional | Given 供应商返回分页 When 扫描 Then 正确处理所有分页数据 |

---

## 4. 模块 C 测试用例（模型准入测试）

### AC-06 准入测试通过

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TC-06-01 | discovered → test_passed + 草稿生成 | Happy Path | Given discovered candidate When 测试全部通过 Then 状态 test_passed，supply_package 草稿生成 |
| TC-06-02 | 草稿字段完整性 | Functional | Given 草稿生成 When 检查字段 Then platform/model/price/suggested 正确 |
| TC-06-03 | 准入测试 30 分钟内完成 | Performance | Given discovered candidate When 测试执行 Then ≤30min 完成 |

### AC-07 准入测试失败

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TC-07-01 | discovered → test_failed | Negative | Given discovered candidate When 测试返回 500 Then 30min 内状态 test_failed，failure_reason 非空 |
| TC-07-02 | 超时视为失败 | Edge | Given 测试用例 60s 无响应 When Then 整体标记为 test_failed，reason = timeout |
| TC-07-03 | 测试账号 suspended 时任务失败 | Edge | Given 测试账号变为 suspended When 准入测试执行 Then 任务标记 test_failed，reason = test_account_unavailable |
| TC-07-04 | ignore 账号 7 天内不重扫 | Edge | Given 运营标记 ignore When 7 天内扫描 Then 该 candidate 不出现 |

---

## 5. 模块 D 测试用例（账号自动注册）

### AC-08 自动注册成功

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TD-08-01 | 账号数 < 阈值时触发注册 | Functional | Given 可用账号数 < 阈值 When 系统检测 Then 10min 内触发注册流程 |
| TD-08-02 | 注册完成 → active | Happy Path | Given 注册流程执行 When 完成 Then 30min 内 supply_accounts 出现 active 记录 |
| TD-08-03 | 凭证 KMS 加密存储 | Security | Given 注册成功 When 检查数据库 Then 凭证字段为密文，无明文 |
| TD-08-04 | 注册结果关联 task | Functional | Given 注册任务完成 When Then auto_registration_tasks 状态为 completed |

### AC-09 自动注册 fail-closed

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TD-09-01 | SMS 网关不可用时 fail-closed | Resilience | Given SMS 网关返回 503 When 注册执行 Then 60s 内任务 failed，审计日志完整，无虚假成功 |
| TD-09-02 | 注册接口返回 400 | Edge | Given 邮箱已注册 When 注册执行 Then 任务 failed，不重试同一邮箱 |
| TD-09-03 | KMS 不可用时 fail-closed | Resilience | Given KMS 超时 When 加密步骤 Then 60s 内任务 failed，明文凭证不出现在日志/DB |

---

## 6. 模块 E 测试用例（运营工作台）

### AC-10 审计日志完整性

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TE-10-01 | 状态变更 5s 内写入审计 | Performance | Given 状态变更 When 执行完成 Then ≤5s 审计记录存在 |
| TE-10-02 | 审计字段完整性 | Functional | Given 审计记录 When 检查 Then 包含 object_type/id/action/before_state/after_state/request_id |
| TE-10-03 | 探针执行记录审计 | Functional | Given 探针执行 When 完成 Then probe_execution_logs 有记录 |

### AC-11 运营工作台干预

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TE-11-01 | 确认上架 draft → active | Happy Path | Given draft package When 点击确认 Then 3s 内变为 active |
| TE-11-02 | 忽略模型 7 天内不出现 | Edge | Given 点击忽略 When Then 7 天内 candidate 不出现在待处理列表 |
| TE-11-03 | 手动触发单账号探针 | Functional | Given 运营手动触发 When Then 立即执行探针，结果可见 |
| TE-11-04 | 并发操作冲突处理 | Concurrency | Given 同时点击确认和忽略 When Then 返回 409，只一个生效 |

### AC-12 配置热更新

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TE-12-01 | 探针周期修改 60s 内生效 | Functional | Given 修改探针周期 When 下发配置 Then 60s 后新周期生效 |

---

## 7. 异常/边缘流程测试（FP-01 ~ FP-10）

| 用例 ID | 场景 | 验证点 | 预期行为 |
|---------|------|-------|---------|
| TFP-01 | 供应商探针 DNS/TCP 超时 | 状态不变 | 标记 inconclusive，指数退避，不触发状态变更 |
| TFP-02 | 供应商返回空/格式突变 | 状态不变 | 解析失败标记 inconclusive，记录日志 |
| TFP-03 | 探针与手动操作并发 | 乐观锁 | 更新失败，探针记录冲突日志，下次覆盖 |
| TFP-04 | 准入测试期间测试账号 suspended | 任务标记失败 | 任务标记 test_failed，reason = test_account_unavailable |
| TFP-05 | 注册接口返回 400（邮箱已注册） | 任务失败 | 任务 failed，同一邮箱不重试，审计记录完整 |
| TFP-06 | 注册成功但验证失败 | pending 不变 | 账号保持 pending，任务标记 verify_failed，触发告警 |
| TFP-07 | 供应商模型列表分页 500 | 整体不中断 | 已获取部分正常处理，失败页下次重试 |
| TFP-08 | 探针期间数据库不可用 | 任务失败重试 | 探针任务失败，连续 5 次失败后暂停批次，触发系统告警 |
| TFP-09 | 确认上架与忽略并发 | 409 冲突 | 只有一个生效，返回 409 |
| TFP-10 | KMS 不可用时注册 | 明文不落盘 | 加密步骤阻塞/失败，明文凭证不出现 |

---

## 8. 灰度发布验证计划

### 8.1 各 Phase 验证内容

| Phase | 交付内容 | 通过标准 | 依赖项 |
|-------|---------|---------|--------|
| **Phase 1** | 模块 A（探针）+ 模块 E 只读视图 | AC-01~AC-03, AC-10~AC-11（只读部分） | Temporal 调度器 |
| **Phase 2** | 模块 B（发现）+ 模块 C（准入测试） | AC-04~AC-07 | Phase 1 + 供应商 API 清单 |
| **Phase 3** | 模块 D（自动注册）+ 模块 E 完整 | AC-08~AC-12 | Phase 1+2 + KMS/SMS 就绪 |

### 8.2 灰度门禁

每次 Phase 升级前：
- [ ] 全部 AC 测试用例通过
- [ ] 覆盖率达标
- [ ] 灰度开关独立验证（每个开关可单独打开/关闭）
- [ ] 回滚条件演练（误报率>5% / 状态变更导致错误率上升>2%）

---

## 9. 回归测试集

### 9.1 快速回归（每次 PR，~10 分钟）

```
TA-01-01, TA-02-01, TA-02-02, TA-02-05,
TB-04-01, TC-06-01, TC-07-01,
TD-08-01, TD-09-01,
TE-10-01, TE-11-01
共 11 条
```

### 9.2 完整回归（Phase 升级，~45 分钟）

```
TA-01-01 ~ TA-03-02（全 8 条）
TB-04-01 ~ TB-05-02（全 4 条）
TC-06-01 ~ TC-07-04（全 4 条）
TD-08-01 ~ TD-09-03（全 4 条）
TE-10-01 ~ TE-12-01（全 7 条）
TFP-01 ~ TFP-10（全 10 条）
共 37 条
```

---

## 10. 技术栈与集成约束验证

### 10.1 统一技术栈与双运行模式验证

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TSI-RUN-01 | 独立运行模式启动 | Happy Path | Given 独立 `config.yaml` 与独立数据库/Redis When 启动 `cmd/supply-intelligence/main.go` Then `/actuator/health/ready` 返回 200，`/api/v1/supply-intelligence/*` 可访问 |
| TSI-RUN-02 | 集成运行模式挂载 | Integration | Given supply-api 主进程加载 `IntegrationPlugin` When 启动 Then `/internal/supply-intelligence/*` 路由与后台任务注册成功 |
| TSI-RUN-03 | 配置分离加载 | Functional | Given 独立模式与集成模式分别启动 When 读取配置 Then 独立模式只加载自身配置，集成模式合并主项目配置且不覆盖无关模块 |
| TSI-RUN-04 | 数据库前缀隔离 | Structural | Given 执行迁移 When 检查 schema Then 仅创建 `supply_intelligence_` 前缀表 |

### 10.2 独立运行与集成运行验证

### 10.3 IntegrationPlugin 与模块挂载验证

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TSI-PLG-01 | IntegrationPlugin 注册 HTTP 路由 | Integration | Given 集成模式 When 插件注册 Then Probe/Discovery/Admission/AutoReg/OpsWorkBench 路由挂载成功 |
| TSI-PLG-02 | 模块开关生效 | Functional | Given `enabled_modules` 关闭某模块 When 启动 Then 对应路由/worker 不注册，其他模块可用 |
| TSI-PLG-03 | 集成模式共享资源 | Integration | Given supply-api 注入共享 DB/Redis/logger When 插件启动 Then 使用共享资源且不重复初始化冲突依赖 |

### 10.3 OpenAPI 契约验证

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TSI-OAS-01 | OpenAPI 文档可访问 | Functional | Given 服务启动 When 请求 `/openapi.json` 或 `/docs` Then 返回 200 且包含探针、发现、准入测试、运营工作台接口 |
| TSI-OAS-02 | 路由与 OpenAPI 一致 | Contract | Given 导出的 OpenAPI 文档 When 对照 HTTP 路由 Then 请求/响应/错误码与实现一致，无缺失公开接口 |
| TSI-OAS-03 | 集成前缀可配置 | Contract | Given 集成模式配置内部前缀 When 导出文档 Then 文档反映 `/internal/supply-intelligence/` 前缀或明确区分暴露面 |

### 10.4 NewAPI / Sub2API 适配层验证

| 用例 ID | 描述 | 类型 | 验证条件 |
|---------|------|------|---------|
| TSI-ADP-01 | 供应商状态同步适配 | Contract | Given NewAPI/Sub2API 拉取供应商状态 When 调用标准化接口 Then 返回字段稳定、延迟满足约束、状态映射正确 |
| TSI-ADP-02 | 模型列表推送适配 | Contract | Given 外部系统拉取模型列表 When 调用 `/models` Then 只返回已发现且允许暴露的数据，字段与约定一致 |
| TSI-ADP-03 | 账号注册适配 | Integration | Given 自动注册模块调用外部账号管理 API When 通过适配层执行 Then 鉴权、错误映射、幂等行为符合契约 |

---

## 11. 发布门禁与阶段结论

### 11.1 发布门禁检查表

以下门禁项全部通过前，不得认定达到生产要求：

- [ ] 独立运行 / 集成运行两种模式均完成启动验证，路由、worker、内部接口真实挂载
- [ ] `IntegrationPlugin`、OpenAPI、NewAPI/Sub2API 适配层合同测试全部通过
- [ ] 凭证保护经日志/DB/异常路径验证无明文，KMS 不可用时 fail-closed
- [ ] 自动注册链路具备频控、审批开关、重复提交阻断与审计留痕
- [ ] 状态机迁移、审计写入、Gateway/外部同步链路完成一致性验证
- [ ] 首次生产放量场景遵循“只告警不自动变更状态”，并验证撤销与人工接管流程
- [ ] 调度器失效、浏览器自动化失效、外部适配越权、错误状态传播四类高风险回归通过
- [ ] 至少一条探针、一条模型发现、一条准入测试、一条自动注册链路完成端到端验证

### 11.2 阶段门控结论

**当前结论：REQUEST_CHANGES**

**进入开发/实现前必须补齐：**
- 将 HLD 中的威胁建模点映射为显式测试与阻断项，尤其是凭证保护、状态传播、自动注册、外部适配。
- 为“定义 → 装配 → 调用 → 入口”四层链路补充 QA 检查要求，覆盖探针、发现、准入、注册、运营干预。
- 明确独立运行与集成运行的最小验证命令、预期输出与失败判定。

**阻断条件：**
- 凭证保护不能证明 fail-closed。
- 状态同步和审计写入无法形成可追踪闭环。
- 无法证明五条主链路真实接入运行主链路。

---

## 12. 性能与安全测试

### 12.1 性能基准

| 指标 | 目标值 | 测试方法 |
|------|-------|---------|
| 探针执行（单账号） | <2s | 计时 1000 次取 P99 |
| 全网扫描（10 供应商） | <5min | 从调度触发到完成计 |
| 准入测试（5 用例） | <30min P99 | 从 discovered 到 test_passed/failed |
| 供应商状态查询 API | <50ms P99 | 并发 100 请求 |
| 审计日志写入 | <1s P99 | 单次变更后计时 |

### 12.2 安全测试

| 测试项 | 方法 | 验证 |
|-------|------|------|
| 凭证明文保护 | 检查日志/DB/内存 dump | 无明文凭证 |
| KMS 密钥轮换 | Mock KMS 不可用 | fail-closed，不暴露明文 |
| 供应商 API 限流绕过 | 连续探针超限 | 正确触发 rate limit |
| 注册接口重复提交 | 并发同一邮箱注册 | 只有一次成功，其余 failed |
