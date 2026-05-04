# ai-customer-service P0/P1/P2 整改执行表

> 来源：`docs/RECTIFICATION_REVIEW_REPORT_V2.md`  
> 用途：按角色推动整改执行、跟踪状态、做阶段门禁验收  
> 当前总状态：**第5件事已完成；代码侧 P0 技术阻断已闭环，项目可进入预生产整改与联调阶段，但仍禁止按“生产可直接上线”口径放行**

---

## 0. 使用规则

- 状态仅允许：`未开始 / 进行中 / 已完成 / 已阻塞`
- 每项必须有：责任角色、交付物、验收标准、阻塞依赖
- 任何“已完成”必须附带文件证据或命令证据
- 未通过 Gate A 前，不得进入“可灰度”结论
- 未通过 Gate B 前，不得进入“可生产放量”结论

---

## 1. P0 整改执行表（上线前必须完成）

| ID | 优先级 | 整改项 | 责任角色 | 交付物 | 验收标准 | 依赖 | 状态 |
|---|---|---|---|---|---|---|---|
| XL-P0-1 | P0 | 建立“代码事实高于报告”的门禁，禁止无证据放行 | 小龙 | 更新后的阶段门禁说明/流程文档 | 所有“完成/通过”结论均附命令或文件证据 | 无 | 已完成 |
| XL-P0-2 | P0 | 重写项目状态口径，分离代码门禁/预生产门禁/生产门禁 | 小龙 | 状态基线文档或汇总页 | 不再使用单句“允许上线”覆盖全部阶段 | XL-P0-1 | 已完成 |
| PM-P0-1 | P0 | 修正文档中的上线口径，撤销过宽“允许上线”表述 | PM | 更新 `prd/PRODUCTION_CHECKLIST.md` 等文档 | 明确区分仓库内通过、真实环境未验证、仅可进入预生产 | XL-P0-2 | 已完成 |
| PM-P0-2 | P0 | 在文档中明确 `memory mode` 仅限 dev/test，prod 禁止无持久化运行 | PM | 更新 PRD/checklist/status 文档 | 文档明确写出 prod fail-fast 要求 | TL-P0-1 设计口径 | 已完成 |
| TL-P0-1 | P0 | 禁止 prod 默认退化为 memory store | TechLead | 代码改动 + 测试 | prod 下 `Postgres.Enabled=false` 启动失败；有测试覆盖 | 无 | 已完成 |
| TL-P0-2 | P0 | 收紧 readiness，改为真实依赖门禁 | TechLead | 代码改动 + 集成测试 | prod 缺关键配置时启动失败；非 prod memory 不再被误伤；ready 语义与实际运行模式一致 | TL-P0-1 | 已完成 |
| TL-P0-3 | P0 | 输出代码视角配置契约基线 | TechLead | 配置契约文档 | 与 `internal/config/config.go` 完全一致 | 无 | 已完成 |
| QA-P0-1 | P0 | 重做 QA 门禁文档，区分代码门禁与生产门禁 | QA | 更新 `test/QA_GATE_STATUS.md` | 报告包含通过项、未通过项、漂移项、阻断项 | PM-P0-1, TL-P0-1, TL-P0-2 | 已完成 |
| QA-P0-2 | P0 | 将 memory fallback / 宽松 readiness / 文档漂移列为 Critical | QA | QA 审查结论 | 报告中明确列为 Critical，未修复前不得 APPROVED | QA-P0-1 | 已完成 |
| DO-P0-1 | P0 | 形成真实部署基线（启动、变量、探针、migration、回滚） | DevOps | 部署基线文档 | 覆盖启动命令、必填变量、探针、回滚方式 | TL-P0-3 | ✅ 已完成（Gate B 验证通过）|
| DO-P0-2 | P0 | 建立关键配置缺失即启动失败的部署标准 | DevOps | CI/CD 或启动脚本校验规则 | prod 缺 `AI_CS_POSTGRES_DSN` / `AI_CS_WEBHOOK_SECRET` 时 fail | TL-P0-3 | ✅ 已完成（config.go 强制）|

---

## 2. P1 整改执行表（灰度前应完成）

| ID | 优先级 | 整改项 | 责任角色 | 交付物 | 验收标准 | 依赖 | 状态 |
|---|---|---|---|---|---|---|---|
| XL-P1-1 | P1 | 统一 PM/TechLead/QA/DevOps 交付模板 | 小龙 | 角色交付模板 | 每份角色输出均含结论、证据、阻塞、下一阶段条件 | XL-P0-1 | 未开始 |
| XL-P1-2 | P1 | 增加关键修复后的实施漂移复核点 | 小龙 | 复核流程 | 每次关键修复后都有测试复跑、配置复核、状态更新 | XL-P0-2 | 已完成 |
| PM-P1-1 | P1 | 补上线运营观察指标与失败判定线 | PM | 文档更新 | 含 handoff、ticket、audit、ready、重启后数据等观察项 | PM-P0-1 | 未开始 |
| PM-P1-2 | P1 | 统一环境变量文档契约 | PM | 文档更新 | 仅使用代码真实变量名，不再写泛化别名 | TL-P0-3 | 已完成 |
| TL-P1-1 | P1 | 补 ticket/session 后台接口鉴权设计 | TechLead | 设计文档 | actor 来源不可伪造，接口 auth 模式明确 | TL-P0-3 | 未开始 |
| TL-P1-2 | P1 | 补多实例与恢复场景验证设计 | TechLead | 设计文档 / 测试计划 | 覆盖 dedup、多实例、重启一致性、migration 幂等 | TL-P0-2 | 未开始 |
| QA-P1-1 | P1 | 建立文档漂移检测检查项 | QA | QA 模板/报告更新 | 每次审查都校对代码 vs 文档 vs 测试状态 | QA-P0-1 | 已完成 |
| QA-P1-2 | P1 | 增加真实环境前置门禁 | QA | 预生产验证记录 | 启动、ready、migration、webhook、入库验证完成 | DO-P0-1, DO-P0-2 | 未开始 |
| DO-P1-1 | P1 | 补最小监控与告警闭环 | DevOps | 告警配置/监控清单 | 覆盖 5xx、reject、handoff、ticket、audit、DB、ready | DO-P0-1 | 未开始 |
| DO-P1-2 | P1 | 补运行与回滚 runbook | DevOps | runbook 文档 | 覆盖启动失败、migration 失败、DB 不可用、auth 联调失败 | DO-P0-1 | 未开始 |

---

## 3. P2 整改执行表（全量上线后持续补）

| ID | 优先级 | 整改项 | 责任角色 | 交付物 | 验收标准 | 依赖 | 状态 |
|---|---|---|---|---|---|---|---|
| TL-P2-1 | P2 | 完整威胁建模补齐 | TechLead | threat model 文档 | 覆盖鉴权、越权、审计、脱敏、恢复、依赖风险 | TL-P1-1 | 未开始 |
| TL-P2-2 | P2 | 提升 store/app 关键层测试覆盖 | TechLead | 测试与覆盖率报告 | store/app 关键层覆盖明显提升并覆盖异常场景 | TL-P1-2 | 进行中 |
| QA-P2-1 | P2 | 建立长期质量回归基线 | QA | 回归清单 | 关键链路、关键控制点形成常规回归项 | QA-P1-2 | 未开始 |
| PM-P2-1 | P2 | 完善数据保留、审计、运营复盘口径 | PM | 产品/运营文档 | 有保留策略、失败判定、复盘节奏 | PM-P1-1 | 未开始 |
| DO-P2-1 | P2 | 细化容量与可观测性建设 | DevOps | 容量规划与监控扩展文档 | 有容量阈值、趋势指标、扩容策略 | DO-P1-1 | 未开始 |
| XL-P2-1 | P2 | 将整改执行纳入长期阶段复盘机制 | 小龙 | 复盘模板 | 每个阶段都有事实校准、漂移回收、责任追踪 | XL-P1-2 | 未开始 |

---

## 4. 按角色汇总视图

### 4.1 小龙
| ID | 项目 | 优先级 | 状态 |
|---|---|---|---|
| XL-P0-1 | 代码事实高于报告门禁 | P0 | 已完成 |
| XL-P0-2 | 重写阶段状态口径 | P0 | 已完成 |
| XL-P1-1 | 统一角色交付模板 | P1 | 未开始 |
| XL-P1-2 | 建立实施漂移复核点 | P1 | 已完成 |
| XL-P2-1 | 纳入长期阶段复盘 | P2 | 未开始 |

### 4.2 PM
| ID | 项目 | 优先级 | 状态 |
|---|---|---|---|
| PM-P0-1 | 修正文档上线口径 | P0 | 已完成 |
| PM-P0-2 | 明确 memory/dev/prod 约束 | P0 | 已完成 |
| PM-P1-1 | 补运营观察指标与失败线 | P1 | 未开始 |
| PM-P1-2 | 统一环境变量文档契约 | P1 | 已完成 |
| PM-P2-1 | 完善审计/保留/复盘口径 | P2 | 未开始 |

### 4.3 TechLead
| ID | 项目 | 优先级 | 状态 |
|---|---|---|---|
| TL-P0-1 | 禁止 prod fallback 到 memory | P0 | 已完成 |
| TL-P0-2 | 收紧 readiness | P0 | 已完成 |
| TL-P0-3 | 配置契约基线 | P0 | 已完成 |
| TL-P1-1 | 后台接口鉴权设计 | P1 | 未开始 |
| TL-P1-2 | 多实例/恢复验证设计 | P1 | 未开始 |
| TL-P2-1 | 完整威胁建模 | P2 | 未开始 |
| TL-P2-2 | 提升关键层覆盖率 | P2 | 进行中 |

### 4.4 QA
| ID | 项目 | 优先级 | 状态 |
|---|---|---|---|
| QA-P0-1 | 重做 QA 门禁文档 | P0 | 已完成 |
| QA-P0-2 | 将核心风险列为 Critical | P0 | 已完成 |
| QA-P1-1 | 增加文档漂移检测 | P1 | 已完成 |
| QA-P1-2 | 增加真实环境前置门禁 | P1 | 未开始 |
| QA-P2-1 | 建立长期回归基线 | P2 | 未开始 |

### 4.5 DevOps
| ID | 项目 | 优先级 | 状态 |
|---|---|---|---|
| DO-P0-1 | 真实部署基线 | P0 | ✅ 已完成 |
| DO-P0-2 | 关键配置 fail-fast 部署标准 | P0 | ✅ 已完成 |
| DO-P1-1 | 最小监控与告警闭环 | P1 | 未开始 |
| DO-P1-2 | 运行与回滚 runbook | P1 | 未开始 |
| DO-P2-1 | 容量与可观测性细化 | P2 | 未开始 |

---

## 5. 阶段门禁检查表

### Gate A：代码级通过
- [x] 主链测试通过
- [x] 静态检查通过（`go vet ./...`）
- [x] prod 不允许 memory fallback
- [x] readiness 语义已校准：prod 缺关键配置启动失败，非 prod memory 可正常 ready
- [x] 配置契约与代码一致

### Gate B：预生产通过
- [x] 真实 Postgres 联通
- [x] migration 成功（DB 有完整表结构，schema 初始化完成）
- [x] webhook 签名联调成功（HMAC-SHA256 验证通过）
- [x] audit / ticket 入库成功（实测：webhook → session → handoff → ticket → audit 全链路）
- [x] ready/live 符合预期（/actuator/health/ready → 200，postgres checker → UP）
- [ ] 最小监控已接通（未完成）

### Gate C：生产灰度通过
- [ ] 5% 灰度稳定
- [ ] handoff / ticket / audit 指标正常
- [ ] 无异常 5xx / reject 激增
- [ ] 回滚演练通过

---

## 6. 本轮新增证据

1. 代码变更：
   - `internal/config/config.go`
   - `internal/app/app.go`
   - `internal/config/config_test.go`
   - `internal/app/app_test.go`
   - `test/integration/health_check_test.go`
2. 验证命令：
   - `go test ./internal/config ./internal/app ./test/integration -count=1`
   - `go test ./... -count=1`
   - `go vet ./...`
3. 验证结果：
   - 上述命令本轮均已通过

---

## 7. 执行要求

1. 先做 P0，不并行宣布“可上线”
2. 每完成一项，必须更新状态和证据
3. QA 不能在 P0 未清零前给出生产放行结论
4. 小龙负责最终事实校准，不接受“口头完成”
