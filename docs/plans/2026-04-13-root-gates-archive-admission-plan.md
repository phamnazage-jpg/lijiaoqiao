# Root Gates Archive Admission Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 把根仓库 `reports/gates` 的历史证据迁移收口成可审计、可提交、可回滚的 Git 变更，同时清理剩余无效文件和待定文件。

**Architecture:** 先承认事实，再提交事实。`reports/archive/gate_verification/` 里只有 `*.md`、`*.csv`、索引文件属于“有效归档”；原始 `.log`、`.log.*`、`.out.log`、`token_runtime_bin_*` 只保留本地留档，不进入 Git。删除 `reports/gates/*` 只能在对应 canonical 归档文件已经正式入库后进行，且必须按批次对齐 basename。

**Tech Stack:** Git, Markdown, Go, `rg`, `find`, `git diff`, `go test`

---

## 当前基线

已完成并已提交：

- `10d79be` `docs(cleanup): add committable cleanup plan`
- `7f8143e` `chore(config): separate local override guidance`
- `a94de1b` `refactor(outbox): share domain backoff policy`
- `64f99a4` `refactor(compensation): use handler registry`
- `90d71eb` `refactor(outbox): remove runner event copy`
- `9ad3980` `docs(gates): add root archive index`
- `24e85ca` `docs(gates): backfill archive csv snapshots`
- `73f4463` `chore(gates): ignore raw archive artifacts`

当前事实：

- `reports/gates/*` staged deletion 仍在，不能直接提交。
- `reports/archive/gate_verification/` 当前本地文件统计为：`md=295 csv=2 log=995 auditjson=30 issuejson=32 serverlog=33 bin=75`。
- basename 对照已经无缺口，说明 staged deletion 里的文件名都能在归档目录找到同名 canonical 文件。
- 归档树里的原始日志和二进制快照已通过 `.gitignore` 降噪，但 `295` 个 `.md` 和 `2` 个 `.csv` 仍待正式入库分批处理。
- 根目录还存在第二套未纳管归档来源：`reports/archive/gates/` 和平铺的 `reports/archive/metrics_daily_snapshots.csv`、`reports/archive/minimax_upstream_daily_snapshots.csv`；后者与 `gate_verification/` 中 canonical CSV 内容完全重复。

---

### Task 1: 冻结有效归档的准入规则

**Files:**
- Modify: `reports/archive/gate_verification/INDEX_2026-04-13.md`
- Modify: `supply-api/reports/CLEANUP_REPORT_2026-04-13.md`
- Review: `reports/archive/gates/`
- Review: `reports/archive/metrics_daily_snapshots.csv`
- Review: `reports/archive/minimax_upstream_daily_snapshots.csv`
- Test: `.gitignore`

**Step 1: 运行状态校验**

Run:
```bash
git status --short -- reports/archive/gate_verification | sed -n '1,120p'
git check-ignore -v reports/archive/gate_verification/backend_verify_2026-04-11_090323.log
git check-ignore -v reports/archive/gate_verification/token_runtime_bin_2026-03-30_173123
```

Expected:
- `.md` / `.csv` 仍显示为未跟踪。
- `.log` 和 `token_runtime_bin_*` 已被忽略。
- `reports/archive/gates/` 和根目录平铺 CSV 被识别为非 canonical 路径，后续需要单独收口，不得混入本轮 admission。

**Step 2: 更新索引中的准入规则**

Write:
```md
- canonical 归档：*.md、*.csv、索引文件
- local-only 留档：*.log、*.out.log、*.log.audit.json、*.log.issue.json、*.log.server、token_runtime_bin_*
- invalid duplicate paths：reports/archive/gates/、reports/archive/*.csv
```

**Step 3: 运行格式校验**

Run:
```bash
git diff --check -- .gitignore reports/archive/gate_verification/INDEX_2026-04-13.md supply-api/reports/CLEANUP_REPORT_2026-04-13.md
```

Expected:
- 无格式错误。

**Step 4: 提交规则冻结**

Run:
```bash
git add .gitignore reports/archive/gate_verification/INDEX_2026-04-13.md supply-api/reports/CLEANUP_REPORT_2026-04-13.md
git commit -m "docs(gates): freeze canonical archive rules"
```

---

### Task 2: 批次 A，纳入指标和上游摘要归档

**Files:**
- Add: `reports/archive/gate_verification/metrics_daily_snapshot_*.md`
- Add: `reports/archive/gate_verification/metrics_trend_7d_*.md`
- Add: `reports/archive/gate_verification/metrics_daily_snapshots.csv`
- Add: `reports/archive/gate_verification/minimax_upstream_daily_snapshot_*.md`
- Add: `reports/archive/gate_verification/minimax_upstream_trend_7d_*.md`
- Add: `reports/archive/gate_verification/minimax_upstream_daily_snapshots.csv`
- Add: `reports/archive/gate_verification/minimax_upstream_smoke_*.md`
- Delete: `reports/gates/metrics_daily_snapshot_*.md`
- Delete: `reports/gates/metrics_trend_7d_*.md`
- Delete: `reports/gates/metrics_daily_snapshots.csv`
- Delete: `reports/gates/minimax_upstream_daily_snapshot_*.md`
- Delete: `reports/gates/minimax_upstream_trend_7d_*.md`
- Delete: `reports/gates/minimax_upstream_daily_snapshots.csv`
- Delete: `reports/gates/minimax_upstream_smoke_*.md`

**Step 1: 精确列出批次文件**

Run:
```bash
find reports/archive/gate_verification -maxdepth 1 -type f | sed 's#.*/##' | rg '^metrics_daily_snapshot_|^metrics_trend_7d_|^metrics_daily_snapshots\.csv$|^minimax_upstream_daily_snapshot_|^minimax_upstream_trend_7d_|^minimax_upstream_daily_snapshots\.csv$|^minimax_upstream_smoke_.*\.md$' | sort
git diff --cached --name-only -- reports/gates | sed 's#.*/##' | rg '^metrics_daily_snapshot_|^metrics_trend_7d_|^metrics_daily_snapshots\.csv$|^minimax_upstream_daily_snapshot_|^minimax_upstream_trend_7d_|^minimax_upstream_daily_snapshots\.csv$|^minimax_upstream_smoke_.*\.md$' | sort
```

Expected:
- 归档侧和删除侧文件族完全对齐。

**Step 2: 暂存 canonical 归档和对应删除**

Run:
```bash
git add reports/archive/gate_verification/metrics_daily_snapshot_*.md
git add reports/archive/gate_verification/metrics_trend_7d_*.md
git add reports/archive/gate_verification/metrics_daily_snapshots.csv
git add reports/archive/gate_verification/minimax_upstream_daily_snapshot_*.md
git add reports/archive/gate_verification/minimax_upstream_trend_7d_*.md
git add reports/archive/gate_verification/minimax_upstream_daily_snapshots.csv
git add reports/archive/gate_verification/minimax_upstream_smoke_*.md
git add reports/gates/metrics_daily_snapshot_*.md
git add reports/gates/metrics_trend_7d_*.md
git add reports/gates/metrics_daily_snapshots.csv
git add reports/gates/minimax_upstream_daily_snapshot_*.md
git add reports/gates/minimax_upstream_trend_7d_*.md
git add reports/gates/minimax_upstream_daily_snapshots.csv
git add reports/gates/minimax_upstream_smoke_*.md
```

**Step 3: 运行 staged 校验**

Run:
```bash
git diff --cached --check
git diff --cached --name-status -- reports/archive/gate_verification reports/gates | sed -n '1,200p'
```

Expected:
- 只有 `.md` / `.csv` 归档被新增。
- 对应 `reports/gates` 摘要文件被删除。

**Step 4: 提交批次 A**

Run:
```bash
git commit -m "docs(gates): admit metrics and upstream gate archives"
```

---

### Task 3: 批次 B，纳入决策和后端验证摘要

**Files:**
- Add: `reports/archive/gate_verification/backend_verify_*.md`
- Add: `reports/archive/gate_verification/final_decision_consistency_*.md`
- Delete: `reports/gates/backend_verify_*.md`
- Delete: `reports/gates/final_decision_consistency_*.md`

**Step 1: 列出批次 B 文件**

Run:
```bash
find reports/archive/gate_verification -maxdepth 1 -type f | sed 's#.*/##' | rg '^backend_verify_.*\.md$|^final_decision_consistency_.*\.md$' | sort
git diff --cached --name-only -- reports/gates | sed 's#.*/##' | rg '^backend_verify_.*\.md$|^final_decision_consistency_.*\.md$' | sort
```

Expected:
- 归档文件和删除文件都可枚举。

**Step 2: 暂存并校验**

Run:
```bash
git add reports/archive/gate_verification/backend_verify_*.md
git add reports/archive/gate_verification/final_decision_consistency_*.md
git add reports/gates/backend_verify_*.md
git add reports/gates/final_decision_consistency_*.md
git diff --cached --check
```

Expected:
- staged diff 干净。

**Step 3: 提交批次 B**

Run:
```bash
git commit -m "docs(gates): admit backend and decision summaries"
```

---

### Task 4: 批次 C，纳入阶段验证和发布流水线摘要

**Files:**
- Add: `reports/archive/gate_verification/superpowers_stage_validation_*.md`
- Add: `reports/archive/gate_verification/superpowers_release_pipeline_*.md`
- Add: `reports/archive/gate_verification/staging_release_pipeline_*.md`
- Add: `reports/archive/gate_verification/staging_real_readiness_*.md`
- Add: `reports/archive/gate_verification/staging_token_go_evidence_autofill_*.md`
- Add: `reports/archive/gate_verification/staging_token_go_evidence_template_v1_2026-03-30.md`
- Add: `reports/archive/gate_verification/stage_gate_drift_drill_report_2026-03-27.md`
- Add: `reports/archive/gate_verification/tok006_release_decision_onepager_template_v1_2026-03-30.md`
- Delete: matching `reports/gates/*.md`

**Step 1: 列出批次 C 文件**

Run:
```bash
find reports/archive/gate_verification -maxdepth 1 -type f | sed 's#.*/##' | rg '^superpowers_stage_validation_.*\.md$|^superpowers_release_pipeline_.*\.md$|^staging_release_pipeline_.*\.md$|^staging_real_readiness_.*\.md$|^staging_token_go_evidence_autofill_.*\.md$|^staging_token_go_evidence_template_v1_2026-03-30\.md$|^stage_gate_drift_drill_report_2026-03-27\.md$|^tok006_release_decision_onepager_template_v1_2026-03-30\.md$' | sort
```

Expected:
- 只出现 canonical `.md`。

**Step 2: 暂存和 staged 校验**

Run:
```bash
git add reports/archive/gate_verification/superpowers_stage_validation_*.md
git add reports/archive/gate_verification/superpowers_release_pipeline_*.md
git add reports/archive/gate_verification/staging_release_pipeline_*.md
git add reports/archive/gate_verification/staging_real_readiness_*.md
git add reports/archive/gate_verification/staging_token_go_evidence_autofill_*.md
git add reports/archive/gate_verification/staging_token_go_evidence_template_v1_2026-03-30.md
git add reports/archive/gate_verification/stage_gate_drift_drill_report_2026-03-27.md
git add reports/archive/gate_verification/tok006_release_decision_onepager_template_v1_2026-03-30.md
git add reports/gates/superpowers_stage_validation_*.md
git add reports/gates/superpowers_release_pipeline_*.md
git add reports/gates/staging_release_pipeline_*.md
git add reports/gates/staging_real_readiness_*.md
git add reports/gates/staging_token_go_evidence_autofill_*.md
git add reports/gates/staging_token_go_evidence_template_v1_2026-03-30.md
git add reports/gates/stage_gate_drift_drill_report_2026-03-27.md
git add reports/gates/tok006_release_decision_onepager_template_v1_2026-03-30.md
git diff --cached --check
```

**Step 3: 提交批次 C**

Run:
```bash
git commit -m "docs(gates): admit stage validation and pipeline summaries"
```

---

### Task 5: 批次 D，纳入 token runtime 摘要并明确不纳入原始产物

**Files:**
- Add: `reports/archive/gate_verification/token_runtime_readiness_*.md`
- Delete: `reports/gates/token_runtime_readiness_*.md`
- Optionally Add: `reports/archive/gate_verification/token_runtime_smoke_*.md` if存在 `.md` 摘要
- Delete: matching `reports/gates/token_runtime_smoke_*.md`
- Modify: `reports/archive/gate_verification/INDEX_2026-04-13.md`

**Step 1: 列出 token runtime 可入库摘要**

Run:
```bash
find reports/archive/gate_verification -maxdepth 1 -type f | sed 's#.*/##' | rg '^token_runtime_readiness_.*\.md$|^token_runtime_smoke_.*\.md$' | sort | sed -n '1,200p'
```

Expected:
- 只纳入 `.md` 摘要，不纳入 `.log` / `.json` / `.server` / `token_runtime_bin_*`。

**Step 2: 更新索引说明**

Write:
```md
- token runtime 原始运行产物只保留本地审计留档
- 仓库只纳入 readiness / smoke 的 markdown 摘要
```

**Step 3: 暂存与校验**

Run:
```bash
git add reports/archive/gate_verification/token_runtime_readiness_*.md
git add reports/archive/gate_verification/token_runtime_smoke_*.md
git add reports/gates/token_runtime_readiness_*.md
git add reports/gates/token_runtime_smoke_*.md
git add reports/archive/gate_verification/INDEX_2026-04-13.md
git diff --cached --check
git diff --cached --name-status -- reports/archive/gate_verification reports/gates | sed -n '1,240p'
```

**Step 4: 提交批次 D**

Run:
```bash
git commit -m "docs(gates): admit token runtime summaries"
```

---

### Task 6: 处理待定文件和伪文档

**Files:**
- Review: `supply-api/e2e/README.md`
- Review: `supply-api/config/config.test.yaml`
- Review: `supply-api/scripts/production_test.sh`
- Modify: `supply-api/reports/CLEANUP_REPORT_2026-04-13.md`

**Step 1: 审查待定文件性质**

Run:
```bash
file supply-api/e2e/README.md
sed -n '1,220p' supply-api/e2e/README.md
sed -n '1,220p' supply-api/config/config.test.yaml
sed -n '1,240p' supply-api/scripts/production_test.sh
```

Expected:
- 明确区分“文档 / 样例 / 本机脚本 / 测试源码伪装文件”。

**Step 2: 形成决策清单**

Write:
```md
- `e2e/README.md`: 重命名成真正的 Go 测试文件或还原成 README
- `config.test.yaml`: 判定是仓库测试样例还是本机绑定配置
- `production_test.sh`: 判定是否保留为可复用脚本
```

**Step 3: 按决策逐个修复并验证**

Run:
```bash
go test ./internal/config
git diff --check
```

**Step 4: 提交待定文件收口**

Run:
```bash
git commit -m "chore(cleanup): resolve pending local-only files"
```

---

### Task 7: 最终删除提交和工作区清理

**Files:**
- Delete: remaining `reports/gates/*.md` already admitted into canonical archive
- Modify: `reports/archive/gate_verification/INDEX_2026-04-13.md`
- Modify: `supply-api/reports/CLEANUP_REPORT_2026-04-13.md`

**Step 1: 做最终对照**

Run:
```bash
archive_list=$(mktemp)
delete_list=$(mktemp)
find reports/archive/gate_verification -maxdepth 1 -type f -printf "%f\n" | sort -u > "$archive_list"
git diff --cached --name-only -- reports/gates | xargs -n1 basename | sort -u > "$delete_list"
comm -23 "$delete_list" "$archive_list"
rm -f "$archive_list" "$delete_list"
```

Expected:
- 无输出。

**Step 2: 更新索引状态**

Write:
```md
- canonical admission complete
- staged deletion paired and committed
- remaining local-only artifacts ignored by policy
```

**Step 3: 运行全局校验**

Run:
```bash
git diff --check
go test ./internal/config
GOCACHE=/tmp/lijiaoqiao-go-cache-final go test ./internal/domain ./internal/outbox ./internal/repository ./internal/compensation
git status --short
```

Expected:
- 相关 Go 包测试通过。
- 工作区只剩明确保留的项，或为空。

**Step 4: 提交最终删除批次**

Run:
```bash
git commit -m "docs(gates): replace root gate summaries with canonical archive"
```

---

### Task 8: 最终复核报告

**Files:**
- Modify: `supply-api/reports/CLEANUP_REPORT_2026-04-13.md`
- Modify: `reports/archive/gate_verification/INDEX_2026-04-13.md`

**Step 1: 写最终结论**

Write:
```md
- 已提交的归档批次
- 未纳入 Git 的原始产物分类
- 剩余本地留档目录
- 最终验证命令及结果
```

**Step 2: 验证**

Run:
```bash
git diff --check
git show --stat --oneline -10
```

**Step 3: 提交复核收尾**

Run:
```bash
git commit -m "docs(cleanup): finalize gate migration review"
```
