# platform-token-runtime（TOK-002/003/004 开发实现）

本目录用于承载 token 运行态实现，当前默认仍以内存存储支持开发验证，但入口装配已经收口到 bootstrap 层。

## 文件说明

1. `cmd/platform-token-runtime/main.go`：可执行服务入口（环境变量 + 启停控制）。
2. `internal/app/bootstrap.go`：运行时与 HTTP server 装配。
3. `internal/httpapi/token_api.go`：`issue/refresh/revoke/introspect/audit-events` 接口处理。
4. `internal/httpapi/token_api_test.go`：HTTP 接口单测。
5. `internal/auth/middleware/*`：TOK-002 中间件与单测。
6. `internal/auth/service/token_verifier.go`：鉴权依赖接口、错误码、审计事件常量。
7. `internal/auth/service/runtime_store.go`：token 记录与幂等键存储抽象的内存实现。
8. `internal/auth/service/audit_store.go`：审计事件存储与查询的内存实现。
9. `internal/auth/service/inmemory_runtime.go`：基于 store 的最小可运行 token runtime。
10. `internal/token/*_template_test.go`：TOK-003/004 测试模板（按 `TOK-LIFE-*`/`TOK-AUD-*` 对齐）。
11. `internal/token/*_executable_test.go`：已转可执行用例（`TOK-LIFE-001~008`、`TOK-AUD-001~007`）。
12. `Dockerfile`：运行时镜像构建工件。

## 设计边界

1. 仅支持 `Authorization: Bearer <token>` 入站。
2. 外部 query key (`key/api_key/token`) 一律拒绝。
3. 不在任何响应或审计字段中输出上游凭证明文。

## 本地测试

```bash
cd "/home/long/project/立交桥/platform-token-runtime"
export PATH="/home/long/project/立交桥/.tools/go-current/bin:$PATH"
export GOCACHE="/tmp/go-cache"
export GOPATH="/tmp/go"
go test ./...
```

## 本地运行

```bash
cd "/home/long/project/立交桥/platform-token-runtime"
export PATH="/home/long/project/立交桥/.tools/go-current/bin:$PATH"
go run ./cmd/platform-token-runtime
```

服务默认监听 `:18081`，可通过 `TOKEN_RUNTIME_ADDR` 覆盖。

## 环境约束

- `TOKEN_RUNTIME_ENV=dev` 时，如果没有显式注入 store，bootstrap 会自动使用内存 runtime store 和审计 store。
- `TOKEN_RUNTIME_ENV=staging` 或 `TOKEN_RUNTIME_ENV=prod` 时，必须由 bootstrap 注入具体 runtime store 与 audit store；当前仓库还没有持久化 store 实现，因此进程会以“缺少 store 配置”快速失败，而不是伪装成可上线服务。

## 仓库级验证

```bash
cd "/home/long/project/立交桥"
bash scripts/ci/repo_integrity_check.sh
```
