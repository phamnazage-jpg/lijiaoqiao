# AI-Customer-Service 全面 Review 与上线距离评估报告

> 审查时间：2026-05-04
> 审查方式：静态代码审查 + 文档对照 + 本地构建/测试验证
> 审查范围：`/home/long/project/立交桥/projects/ai-customer-service`

## 1. 结论摘要

当前项目**不是“接近完整的生产客服系统”**，而是一个**质量尚可的生产一期后端原型 / 最小闭环服务**。

从“完全完成规划设计和生产上线”这个目标看，当前状态更接近：

| 维度 | 当前完成度 | 结论 |
|---|---:|---|
| 规划与设计文档 | 75% | 文档数量充足，但存在明显漂移和口径冲突 |
| 核心后端最小闭环实现 | 45% | webhook、session、ticket、audit、health 基本具备 |
| 相对 PRD 的真实功能完成度 | 25% | 缺少 LLM/RAG、真实诊断查询、身份核验、多渠道适配、运营后台 |
| 生产放量准备度 | 20% | 缺少鉴权/RBAC、可观测性、真实联调、灰度回滚闭环 |

结论可以直接表述为：

1. **代码级可运行、可测试，但不是 PRD 意义上的“智能客服系统已完成”。**
2. **不具备直接生产上线条件。**
3. **更适合被定义为“Phase 1 后端骨架 + 最小工单闭环”，距离生产上线至少还差 3 个阶段。**

### 1.1 整改后状态更新（2026-05-04 当日追加）

在本次 review 之后，已继续完成并验证：

1. 文档口径与配置契约收口
2. 后台最小鉴权落地
3. 工单 `assign -> resolve -> close` 语义收口
4. Gate B 预生产验证脚本建立并完成本地/容器化实测
5. 灰度最小监控、阈值、放量与回滚门禁文档建立
6. 一页式灰度放行清单建立

这意味着项目状态已经从“只有代码级可运行”提升到了：

> **代码级门禁通过 + 本地/容器化 Gate B 通过 + Gate C 门禁已定义，但真实共享预生产与真实灰度放量仍未通过。**

相应地，这份报告中的“生产放量准备度”需要更新为：

| 维度 | 初始判断 | 当前更新判断 |
|---|---:|---:|
| 代码级可信度 | 45% | 60% |
| 预生产可验证度 | 20% | 55% |
| 灰度放量准备度 | 20% | 40% |

但这仍然**不构成“允许灰度上线”**。当前主要剩余阻断是：

1. 共享预生产环境尚未复跑 Gate B 脚本
2. 共享预生产/灰度环境监控接线未完成
3. 回滚演练未完成
4. 首轮 5% 灰度稳定性尚无证据

## 2. 本次实际验证

本次实际执行并确认了以下检查：

```bash
go test ./...
go test -race ./...
go build ./...
```

结果：

- `go test ./...` 通过
- `go test -race ./...` 通过
- `go build ./...` 通过

这说明当前仓库的**现有实现质量**整体不差，但这些结果只能证明：

- 当前代码可以编译
- 当前测试覆盖的行为成立
- 当前并发路径未被 race 检测发现问题

这些结果**不能证明**：

- PRD 功能已完成
- 真实依赖已联通
- 生产链路已验证
- 灰度和回滚具备可执行性

## 3. 关键发现

### P0-1 文档将“原型/最小实现”误表述为“可灰度发布”

`docs/PRODUCTION_LAUNCH.md` 明确写了：

- “已通过全部上线门禁，可灰度发布”
- “多渠道 Webhook 接收（Telegram/Discord/微信/网页）”
- “基于 LLM 的意图识别 + 知识库 RAG”

对应证据：

- `docs/PRODUCTION_LAUNCH.md:4`
- `docs/PRODUCTION_LAUNCH.md:15-18`

但实际代码实现是：

- 意图识别为关键词规则，不是 LLM
- 回复来自内存 FAQ，不是 RAG
- 没有 Telegram / Discord / 微信独立适配器实现

对应代码：

- `internal/service/intent/service.go:15-49`
- `internal/store/memory/knowledge_store.go:7-20`
- `internal/http/router.go:29-52`

影响：

- 会误导团队把“代码骨架可运行”当成“产品能力可上线”
- 会直接污染 PM、QA、运维对项目状态的判断

### P0-2 管理与工单接口无鉴权，不能作为生产后台暴露

当前 `tickets` 和 `sessions` 相关接口直接挂在路由上，没有任何认证或权限中间件：

- `internal/http/router.go:54-123`

同时，关键操作人信息仅来自 query 参数：

- `internal/http/handlers/ticket_handler.go:63-65`
- `internal/http/handlers/ticket_handler.go:86-88`
- `internal/http/handlers/ticket_handler.go:109-111`
- `internal/http/handlers/session_handler.go:72-75`
- `internal/http/handlers/session_handler.go:140-143`

也就是说：

- 任意调用方只要能访问接口，就可以尝试分配、解决、关闭工单
- `actor_id` 可以伪造
- 审计里的操作者身份不可信

虽然仓库文档已经承认“权限模型当前未落地”，但这也恰恰说明**生产放量前它仍是阻断项**：

- `prd/IDENTITY_AND_PERMISSION_STRATEGY.md:71-79`

### P0-3 当前实现与 PRD 的核心能力差距仍然很大

PRD 的 in-scope 能力包含：

- 多渠道接入
- 基于大模型的意图识别
- RAG 检索
- 知识库管理
- 诊断查询
- 运营后台
- 埋点与监控

证据：

- `prd/PRD.md:44-51`
- `prd/PRD.md:73-85`
- `prd/PRD.md:97-105`

而当前代码真实提供的是：

- 一个统一 webhook 入口
- 基于规则的 intent
- 基于内存 map 的固定回复
- 工单与审计的最小后端接口

对应代码：

- `internal/http/router.go:29-52`
- `internal/service/intent/service.go:15-49`
- `internal/store/memory/knowledge_store.go:7-20`
- `internal/service/dialog/service.go:69-145`

这不是“差一点上线”，而是**产品层级仍处于缩 scope 的后端一期**。

### P1-1 上下文能力低于设计规格

设计文档要求保留最近 5 轮对话，即 10 条消息：

- `tech/HLD.md:176-179`

实际代码只保留最近 6 条消息：

- `internal/service/dialog/service.go:95-98`
- `internal/service/dialog/service.go:129-132`

影响：

- 多轮对话理解能力低于设计要求
- 一旦未来接入真实 LLM，上下文容量会先成为效果瓶颈

### P1-2 生产文档中的 API 与真实路由不一致

`docs/PRODUCTION_LAUNCH.md` 声称已实现：

- `GET /api/v1/customer-service/webhook/channels`
- `GET /live`
- `GET /ready`

证据：

- `docs/PRODUCTION_LAUNCH.md:57-58`
- `docs/PRODUCTION_LAUNCH.md:83-86`
- `docs/PRODUCTION_LAUNCH.md:176-179`

但真实路由只有：

- `/actuator/health`
- `/actuator/health/live`
- `/actuator/health/ready`
- `/api/v1/customer-service/webhook`
- `/api/v1/customer-service/webhook/{channel}`

对应代码：

- `internal/http/router.go:25-27`
- `internal/http/router.go:34`
- `internal/http/router.go:52`

`/webhook/channels` 根本不存在，`/live` 与 `/ready` 也不是实际路径。

这说明发布文档本身不可直接用于部署或联调。

### P1-3 配置文档与真实配置契约不一致

生产文档列出的环境变量是：

- `POSTGRES_HOST`
- `POSTGRES_USER`
- `POSTGRES_PASSWORD`
- `SERVER_PORT`
- `WEBHOOK_HMAC_KEY`

证据：

- `docs/PRODUCTION_LAUNCH.md:154-163`

但代码真实读取的是：

- `AI_CS_ADDR`
- `AI_CS_POSTGRES_ENABLED`
- `AI_CS_POSTGRES_DSN`
- `AI_CS_WEBHOOK_SECRET`
- `AI_CS_RUNTIME_ENV`

对应代码：

- `internal/config/config.go:47-97`

影响：

- 直接按发布文档配置环境，服务不会按预期启动
- 部署侧会产生“文档正确但服务读不到配置”的高风险误操作

## 4. 当前已经做对的部分

这部分需要客观肯定，否则会误判为“完全不可用”：

1. **HTTP 服务骨架清晰**
   - `cmd/ai-customer-service/main.go`
   - `internal/app/app.go`

2. **Webhook 安全基础比一般 demo 强**
   - HMAC/时间戳/body limit/rate limit/dedup 都已经接到主路径
   - 相关路由见 `internal/http/router.go:29-52`

3. **健康检查、优雅停机、Postgres 模式切换具备基础能力**
   - `internal/http/handlers/health_handler.go`
   - `internal/store/postgres/db.go`
   - `internal/app/app.go`

4. **测试现状良好**
   - `go test ./...` 通过
   - `go test -race ./...` 通过
   - `go build ./...` 通过

所以这个项目的真实评价应当是：

> **不是“乱写的 demo”，而是“工程质量尚可，但业务完成度和生产 readiness 明显不足的后端一期骨架”。**

## 5. 与“完整规划设计”之间的具体距离

如果目标是“规划设计完全完成”，当前还差的不是“再补几页文档”，而是**文档统一口径和事实对齐**。

### 已完成

- PRD、HLD、接口、测试、运行、SOP、灰度、合规文档已经有较完整框架
- 项目内部已经意识到自己是“生产一期未完成”
  - `PRODUCTION_EXECUTION_PLAN.md:5-18`

### 未完成

1. **文档单一真相源还没有建立**
   - `PRODUCTION_LAUNCH.md` 仍然过度乐观
   - `PRODUCTION_EXECUTION_PLAN.md` 更接近真实状态

2. **Phase 1 / Phase 2 / 最终 PRD 的边界没有完全收敛**
   - `prd/SCOPE_PHASE1_VS_PHASE2.md` 在降 scope
   - `docs/PRODUCTION_LAUNCH.md` 却仍按最终系统表述

3. **部署文档、API 文档、配置文档尚未完全和代码对齐**

我的判断：

- **规划设计完成度约 75%**
- 距离“设计冻结、文档可直接驱动实施和上线”还差 **25% 左右**

## 6. 与“生产上线”之间的具体距离

### 当前可视为已完成的生产前置能力

- 基础 HTTP 服务
- 基础 webhook 入口
- 基础工单后端
- 基础审计
- 基础 Postgres 支持
- 基础测试

### 距离生产上线仍缺的关键阶段

#### 阶段 A：收口事实口径

- 清理错误上线表述
- 统一 Phase 1 / Phase 2 / 最终版边界
- 让所有文档和真实路由、真实配置、真实依赖一致

#### 阶段 B：补齐生产级后台安全

- Auth middleware
- RBAC
- 跨用户数据隔离
- 工单/会话接口权限校验
- 审计 actor 可信来源

#### 阶段 C：补齐真实业务能力

- 真实身份核验
- 只读 quota/token/error logs 查询
- 真实多渠道适配
- 真实知识库/RAG
- 真实 LLM/failover
- 人工回复用户闭环

#### 阶段 D：补齐生产运维能力

- metrics / tracing / SLO
- 告警
- 灰度开关
- 回滚 Runbook
- 真实环境联调证据

我的判断：

- **距离“生产可灰度”仍差至少 3 个实质阶段**
- **距离“按 PRD 完整上线”仍差至少 4 个阶段**

如果用工作量粗估：

| 目标 | 距离 |
|---|---|
| 代码级稳定后端一期 | 已基本达到 |
| 可进入预生产联调 | 还差 2~4 周，取决于是否只做 Phase 1 |
| 可做小流量灰度 | 还差 4~8 周，取决于鉴权、观测、联调资源 |
| 接近 PRD 完整版上线 | 还差 8~16 周，且前提是追加 LLM/RAG/运营后台/多渠道资源 |

## 7. 建议的下一步顺序

### 第一优先级

1. 修正文档口径
2. 建立单一上线基线文档
3. 停止使用 `docs/PRODUCTION_LAUNCH.md` 作为上线依据

### 第二优先级

1. 为 `tickets` / `sessions` 全部接口补鉴权与角色校验
2. 修复部署文档与真实环境变量不一致
3. 修复发布文档与真实路由不一致

### 第三优先级

1. 明确生产一期是否只做“工单后端 + webhook”
2. 如果是，就把 LLM/RAG/运营后台全部降到 Phase 2，且文档同步
3. 如果不是，就必须补真实 LLM/RAG/诊断查询链路，而不是继续用规则和静态 FAQ

## 8. 最终判定

本项目当前更准确的定位是：

> **一个通过本地测试验证的、工程质量尚可的客服后端一期原型，而不是接近完整生产上线的 AI 客服系统。**

正式结论：

- **全面 review 结果：不建议按“已完成规划设计并可生产上线”口径汇报**
- **真实状态：可继续推进为生产一期后端服务**
- **距离完整规划设计完成：约 25%**
- **距离生产可灰度上线：约 75% 的关键工作仍未闭环**
- **距离 PRD 全量目标上线：约 70%~80% 的业务能力仍未落地**

---

## 9. 2026-05-05 实测更新

### Gate B 本地/容器化验证（实测通过）

| 项目 | 值 |
|------|------|
| 运行 ID | `gateb-20260505101654` |
| PASS/FAIL | **30/0** |
| 验证范围 | postgres连通、migration账本、live/ready、webhook签名、dedup、ticket全链路(assign/resolve/close)、audit入库 |

### Gate C 回滚演练本地验证（实测通过）

| 项目 | 值 |
|------|------|
| 运行 ID | `gatec-rollback-20260505101646` |
| PASS/FAIL | **25/0** |
| 验证范围 | 源码构建、baseline启动、broken release退出、回滚重启、主链路恢复、dedup/audit/ticket验证 |

### 结论升级

| 维度 | 更新前 | 更新后 |
|------|--------|--------|
| 代码级可信度 | 60% | **75%** |
| 预生产可验证度 | 55% | **70%** |
| 灰度放量准备度 | 40% | **50%** |

**仍需线下验证**：真实共享预生产环境 Gate B + 灰度监控接线 + 5%灰度稳定性
