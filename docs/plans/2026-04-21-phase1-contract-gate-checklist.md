# 2026-04-21 Phase 1 Contract Gate Checklist

## 关闭条件

只有当以下项目全部满足，Phase 1 才能标记完成：

- [ ] 合法 token contract 场景通过，并保留 gateway / token runtime / supply-api 的证据。
- [ ] 吊销 token contract 场景通过，并确认 gateway 与 supply-api 都拒绝。
- [ ] scope 不足 contract 场景通过，并确认拒绝码稳定。
- [ ] token runtime 不可用 contract 场景通过，并确认失败语义稳定。
- [ ] `backend-verify.sh` 已接入 contract gate 执行位。
- [ ] `repo_integrity_check.sh` 已接入 contract gate 入口。
- [ ] contract gate 产物落在 `reports/archive/gate_verification/contract_gate_<timestamp>.log|md`。
- [ ] 最近一次 Phase 1 执行记录引用了 contract gate evidence。

## 执行顺序

1. 先跑 service-local suites。
2. 再跑 contract gate。
3. 最后核对 execution log、plan 和 evidence 是否一致。

## 一票否决项

任一情况出现，Phase 1 不得关闭：

1. contract gate 没有 evidence。
2. contract gate 有 `FAIL`。
3. principal 关键字段缺失。
4. 吊销 token 仍被 gateway 或 supply-api 放行。
5. token runtime 不可用时出现不稳定错误码或无限等待。
