# Staging 发布流水报告

- 时间戳：2026-03-30_205035
- 执行脚本：`scripts/ci/staging_release_pipeline.sh`
- 环境文件：`scripts/supply-gate/.env`
- 环境分类：`REAL_STAGING`
- local/mock 显式确认：`0`
- 结果：**FAIL**
- 说明：at least one step failed

## 步骤结果

| 步骤 | 结果 | 说明 | 证据 |
|---|---|---|---|
| STEP-01 | FAIL | Staging precheck and run_all | /home/long/project/立交桥/reports/gates/step-01_2026-03-30_205035.out.log |
| STEP-02 | FAIL | Superpowers release pipeline with staging env | /home/long/project/立交桥/reports/gates/step-02_2026-03-30_205035.out.log |
| STEP-03 | PASS | Staging evidence autofill | /home/long/project/立交桥/reports/gates/step-03_2026-03-30_205035.out.log |
