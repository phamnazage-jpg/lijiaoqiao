# 对外变更说明（2026-04-15）

- 发布日期：`2026-04-15`
- 适用范围：`2026-04-15` 合并的一组工程治理、验证链路、测试覆盖与证据归档更新。
- 口径边界：本说明仅覆盖仓内已提交变更；真实 STG 放行、正式签署结论与生产可用性，仍以现行签署与门禁文档为准。

## 1. 变更摘要

1. 收紧真实 STG 验证口径。`PHASE-07` 仅在真实 staging 条件满足时才允许判定 `PASS`；`local/mock` 输入统一记为 `DEFERRED` 或 `BLOCKED`。
2. 统一验证链路内核。`repo_integrity_check`、`backend-verify`、`superpowers_stage_validate` 已改为复用同一套验证公共逻辑，减少重复解析与结果漂移。
3. 补齐核心包测试覆盖。为此前覆盖薄弱的核心包补充单元测试，并对少量注入边界做了最小化可测性改造。
4. 收紧机审稿引用治理。活文档只允许引用 `review/outputs/current_machine_review_sources.md` 中声明的现行机审稿；其余 `review/outputs/` 同类文件全部按历史快照处理。
5. 完成历史报告与最新验证证据归档。历史评审与阶段性报告已补归档头；`2026-04-14` 至 `2026-04-15` 期间新增的门禁、趋势与漂移证据已归档入库。

## 2. 对外影响

- 本次不新增对外 API，不改变现有接口字段、鉴权协议或调用入口。
- 本次不放宽任何放行门槛；相反，真实 staging 判定比此前更严格。
- 本次提升了验证结果的一致性、可追溯性与审计可读性，减少历史快照误用为当前事实源的风险。
- 本次对批量补偿与 outbox 领域层做了收敛，只保留运行路径实际消费的共享重试语义，删除未被实际运行层使用的冗余领域表述。

## 3. 当前验证状态

截至 `2026-04-15`，与本次变更直接相关的仓内验证结论如下：

1. `scripts/ci/repo_integrity_check.sh`：通过。
2. `scripts/ci/backend-verify.sh`：通过。
3. `go test ./internal/domain ./internal/outbox`：通过。
4. `scripts/ci/superpowers_stage_validate.sh`：结果为 `CONDITIONAL_GO`。

当前 `CONDITIONAL_GO` 的直接原因不是代码失败，而是 `PHASE-07` 在 `local-mock` 环境下只能记为 `DEFERRED`，尚未获得真实 staging `PASS`。相关现行事实源如下：

- `review/outputs/current_machine_review_sources.md`
- `reports/archive/gate_verification/superpowers_stage_validation_2026-04-15_100559.md`
- `reports/archive/gate_verification/staging_real_readiness_2026-04-15_072308.md`

## 4. 当前不宣称事项

- 不宣称已取得真实 staging 放行。
- 不宣称正式签署结论已随本次工程治理自动变更。
- 不将任何历史评审快照、历史机审稿或历史门禁报告作为当前事实源或对外依据。

## 5. 提交映射

| 提交 | 主题 | 说明 |
|---|---|---|
| `c719402` | 验证内核与 staging 口径 | 收紧 `PHASE-07` 语义，并抽出共享验证内核。 |
| `567446b` | 核心测试补齐 | 为关键包补齐单测并做最小化可测性改造。 |
| `3bedb37` | 机审稿治理自动化 | 建立现行机审稿指针与历史快照自动降级机制。 |
| `ec639dd` | 历史报告归档头 | 为历史 `reports/` 与 `review/` 文档补归档头与批次索引。 |
| `f958ab4` | 门禁产物目录收口 | 将活脚本统一切换到 `reports/archive/gate_verification`。 |
| `0c370e9` | 领域层收敛 | 简化 outbox 重试契约，并让 batch compensation 显式处理 `retrying` 状态。 |
| `852abc3` | 验证证据归档 | 归档 `2026-04-14` 至 `2026-04-15` 的门禁、趋势与漂移报告。 |

## 6. 建议对外口径

可对外表述为：本轮更新以工程治理、验证一致性、测试覆盖和证据归档为主，重点修正了真实 staging 判定口径、现行机审稿引用约束和验证链路重复实现问题。当前仓内关键验证已通过，但真实 staging 放行仍待独立完成，不应提前表述为“已完成正式放行”。
