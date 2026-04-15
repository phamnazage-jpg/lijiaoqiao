# Post-Verification Refactor Follow-up Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 基于 2026-04-14 最新验证结果，把当前 `CONDITIONAL_GO` 收敛到真实 staging 可复核口径，并清理验证脚本重复、测试空洞与历史快照治理缺口。

**Architecture:** 先修正门禁语义和验证入口，避免 local/mock 与真实 staging 继续混用；再抽离重复验证逻辑，降低三套脚本长期漂移风险；最后按服务补齐测试空洞，并把历史快照治理自动化，避免后续文档和证据再次回到“历史文件充当当前事实源”的状态。

**Tech Stack:** Bash CI 脚本、Go、PostgreSQL、`go test`、`rg`、Markdown

---

## 输入证据（2026-04-14）

1. `bash "scripts/ci/repo_integrity_check.sh"` 通过。
2. `cd "supply-api" && bash "scripts/run_integration_tests.sh" "./internal/repository"` 通过。
3. `bash "scripts/ci/superpowers_stage_validate.sh"` 返回 `CONDITIONAL_GO`。
4. `reports/archive/gate_verification/superpowers_stage_validation_2026-04-14_230042.md` 明确给出原因：`all phases passed but PHASE-07 used local/mock staging env`。
5. 当前测试空洞仍存在于：
   - `gateway/pkg/model`
   - `platform-token-runtime/internal/auth/model`
   - `supply-api/internal/cache`
   - `supply-api/internal/iam/repository`
   - `supply-api/internal/messaging`
   - `supply-api/internal/storage`

## Task 1: 修正 PHASE-07 语义，强制真实 staging 与 local/mock 明确分流

**Files:**
- Modify: `scripts/ci/superpowers_stage_validate.sh`
- Modify: `scripts/supply-gate/staging_precheck_and_run.sh`
- Modify: `scripts/ci/staging_real_readiness_check.sh`
- Modify: `docs/supply_gate_command_playbook_v1_2026-03-25.md`
- Create: `tests/supply/test_stage_env_classification.sh`

**Step 1: 写失败回归脚本，锁定 PHASE-07 不得把 local/mock 记成 PASS**

Write:
```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
OUT="$(mktemp)"

STAGING_ENV_FILE="scripts/supply-gate/.env.local-mock" \
  bash "${ROOT_DIR}/scripts/ci/superpowers_stage_validate.sh" >"${OUT}" 2>&1 || true

grep -Eq 'PHASE-07 \| (DEFERRED|MOCK)' "${OUT}"
grep -Eq 'CONDITIONAL_GO' "${OUT}"
```

**Step 2: 运行脚本，确认当前实现会失败**

Run:
```bash
bash "tests/supply/test_stage_env_classification.sh"
```

Expected:
- 失败，因为当前报告会把 `PHASE-07` 记为 `PASS`，只是最终决策降为 `CONDITIONAL_GO`。

**Step 3: 写最小修正**

Write:
- `superpowers_stage_validate.sh` 在 `is_mock_staging_env` 命中时，把 `PHASE-07` 结果写成 `DEFERRED` 或 `MOCK`，不要再写 `PASS`。
- `staging_precheck_and_run.sh` 与 `staging_real_readiness_check.sh` 输出统一的环境分类字段，例如 `env_class=real-staging|local-mock|placeholder`。
- 文档把 `PHASE-07` 的判定语义写清楚：真实 staging 才能 `PASS`，local/mock 只能 `DEFERRED/MOCK`。

**Step 4: 运行验证**

Run:
```bash
bash "tests/supply/test_stage_env_classification.sh"
bash "scripts/ci/superpowers_stage_validate.sh"
```

Expected:
- 回归脚本通过。
- `superpowers_stage_validation_*.md` 的 `PHASE-07` 不再显示为 `PASS + local/mock` 这种混合语义。

**Step 5: Commit**

```bash
git add tests/supply/test_stage_env_classification.sh scripts/ci/superpowers_stage_validate.sh scripts/supply-gate/staging_precheck_and_run.sh scripts/ci/staging_real_readiness_check.sh docs/supply_gate_command_playbook_v1_2026-03-25.md
git commit -m "fix(ci): split mock and real staging gate semantics"
```

## Task 2: 抽离统一验证内核，消除 `repo_integrity` / `backend-verify` / `superpowers` 三套脚本重复

**Files:**
- Create: `scripts/ci/lib/verification_common.sh`
- Modify: `scripts/ci/repo_integrity_check.sh`
- Modify: `scripts/ci/backend-verify.sh`
- Modify: `scripts/ci/superpowers_stage_validate.sh`
- Create: `tests/ci/test_verification_common.sh`

**Step 1: 写失败回归脚本，锁定三套脚本的 suite 名称与命令矩阵**

Write:
```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

rg -n 'gateway|platform-token-runtime|supply-api unit|supply-api e2e' \
  "${ROOT_DIR}/scripts/ci/repo_integrity_check.sh" \
  "${ROOT_DIR}/scripts/ci/backend-verify.sh" \
  "${ROOT_DIR}/scripts/ci/superpowers_stage_validate.sh"
```

**Step 2: 运行脚本，确认当前实现存在重复定义**

Run:
```bash
bash "tests/ci/test_verification_common.sh"
```

Expected:
- 能看到相同或近似的验证命令散落在三份脚本里。

**Step 3: 写最小实现**

Write:
- 新建 `verification_common.sh`，统一提供：
  - `resolve_go_bin`
  - `run_go_suite`
  - `check_fact_sources`
  - `write_step_result`
- 三个入口脚本只保留各自的“编排职责”，不再内嵌同一份 suite 细节。

**Step 4: 运行验证**

Run:
```bash
bash "scripts/ci/repo_integrity_check.sh"
./scripts/ci/backend-verify.sh
bash "scripts/ci/superpowers_stage_validate.sh"
```

Expected:
- 三个入口脚本都通过。
- 共享 suite 的定义只保留一份。

**Step 5: Commit**

```bash
git add scripts/ci/lib/verification_common.sh scripts/ci/repo_integrity_check.sh scripts/ci/backend-verify.sh scripts/ci/superpowers_stage_validate.sh tests/ci/test_verification_common.sh
git commit -m "refactor(ci): unify verification script core"
```

## Task 3: 补齐当前仍无测试的包，先补契约层，再补工具层

**Files:**
- Create: `gateway/pkg/model/model_test.go`
- Create: `platform-token-runtime/internal/auth/model/model_test.go`
- Create: `supply-api/internal/cache/cache_test.go`
- Create: `supply-api/internal/iam/repository/repository_test.go`
- Create: `supply-api/internal/messaging/messaging_test.go`
- Create: `supply-api/internal/storage/storage_test.go`

**Step 1: 为每个空包先写一个最小契约测试**

Write:
```go
func TestPackageContract(t *testing.T) {
	t.Helper()
}
```

**Step 2: 逐包运行，确认当前确实没有测试覆盖**

Run:
```bash
cd "gateway" && go test ./pkg/model -cover
cd "platform-token-runtime" && go test ./internal/auth/model -cover
cd "supply-api" && go test ./internal/cache ./internal/iam/repository ./internal/messaging ./internal/storage -cover
```

Expected:
- 当前覆盖率接近 0，或仅因为新增空测试文件而刚起步。

**Step 3: 写最小但有意义的测试**

Write:
- `gateway/pkg/model`：补字段约束、默认值、序列化契约。
- `platform-token-runtime/internal/auth/model`：补 token/claims/role model 的不变量。
- `supply-api/internal/cache`：补 key 生成、TTL 与空值语义。
- `supply-api/internal/iam/repository`：补错误映射、查询参数与分页边界。
- `supply-api/internal/messaging`：补消息 envelope、topic 选择与重试字段契约。
- `supply-api/internal/storage`：补路径/对象键生成、元数据约束、空输入拒绝。

**Step 4: 运行验证**

Run:
```bash
cd "gateway" && go test ./pkg/model ./...
cd "platform-token-runtime" && go test ./internal/auth/model ./...
cd "supply-api" && go test ./internal/cache ./internal/iam/repository ./internal/messaging ./internal/storage ./...
```

Expected:
- 空包被消除。
- 三个服务全量测试仍通过。

**Step 5: Commit**

```bash
git add gateway/pkg/model/model_test.go platform-token-runtime/internal/auth/model/model_test.go supply-api/internal/cache/cache_test.go supply-api/internal/iam/repository/repository_test.go supply-api/internal/messaging/messaging_test.go supply-api/internal/storage/storage_test.go
git commit -m "test(repo): cover untested core packages"
```

## Task 4: 把历史快照治理自动化，停止手工批量改写归档文件

**Files:**
- Create: `scripts/ci/mark_historical_snapshots.sh`
- Modify: `review/README.md`
- Modify: `docs/plans/2026-04-14-repo-integrity-refactor-plan.md`
- Modify: `scripts/ci/tok007_release_recheck.sh`
- Modify: `scripts/ci/tok007_generate_final_decision_candidate.sh`
- Create: `tests/supply/test_tok007_candidate_paths.sh`

**Step 1: 写失败回归脚本，锁定现行机审稿路径规则**

Write:
```bash
#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

rg -n 'current_machine_review_sources.md' \
  "${ROOT_DIR}/docs/plans/2026-04-14-repo-integrity-refactor-plan.md" \
  "${ROOT_DIR}/review/README.md"
```

**Step 2: 运行脚本，确认当前规则仅存在于文档层**

Run:
```bash
bash "tests/supply/test_tok007_candidate_paths.sh"
```

Expected:
- 当前只能证明文档引用规则存在，脚本层还没有统一治理入口。

**Step 3: 写最小实现**

Write:
- `mark_historical_snapshots.sh` 负责给历史机审快照批量加 header，不再手工逐份改写。
- `tok007_release_recheck.sh` 与 `tok007_generate_final_decision_candidate.sh` 输出时同步写出现行稿指针。
- 文档只负责声明规则，不再承担批量修正职责。

**Step 4: 运行验证**

Run:
```bash
bash "tests/supply/test_tok007_candidate_paths.sh"
bash "scripts/ci/tok007_release_recheck.sh"
bash "scripts/ci/tok007_generate_final_decision_candidate.sh"
```

Expected:
- 现行稿指针可由脚本自动生成和更新。
- 历史快照的标记逻辑不再依赖手工维护。

**Step 5: Commit**

```bash
git add scripts/ci/mark_historical_snapshots.sh scripts/ci/tok007_release_recheck.sh scripts/ci/tok007_generate_final_decision_candidate.sh tests/supply/test_tok007_candidate_paths.sh review/README.md docs/plans/2026-04-14-repo-integrity-refactor-plan.md
git commit -m "refactor(review): automate historical snapshot governance"
```

## 验证出口

完成以上四个任务后，必须重新执行：

```bash
bash "scripts/ci/repo_integrity_check.sh"
cd "supply-api" && bash "scripts/run_integration_tests.sh" "./internal/repository"
bash "scripts/ci/superpowers_stage_validate.sh"
```

预期结果：

1. `repo_integrity_check.sh` 继续通过。
2. 仓储集成继续通过。
3. `superpowers_stage_validate.sh` 在 local/mock 输入下明确给出 `CONDITIONAL_GO + PHASE-07=DEFERRED/MOCK`，在真实 staging 输入下才允许全量 `PASS`。
