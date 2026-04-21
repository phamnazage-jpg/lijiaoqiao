# Project Quality / Performance / Ops Optimization Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 通过四个阶段收敛项目的身份与鉴权真源、发布真源、运行时治理和维护边界，使项目质量更高、性能更强、运维更简单。

**Architecture:** 先解决“谁说了算”的问题，再解决“怎么发布”和“怎么验证”的问题，最后收口运行时与代码结构。每个阶段都要求同时覆盖设计、实现、验证、回滚和文档，不允许只做局部修补。

**Tech Stack:** Go, Bash, PostgreSQL, Redis, HTTP, OpenAPI, Markdown, CI Shell Scripts

---

## 1. 文档用途

本文件同时承担四个用途：

1. 汇总本轮综合 review 的关键发现。
2. 给出按阶段推进的优化目标和退出标准。
3. 把每个阶段拆成可自行细分、几分钟内可完成的最小任务清单。
4. 复审任务清单是否足以彻底达成各阶段目标。

本文件不是“方向性建议”，而是执行前的落地任务底稿。

## 2. 范围

本计划覆盖以下代码和流程边界：

- `gateway`
- `supply-api`
- `platform-token-runtime`
- `scripts/ci`
- `scripts/devtest`
- `scripts/auto_review`
- `sql/postgresql`
- `docs/`
- `review/`

## 3. 关键发现

### 3.1 架构与边界问题

1. `token`/鉴权存在三套并行权威源。
   证据：`gateway/internal/app/bootstrap.go`、`gateway/internal/middleware/remote_runtime.go`、`supply-api/internal/middleware/auth.go`、`supply-api/internal/app/runtime.go`、`platform-token-runtime/internal/auth/service/inmemory_runtime.go`
   影响：跨服务身份语义漂移，审计口径不一致，联调和回归验证成本持续上升。

2. `platform-token-runtime` 的 schema 与运行时模型漂移。
   证据：`sql/postgresql/token_runtime_schema_v1.sql`、`platform-token-runtime/internal/auth/service/inmemory_runtime.go`、`platform-token-runtime/internal/auth/service/postgres_runtime_store.go`、`platform-token-runtime/internal/auth/service/postgres_audit_store.go`
   影响：多租户字段、操作人字段和 metadata 形同虚设，后续审计和权限治理会缺证据链。

3. 鉴权链、审计模型和日志实现重复复制并开始分叉。
   证据：`gateway/internal/middleware/chain.go`、`platform-token-runtime/internal/auth/middleware/token_auth_middleware.go`、`gateway/internal/pkg/logging/logger.go`、`platform-token-runtime/internal/pkg/logging/logger.go`、`supply-api/internal/pkg/logging/logger.go`
   影响：安全修复和兼容调整需要多处同步，容易遗漏。

4. `gateway` 长期保留未接入主链路的实验能力。
   证据：`gateway/internal/app/bootstrap.go`、`gateway/internal/router/engine/routing_engine.go`、`gateway/internal/router/strategy/cost_aware.go`、`gateway/internal/router/fallback/fallback.go`、`gateway/README.md`
   影响：认知负担高，路由行为与仓库实现不一致。

5. `supply-api` 的 `InvariantChecker` 已实现但未进入执行路径，IAM 也未闭环。
   证据：`supply-api/internal/app/runtime.go`、`supply-api/internal/domain/invariants.go`、`supply-api/internal/iam/handler/iam_handler.go`
   影响：规则定义与真实行为会继续漂移，权限边界难以稳定。

### 3.2 发布与运维问题

1. 发布门禁把 `mock`/`DEFERRED` 结果计入可通过链路。
   证据：`scripts/ci/superpowers_stage_validate.sh`、`scripts/ci/metrics_daily_snapshot.sh`、`scripts/ci/staging_release_pipeline.sh`
   影响：真实 staging 不是唯一硬阻断条件，上线结论可能偏离真实环境。

2. 发布证据按“最新文件”拼装，存在跨批次串用风险。
   证据：`scripts/ci/staging_release_pipeline.sh`、`scripts/ci/staging_evidence_autofill.sh`、`scripts/ci/tok007_release_recheck.sh`、`scripts/ci/final_decision_consistency_check.sh`
   影响：历史工件可能污染当前发布决策。

3. `supply-api` 的 staging/devtest 仍复用 `dev` 配置。
   证据：`scripts/devtest/start_dev_stack.sh`、`supply-api/cmd/supply-api/main.go`、`supply-api/config/config.dev.yaml`
   影响：环境语义不统一，难以建立稳定的预发布信任。

4. 当前 `e2e` 主要是进程内装配，不是跨服务真实部署验证。
   证据：`scripts/ci/repo_integrity_check.sh`、`scripts/ci/backend-verify.sh`、`supply-api/e2e/e2e_test.go`、`supply-api/e2e/README.md`
   影响：通过了测试，不等于通过了发布链路。

5. `auto_review` 目前只是文档巡检，不是工程门禁。
   证据：`scripts/auto_review/auto_review.sh`、`scripts/auto_review/README.md`
   影响：如果被误解为质量守门员，会形成错误安全感。

### 3.3 性能与运行时问题

1. `gateway` 的远程 token runtime 默认使用 `http.DefaultClient`，没有显式超时。
   证据：`gateway/internal/middleware/remote_runtime.go`
   影响：远端抖动会直接放大为入口尾延迟。

2. 远程 token 状态缓存只增不减，没有淘汰策略。
   证据：`gateway/internal/middleware/remote_runtime.go`
   影响：长时间运行后会形成内存增长风险。

3. `gateway` 的加权路由使用全局随机源，并且 provider 被摘除后没有自动恢复路径。
   证据：`gateway/internal/router/router.go`
   影响：并发选择质量不可控，瞬时故障可能演变为长期降级。

4. 运行时指标主要靠离线脚本推导，服务自身缺少统一 `/metrics` 暴露。
   证据：`scripts/ci/metrics_daily_snapshot.sh`、`gateway/internal/app/bootstrap.go`、`platform-token-runtime/internal/app/bootstrap.go`
   影响：容量判断和故障定位都偏慢。

5. 后台 worker 的 shutdown 纪律不够完整。
   证据：`supply-api/internal/app/background.go`
   影响：优雅关闭、分区维护和后台任务退出时行为不够透明。

### 3.4 安全与治理问题

1. `supply-api` 的“KMS”实现仍是本地派生密钥方案，默认 provider 也是 `local`。
   证据：`supply-api/internal/security/kms_service.go`
   影响：如果被误认为生产级 KMS，会形成严重的密钥治理假象。

2. 仓库中存在被跟踪的环境文件，且本地脚本内置默认 secret。
   证据：`scripts/supply-gate/.env`、`scripts/devtest/start_dev_stack.sh`
   影响：误提交、误复用、误传播风险持续存在。

3. `gateway` 仍保留默认加密 key 常量，虽然生产有 guard，但治理方式偏脆弱。
   证据：`gateway/internal/config/config.go`、`gateway/internal/app/bootstrap.go`
   影响：一旦环境语义校验被绕开，会退回到静态默认 key。

## 4. 阶段化优化目标

### Phase 1: 身份、鉴权、审计真源收敛

**目标：**
建立唯一 token authority、唯一 principal 契约、唯一审计字段契约。

**阶段完成标准：**

1. `platform-token-runtime` 成为唯一 token authority。
2. `gateway` 在非 dev 环境不再持有本地 token authority。
3. `supply-api` 不再自带独立 token 语义，只消费统一 principal。
4. token schema、HTTP contract、审计模型三者字段一致。
5. 跨服务 contract tests 能覆盖 `gateway -> token-runtime -> supply-api` 的身份链路。

### Phase 2: 发布真源、证据真源、环境真源收敛

**目标：**
让发布结论只建立在“本次 run 的真实 staging 证据”之上。

**阶段完成标准：**

1. 发布流水线按 `run_id` 隔离工件和报告。
2. `mock`/`local` 只作为 rehearsal，不参与 release pass 计算。
3. `WARN` 不再默认放行 release decision。
4. staging/prod 配置模板独立存在，不再复用 `dev` 配置。
5. 至少存在一条真实跨服务 smoke 链路。

### Phase 3: 运行时稳定性、性能、可观测性收敛

**目标：**
降低入口尾延迟风险，补齐运行时指标和 shutdown 纪律。

**阶段完成标准：**

1. 远程 introspection 有超时、连接池、缓存淘汰和指标。
2. provider 健康摘除后存在自动恢复路径。
3. 三个核心服务都有统一 `/metrics` 或等效指标出口。
4. 关键 SLI 来自运行时采集，而不是 markdown/CSV 推导。
5. 后台 worker、分区维护、补偿处理器具备可观测退出行为。

### Phase 4: 代码结构、IAM、安全治理收敛

**目标：**
降低重复实现和大文件复杂度，补齐 IAM 闭环与 secret 治理。

**阶段完成标准：**

1. auth/audit/logging 共享实现不再三处分叉。
2. `supply-api` 的大文件完成按职责拆分。
3. `InvariantChecker` 要么接入真实写路径，要么被删除。
4. IAM 路由接入 tenant/operator/scope 闭环。
5. tracked `.env` 清理完成，devtest 默认 secret 被替换。
6. 本地派生密钥方案不再被表述为生产可用 KMS。

## 5. 详细任务清单

说明：

- 每个任务设计为 2 到 5 分钟内可完成。
- 每个任务必须产生一个可见输出。
- 每个任务都包含完成标准。
- 若执行中发现新事实，必须先补文档，再改代码。

---

## Phase 1 任务清单

### Task P1-A: 定义唯一 token authority 与 canonical principal 契约

**Files:**
- Modify: `docs/token_runtime_minimal_spec_v1.md`
- Modify: `docs/platform_token_api_contract_openapi_draft_v1_2026-03-29.yaml`
- Modify: `gateway/README.md`
- Modify: `supply-api/README.md`
- Modify: `platform-token-runtime/README.md`

- [x] `P1-A-01` 运行 `rg -n "inmemory|remote_introspection|token runtime|JWT|Bearer" gateway supply-api platform-token-runtime`，收集当前身份入口。
  完成标准：执行记录中列出全部入口文件。
- [x] `P1-A-02` 在 `docs/token_runtime_minimal_spec_v1.md` 增加“唯一 authority”段落草稿。
  完成标准：文档里明确写出 authority 所属服务。
- [x] `P1-A-03` 在同一文档新增 canonical principal 字段清单。
  完成标准：至少包含 `token_id`、`subject_id`、`tenant_id`、`scope`、`issued_at`、`expires_at`、`status`。
- [x] `P1-A-04` 打开 `docs/platform_token_api_contract_openapi_draft_v1_2026-03-29.yaml`，比对现有 introspection 响应字段。
  完成标准：差异项被记录到计划执行日志。
- [x] `P1-A-05` 在 OpenAPI 草案里补 canonical principal 字段定义。
  完成标准：OpenAPI 草案与最小规范字段一致。
- [x] `P1-A-06` 在 `gateway/README.md` 增加“非 dev 环境只允许远程 introspection”说明。
  完成标准：README 不再给 staging/prod 留下本地 authority 的解释空间。
- [x] `P1-A-07` 在 `supply-api/README.md` 增加“消费统一 principal，不自持独立 authority”说明。
  完成标准：README 用词与 Phase 1 目标一致。
- [x] `P1-A-08` 在 `platform-token-runtime/README.md` 增加“唯一 authority”声明和字段边界。
  完成标准：三份 README 口径一致。

### Task P1-B: 收敛 token schema、存储模型和审计字段

**Files:**
- Modify: `sql/postgresql/token_runtime_schema_v1.sql`
- Modify: `platform-token-runtime/internal/auth/service/inmemory_runtime.go`
- Modify: `platform-token-runtime/internal/auth/service/postgres_runtime_store.go`
- Modify: `platform-token-runtime/internal/auth/service/postgres_audit_store.go`
- Create: `docs/plans/2026-04-21-token-runtime-schema-alignment-notes.md`

- [ ] `P1-B-01` 打开 schema，圈出 `tenant_id`、`project_id`、`operator_id`、`metadata` 所在列。
  完成标准：对齐笔记里列出字段和表名。
- [ ] `P1-B-02` 打开 `TokenRecord`，列出当前缺失字段。
  完成标准：差异清单写入对齐笔记。
- [ ] `P1-B-03` 决策“保留字段并贯穿”还是“删除字段并收缩契约”。
  完成标准：对齐笔记里只保留一个决策，不允许双轨。
- [ ] `P1-B-04` 如果选择保留，先写字段迁移顺序。
  完成标准：顺序包含 model、store、API、audit、tests。
- [ ] `P1-B-05` 如果选择删除，先写 shrink SQL 和历史数据影响说明。
  完成标准：说明中覆盖 fresh setup 和 existing DB 两种情况。
- [ ] `P1-B-06` 检查 `postgres_runtime_store.go` 的 `INSERT`/`SELECT` 字段映射。
  完成标准：每个字段映射都被记录。
- [ ] `P1-B-07` 检查 `postgres_audit_store.go` 的写入字段映射。
  完成标准：审计模型缺口被单独列出。
- [ ] `P1-B-08` 为字段一致性写一个最小执行清单。
  完成标准：包含“改 model、改 SQL、改 test、跑验证”四类动作。

### Task P1-C: 收敛 gateway 与 supply-api 的身份实现

**Files:**
- Modify: `gateway/internal/app/bootstrap.go`
- Modify: `gateway/internal/middleware/remote_runtime.go`
- Modify: `supply-api/internal/middleware/auth.go`
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/bootstrap.go`

- [ ] `P1-C-01` 在 `gateway/internal/app/bootstrap.go` 标出 `inmemory` 与 `remote_introspection` 分支。
  完成标准：分支入口和调用点被记录。
- [ ] `P1-C-02` 在 `supply-api/internal/middleware/auth.go` 标出 JWT 验证、token 状态检查和 principal 注入点。
  完成标准：三个位置都被记录。
- [ ] `P1-C-03` 在 `supply-api/internal/app/runtime.go` 标出 token backend 的装配点。
  完成标准：DB-backed 与 memory-backed 装配点均被记录。
- [ ] `P1-C-04` 写出 `gateway` 非 dev 禁用本地 authority 的代码改动清单。
  完成标准：至少包含 config、bootstrap、tests。
- [ ] `P1-C-05` 写出 `supply-api` 从“JWT authority”迁到“principal consumer”的代码改动清单。
  完成标准：至少包含 middleware、runtime、HTTP handler、tests。
- [ ] `P1-C-06` 写出过渡期兼容策略。
  完成标准：明确单写、双读、切流或一次性切换的方案。
- [ ] `P1-C-07` 写出回滚条件。
  完成标准：明确何时回滚、回滚到哪一版契约。
- [ ] `P1-C-08` 为兼容窗口补一段 README/ADR 说明草稿。
  完成标准：说明中同时覆盖上线前、上线中、上线后三个时段。

### Task P1-D: 建立跨服务 contract tests 与阶段 gate

**Files:**
- Modify: `scripts/ci/backend-verify.sh`
- Modify: `scripts/ci/repo_integrity_check.sh`
- Create: `tests/contract/README.md`
- Create: `tests/contract/gateway_token_runtime_supply_chain.md`
- Create: `docs/plans/2026-04-21-phase1-contract-gate-checklist.md`

- [ ] `P1-D-01` 盘点当前 CI 对三服务身份链路的真实覆盖缺口。
  完成标准：清单里明确“当前没有覆盖”的场景。
- [ ] `P1-D-02` 定义最小 contract 场景 1：合法 token。
  完成标准：说明中包含请求入口、期望状态码和关键字段。
- [ ] `P1-D-03` 定义最小 contract 场景 2：吊销 token。
  完成标准：说明中包含吊销后的 gateway 与 supply-api 预期行为。
- [ ] `P1-D-04` 定义最小 contract 场景 3：scope 不足。
  完成标准：说明中包含 principal 字段和拒绝行为。
- [ ] `P1-D-05` 定义最小 contract 场景 4：token runtime 不可用。
  完成标准：说明中包含入口超时和错误码约束。
- [ ] `P1-D-06` 在 `backend-verify.sh` 设计新增执行位。
  完成标准：写清楚命令入口、产物路径和失败语义。
- [ ] `P1-D-07` 在 `repo_integrity_check.sh` 设计新增 contract gate 入口。
  完成标准：文件里明确放在何处执行。
- [ ] `P1-D-08` 写 Phase 1 gate checklist。
  完成标准：只有当 contract tests 通过，Phase 1 才能标记完成。

---

## Phase 2 任务清单

### Task P2-A: 引入 run_id 和 manifest，隔离发布工件

**Files:**
- Modify: `scripts/ci/staging_release_pipeline.sh`
- Modify: `scripts/ci/staging_evidence_autofill.sh`
- Modify: `scripts/ci/tok007_release_recheck.sh`
- Modify: `scripts/ci/final_decision_consistency_check.sh`
- Create: `reports/releases/.gitkeep`
- Create: `docs/plans/2026-04-21-release-manifest-contract.md`

- [ ] `P2-A-01` 盘点所有依赖 `latest_file_or_empty` 的脚本入口。
  完成标准：清单覆盖全部脚本和调用行。
- [ ] `P2-A-02` 设计 `run_id` 生成规则。
  完成标准：规则包含日期、提交号或流水号。
- [ ] `P2-A-03` 设计发布目录结构 `reports/releases/<run_id>/...`。
  完成标准：目录草图写入 manifest 合同文档。
- [ ] `P2-A-04` 设计 `manifest.json` 必填字段。
  完成标准：至少包含 `run_id`、`commit_sha`、`env`、`artifact_paths`、`decision_inputs`。
- [ ] `P2-A-05` 把 `staging_release_pipeline.sh` 的输入改成 manifest 读取方案草稿。
  完成标准：不再依赖“最新文件”。
- [ ] `P2-A-06` 把 `staging_evidence_autofill.sh` 的输入改成 manifest 读取方案草稿。
  完成标准：只消费本次 run 的工件。
- [ ] `P2-A-07` 把 `tok007_release_recheck.sh` 的输入改成 manifest 读取方案草稿。
  完成标准：复检脚本不再扫历史目录。
- [ ] `P2-A-08` 把 `final_decision_consistency_check.sh` 的输入改成 manifest 读取方案草稿。
  完成标准：最终一致性检查绑定单次 run。

### Task P2-B: 把真实 staging 设为唯一发布硬门禁

**Files:**
- Modify: `scripts/ci/superpowers_stage_validate.sh`
- Modify: `scripts/ci/staging_real_readiness_check.sh`
- Modify: `scripts/ci/superpowers_release_pipeline.sh`
- Modify: `scripts/ci/final_decision_consistency_check.sh`
- Create: `docs/plans/2026-04-21-real-staging-gate-rules.md`

- [ ] `P2-B-01` 找出 `mock`、`local`、`DEFERRED` 被计为通过的判定点。
  完成标准：每个判定点都有文件和行号。
- [ ] `P2-B-02` 定义 rehearsal 与 real staging 的术语。
  完成标准：规则文档中两者语义不重叠。
- [ ] `P2-B-03` 定义“只有 real staging 通过才允许 release pass”的总规则。
  完成标准：总规则写成一句可执行判定语句。
- [ ] `P2-B-04` 设计 `superpowers_stage_validate.sh` 的状态枚举。
  完成标准：至少区分 `PASS_REAL`、`PASS_REHEARSAL`、`FAIL`。
- [ ] `P2-B-05` 设计 `staging_real_readiness_check.sh` 的阻断输出格式。
  完成标准：输出里包含明确的 pass/fail 字段。
- [ ] `P2-B-06` 设计 `superpowers_release_pipeline.sh` 的 fail-fast 位置。
  完成标准：release 在 real staging 未通过时立即退出。
- [ ] `P2-B-07` 设计 override 机制。
  完成标准：override 必须要求审批人、时间戳、run_id、理由。
- [ ] `P2-B-08` 设计禁止 `DEFERRED` 计入完成率的规则。
  完成标准：文档里写出精确计算口径。

### Task P2-C: 统一环境语义和配置模板

**Files:**
- Modify: `gateway/internal/config/config.go`
- Modify: `gateway/internal/app/bootstrap.go`
- Modify: `supply-api/cmd/supply-api/main.go`
- Modify: `scripts/devtest/start_dev_stack.sh`
- Create: `supply-api/config/config.staging.example.yaml`
- Create: `supply-api/config/config.prod.example.yaml`
- Create: `docs/plans/2026-04-21-env-normalization-checklist.md`

- [ ] `P2-C-01` 盘点三服务支持的环境枚举和值别名。
  完成标准：清单覆盖 `dev`、`staging`、`prod`、`production`、`online`。
- [ ] `P2-C-02` 定义统一枚举。
  完成标准：计划文档只保留 `dev`、`staging`、`prod`。
- [ ] `P2-C-03` 设计 `gateway` 对 `production/online -> prod` 的归一化入口。
  完成标准：只保留一个真正规则。
- [ ] `P2-C-04` 设计 `supply-api` 对 `-env=staging -config=config.dev.yaml` 的拒绝规则。
  完成标准：规则中包含错误信息草稿。
- [ ] `P2-C-05` 复制 `config.dev.yaml` 所需段落，生成 staging 模板骨架。
  完成标准：`config.staging.example.yaml` 包含必要配置段。
- [ ] `P2-C-06` 复制 prod 模板骨架。
  完成标准：`config.prod.example.yaml` 包含关键安全配置占位。
- [ ] `P2-C-07` 在 `scripts/devtest/start_dev_stack.sh` 设计改用 staging 模板的参数。
  完成标准：脚本不再硬编码 `config.dev.yaml`。
- [ ] `P2-C-08` 写环境归一化检查清单。
  完成标准：包括启动前检查、启动后检查、CI 检查三类项。

### Task P2-D: 重构测试分类，补真实跨服务 smoke

**Files:**
- Modify: `scripts/ci/backend-verify.sh`
- Modify: `scripts/ci/repo_integrity_check.sh`
- Modify: `supply-api/e2e/README.md`
- Modify: `supply-api/e2e/e2e_test.go`
- Create: `tests/smoke/README.md`
- Create: `scripts/ci/cross_service_smoke.sh`

- [ ] `P2-D-01` 盘点当前被叫做 `e2e` 的测试真实边界。
  完成标准：区分进程内、单服务、跨服务三类。
- [ ] `P2-D-02` 定义新的测试分类名。
  完成标准：至少包含 `unit`、`integration`、`service-http`、`cross-service-smoke`。
- [ ] `P2-D-03` 在 `supply-api/e2e/README.md` 改写术语草稿。
  完成标准：README 不再把进程内测试叫成真实部署 E2E。
- [ ] `P2-D-04` 设计 `cross_service_smoke.sh` 的最小链路。
  完成标准：至少覆盖 `gateway -> token-runtime -> supply-api`。
- [ ] `P2-D-05` 为 smoke 脚本设计输入环境变量。
  完成标准：包含地址、token、期望模型或 scope。
- [ ] `P2-D-06` 为 smoke 脚本设计输出报告路径。
  完成标准：输出可被 manifest 收录。
- [ ] `P2-D-07` 设计 `backend-verify.sh` 中新增 smoke 入口。
  完成标准：能区分“本地占位跳过”和“真实 smoke 失败”。
- [ ] `P2-D-08` 设计 `repo_integrity_check.sh` 的职责边界说明。
  完成标准：明确它不是 release gate，只是代码完整性 gate。

---

## Phase 3 任务清单

### Task P3-A: 加固远程 token runtime 客户端

**Files:**
- Modify: `gateway/internal/middleware/remote_runtime.go`
- Modify: `gateway/internal/app/bootstrap.go`
- Modify: `gateway/internal/config/config.go`
- Create: `gateway/internal/middleware/remote_runtime_metrics_test.go`

- [ ] `P3-A-01` 列出现有 `http.DefaultClient` 的使用点。
  完成标准：清单覆盖文件和构造入口。
- [ ] `P3-A-02` 设计专用 `http.Client` 超时值。
  完成标准：至少包含总超时、连接超时、空闲连接策略。
- [ ] `P3-A-03` 设计 `remote_runtime` 的缓存 TTL。
  完成标准：定义 active、expired、revoked 三类缓存寿命。
- [ ] `P3-A-04` 设计缓存淘汰策略。
  完成标准：明确按 TTL、容量还是两者结合。
- [ ] `P3-A-05` 设计缓存命中率指标。
  完成标准：至少定义 hit、miss、evict 三项。
- [ ] `P3-A-06` 设计上游调用延迟指标。
  完成标准：定义 histogram 或 bucket 方案。
- [ ] `P3-A-07` 设计配置项命名。
  完成标准：写出环境变量草稿。
- [ ] `P3-A-08` 补最小测试矩阵。
  完成标准：覆盖 timeout、cache hit、cache evict、upstream fail。

### Task P3-B: 修正 router 健康摘除与恢复行为

**Files:**
- Modify: `gateway/internal/router/router.go`
- Modify: `gateway/internal/router/router_test.go`
- Create: `docs/plans/2026-04-21-router-health-recovery-notes.md`

- [ ] `P3-B-01` 记录现有 provider 摘除条件。
  完成标准：笔记里写出失败率阈值和现状行为。
- [ ] `P3-B-02` 记录现有恢复条件。
  完成标准：如果没有恢复路径，要明确写“缺失”。
- [ ] `P3-B-03` 定义自动恢复策略。
  完成标准：至少说明探测频率、恢复门槛、半开状态。
- [ ] `P3-B-04` 定义随机源实现策略。
  完成标准：明确使用线程安全方案。
- [ ] `P3-B-05` 定义 provider 健康指标。
  完成标准：至少包含 availability、failure_rate、latency_ema。
- [ ] `P3-B-06` 补测试场景 1：瞬时故障后恢复。
  完成标准：写出期望结果。
- [ ] `P3-B-07` 补测试场景 2：长时间故障保持摘除。
  完成标准：写出期望结果。
- [ ] `P3-B-08` 补测试场景 3：并发权重选择无竞态。
  完成标准：写出期望结果。

### Task P3-C: 统一三服务的 metrics、trace、health 可观测面

**Files:**
- Modify: `gateway/internal/app/bootstrap.go`
- Modify: `supply-api/internal/app/bootstrap.go`
- Modify: `platform-token-runtime/internal/app/bootstrap.go`
- Modify: `scripts/ci/metrics_daily_snapshot.sh`
- Create: `docs/plans/2026-04-21-observability-surface-plan.md`

- [ ] `P3-C-01` 盘点三个服务当前暴露的健康接口。
  完成标准：清单里有路径、响应格式和用途。
- [ ] `P3-C-02` 定义统一 metrics 路径。
  完成标准：文档中明确路径和鉴权要求。
- [ ] `P3-C-03` 定义统一 trace 字段。
  完成标准：至少包含 request_id、trace_id、span_id。
- [ ] `P3-C-04` 定义统一基础指标集。
  完成标准：至少包含请求数、错误数、延迟、上游调用、缓存命中。
- [ ] `P3-C-05` 定义 token runtime 指标集。
  完成标准：至少包含 introspection 次数、缓存命中、超时数。
- [ ] `P3-C-06` 定义 synthetic probe 指标集。
  完成标准：至少包含 gateway health、token introspection、supply auth path。
- [ ] `P3-C-07` 设计 `metrics_daily_snapshot.sh` 从运行时采集而非 grep 文档的方案。
  完成标准：至少写出三个采集入口。
- [ ] `P3-C-08` 设计 Dashboard/告警最小模板。
  完成标准：包含 SLI、SLO、报警阈值草稿。

### Task P3-D: 收紧后台 worker 的启动、退出和验证纪律

**Files:**
- Modify: `supply-api/internal/app/background.go`
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/cmd/supply-api/main.go`
- Create: `docs/plans/2026-04-21-background-worker-exit-checklist.md`

- [ ] `P3-D-01` 盘点所有后台 worker 入口。
  完成标准：清单覆盖 outbox、revocation、partition maintenance、compensation。
- [ ] `P3-D-02` 盘点哪些 worker 在内部使用了 `context.Background()`。
  完成标准：清单逐个写出调用点。
- [ ] `P3-D-03` 定义 worker 启动日志最小字段。
  完成标准：至少包含 worker 名称、配置、间隔、run_id。
- [ ] `P3-D-04` 定义 worker 退出日志最小字段。
  完成标准：至少包含 worker 名称、退出原因、持续时间。
- [ ] `P3-D-05` 定义 worker shutdown 超时策略。
  完成标准：写出 rootCtx、initCtx、shutdownCtx 的边界。
- [ ] `P3-D-06` 定义 worker 健康指标。
  完成标准：至少包含 last_success_ts、last_error_ts、loop_duration。
- [ ] `P3-D-07` 设计 startup precheck。
  完成标准：在 prod/staging 缺少 broker 时行为清晰。
- [ ] `P3-D-08` 编写后台 worker 退出检查清单。
  完成标准：可用于人工演练或 CI smoke 复核。

---

## Phase 4 任务清单

### Task P4-A: 抽取共享 auth / audit / logging 能力

**Files:**
- Modify: `gateway/internal/middleware/chain.go`
- Modify: `platform-token-runtime/internal/auth/middleware/token_auth_middleware.go`
- Modify: `gateway/internal/pkg/logging/logger.go`
- Modify: `platform-token-runtime/internal/pkg/logging/logger.go`
- Modify: `supply-api/internal/pkg/logging/logger.go`
- Create: `internal/shared/auth/README.md`
- Create: `internal/shared/logging/README.md`

- [ ] `P4-A-01` 盘点三个服务的 auth middleware 差异点。
  完成标准：按“可共享”和“必须保留差异”两列输出。
- [ ] `P4-A-02` 盘点三个服务的 logging 差异点。
  完成标准：列出字段名、输出格式、fatal 行为差异。
- [ ] `P4-A-03` 盘点 audit emitter/表契约差异。
  完成标准：写出当前表名和字段差异。
- [ ] `P4-A-04` 定义共享包边界。
  完成标准：明确什么放共享包，什么保留在服务内。
- [ ] `P4-A-05` 定义迁移顺序。
  完成标准：先抽 logging、再抽 auth、再抽 audit，或给出其他单一路径。
- [ ] `P4-A-06` 定义兼容适配层。
  完成标准：不允许一次性跨三个服务大爆炸改动。
- [ ] `P4-A-07` 为共享日志格式写 golden output 测试方案。
  完成标准：至少覆盖 info、warn、error、fatal。
- [ ] `P4-A-08` 为共享 auth 行为写契约测试方案。
  完成标准：至少覆盖 missing bearer、invalid token、inactive token。

### Task P4-B: 拆分大文件并收口未接入能力

**Files:**
- Modify: `supply-api/internal/httpapi/supply_api.go`
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/middleware/auth.go`
- Modify: `gateway/internal/app/bootstrap.go`
- Modify: `gateway/README.md`

- [ ] `P4-B-01` 记录 `supply_api.go` 的 handler 分区。
  完成标准：至少分账户、套餐、结算、收益、审计。
- [ ] `P4-B-02` 记录 `runtime.go` 的职责分区。
  完成标准：至少分输入解析、资源初始化、store bundle、security bundle、api bundle。
- [ ] `P4-B-03` 记录 `auth.go` 的职责分区。
  完成标准：至少分 query reject、bearer extract、verify、error helpers。
- [ ] `P4-B-04` 记录 `gateway` 实验能力清单。
  完成标准：标出已接入与未接入模块。
- [ ] `P4-B-05` 决策未接入路由能力“删除/迁移到 experimental/正式接入”三选一。
  完成标准：只保留一个决策。
- [ ] `P4-B-06` 写 `InvariantChecker` 的去留决策。
  完成标准：明确“接入真实写路径”还是“删除”。
- [ ] `P4-B-07` 如果接入，定义接入点顺序。
  完成标准：至少包含 account、package、settlement 三类写路径。
- [ ] `P4-B-08` 如果删除，写删除清单和回归测试清单。
  完成标准：包含代码、测试、README 三类位置。

### Task P4-C: 补齐 IAM 的 tenant / operator / scope 闭环

**Files:**
- Modify: `supply-api/internal/iam/handler/iam_handler.go`
- Modify: `supply-api/internal/iam/middleware`
- Modify: `supply-api/internal/app/runtime.go`
- Modify: `supply-api/internal/app/bootstrap.go`
- Create: `docs/plans/2026-04-21-iam-closure-checklist.md`

- [ ] `P4-C-01` 标记 IAM handler 中所有直接写死 `tenantID` 或上下文占位注释的位置。
  完成标准：清单覆盖全部待收口点。
- [ ] `P4-C-02` 定义 IAM 路由消费的 principal 字段。
  完成标准：至少包含 tenant、operator、role、scope。
- [ ] `P4-C-03` 定义 IAM middleware 链顺序。
  完成标准：明确 bearer extract、verify、scope check、operator inject 顺序。
- [ ] `P4-C-04` 标记当前启动链缺少 scope middleware 的位置。
  完成标准：清单写出具体注册入口。
- [ ] `P4-C-05` 设计 IAM handler 的错误码口径。
  完成标准：区分 unauthenticated、unauthorized、tenant mismatch。
- [ ] `P4-C-06` 设计 IAM contract tests。
  完成标准：至少覆盖 viewer、operator、admin 三个角色。
- [ ] `P4-C-07` 设计 IAM 审计事件字段。
  完成标准：至少包含 operator_id、tenant_id、action、result_code。
- [ ] `P4-C-08` 写 IAM 闭环检查清单。
  完成标准：只有 principal、scope、audit、tests 四者齐全才算完成。

### Task P4-D: 清理 secret 治理和 KMS 误导性表述

**Files:**
- Modify: `scripts/supply-gate/.env`
- Modify: `scripts/devtest/start_dev_stack.sh`
- Modify: `supply-api/internal/security/kms_service.go`
- Modify: `supply-api/config/config.dev.yaml`
- Modify: `gateway/internal/config/config.go`
- Create: `docs/plans/2026-04-21-secret-and-kms-governance-checklist.md`

- [ ] `P4-D-01` 检查被 git 跟踪的 `.env` 和本地凭据文件。
  完成标准：形成“必须去跟踪”和“可保留模板”两列清单。
- [ ] `P4-D-02` 把 `scripts/supply-gate/.env` 转成模板或移出版本控制方案。
  完成标准：文档中明确迁移方式。
- [ ] `P4-D-03` 盘点 `start_dev_stack.sh` 中的默认密码和默认 secret。
  完成标准：全部列入治理清单。
- [ ] `P4-D-04` 设计“本地默认 secret 只允许通过显式 dev profile 注入”的规则。
  完成标准：规则可落到脚本和 README。
- [ ] `P4-D-05` 重写 `kms_service.go` 的能力说明。
  完成标准：文案中明确这是 local envelope demo，不是生产 KMS。
- [ ] `P4-D-06` 定义真正生产 KMS 的接入前置条件。
  完成标准：至少包含 provider、auth、rotation、audit、failure mode。
- [ ] `P4-D-07` 盘点 `gateway` 默认加密 key 的保留理由与替代方案。
  完成标准：决策文档中只保留一个替代方案。
- [ ] `P4-D-08` 写 secret 与 KMS 治理检查清单。
  完成标准：包含“模板、注入、轮换、审计、回收”五类项。

## 6. 阶段退出验证清单

### Phase 1 Exit Gate

- [ ] authority 决策已写入规范和 README。
- [ ] schema、model、audit 字段对齐决策已确定。
- [ ] gateway 与 supply-api 的迁移方案和回滚方案已成文。
- [ ] contract tests 场景清单已形成并进入 CI 设计。

### Phase 2 Exit Gate

- [ ] `run_id + manifest` 规则已形成。
- [ ] real staging 成为唯一硬门禁。
- [ ] `DEFERRED` 不再计入通过率。
- [ ] staging/prod 模板不再复用 dev 配置。
- [ ] cross-service smoke 已被纳入发布链。

### Phase 3 Exit Gate

- [ ] remote runtime 有超时、缓存和指标设计。
- [ ] router 有自动恢复设计。
- [ ] 三服务统一 observability surface 已定义。
- [ ] worker 启动、退出和 shutdown 纪律已可验证。

### Phase 4 Exit Gate

- [ ] shared package 边界已定义。
- [ ] 大文件拆分顺序已成文。
- [ ] IAM 闭环路径已定义。
- [ ] tracked `.env`、默认 secret、KMS 表述风险都已有治理路径。

## 7. 任务清单复审

### 7.1 复审方法

本次复审按五个维度检查每个阶段是否闭环：

1. 设计闭环：是否先明确单一规则，而不是边改边猜。
2. 实现闭环：是否覆盖会改动的代码、脚本、配置和文档。
3. 验证闭环：是否为阶段目标设计了可执行 gate。
4. 发布闭环：是否考虑回滚、兼容窗口和放量顺序。
5. 治理闭环：是否考虑长期维护、README、模板和审计口径。

### 7.2 复审结论

**Phase 1 复审结论：可以达成。**

原因：

- 已覆盖 authority 决策、schema 对齐、实现收敛、contract gate。
- 已包含回滚和兼容窗口任务。
- 不再只停留在“代码该怎么改”，而是先收敛规范和契约。

前提：

- 执行时必须坚持“schema 保留或删除二选一”，不能双轨。

**Phase 2 复审结论：可以达成。**

原因：

- 已覆盖 run_id、manifest、real staging、环境模板、真实 smoke。
- 已补上 override 机制和 `DEFERRED` 口径调整。

前提：

- 执行时必须同时修改所有消费旧工件查找逻辑的脚本，不能只改主流水线。

**Phase 3 复审结论：可以达成。**

原因：

- 已覆盖 remote runtime、router 恢复、metrics/trace、worker shutdown。
- 已把“离线报告推导”替换为“运行时采集”纳入目标。

前提：

- 执行时必须把指标出口和 CI 采集脚本一起改，否则会留下半成品。

**Phase 4 复审结论：可以达成。**

原因：

- 已覆盖共享能力抽取、大文件拆分、IAM 闭环、secret/KMS 治理。
- 已把“未接入能力”与“误导性文案”都纳入治理，不只是改代码。

前提：

- 执行时必须遵循“小步迁移 + 兼容适配层”，不能跨多个服务一次性重构。

### 7.3 复审后补充的强制约束

为保证这份清单能彻底达成四阶段目标，执行时必须额外遵守以下约束：

1. 每个阶段结束前，必须先通过该阶段 Exit Gate，再开始下一阶段。
2. 每个阶段至少保留一次回滚演练记录。
3. 每个阶段的关键决策必须同步更新 README 或规范文档。
4. 任何“先改代码，之后补文档”的做法都视为阶段未完成。

### 7.4 最终判断

在满足以下三个条件时，这份任务清单足以彻底达成各阶段目标：

1. 执行顺序严格按 Phase 1 -> Phase 2 -> Phase 3 -> Phase 4 推进。
2. 每个阶段都先完成规范与 gate，再进入实现。
3. 所有脚本、配置、README、契约测试都跟着代码一起收口。

如果执行中跳过任一类任务，这份清单就只能“局部改善”，不能“彻底达成”。

## 8. 推荐执行顺序

1. 先完成 `P1-A` 到 `P1-D`，确认身份和契约真源。
2. 再完成 `P2-A` 到 `P2-D`，确认发布和环境真源。
3. 再完成 `P3-A` 到 `P3-D`，补齐性能和可观测性。
4. 最后完成 `P4-A` 到 `P4-D`，做结构和治理收口。

## 9. 建议的执行节奏

1. 每次只执行一个 Task Group。
2. 每完成一个 Task Group 就提交一次中间文档或代码。
3. 每完成一个 Phase 就做一次人工复审和一次脚本复核。
