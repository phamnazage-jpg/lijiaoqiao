# Staging 发布流水报告

- 时间戳：2026-03-31_100116
- 执行脚本：`scripts/ci/staging_release_pipeline.sh`
- 环境文件：`scripts/supply-gate/.env.local-mock`
- 环境分类：`LOCAL_MOCK`
- local/mock 显式确认：`1`
- 结果：**PASS**
- 说明：all steps finished

## 步骤结果

| 步骤 | 结果 | 说明 | 证据 |
|---|---|---|---|
| STEP-01 | PASS | Staging precheck and run_all | /home/long/project/立交桥/reports/gates/step-01_2026-03-31_100116.out.log |
| STEP-02 | PASS | Superpowers release pipeline with staging env | /home/long/project/立交桥/reports/gates/step-02_2026-03-31_100116.out.log |
| STEP-03 | PASS | Staging evidence autofill | /home/long/project/立交桥/reports/gates/step-03_2026-03-31_100116.out.log |
