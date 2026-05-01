# 立交桥项目真实状态评审报告

- 评审日期：2026-04-24
- 评审范围：`gateway/`、`platform-token-runtime/`、`supply-api/`、`scripts/ci/`、`tests/`
- 评审基线：当前工作区实时状态（非干净工作树）
- 评审方法：仓库级 gate、模块级测试、稳定性回归、readiness 检查、脚本实现审查、契约对照审查

## 一、执行摘要

当前仓库已经具备较完整的单服务测试与部分仓储级验证能力，但“真实可放行状态”并不成立。最关键的问题不在业务代码主干，而在发布门禁与跨服务契约验证链：`repo_integrity_check.sh` 在单服务测试通过后，会继续依赖一个实现错误且存在假阳性的 contract gate；因此仓库当前对“跨服务契约已验证”的表达并不可信。

从已执行的验证结果看：

1. `gateway`、`platform-token-runtime`、`supply-api` 的单服务 Go 测试通过。
2. `supply-api` 仓储集成测试通过。
3. `supply-api` 领域稳定性回归 5 轮通过。
4. 仓库级 `repo_integrity_check.sh` 在 Phase 1 contract gate 失败。
5. `token_runtime_readiness_check.sh` 结果为 `FAIL (11/13)`。
6. `staging_real_readiness_check.sh` 结果为 `BLOCKED`，当前默认环境被识别为 `local-mock`。
7. `dependency-audit-check.sh` 因缺少当日工件直接失败，无法证明依赖审计已完成。

综合判断：项目当前更接近“单服务质量基线尚可，但跨服务放行链路和发布证据体系不可靠”的状态，不应把当前状态包装成已完成生产级联调验证。

## 二、执行过的验证

### 2.1 仓库级与模块级验证

执行命令：

```bash
bash scripts/ci/repo_integrity_check.sh
bash scripts/ci/token_runtime_readiness_check.sh
bash scripts/ci/staging_real_readiness_check.sh
bash scripts/ci/dependency-audit-check.sh
bash scripts/ci/supply_domain_stability_check.sh 5
```

结果摘要：

| 验证项 | 结果 | 关键证据 |
|---|---|---|
| 仓库级完整性 gate | FAIL | `reports/archive/gate_verification/repo_integrity_contract_gate_20260424_100216.log` |
| gateway 全量 Go 测试 | PASS | `repo_integrity_check.sh` 执行输出 |
| platform-token-runtime 全量 Go 测试 | PASS | `repo_integrity_check.sh` 执行输出 |
| supply-api 单元测试 | PASS | `repo_integrity_check.sh` 执行输出 |
| supply-api 仓储集成测试 | PASS | `repo_integrity_check.sh` 执行输出 |
| supply-api service-http 测试 | PASS | `repo_integrity_check.sh` 执行输出 |
| contract gate 场景汇总 | FAIL | `reports/archive/gate_verification/contract_gate_2026-04-24_100216.md` |
| token runtime readiness | FAIL | `reports/archive/gate_verification/token_runtime_readiness_2026-04-24_100355.md` |
| staging 真实环境 readiness | BLOCKED | `reports/archive/gate_verification/staging_real_readiness_2026-04-24_100355.md` |
| 依赖审计门禁 | FAIL | `scripts/ci/dependency-audit-check.sh` 输出 |
| supply-api 领域稳定性 5 轮 | PASS | `bash scripts/ci/supply_domain_stability_check.sh 5` 输出 |

### 2.2 当前工作树状态

本次评审不是基于干净工作树。`git status --short` 显示存在已修改文件、未跟踪目录和两个未忽略的本地 ELF 二进制：

- `gateway/gateway`
- `supply-api/supply-api`

这说明当前仓库还带有本地构建产物和正在进行中的改动，评审结论反映的是“当前真实工作区状态”，不是某个已收敛提交的冻结快照。

## 三、主要发现

### P0-1：Phase 1 contract gate 存在假阳性，`SKIP` 与“best-effort”会被记成 `PASS`

严重性：高

影响：

- 直接削弱 `repo_integrity_check.sh` 的发布门禁可信度。
- 即使跨服务场景没有实际跑通，也可能被报告写成通过。
- 会把“缺证据”错误包装成“已验证”，违背项目要求的生产质量闭环。

证据：

1. `scripts/ci/backend-verify.sh` 在场景 2、3 结束后，用 `$(cat "${s2_log}") == "FAIL"` / `"SKIP"*` 做整文件精确匹配，但场景日志本身包含多行调试输出，几乎不可能只等于一个字面量；不匹配时直接落入 `PASS` 分支。[scripts/ci/backend-verify.sh](/home/long/project/立交桥/scripts/ci/backend-verify.sh:217) [scripts/ci/backend-verify.sh](/home/long/project/立交桥/scripts/ci/backend-verify.sh:279)
2. 场景 4 无论验证是否成立，汇总都被无条件写成 `PASS`。[scripts/ci/backend-verify.sh](/home/long/project/立交桥/scripts/ci/backend-verify.sh:332)
3. 本次实测中，场景 2 和场景 3 的证据日志都明确写了 `SKIP (cannot create token)`。[contract_scenario2_2026-04-24_100216.log](/home/long/project/立交桥/reports/archive/gate_verification/contract_scenario2_2026-04-24_100216.log:1) [contract_scenario3_2026-04-24_100216.log](/home/long/project/立交桥/reports/archive/gate_verification/contract_scenario3_2026-04-24_100216.log:1)
4. 但汇总报告仍把场景 2 和场景 3 标成 `PASS`。[contract_gate_2026-04-24_100216.md](/home/long/project/立交桥/reports/archive/gate_verification/contract_gate_2026-04-24_100216.md:15)

结论：

当前 contract gate 不是严格门禁，而是“可能误报通过”的门禁。这个问题必须先修，否则后续跨服务验证报告没有决策价值。

### P0-2：contract gate 调用的 token runtime 契约与真实实现不一致，脚本本身无法验证真实链路

严重性：高

影响：

- 即使三个服务都正常运行，当前 gate 也无法按真实契约完成 issue / introspect / revoke 流程。
- contract gate 失败不能直接说明业务链路有问题，因为 gate 自己先违背了 API 契约。
- 这会造成“脚本失败”和“系统失败”混淆，严重污染评审与放行口径。

证据：

1. 脚本创建 token 时调用 `POST /api/v1/platform/tokens`，但真实接口注册的是 `POST /api/v1/platform/tokens/issue`。[scripts/ci/backend-verify.sh](/home/long/project/立交桥/scripts/ci/backend-verify.sh:97) [platform-token-runtime/internal/httpapi/token_api.go](/home/long/project/立交桥/platform-token-runtime/internal/httpapi/token_api.go:54)
2. 真实 `issue` 接口强制要求 `X-Request-Id` 和 `Idempotency-Key`，但脚本没有传这两个头。[platform-token-runtime/internal/httpapi/token_api.go](/home/long/project/立交桥/platform-token-runtime/internal/httpapi/token_api.go:116)
3. 脚本按根字段读取 `token_id`，而真实返回是 `data.token_id`。[scripts/ci/backend-verify.sh](/home/long/project/立交桥/scripts/ci/backend-verify.sh:106) [platform-token-runtime/internal/httpapi/token_api.go](/home/long/project/立交桥/platform-token-runtime/internal/httpapi/token_api.go:150)
4. 脚本调用 introspect 时发送的是 `{"token_id":"..."}`，但真实接口字段是 `{"token":"..."}`，而且还要求 `X-Request-Id`。[scripts/ci/backend-verify.sh](/home/long/project/立交桥/scripts/ci/backend-verify.sh:117) [platform-token-runtime/internal/httpapi/token_api.go](/home/long/project/立交桥/platform-token-runtime/internal/httpapi/token_api.go:264)
5. 脚本撤销 token 时使用 `DELETE /api/v1/platform/tokens/{id}`，但真实契约是 `POST /api/v1/platform/tokens/{id}/revoke`。[scripts/ci/backend-verify.sh](/home/long/project/立交桥/scripts/ci/backend-verify.sh:188) [platform-token-runtime/internal/httpapi/token_api.go](/home/long/project/立交桥/platform-token-runtime/internal/httpapi/token_api.go:61)
6. 对照测试已经明确固化了真实调用方式：`/issue`、`/introspect` 的 `token` 字段、`/{tokenId}/revoke` 与所需 headers。[platform-token-runtime/internal/httpapi/token_api_test.go](/home/long/project/立交桥/platform-token-runtime/internal/httpapi/token_api_test.go:33)

结论：

当前 Phase 1 contract gate 不是“真实链路验证失败”，而是“验证器没有遵守被验证系统的契约”。修 gate 之前，任何基于该脚本的 release 结论都不可靠。

### P1-1：`repo_integrity_check.sh` 把跨服务 localhost contract gate 绑定进仓库完整性门禁，但没有自举真实服务依赖

严重性：中高

影响：

- 仓库完整性检查变成环境依赖型检查，不再是可重复、可移植的代码门禁。
- 在没有显式启动 `gateway`、`platform-token-runtime`、`supply-api` 三个进程的机器上，该 gate 天然不稳定。
- 与 `tests/contract/README.md` 中“当前仍以单服务测试为主、尚未形成硬门禁”的状态存在偏差。

证据：

1. `repo_integrity_check.sh` 在完成单服务测试后，无条件执行 `backend-verify.sh --phase1-contract-gate`。[scripts/ci/repo_integrity_check.sh](/home/long/project/立交桥/scripts/ci/repo_integrity_check.sh:44)
2. `backend-verify.sh` 默认把三端地址写死为 `127.0.0.1:18080/18081/18082`，但脚本自身没有任何服务拉起逻辑。[scripts/ci/backend-verify.sh](/home/long/project/立交桥/scripts/ci/backend-verify.sh:77)
3. 本次 `staging_real_readiness_check` 也证明当前默认“staging-real”环境其实是 `local-mock` 且 `API_BASE_URL` 不可达，不具备真实放行前提。[staging_real_readiness_2026-04-24_100355.md](/home/long/project/立交桥/reports/archive/gate_verification/staging_real_readiness_2026-04-24_100355.md:1)

结论：

建议把“代码完整性 gate”和“真实跨服务 gate”拆开。前者应保持自包含，后者应明确要求自举或显式注入运行环境，并将其产物归类为 release/staging 证据，而不是 repo integrity 证据。

### P1-2：`token_runtime_readiness_check.sh` 的 readiness 指标存在口径漂移，未跑 smoke 也记作 `PASS`

严重性：中

影响：

- readiness 百分比会高估真实可运行性。
- 在 smoke 默认关闭时，指标会把“未验证”包装成“通过”。
- 容易让评审者误以为本地可运行冒烟已经完成。

证据：

1. 默认情况下，仅当 `ENABLE_TOKEN_RUNTIME_SMOKE=1` 才真正执行本地 smoke。[scripts/ci/token_runtime_readiness_check.sh](/home/long/project/立交桥/scripts/ci/token_runtime_readiness_check.sh:112)
2. 如果未开启 smoke，脚本会直接把 `TOK-REAL-001-C8` 标为 `PASS`，证据写成 `N/A`。[scripts/ci/token_runtime_readiness_check.sh](/home/long/project/立交桥/scripts/ci/token_runtime_readiness_check.sh:167)
3. 本次 readiness 报告确实把未执行的 smoke 计为 `PASS`。[token_runtime_readiness_2026-04-24_100355.md](/home/long/project/立交桥/reports/archive/gate_verification/token_runtime_readiness_2026-04-24_100355.md:1)

补充说明：

本次同一脚本里的 Go test/build 失败，主要是它强制使用项目内 GOPATH/GOCACHE，触发了新的依赖下载，而当前沙箱网络不允许访问代理地址 `127.0.0.1:7897`；这类失败更接近“脚本不够自洽/环境不自包含”，不等价于模块本身不可编译。[scripts/ci/token_runtime_readiness_check.sh](/home/long/project/立交桥/scripts/ci/token_runtime_readiness_check.sh:68) [token_runtime_go_test_2026-04-24_100355.log](/home/long/project/立交桥/reports/archive/gate_verification/token_runtime_go_test_2026-04-24_100355.log:1)

### P2-1：依赖审计产物在当天基线缺失，无法形成当日供应链审计闭环

严重性：中

影响：

- 无法证明当日依赖 SBOM、锁文件差异、兼容矩阵、风险登记已经更新。
- `dependency-audit-check.sh` 只能验证产物存在，不负责生成，因此当前流水线在“生成”和“验证”之间存在空档。

证据：

1. `dependency-audit-check.sh` 本次执行直接因为缺少 `2026-04-24` 的四个工件而失败。
2. 当前仓库中只有 `2026-03-27` 的历史依赖审计工件，缺少今日快照。

结论：

如果团队把依赖审计当作 release gate，一定要把“生成当日工件”纳入同一条流水线；否则这个 gate 只能阻断，不能真正提供审计证明。

## 四、正向结论

以下部分在本次评审中表现正常，应作为后续整改的保留基础：

1. `gateway` 全量 Go 测试通过，入口层回归基线存在。
2. `platform-token-runtime` 在仓库级 gate 中通过全量 Go 测试，说明其主代码树在当前缓存环境下可编译、可测试。
3. `supply-api` 单元测试、仓储集成测试、service-http 测试均通过，说明服务内主链路质量基础优于脚本门禁质量。
4. `supply-api/internal/domain` 连续 5 轮稳定性回归通过，未观察到显性随机失败。

## 五、风险评估

### 当前最主要风险

1. 决策层可能会误把“脚本报告 PASS”当成“真实跨服务契约已验证”。
2. repo integrity 与 release/staging evidence 混在一起，导致失败原因不清、修复优先级失真。
3. readiness / dependency audit 的证据链还没有形成当天闭环，无法支撑生产口径的审计要求。

### 如果本周要继续推进生产化

优先级建议：

1. 先修 `scripts/ci/backend-verify.sh` 的契约路径、请求头、响应解析和结果归档逻辑。
2. 再把 `repo_integrity_check.sh` 中的跨服务 gate 从“默认本地 hard gate”改为“显式环境驱动的 release gate”。
3. 调整 `token_runtime_readiness_check.sh`，把未执行 smoke 标成 `SKIP` 而不是 `PASS`。
4. 补齐依赖审计工件生成链路，确保当天有可追溯产物。
5. 在真实或可自举的三服务环境下，重新跑一次 contract gate 与 cross-service smoke，生成新的放行证据。

## 六、最终结论

立交桥项目当前不能被定义为“跨服务主链路已经严格验证完成”。真实状态更准确的表述是：

- 单服务代码质量与部分集成验证已有基础；
- 跨服务契约 gate 的实现存在明显缺陷；
- staging / release 证据体系尚未闭环；
- 现阶段适合继续做门禁和验证链路整改，不适合以“已完成生产级验证”对外口径宣称。

