# 整改关闭确认单

生成时间：2026-04-20
仓库路径：`/home/long/project/立交桥`
确认对象：`2026-04-20` 真实环境验证报告、接口矩阵报告及其派生整改任务单

关联文档：

- [REAL_ENV_REVIEW_AND_VALIDATION_REPORT_2026-04-20.md](/home/long/project/立交桥/review/REAL_ENV_REVIEW_AND_VALIDATION_REPORT_2026-04-20.md)
- [API_MATRIX_VALIDATION_REPORT_2026-04-20.md](/home/long/project/立交桥/review/API_MATRIX_VALIDATION_REPORT_2026-04-20.md)
- [2026-04-20-remediation-tasklist-from-real-validation.md](/home/long/project/立交桥/docs/plans/2026-04-20-remediation-tasklist-from-real-validation.md)

## 1. 关闭结论

截至 2026-04-20 本次复核完成时，上述三份文档中已拆入整改任务单的 `13` 个已证实问题已全部完成修复，并通过整仓基线、focused 回归、仓储集成和稳定性复跑核验。

本确认单给出的正式结论是：

- 本轮整改任务单内的 `13` 个问题，已全部关闭。
- 当前没有证据表明这些问题仍然存在于当前分支。
- 两份 `review` 文档仍然保留“整改前基线快照”属性，不能再被直接引用为当前未修复缺陷清单。

## 2. 复核命令

以下命令为本次关闭确认实际执行并通过的核心命令：

```bash
bash scripts/ci/repo_integrity_check.sh
bash scripts/ci/supply_domain_stability_check.sh 20

cd "/home/long/project/立交桥/platform-token-runtime" && \
  go test -count=1 ./internal/auth/service \
  -run 'Test(PostgresRuntimeStore_SavePreservesExistingFingerprintWhenAccessTokenMissing|InMemoryTokenRuntimeWithPostgresStore_RefreshAndRevokePersistLifecycle)$' -v

cd "/home/long/project/立交桥/supply-api" && \
  go test -count=1 ./internal/httpapi \
  -run 'TestSupplyAPI_(ActivateAccount_ConcurrencyConflict|PublishPackage_ConcurrencyConflict|ClonePackage_UnexpectedCreateFailureReturnsInternalServerError|CancelSettlement_ConcurrencyConflict|ActivateAccount_NotFound|PublishPackage_NotFound|ClonePackage_NotFound|CancelSettlement_NotFound)$' -v

cd "/home/long/project/立交桥/supply-api" && \
  go test -count=1 ./internal/iam/... \
  -run 'Test.*(AssignRole|RevokeRole|ListRoles|GetUserRoles|UpdateRole)' -v

cd "/home/long/project/立交桥/supply-api" && \
  go test -count=1 ./internal/domain ./internal/httpapi \
  -run 'Test.*(Activate|Suspend|Delete|Publish|Pause|Unlist|Clone|Cancel)' -v

cd "/home/long/project/立交桥/supply-api" && \
  bash scripts/run_integration_tests.sh ./internal/iam/repository

cd "/home/long/project/立交桥/supply-api" && \
  bash scripts/run_integration_tests.sh ./internal/audit/...
```

## 3. 问题关闭映射

| 问题类别 | 当前状态 | 复核依据 |
| --- | --- | --- |
| `platform-token-runtime` PostgreSQL `refresh/revoke` 失败 | 已关闭 | focused store/runtime 测试通过 |
| `supply-api` 幂等锁 DDL/仓储契约失配 | 已关闭 | repository integration 与整仓校验通过 |
| `supply-api` 套餐创建 SQL 占位符错误 | 已关闭 | handler/domain focused 回归通过 |
| `supply-api` 账号状态流转乐观锁错误 | 已关闭 | handler/domain focused 回归通过 |
| `supply-api` 套餐状态流转乐观锁错误 | 已关闭 | handler/domain focused 回归通过 |
| `supply-api` 套餐读取字段映射错误 | 已关闭 | lifecycle focused 回归通过 |
| `supply-api` 审计仓储与 `audit_events` 契约失配 | 已关闭 | `./internal/audit/...` integration 通过 |
| `IAM` DDL 无法在干净库落地 | 已关闭 | `TestIAMSchemaV1_AppliesOnCleanSchema` 通过 |
| `IAM` 角色列表 / 用户角色 null scan | 已关闭 | `./internal/iam/...` focused 回归通过 |
| `IAM` 更新空字符串写入 `INET` | 已关闭 | `UpdateRole` focused 回归通过 |
| `IAM` 角色分配 `granted_by` 外键失败 | 已关闭 | `AssignRole` focused 回归通过 |
| handler 冲突/内部错误语义错误 | 已关闭 | HTTP focused 回归通过 |
| `supply-api/internal/domain` 波动信号 | 已关闭 | `20` 轮 stability check 未复现 |

## 4. 范围边界

以下事项不应与本次“13 项整改关闭”混淆：

1. 提现能力仍受 SMS readiness 门禁控制。门禁关闭属于设计行为，不是未修复缺陷。
2. 两份 `review` 报告是整改前基线快照。它们用于说明问题来源，不等于当前系统状态。
3. 结构化日志统一尚未在三套服务入口完全收口。这是**任务单外治理项**，不属于本次 13 项缺陷。

结构化日志现状：

- [main.go](/home/long/project/立交桥/supply-api/cmd/supply-api/main.go) 已使用结构化 JSON logger。
- [main.go](/home/long/project/立交桥/gateway/cmd/gateway/main.go) 仍使用标准库 `log`。
- [main.go](/home/long/project/立交桥/platform-token-runtime/cmd/platform-token-runtime/main.go) 仍使用标准库 `log`。

因此，若单独追踪“结构化日志统一”这一治理目标，当前结论应是：

- `supply-api`：已完成
- `gateway`：未完成
- `platform-token-runtime`：未完成

但这不应被回写成“2026-04-20 真实验证缺陷仍未关闭”。

## 5. 最终确认

本次复核后的正式确认如下：

- 真实验证报告与接口矩阵报告中拆出的 `13` 个已证实问题，已全部真实解决。
- 当前分支未发现这些问题的残留复现。
- 若后续需要继续推进，可把“结构化日志统一”作为新的治理任务立项，而不是复开本次整改单。
