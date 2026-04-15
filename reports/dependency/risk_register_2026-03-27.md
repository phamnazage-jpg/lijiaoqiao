> 归档状态：已归档
> 归档批次：ARCHIVE-HISTORY-2026-04-14-A01
> 归档标识：AHR-20260414-009
> 当前用途：历史快照，仅供追溯；不得作为现行门禁、发布决议或实现状态事实源。
> 当前事实源：`docs/plans/2026-04-14-repo-integrity-refactor-plan.md`、`reports/archive/gate_verification/` 与仍在维护的签署/门禁文档。

# Dependency Risk Register（2026-03-27）

- Audit-Status: PASS

| Risk ID | Risk | Severity | Mitigation | Owner | Status |
|---|---|---|---|---|---|
| DEP-R-001 | 未锁定 subapi 精确版本导致回归 | High | 固定 `X.Y.Z` + 三重Gate | ARCH | Open |
| DEP-R-002 | 锁文件漂移未触发审计 | Medium | CI 强制执行 dependency-audit-check | PLAT | Open |
| DEP-R-003 | 漏洞库更新导致新 Critical CVE | High | 夜间扫描 + 发布阻断 | SEC | Open |

## Conclusion

1. 当前无新增依赖变更触发的阻断项。
2. 风险条目已登记并进入持续治理。
