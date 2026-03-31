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
