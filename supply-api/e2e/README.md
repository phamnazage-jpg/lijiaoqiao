# E2E 测试说明

`e2e/` 目录只存放带 `//go:build e2e` 的端到端测试源码，不再混放伪装成文档的 Go 文件。

当前测试分层如下：

- `e2e_test.go`: 核心 HTTP API、鉴权和审计行为的端到端断言。
- `playbook_test.go`: 按业务剧本组织的多步骤流程验证。
- `production_flow_test.go`: 面向上线前复核的关键流程和安全边界检查。

运行方式：

```bash
go test -tags=e2e ./e2e
```

如果只想跑单个测试：

```bash
go test -tags=e2e ./e2e -run TestPlaybook_SupplierOnboarding
```

约束说明：

- E2E 测试应保留在 `*_test.go` 文件内。
- 说明文档只保留 Markdown 内容，不内嵌 Go 源码。
- 新增剧本时优先复用 `newE2ESystem`，避免重复搭建测试系统。
