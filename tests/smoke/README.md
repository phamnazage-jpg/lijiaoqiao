# Cross-Service Smoke Design

## 测试边界

`tests/smoke/` 只描述真实跨服务 smoke 的链路与证据，不承载单服务进程内测试。

分类边界：

1. `unit`
   - 单函数或单组件测试。
2. `integration`
   - 真实数据库、仓储或适配层集成测试。
3. `service-http`
   - 单服务 HTTP surface，通常在进程内组装依赖。
4. `cross-service-smoke`
   - 至少覆盖 `gateway -> token-runtime -> supply-api` 的真实链路。

## 最小链路

最小 smoke 步骤：

1. 检查 `gateway` 健康状态。
2. 检查 `platform-token-runtime` 健康状态。
3. 检查 `supply-api` 健康状态。
4. 使用真实 smoke token 通过 `gateway` 发起一次受保护请求。
5. 验证该请求需要经过 token-runtime 权限解释，并落到 supply-api 受保护 HTTP surface。

## 输入环境变量

`scripts/ci/cross_service_smoke.sh` 计划使用以下输入：

- `SMOKE_GATEWAY_BASE_URL`
- `SMOKE_TOKEN_RUNTIME_BASE_URL`
- `SMOKE_SUPPLY_API_BASE_URL`
- `SMOKE_BEARER_TOKEN`
- `SMOKE_EXPECTED_SCOPE`
- `SMOKE_EXPECTED_MODEL`
- `SMOKE_ALLOW_LOCAL_PLACEHOLDER`

## 输出工件

输出必须稳定落到：

- `reports/archive/gate_verification/cross_service_smoke_<timestamp>.log`
- `reports/archive/gate_verification/cross_service_smoke_<timestamp>.md`

后续要求：

1. smoke markdown 报告可被 release manifest 收录。
2. `SKIP_LOCAL_PLACEHOLDER` 不得计入 release pass。
3. 只有真实 smoke `PASS` 才能作为发布路径证据。
