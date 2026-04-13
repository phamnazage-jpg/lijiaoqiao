# Final Decision Consistency Check

- 时间戳：2026-03-30_145749
- 执行脚本：`scripts/ci/final_decision_consistency_check.sh`

## 1. 输入源

| 来源 | 路径 | 解析结论 |
|---|---|---|
| final_decision | /home/long/project/立交桥/review/final_decision_2026-03-31.md | NO_GO |
| tok007_recheck | /home/long/project/立交桥/review/outputs/tok007_release_recheck_2026-03-30_145306.md | CONDITIONAL_GO |
| superpowers_stage_validation | /home/long/project/立交桥/reports/gates/superpowers_stage_validation_2026-03-30_145305.md | CONDITIONAL_GO |

## 2. 一致性结果

- 状态：**WARN**
- 说明：final signed decision lags latest machine recheck; requires manual review update

## 3. 建议动作

1. 若状态为 WARN：人工确认是否需要更新 `review/final_decision_2026-03-31.md` 的勾选与签署记录。
2. 若状态为 FAIL：先修复报告来源或解析格式，再重新执行本检查。
3. staging 真值就绪后，按顺序重跑：
   1. `scripts/ci/superpowers_stage_validate.sh`
   2. `scripts/ci/tok007_release_recheck.sh`
   3. `scripts/ci/final_decision_consistency_check.sh`
