> 归档状态：已归档
> 归档批次：ARCHIVE-HISTORY-2026-04-14-A01
> 归档标识：AHR-20260414-008
> 当前用途：历史快照，仅供追溯；不得作为现行门禁、发布决议或实现状态事实源。
> 当前事实源：`docs/plans/2026-04-14-repo-integrity-refactor-plan.md`、`reports/archive/gate_verification/` 与仍在维护的签署/门禁文档。

# Lockfile Diff（2026-03-27）

- Audit-Status: PASS
- Scope: Baseline document-only sync

## Summary

1. `go.mod/go.sum`：无本次变更。
2. `package-lock.json` / `pnpm-lock.yaml`：无本次变更。
3. `pom.xml`：无本次变更。

## Risk

1. 本次提交仅含文档与 SQL，不涉及应用依赖升级。
2. 依赖风险等级：Low。
