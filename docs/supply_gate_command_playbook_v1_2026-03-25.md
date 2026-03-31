# SUP Gate 命令级执行清单（SUP-004~SUP-007）

- 版本：v1.0
- 日期：2026-03-25
- 目标：为测试团队提供可直接执行的命令清单，输出可回填证据
- 关联任务：`SUP-004`、`SUP-005`、`SUP-006`、`SUP-007`
- 关联报告模板：
  - `tests/supply/ui_sup_acc_report_2026-03-28.md`
  - `tests/supply/ui_sup_pkg_report_2026-03-29.md`
  - `tests/supply/ui_sup_set_report_2026-03-29.md`
  - `tests/supply/sec_sup_boundary_report_2026-03-30.md`

---

## 1. 执行前准备

## 1.1 环境变量

在项目根目录创建并编辑 `scripts/supply-gate/.env`：

```bash
API_BASE_URL="https://staging.example.com"
OWNER_BEARER_TOKEN="replace-me-owner-token"
VIEWER_BEARER_TOKEN="replace-me-viewer-token"
ADMIN_BEARER_TOKEN="replace-me-admin-token"

# 测试数据（可按环境替换）
TEST_PROVIDER="openai"
TEST_MODEL="gpt-4o"
TEST_ACCOUNT_ALIAS="sup_acc_cmd"
TEST_CREDENTIAL_INPUT="sk-test-replace-me"
TEST_PAYMENT_METHOD="alipay"
TEST_PAYMENT_ACCOUNT="tester@example.com"
TEST_SMS_CODE="123456"

# 可选：绕过平台直连供应方探测目标
SUPPLIER_DIRECT_TEST_URL=""
```

## 1.2 依赖检查

```bash
command -v curl >/dev/null
command -v jq >/dev/null
```

## 1.3 执行入口

```bash
cd "/home/long/project/立交桥"
bash "scripts/supply-gate/run_all.sh"
```

真实 staging 推荐使用（含占位值与可达性预检）：

```bash
cd "/home/long/project/立交桥"
bash "scripts/supply-gate/staging_precheck_and_run.sh" "scripts/supply-gate/.env"
```

说明：
1. `staging_precheck_and_run.sh` 默认会先执行 `M-021` 预检（token runtime readiness）。
2. `staging_precheck_and_run.sh` 默认会再执行 `TOK-005 dry-run`。
3. 如需临时跳过可设置：`ENABLE_M021_PRECHECK=0` 或 `ENABLE_TOK005_DRYRUN=0`。

---

## 2. SUP-004 账号挂载链路（UI-SUP-ACC-001~006）

执行脚本：

```bash
cd "/home/long/project/立交桥"
bash "scripts/supply-gate/sup004_accounts.sh"
```

最低断言：

1. 验证接口返回 `verify_status=pass/review_required`。
2. 创建账号成功并返回 `account_id`。
3. 激活/暂停接口返回状态迁移成功。
4. 审计日志接口可查询并返回 `request_id`。

证据输出：

1. `tests/supply/artifacts/sup004/*.json`
2. `tests/supply/artifacts/sup004/summary.txt`

---

## 3. SUP-005 套餐发布链路（UI-SUP-PKG-001~006）

执行脚本：

```bash
cd "/home/long/project/立交桥"
bash "scripts/supply-gate/sup005_packages.sh"
```

最低断言：

1. 草稿创建成功并返回 `package_id`。
2. 上架后状态为 `active`。
3. 暂停后状态为 `paused`。
4. 下架返回成功（`expired/paused` 合法）。
5. 批量调价返回 `success_count + failed_count = total`。
6. 复制成功并返回新的 `package_id`。

证据输出：

1. `tests/supply/artifacts/sup005/*.json`
2. `tests/supply/artifacts/sup005/summary.txt`

---

## 4. SUP-006 结算提现链路（UI-SUP-SET-001~005）

执行脚本：

```bash
cd "/home/long/project/立交桥"
bash "scripts/supply-gate/sup006_settlements.sh"
```

最低断言：

1. 账单查询成功返回 `summary`。
2. 提现申请成功返回 `settlement_id` 且状态 `pending`。
3. 撤销申请接口返回状态变更。
4. 对账单下载接口返回 `download_url`。
5. 收益流水接口返回分页与记录字段。

证据输出：

1. `tests/supply/artifacts/sup006/*.json`
2. `tests/supply/artifacts/sup006/summary.txt`

---

## 5. SUP-007 凭证边界专项（SEC-SUP-001~002）

执行脚本：

```bash
cd "/home/long/project/立交桥"
bash "scripts/supply-gate/sup007_boundary.sh"
```

最低断言：

1. 平台凭证访问主路径成功（映射 M-014）。
2. 外部 query key 请求被拒绝（映射 M-016）。
3. 响应/导出样本脱敏扫描无可复用凭证片段（映射 M-013）。
4. 若配置 `SUPPLIER_DIRECT_TEST_URL`，直连探测应失败或被阻断（映射 M-015）。

证据输出：

1. `tests/supply/artifacts/sup007/*.json`
2. `tests/supply/artifacts/sup007/summary.txt`

---

## 6. 回填要求

执行完成后，必须回填：

1. `tests/supply/ui_sup_acc_report_2026-03-28.md`
2. `tests/supply/ui_sup_pkg_report_2026-03-29.md`
3. `tests/supply/ui_sup_set_report_2026-03-29.md`
4. `tests/supply/sec_sup_boundary_report_2026-03-30.md`
5. `reports/supply_gate_review_2026-03-31.md`

所有回填项需要包含：

1. 结论（PASS/FAIL/BLOCKED）
2. 证据路径（json/screenshot/log）
3. 责任人签字

---

## 7. 依赖兼容审计命令（M-017）

执行脚本：

```bash
cd "/home/long/project/立交桥"
./scripts/ci/dependency-audit-check.sh 2026-03-27
```

最低断言：

1. 四件套文件存在且非空：
   1. `reports/dependency/sbom_2026-03-27.spdx.json`
   2. `reports/dependency/lockfile_diff_2026-03-27.md`
   3. `reports/dependency/compat_matrix_2026-03-27.md`
   4. `reports/dependency/risk_register_2026-03-27.md`
2. 输出结果为 `PASS`，并生成 `dependency_audit_result_2026-03-27.md`。

---

## 8. 分阶段门禁失败回退演练（M-018/M-019）

执行脚本：

```bash
cd "/home/long/project/立交桥"
./scripts/ci/stage-gate-drill.sh G3 2026-03-27
```

最低断言：

1. G3 失败后必须触发回退到 G2。
2. 后续阶段冻结，不允许继续升波。
3. 生成原始日志与演练报告：
   1. `reports/gates/stage_gate_drill_2026-03-27.log`
   2. `reports/gates/stage_gate_drift_drill_report_2026-03-27.md`

---

## 9. 本地 Mock 联调模式（仅演练）

执行命令：

```bash
cd "/home/long/project/立交桥"
python3 "scripts/mock/supply_gateway_mock_server.py"
```

另开终端执行：

```bash
cd "/home/long/project/立交桥"
bash "scripts/supply-gate/run_all.sh" "scripts/supply-gate/.env.local-mock"
```

说明：

1. 本模式仅用于脚本联调与产物验证，不代表 staging/生产可发布。
2. 生产放行仍需在真实 staging 地址与真实短期 token 下复跑并验收。

---

## 10. TOK-005 凭证边界 Dry-Run（开发阶段）

执行命令：

```bash
cd "/home/long/project/立交桥"
bash "scripts/supply-gate/tok005_boundary_dryrun.sh" "scripts/supply-gate/.env"
```

最低断言：

1. `go test ./...` 在 `platform-token-runtime` 通过。
2. Query key 外拒规则存在（`key/api_key/token`）。
3. 审计脱敏断言存在且通过（禁止敏感值落审计）。
4. TOK 用例可执行覆盖完整（`TOK-LIFE-001~008` 与 `TOK-AUD-001~007`）。
5. staging 就绪性检查结果可追溯（NO 时需明确阻塞原因）。

证据输出：

1. `reports/gates/tok005_dryrun_*.md`
2. `reports/gates/tok005_dryrun_*.log`
3. `tests/supply/artifacts/tok005_dryrun_*/go_test_output.txt`

说明：

1. Dry-run 仅用于开发阶段门禁前置验证，不可替代真实 staging 联调结论。
2. 真实放行仍以 `staging_precheck_and_run.sh` + `SUP-007/TOK-005` 实测结果为准。

---

## 11. TOK-006 统一 Gate 汇总（Dry-Run + SUP-004~007）

执行命令：

```bash
cd "/home/long/project/立交桥"
bash "scripts/supply-gate/tok006_gate_bundle.sh" "scripts/supply-gate/.env"
```

可选开关：

```bash
# 默认 1：先执行 TOK-005 dry-run
ENABLE_TOK005_DRYRUN=1

# 默认 0：仅汇总现有 SUP 报告，不触发 run_all
ENABLE_SUP_RUN=0
```

最低断言：

1. 输出单页 gate 汇总报告（含 TOK-005 + SUP-004~007）。
2. 生成明确发布判定：`GO / CONDITIONAL_GO / NO_GO`。
3. 若存在 mock 证据或 `staging readiness != YES`，不得输出 GO。

证据输出：

1. `reports/gates/tok006_gate_bundle_*.md`
2. `reports/gates/tok006_gate_bundle_*.log`
3. `reports/gates/tok006_release_decision_onepager_template_v1_2026-03-30.md`（模板）

---

## 12. Superpowers 严格分阶段验证（代码+脚本+门禁）

执行命令：

```bash
cd "/home/long/project/立交桥"
bash "scripts/ci/superpowers_stage_validate.sh"
```

阶段定义（当前实现）：

1. PHASE-01：TOK 运行时代码测试（Go 单测）
2. PHASE-02：SUP-004~SUP-007 本地 mock 脚本联调
3. PHASE-03：TOK-005 凭证边界 dry-run（mock 环境）
4. PHASE-04：TOK-006 统一 Gate 汇总
5. PHASE-05：依赖兼容审计门禁（M-017）
6. PHASE-06：分阶段回退演练门禁（M-018/M-019）
7. PHASE-07：真实 staging 预检（无真值时应 DEFERRED）
8. PHASE-08：每日指标快照生成（M-017/M-018/M-019）
9. PHASE-09：7日趋势报告生成（M-017/M-018/M-019）
10. PHASE-10：token 运行态就绪度检查（M-021）

结果判定：

1. 任一阶段 FAIL => `NO_GO`
2. 无 FAIL 且存在 DEFERRED => `CONDITIONAL_GO`
3. 全部 PASS => `GO`

可选环境变量：

```bash
# PHASE-07 使用的环境文件，默认 scripts/supply-gate/.env
STAGING_ENV_FILE="scripts/supply-gate/.env"
```

证据输出：

1. `reports/gates/superpowers_stage_validation_*.md`
2. `reports/gates/superpowers_stage_validation_*.log`
3. `tests/supply/artifacts/superpowers_stage_validation_*/phase*.log`

---

## 13. TOK-007 发布门禁复审（自动汇总）

执行命令：

```bash
cd "/home/long/project/立交桥"
bash "scripts/ci/tok007_release_recheck.sh"
```

最低断言：

1. 自动读取最新 `TOK-006` 汇总报告。
2. 自动读取最新 `Superpowers` 阶段验证报告。
3. 自动读取 `SUP Gate` 汇总评审结论。
4. 输出复审结论（`GO / CONDITIONAL GO / NO-GO`）与动作建议。

证据输出：

1. `review/outputs/tok007_release_recheck_*.md`
2. `reports/gates/tok007_release_recheck_*.log`

---

## 14. 最终决议一致性校验（Final vs TOK-007）

执行命令：

```bash
cd "/home/long/project/立交桥"
bash "scripts/ci/final_decision_consistency_check.sh"
```

最低断言：

1. 可解析 `final_decision`、`TOK-007`、`superpowers_stage_validation` 三类结论。
2. 若 `final_decision` 与 `TOK-007` 不一致，输出 `WARN`（不自动覆盖签署结论）。
3. 若任一来源不可解析，输出 `FAIL` 并阻断自动流程。

证据输出：

1. `reports/gates/final_decision_consistency_*.md`
2. `reports/gates/final_decision_consistency_*.log`

---

## 15. 最终决议候选稿生成（不覆盖签署原件）

执行命令：

```bash
cd "/home/long/project/立交桥"
bash "scripts/ci/tok007_generate_final_decision_candidate.sh"
```

最低断言：

1. 输入源必须包括 `review/final_decision_2026-03-31.md` 与最新 `tok007_release_recheck_*.md`。
2. 输出文件位于 `review/outputs/final_decision_candidate_from_tok007_*.md`。
3. 仅生成候选稿，不覆盖原签署文件。

证据输出：

1. `review/outputs/final_decision_candidate_from_tok007_*.md`
2. `reports/gates/tok007_generate_candidate_*.log`

---

## 16. M-021 Token Runtime 就绪度检查

执行命令：

```bash
cd "/home/long/project/立交桥"
bash "scripts/ci/token_runtime_readiness_check.sh" "$(date +%F)"
```

可选开关：

```bash
# 默认 0：跳过本地端口冒烟（适配受限沙箱环境）
ENABLE_TOKEN_RUNTIME_SMOKE=0

# 置 1：执行本地服务启动 + issue + audit-events 冒烟
ENABLE_TOKEN_RUNTIME_SMOKE=1

# 可选：指定冒烟起始端口（默认 18082，若被占用会自动顺延）
TOKEN_RUNTIME_SMOKE_PORT=18082
```

最低断言：

1. 输出 `token_runtime_readiness_*.md` 报告并给出百分比结果。
2. 运行态代码与契约工件完整（API入口/HTTP处理/OpenAPI/Dockerfile）。
3. `platform-token-runtime` 测试与构建均通过。
4. 若就绪度 `<100%`，脚本必须返回失败并阻断后续门禁。

证据输出：

1. `reports/gates/token_runtime_readiness_*.md`
2. `reports/gates/token_runtime_readiness_*.log`
3. `reports/gates/token_runtime_go_test_*.log`
4. `reports/gates/token_runtime_go_build_*.log`

---

## 17. Token 审计事件查询（TOK-REAL-002）

本地服务启动：

```bash
cd "/home/long/project/立交桥/platform-token-runtime"
export PATH="/home/long/project/立交桥/.tools/go-current/bin:$PATH"
go run ./cmd/platform-token-runtime
```

审计查询示例：

```bash
curl -sS "http://127.0.0.1:18081/api/v1/platform/tokens/audit-events?limit=20" \
  -H "X-Request-Id: req-audit-query-demo"
```

最低断言：

1. 返回 `200`，且结构包含 `request_id/data.total/data.items`。
2. 返回项包含 `event_id/event_name/request_id/route/result_code/created_at`。
3. 响应不包含 `access_token` 或上游敏感凭证明文。

证据输出：

1. `platform-token-runtime/internal/httpapi/token_api_test.go`（自动化用例）
2. `reports/gates/token_runtime_readiness_*.md`（检查项 `TOK-REAL-002-C1/C2`）

---

## 18. Staging 证据自动回填草稿

执行命令：

```bash
cd "/home/long/project/立交桥"
bash "scripts/ci/staging_evidence_autofill.sh"
```

可选参数（用于锁定本次流水证据，避免误取历史 latest）：

```bash
bash "scripts/ci/staging_evidence_autofill.sh" \
  --staging-run-log "reports/gates/staging_run_2026-03-30_184432.log" \
  --stage-report "reports/gates/superpowers_stage_validation_2026-03-30_184433.md" \
  --token-readiness "reports/gates/token_runtime_readiness_2026-03-30_184435.md" \
  --tok007-report "review/outputs/tok007_release_recheck_2026-03-30_184436.md" \
  --pipeline-report "reports/gates/superpowers_release_pipeline_2026-03-30_184434.md"
```

最低断言：

1. 自动抽取 `PHASE-07`、`M-013~M-016`、`M-021` 与 TOK-007 机判结论。
2. 输出证据路径清单，便于人工补齐与签署。
3. 不得自动上调为 GO，仅生成草稿。

证据输出：

1. `reports/gates/staging_token_go_evidence_autofill_*.md`
2. `reports/gates/staging_token_go_evidence_autofill_*.log`

---

## 19. 一键 Staging 发布流水

执行命令：

```bash
cd "/home/long/project/立交桥"
bash "scripts/ci/staging_release_pipeline.sh" "scripts/supply-gate/.env"
```

local/mock 防误跑（默认拦截）：

```bash
# 仅当明确要做 local/mock 演练时启用
ALLOW_LOCAL_MOCK_STAGING=1 \
bash "scripts/ci/staging_release_pipeline.sh" "scripts/supply-gate/.env.local-mock"
```

说明：

1. STEP-01：`staging_precheck_and_run.sh`（含 M-021/TOK-005/SUP run_all）。
2. STEP-02：`superpowers_release_pipeline.sh`（使用 `STAGING_ENV_FILE`）。
3. STEP-03：`staging_evidence_autofill.sh` 自动生成回填草稿（显式绑定本次流水证据文件）。
4. 检测到 local/mock env 且未设置 `ALLOW_LOCAL_MOCK_STAGING=1` 时，脚本应直接失败，防止误把演练结果当成真实 staging 证据。

可选监控（默认关闭、非阻断）：

```bash
ENABLE_MINIMAX_MONITORING=1 \
MINIMAX_ENV_FILE="scripts/supply-gate/.env.minimax-dev" \
MINIMAX_RUN_ACTIVE_SMOKE=0 \
bash "scripts/ci/superpowers_release_pipeline.sh"
```

说明：
1. 开启后会在 `STEP-05` 额外执行 Minimax 每日快照 + 7 日趋势生成。
2. 该步骤是监控辅助项，失败仅记 `WARN`，不阻断 SUP 主门禁判定。

证据输出：

1. `reports/gates/staging_release_pipeline_*.md`
2. `reports/gates/staging_release_pipeline_*.log`

---

## 20. Minimax 上游独立 Smoke（不并入 SUP 发布门禁）

执行命令：

```bash
cd "/home/long/project/立交桥"
bash "scripts/supply-gate/minimax_upstream_smoke.sh" "scripts/supply-gate/.env.minimax-dev"
```

可选环境变量：

```bash
# 默认 /v1/messages
MINIMAX_SMOKE_PATH="/v1/messages"

# 默认 minimax-smoke-model（可替换为实际模型）
MINIMAX_SMOKE_MODEL="your-model-id"

# 默认 20 秒
MINIMAX_TIMEOUT_SECONDS=20
```

最低断言：

1. 输出 `reports/gates/minimax_upstream_smoke_*.md` 报告。
2. 报告必须包含 base 连通探测与 active 鉴权探测两段结果。
3. 分类规则需区分：`PASS / PASS_AUTH_REACHED / FAIL_AUTH / FAIL_PATH / FAIL_NETWORK`。

说明：

1. 该脚本仅用于“上游（Minimax）连通与鉴权可达性”验证。
2. 该脚本不参与 `SUP-004~SUP-007` 业务契约发布门禁判定。
3. 若 Minimax 返回 `404/405`，优先检查 `API_BASE_URL + MINIMAX_SMOKE_PATH` 组合是否正确。

---

## 21. Minimax 上游每日快照（CI 汇总）

执行命令：

```bash
cd "/home/long/project/立交桥"
bash "scripts/ci/minimax_upstream_daily_snapshot.sh" "$(date +%F)" "scripts/supply-gate/.env.minimax-dev"
```

可选环境变量：

```bash
# 默认 0：仅汇总最新 smoke 报告，不触发网络请求
RUN_ACTIVE_SMOKE=0

# 置 1：执行一次实时 smoke 后再汇总
RUN_ACTIVE_SMOKE=1
```

最低断言：

1. 生成 `reports/gates/minimax_upstream_daily_snapshot_*.md`。
2. 生成/更新 `reports/gates/minimax_upstream_daily_snapshots.csv`。
3. 明确标注 `RUN_ACTIVE_SMOKE` 取值，区分“实时探测”与“仅汇总”。
4. 默认优先汇总“非 dry-run”最新报告，避免将联调证据误当真实上游证据。

说明：

1. 该快照是“上游可达性趋势”证据，不替代 SUP 发布门禁。
2. 建议在定时任务中默认 `RUN_ACTIVE_SMOKE=0`，将实时探测作为受控任务执行。
3. 若仅存在 `PASS_DRY_RUN` 报告，快照状态应为 `CONDITIONAL_PASS`。

---

## 22. Minimax 上游 7 日趋势报告

执行命令：

```bash
cd "/home/long/project/立交桥"
bash "scripts/ci/minimax_upstream_trend_report.sh" "$(date +%F)"
```

最低断言：

1. 生成 `reports/gates/minimax_upstream_trend_7d_*.md`。
2. 报告包含最近 7 条（不足 7 条按实际）快照的状态统计。
3. 趋势状态遵循 `PASS_7D / CONDITIONAL_7D / NOT_READY / INSUFFICIENT_DATA`。

说明：

1. 该趋势报告用于 F-03（连续观测证据）收敛，不替代 staging 发布门禁。
2. 建议与第 21 节每日快照搭配执行，形成“日报 + 周趋势”组合。

---

## 23. 一键生成本地 STG 环境（owner/viewer/admin token）

执行命令：

```bash
cd "/home/long/project/立交桥"
bash "scripts/ci/generate_local_staging_env.sh" "scripts/supply-gate/.env.staging-real"
```

可选环境变量：

```bash
# 默认 http://127.0.0.1:18080
API_BASE_URL_VALUE="http://127.0.0.1:18080"

# 默认 http://127.0.0.1:18081
TOKEN_RUNTIME_URL="http://127.0.0.1:18081"

# 默认 7200 秒（2小时）
TOKEN_TTL_SECONDS=7200

# 默认 1：若 token runtime 不可用则自动拉起临时实例
START_RUNTIME_IF_NEEDED=1
```

最低断言：

1. 生成 `scripts/supply-gate/.env.staging-real`（权限 `600`）。
2. 文件包含 `OWNER_BEARER_TOKEN / VIEWER_BEARER_TOKEN / ADMIN_BEARER_TOKEN` 三类 token。
3. 生成摘要报告 `reports/gates/local_staging_env_generation_*.md`（仅 hash，不泄露明文 token）。

说明：

1. 该脚本生成的是“本地开发/联调用”平台 token，非外部 LLM 厂商 key。
2. 切换真实 staging 时，只需替换 `API_BASE_URL_VALUE` 并重新执行脚本即可刷新 token 与 env。

---

## 24. 真实 STG 就绪度检查（地址+token+可达性）

执行命令：

```bash
cd "/home/long/project/立交桥"
bash "scripts/ci/staging_real_readiness_check.sh" "scripts/supply-gate/.env.staging-real"
```

最低断言：

1. `API_BASE_URL` 非占位值，且不是 `localhost/127.0.0.1`。
2. 三类 token 非空且非占位值。
3. `API_BASE_URL` 基础可达性检查通过（`curl -I` 非 `000`）。
4. 生成报告 `reports/gates/staging_real_readiness_*.md`。

说明：

1. 结果为 `READY` 才建议进入真实 STG 放行口径验证。
2. 结果为 `BLOCKED` 时，应先修复地址或 token，再执行 `staging_release_pipeline.sh`。
