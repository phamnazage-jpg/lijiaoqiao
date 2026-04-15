> 归档状态：已归档
> 归档批次：ARCHIVE-HISTORY-2026-04-14-A01
> 归档标识：AHR-20260414-006
> 当前用途：历史快照，仅供追溯；不得作为现行门禁、发布决议或实现状态事实源。
> 当前事实源：`docs/plans/2026-04-14-repo-integrity-refactor-plan.md`、`reports/archive/gate_verification/` 与仍在维护的签署/门禁文档。

# Dependency Compatibility Matrix（2026-03-27）

- Audit-Status: PASS

| Component | Baseline | Current | Result | Note |
|---|---|---|---|---|
| Go | 1.21.x | 1.21.x（文档基线） | PASS | 与架构基线一致 |
| PostgreSQL | 15.x | 15.x（SQL 语法） | PASS | DDL 在 PG15 实测通过 |
| Redis | 7.x | 7.x（文档基线） | PASS | 与架构基线一致 |
| subapi | X.Y.Z fixed | 未变更 | PASS | 无依赖升级 |
| Frontend Node | 20.x LTS | 未变更 | PASS | 无依赖升级 |

## Conclusion

1. 本次无 runtime 依赖变更。
2. 兼容性审计结果可放行。
