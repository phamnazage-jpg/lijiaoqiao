# 2026-04-21 Release Manifest Contract

## P2-A-01 依赖 `latest_file_or_empty` 的脚本入口

### `scripts/ci/staging_release_pipeline.sh`

1. `LATEST_STAGING_RUN_LOG`
2. `LATEST_STAGE_REPORT`
3. `LATEST_TOKEN_READINESS`
4. `LATEST_TOK007_REPORT`
5. `LATEST_PIPELINE_REPORT`

### `scripts/ci/staging_evidence_autofill.sh`

1. `STAGING_RUN_LOG`
2. `SP_REPORT`
3. `TOK021_REPORT`
4. `TOK007_REPORT`
5. `PIPELINE_REPORT`

### `scripts/ci/tok007_release_recheck.sh`

1. `TOK006_REPORT`
2. `SP_REPORT`
3. `TOK_RUNTIME_READINESS_REPORT`

### `scripts/ci/final_decision_consistency_check.sh`

1. `TOK007_FILE`
2. `SP_FILE`

结论：

1. 当前四个脚本都通过“扫历史目录里最新文件”的方式串联证据。
2. 这种方式无法证明输入属于同一次发布运行，必须被 `run_id + manifest` 方案替换。

## P2-A-02 `run_id` 生成规则

格式：

`YYYYMMDD_HHMMSS_<short_sha>_<env>[-rNN]`

规则：

1. `YYYYMMDD_HHMMSS` 使用执行开始时间。
2. `<short_sha>` 使用 8 位提交号。
3. `<env>` 使用本次发布环境标识，如 `staging`、`prod`、`localmock`。
4. 若同秒内重复生成，追加 `-rNN` 顺序号，避免目录冲突。

## P2-A-03 发布目录结构

```text
reports/releases/<run_id>/
  manifest.json
  logs/
  gate_verification/
  review_outputs/
  evidence/
```

约束：

1. 同一次运行的产物只能写入自己的 `<run_id>` 目录。
2. 历史目录只读，不允许被当前运行覆盖。

## P2-A-04 `manifest.json` 必填字段

最小字段：

- `run_id`
- `commit_sha`
- `env`
- `created_at`
- `source_env_file`
- `artifact_paths`
- `decision_inputs`

`artifact_paths` 最少应覆盖：

- `staging_release_pipeline_report`
- `superpowers_release_pipeline_report`
- `staging_evidence_autofill_report`
- `tok007_recheck_report`
- `final_decision_consistency_report`

`decision_inputs` 最少应覆盖：

- `staging_run_log`
- `superpowers_stage_validation_report`
- `token_runtime_readiness_report`
- `tok006_gate_bundle_report`
- `supply_gate_review_report`
- `final_decision_report`

## P2-A-05 `staging_release_pipeline.sh` 输入改造草稿

方案：

1. 该脚本成为 manifest 的创建者。
2. 启动时生成 `run_id` 和 `reports/releases/<run_id>/manifest.json`。
3. 下游脚本统一接收 `--manifest <path>`。
4. 不再从历史目录回扫 `LATEST_*` 文件，而是把本次步骤产物写回 manifest。

## P2-A-06 `staging_evidence_autofill.sh` 输入改造草稿

方案：

1. 增加 `--manifest` 作为首选输入。
2. 只读取 manifest 中的 `decision_inputs` 和 `artifact_paths`。
3. 当 manifest 缺少必填路径时直接失败，不再回退到“最新文件”。

## P2-A-07 `tok007_release_recheck.sh` 输入改造草稿

方案：

1. 增加 `--manifest` 输入。
2. 复审所需的 TOK006 / stage validation / token readiness / SUP review / final decision 路径全部从 manifest 读取。
3. 复检脚本只审当前 `run_id` 的证据，不扫描历史目录。

## P2-A-08 `final_decision_consistency_check.sh` 输入改造草稿

方案：

1. 增加 `--manifest` 输入。
2. final decision、tok007 recheck、stage validation 三个来源全部绑定到同一个 `run_id`。
3. 若 manifest 中任一路径缺失或跨 run_id，直接判定 `FAIL`。
