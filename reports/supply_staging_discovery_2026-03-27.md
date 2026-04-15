> 归档状态：已归档
> 归档批次：ARCHIVE-HISTORY-2026-04-14-A01
> 归档标识：AHP-20260414-013
> 当前用途：历史快照，仅供追溯；不得作为现行门禁、发布决议或实现状态事实源。
> 当前事实源：`docs/plans/2026-04-14-repo-integrity-refactor-plan.md`、`reports/archive/gate_verification/` 与仍在维护的签署/门禁文档。

# SUP Staging 环境发现报告（2026-03-27）

## 1. 目标

确认当前机器上是否存在可直接用于 `SUP-004~SUP-007` 的真实供应侧 API 环境。

## 2. 检查结果

1. 端口 `8080` 服务可用，健康检查通过：`/actuator/health -> UP`。
2. 但供应侧接口全部返回 `404`：
   - `/api/v1/supply/accounts/verify`
   - `/api/v1/supply/accounts`
   - `/api/v1/supply/packages/draft`
   - `/api/v1/supply/settlements/withdraw`
   - `/api/v1/chat/completions`
3. `8080` 的 `/v3/api-docs` 显示其为“蚊子项目 API 文档”，非立交桥供应侧服务。

## 3. 结论

1. 当前机器未发现可直接复用的“立交桥供应侧真实 staging API”。
2. 因此本轮 `SUP` 全链路证据先采用 local-mock 完成脚本联调闭环。
3. 生产放行仍需你提供真实 `API_BASE_URL` 与短期 token 后复跑。
4. 新增实现审计确认：当前仓库 token 能力未形成可验证运行态实现（仅文档与 mock）。

## 4. 下一步

1. 获取真实 staging 地址与三类 token（owner/viewer/admin）。
2. 使用 `scripts/supply-gate/.env` 填写真实值。
3. 执行 `bash scripts/supply-gate/run_all.sh scripts/supply-gate/.env`。
4. 将 `PASS（mock）` 替换为 `PASS（staging）` 并更新最终决议。
5. 先关闭 `reports/token_runtime_implementation_gap_review_2026-03-27.md` 中 P0 缺口，再申请生产 GO。
