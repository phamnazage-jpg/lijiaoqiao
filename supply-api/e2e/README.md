# supply-api service-http 测试说明

`e2e/` 目录保留现有 build tag 路径，但这里的测试分类在 Phase P2-D 之后明确记为 `service-http`，不是“真实部署 E2E”。

当前边界：

- 进程内：`newE2ESystem` 直接在单进程内装配 handler、内存审计存储、静态 token backend。
- 单服务 HTTP：覆盖 `supply-api` 的路由、鉴权、审计和关键业务剧本。
- 非跨服务：不会启动 `gateway`、`platform-token-runtime`、真实数据库或外部网络依赖。

新的测试分类名：

- `unit`: 单函数、单组件、无跨进程依赖。
- `integration`: 真实数据库/仓储/适配层集成。
- `service-http`: 单服务 HTTP surface，在进程内装配依赖。
- `cross-service-smoke`: `gateway -> token-runtime -> supply-api` 的真实跨服务链路。

运行方式：

```bash
go test -tags=e2e ./e2e
```

如果只想跑单个测试：

```bash
go test -tags=e2e ./e2e -run TestPlaybook_SupplierOnboarding
```

约束说明：

- `e2e/` 目录下的 build-tag 测试应保留在 `*_test.go` 文件内，但对外一律称为 `service-http`。
- 真实跨服务 smoke 不得继续写进 `supply-api/e2e/`，应放到 `tests/smoke/` 与 `scripts/ci/cross_service_smoke.sh`。
- 新增剧本时优先复用 `newE2ESystem`，避免重复搭建单服务测试系统。
