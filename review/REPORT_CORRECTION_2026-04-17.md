# 报告纠偏版

- 项目：立交桥
- 路径：`/home/long/project/立交桥`
- 日期：2026-04-17
- 目的：核验 `review/` 目录下 2026-04-16 审查报告中的结论是否符合当前代码真实状态，并输出纠偏后的问题清单。

---

## 一、核验范围与方法

本次纠偏只基于当前仓库事实，不沿用旧报告的结论。

核验对象：

1. `SYSTEMATIC_REVIEW_REPORT_2026-04-16.md`
2. `code_quality_supply_api_2026-04-16.md`
3. `code_quality_gateway_2026-04-16.md`
4. `code_quality_token_runtime_2026-04-16.md`
5. `code_quality_sql_2026-04-16.md`
6. `code_quality_python_2026-04-16.md`
7. `ARCHITECTURAL_ANALYSIS_2026-04-16.md`
8. `ARCHITECTURAL_ANALYSIS_FRAMEWORK_2026-04-16.md`
9. `CONVENTION_CONSISTENCY_2026-04-16.md`

实际核验动作：

1. 阅读三块核心服务当前代码：`gateway`、`platform-token-runtime`、`supply-api`
2. 运行无缓存测试复核：
   - `cd gateway && go test -count=1 ./...`
   - `cd platform-token-runtime && go test -count=1 ./...`
   - `cd supply-api && go test -count=1 ./...`
3. 运行 `supply-api` 仓储集成基线：
   - `cd supply-api && bash scripts/run_integration_tests.sh ./internal/repository`
4. 对照 README、入口装配、路由挂载、配置门禁、持久化能力与测试夹具

本报告使用三个分类：

1. `已证实`：当前代码和命令能直接证明属实
2. `已过时`：报告曾经可能属实，但当前代码已修复或结论不再成立
3. `漏报`：旧报告没有抓到，但当前代码中真实存在

---

## 二、纠偏后的总体结论

旧报告的总体方向并非完全错误，但**不能作为当前真实状态的直接依据**。

更准确的结论是：

1. 仓库当前仍然**不可作为生产就绪版本判断为通过**。
2. 主要原因不是旧报告列出的全部 P0 都还存在，而是：
   - 三块核心服务里有两块测试已经和代码失配
   - `supply-api` 在当前工具链下存在编译阻塞
   - `platform-token-runtime` 仍然没有仓库内可直接上线的持久化实现
   - `supply-api` 补偿、IAM 接入等能力仍未闭环
3. SQL 报告相对可信，Python / framework / architecture 类报告噪音较大，混入了竞品与归档目录，不能直接映射到核心项目完成度。

### 2.1 执行更新（2026-04-17）

本纠偏报告形成后，整改执行清单中的 `Task 1` 到 `Task 12` 已经完成并分别提交。当前状态与本报告前文提到的“初始阻塞项”已经不同：

1. `supply-api` 编译阻塞已修复，整仓 `go test -count=1 ./...` 已恢复
2. `gateway` 与 `platform-token-runtime` 的测试失配已修复，当前无缓存回归已恢复
3. `platform-token-runtime` 已具备 PostgreSQL-backed 的最小 runtime / audit store
4. `supply-api` 的补偿执行器、IAM 显式挂载、提现 / SMS readiness 门禁已经收口
5. `gateway` 的默认密钥 / 默认 CORS 已改为生产态 fail-closed，`/v1/models` 也已切到真实 provider 聚合输出

因此，阅读本报告时应区分：

1. 第三到第五章记录的是“纠偏启动时的真实问题”
2. 第七章给出的是“执行更新后的剩余治理项”

---

## 三、已证实

### 3.1 P0：`supply-api` 当前无法通过 `go test -count=1 ./...`

证据：

1. 当前 `go version` 为 `go1.22.2`
2. `supply-api/internal/security/kms_service.go` 引入了 `crypto/hkdf`
3. 当前环境下执行 `cd supply-api && go test -count=1 ./...` 失败，报错：
   `package crypto/hkdf is not in std`

代码位置：

- `supply-api/internal/security/kms_service.go:7`

影响：

1. `supply-api` 当前不是“测试待补齐”，而是**基础构建已断裂**
2. 任何覆盖率、质量评分、上线判断都必须让位于“先恢复可编译”

纠偏说明：

- 旧报告提到过 KMS 派生算法问题，但**没有识别到当前更直接的构建阻塞**

### 3.2 P0：`gateway` 测试套件已与实现失配

证据：

1. `BuildMux` 当前签名为：
   `BuildMux(h, limiter, authConfig, corsConfig)`
2. `cmd/gateway/main_test.go` 仍按旧签名调用：
   `app.BuildMux(h, limiter, authConfig)`
3. 执行 `cd gateway && go test -count=1 ./...` 失败

代码位置：

- `gateway/internal/app/bootstrap.go:75`
- `gateway/cmd/gateway/main_test.go:89`
- `gateway/cmd/gateway/main_test.go:122`
- `gateway/cmd/gateway/main_test.go:157`

影响：

1. `gateway` 当前“测试通过”的旧结论不成立
2. 发布前无法信任该模块的自动验证链路

### 3.3 P0：`platform-token-runtime` 测试套件已与实现失配

证据一：中间件签名变化未同步到测试

1. `QueryKeyRejectMiddleware` 当前需要 `trustedProxies []string`
2. 测试仍按三参数调用
3. `cd platform-token-runtime && go test -count=1 ./...` 直接构建失败

代码位置：

- `platform-token-runtime/internal/auth/middleware/query_key_reject_middleware.go:13`
- `platform-token-runtime/internal/auth/middleware/token_auth_middleware_test.go:62`

证据二：`audit-events` 已要求鉴权，但测试仍断言匿名访问成功

1. 当前 `handleAuditEvents` 明确要求 Bearer Token
2. 旧测试未带 `Authorization`，仍期望 `200 OK`
3. 因此无缓存测试还会在 `internal/httpapi` 继续失败

代码位置：

- `platform-token-runtime/internal/httpapi/token_api.go:337`
- `platform-token-runtime/internal/httpapi/token_api_test.go:208`
- `platform-token-runtime/internal/httpapi/token_api_test.go:239`
- `platform-token-runtime/internal/httpapi/token_api_test.go:257`

影响：

1. `platform-token-runtime` 当前不是“七个包稳定通过”，而是**测试语义已落后于实现**
2. 旧报告没有把这类验证链路断裂识别为一类独立风险

### 3.4 P0：`platform-token-runtime` 仍无仓库内可直接上线的持久化 store

证据：

1. `BuildRuntime` 在 `prod/staging` 下要求显式注入 `RuntimeStore` 与 `AuditStore`
2. 当前仓库提供的仍然是内存实现类型
3. 仓库内没有完成的 PostgreSQL / Redis / 其他持久化 runtime store 装配

代码位置：

- `platform-token-runtime/internal/app/bootstrap.go:39`
- `platform-token-runtime/internal/app/bootstrap.go:40`
- `platform-token-runtime/internal/app/bootstrap.go:43`

影响：

1. 即使 HTTP API 本身可跑，也仍不具备 staging/prod 上线闭环
2. 这是真实的上线阻塞项

### 3.5 P1：`supply-api` 批量补偿链路存在空实现

证据：

1. 默认补偿执行器四类 handler 都只打日志并 `return nil`
2. 代码注释明确写着“实际实现”

代码位置：

- `supply-api/internal/compensation/compensation.go:70`
- `supply-api/internal/compensation/compensation.go:82`
- `supply-api/internal/compensation/compensation.go:94`
- `supply-api/internal/compensation/compensation.go:106`

影响：

1. 补偿框架、后台 worker、状态流转已经存在
2. 但真正的回滚动作没有落地，故障恢复能力仍不闭环

### 3.6 P1：`supply-api` 的 IAM 代码已写出，但主服务未挂载

证据：

1. `IAMHandler.RegisterRoutes` 已定义完整路由
2. `BuildServer` 仅注册了 health、supply、alert
3. 主运行时没有构建 IAM handler，也没有挂载对应路由

代码位置：

- `supply-api/internal/iam/handler/iam_handler.go:72`
- `supply-api/internal/app/bootstrap.go:146`

影响：

1. 这不是“没有 IAM 代码”，而是“已有实现未接入主服务”
2. 从验收角度看，IAM 仍不能算已交付能力

### 3.7 P1：`gateway` 默认加密密钥回退仍然存在

证据：

1. `PASSWORD_ENCRYPTION_KEY` 未设置时，直接使用硬编码默认值

代码位置：

- `gateway/internal/config/config.go:18`

影响：

1. 生产环境若误用默认值，会导致密钥管理失守
2. 这是可信的安全问题

纠偏说明：

- 这条在旧报告中成立，结论保留

### 3.8 P1：`gateway` 默认 CORS 配置仍然过宽，但旧报告的风险表述偏重

证据：

1. 默认 `AllowOrigins` 为 `*`
2. 但 `AllowCredentials` 为 `false`
3. 实际风险是浏览器侧暴露面过宽，而不是旧报告所写的典型 CSRF 结论

代码位置：

- `gateway/internal/middleware/cors.go:23`
- `gateway/internal/middleware/cors.go:27`

影响：

1. 不适合作为 P0 级别安全结论
2. 仍建议在非开发环境收口为显式白名单

### 3.9 P2：`gateway` 的 `/v1/models` 仍为硬编码静态列表

证据：

1. 响应内容直接写死在 handler 中
2. 没有按已注册 provider 或已配置模型动态生成

代码位置：

- `gateway/internal/handler/handler.go:274`

影响：

1. 对外行为与实际 provider 配置可能不一致
2. 会误导调用方对真实可用模型的认知

### 3.10 P2：`gateway` 中高级路由策略代码存在，但未进入主启动链路

证据：

1. bootstrap 仅走 `router.NewRouter(resolveStrategy(...))`
2. `cost_based` / `cost_aware` / `engine` / `fallback` 代码存在
3. 这些能力没有在当前主装配链路中生效

代码位置：

- `gateway/internal/app/bootstrap.go:105`
- `gateway/internal/router/strategy/cost_based.go:84`

影响：

1. 相关模块更像“已开发的扩展能力”而不是“当前已交付主功能”
2. 旧架构报告对这部分存在感知，但没有指出“未接入主链路”这个更关键的事实

---

## 四、已过时

### 4.1 `platform-token-runtime` “Refresh TTL 不持久化”结论已过时

旧报告来源：

- `SYSTEMATIC_REVIEW_REPORT_2026-04-16.md`
- `code_quality_token_runtime_2026-04-16.md`

当前事实：

1. `Refresh` 已在修改 `record.ExpiresAt` 后调用 `r.store.Save(*record, "", "")`

代码位置：

- `platform-token-runtime/internal/auth/service/inmemory_runtime.go:145`
- `platform-token-runtime/internal/auth/service/inmemory_runtime.go:146`

纠偏结论：

- 这条不能继续保留为当前严重缺陷

### 4.2 `platform-token-runtime` “audit-events 接口无鉴权”结论已过时

旧报告来源：

- `SYSTEMATIC_REVIEW_REPORT_2026-04-16.md`

当前事实：

1. 当前 `handleAuditEvents` 已要求 Bearer Token
2. 旧测试失败正说明接口语义已经变化

代码位置：

- `platform-token-runtime/internal/httpapi/token_api.go:337`

纠偏结论：

- 当前问题不是“无鉴权”，而是“测试没有跟上已加的鉴权”

### 4.3 `supply-api` “KMS 派生仍是 SHA-256 + 固定盐”结论已过时

旧报告来源：

- `SYSTEMATIC_REVIEW_REPORT_2026-04-16.md`
- `code_quality_supply_api_2026-04-16.md`

当前事实：

1. `deriveDEK` 已切为 `HKDF-SHA256`
2. 旧报告引用的实现片段不再匹配当前代码

代码位置：

- `supply-api/internal/security/kms_service.go:200`
- `supply-api/internal/security/kms_service.go:209`

纠偏结论：

- 该安全结论已失效
- 但当前新问题变成了：代码引入 `crypto/hkdf` 后在现有工具链下无法通过测试

### 4.4 `gateway` “请求 ID 信任用户输入导致日志注入”结论已过时

旧报告来源：

- `SYSTEMATIC_REVIEW_REPORT_2026-04-16.md`

当前事实：

1. 请求 ID 已做字符白名单过滤与长度限制

代码位置：

- `gateway/internal/handler/handler.go:376`

纠偏结论：

- 旧问题已被显式修补

### 4.5 `gateway` “内部错误信息直接泄漏给客户端”结论已过时

旧报告来源：

- `SYSTEMATIC_REVIEW_REPORT_2026-04-16.md`

当前事实：

1. `writeError` 会对 `COMMON_INTERNAL_ERROR`、`COMMON_INVALID_REQUEST`、`PROVIDER_ERROR` 等错误做对外脱敏

代码位置：

- `gateway/internal/handler/handler.go:331`

纠偏结论：

- 当前真实问题变成了：测试还在断言旧行为，导致 `internal/handler` 无缓存测试失败

### 4.6 `platform-token-runtime` “go.mod 依赖完全缺失属于异常”结论不成立

旧报告来源：

- `ARCHITECTURAL_ANALYSIS_FRAMEWORK_2026-04-16.md`

当前事实：

1. `platform-token-runtime` 当前对第三方依赖需求本来就很少
2. 该服务主要由标准库和仓库内代码组成

代码位置：

- `platform-token-runtime/go.mod`

纠偏结论：

- 不能仅凭“依赖少”就认定依赖管理异常

### 4.7 “核心三服务存在 Gin 与标准库混用”结论不成立

旧报告来源：

- `ARCHITECTURAL_ANALYSIS_FRAMEWORK_2026-04-16.md`

当前事实：

1. 核心三服务中没有查到 Gin
2. 报告把 `sub2api` 等竞品/对照目录混入了核心服务架构判断

纠偏结论：

- 这条不能用于评估 `gateway` / `supply-api` / `platform-token-runtime` 的真实完成度

---

## 五、漏报

### 5.1 P0：旧报告没有识别“无缓存测试失败”

旧报告普遍引用了“测试通过”“覆盖率正常”等表述，但没有发现：

1. `gateway` 无缓存测试失败
2. `platform-token-runtime` 无缓存测试失败
3. `supply-api` 当前直接编译失败

这意味着旧报告对“测试通过”的引用可靠性不足。

### 5.2 P0：旧报告没有识别验证脚本本身的盲区

证据：

1. `scripts/ci/repo_integrity_check.sh` 只执行：
   - `gateway test ./...`
   - `platform-token-runtime test ./...`
   - `supply-api test ./...`
   - `supply-api test -tags=e2e ./e2e`
2. 没有强制无缓存
3. 没有跑 `integration` tag 的仓储集成基线

代码位置：

- `scripts/ci/repo_integrity_check.sh:26`
- `scripts/ci/repo_integrity_check.sh:29`

影响：

1. 旧报告如果只参考统一脚本，很可能会漏掉当前真实问题
2. 当前验证体系不能单独作为“项目健康”依据

### 5.3 P1：旧报告没有识别 `supply-api` 的 e2e 大量依赖 mock 夹具

证据：

1. `newE2ESystem` 中账户、套餐、结算、收益服务均为测试替身
2. `idempotency` 关闭
3. `rateLimit` 关闭
4. `alert` 使用内存实现

代码位置：

- `supply-api/e2e/e2e_test.go:163`
- `supply-api/e2e/e2e_test.go:177`
- `supply-api/e2e/e2e_test.go:210`
- `supply-api/e2e/e2e_test.go:216`

影响：

1. 当前 e2e 更接近“带认证的 HTTP 流程回归”
2. 不能等价为真实数据库、Redis、短信、消息总线全部闭环

### 5.4 P1：旧报告没有识别 `supply-api` 提现能力当前是门禁关闭态，而非待上线态

证据：

1. 默认 SMS verifier 拒绝所有验证码
2. 运行时通过 `withdrawEnabled` 进行显式门禁
3. prod 配置校验也明确禁止开启提现，直到短信链路生产就绪

代码位置：

- `supply-api/internal/domain/settlement.go:167`
- `supply-api/internal/httpapi/supply_api.go:765`
- `supply-api/internal/config/config.go:309`

影响：

1. 这不是一个普通 bug
2. 这是当前能力边界和上线状态的明确信号

### 5.5 P1：旧报告没有识别 `gateway` 的能力宣传与主链路实现存在差距

表现：

1. 纠偏启动时，`models` 为硬编码
2. 成本/感知/回退等策略代码未进入主装配链路

影响：

1. 会造成“仓库里有代码”被误判成“主功能已完成”
2. 其中 `/v1/models` 已在后续整改中修复；高级路由策略未进入主装配链路仍然成立

### 5.6 P2：旧报告没有区分“规范问题”和“上线阻塞问题”

例如：

1. `ClientIP` / `SourceIP` 混用是真实的维护性问题
2. 但它不应与“编译失败”“测试失配”“缺少持久化 store”放在同一优先级讨论

---

## 六、按报告逐份修正后的可信度

| 报告 | 可信度 | 纠偏结论 |
|---|---|---|
| `SYSTEMATIC_REVIEW_REPORT_2026-04-16.md` | 低到中 | 有真实问题，但严重项中包含多条已过时结论，且漏掉当前更直接的构建/测试断裂 |
| `code_quality_supply_api_2026-04-16.md` | 中 | 数据库与门禁观察有价值，但漏报编译阻塞、补偿空实现、IAM 未挂载 |
| `code_quality_gateway_2026-04-16.md` | 中 | 安全与配置问题部分成立，但未识别测试失配和主链路能力缺口 |
| `code_quality_token_runtime_2026-04-16.md` | 低 | 至少两条关键结论已经过时，同时漏掉当前测试失配 |
| `code_quality_sql_2026-04-16.md` | 高 | 大方向基本可信，但应明确区分当前基线与历史/归档 SQL |
| `code_quality_python_2026-04-16.md` | 低相关 | 范围掺入竞品与归档目录，对核心项目完成度参考价值有限 |
| `ARCHITECTURAL_ANALYSIS_2026-04-16.md` | 中低 | 可作为重构输入，不适合作为事实缺陷报告 |
| `ARCHITECTURAL_ANALYSIS_FRAMEWORK_2026-04-16.md` | 低 | 存在前提错误，把竞品/对照目录混入核心项目架构判断 |
| `CONVENTION_CONSISTENCY_2026-04-16.md` | 中 | 规范治理价值存在，但不应替代生产 readiness 判断 |

---

## 七、执行更新后的剩余优先级

### 已完成（2026-04-17）

1. `supply-api` 编译恢复，`go test -count=1 ./...` 已通过
2. `gateway` / `platform-token-runtime` 无缓存测试链路已恢复
3. `repo_integrity_check.sh` 已切到可信基线
4. `platform-token-runtime` 最小 PostgreSQL 持久化已落地
5. `supply-api` 补偿执行器、IAM、提现 / SMS readiness 已收口
6. `gateway` 默认密钥 / 默认 CORS 已收紧，`/v1/models` 已改为动态输出

### 剩余项

#### P2

1. 收敛 SQL 基线边界：
   - 明确 platform / supply / token-runtime 三套 DDL 的服务边界
   - 把历史迁移脚本和 fresh setup 基线分开说明
2. 用当前真实领域状态集合补齐 `supply_core_schema_v2.sql` 的 `CHECK`
3. 将 `ClientIP` / `SourceIP` 的统一方向固化为规范：
   - 审计 / 持久化域统一 `SourceIP`
   - 入口请求局部上下文允许保留 `ClientIP`

---

## 八、最终纠偏结论

如果只问一句“这些报告反馈是否真实”，更准确的回答是：

1. **部分真实，但不能直接信。**
2. **最容易误导的是旧报告里的严重问题列表。**
3. **当前最真实的阻塞项不是那些已过时的安全结论，而是构建/测试断裂、持久化缺失和能力未闭环。**

因此，后续所有治理和排期建议都应以本纠偏版为准，而不是继续直接引用 2026-04-16 的原始报告。
