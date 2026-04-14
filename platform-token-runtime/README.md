# Platform Token Runtime

> token 生命周期、introspection 与审计查询服务。

## 当前真实状态

- 服务入口是 `cmd/platform-token-runtime/main.go`，装配逻辑收口在 `internal/app/bootstrap.go`。
- 当前可用接口包括 `issue`、`refresh`、`revoke`、`introspect`、`audit-events`。
- `TOKEN_RUNTIME_ENV=dev` 且未显式注入 store 时，bootstrap 会自动使用内存 runtime store 与内存 audit store。
- `TOKEN_RUNTIME_ENV=staging` 或 `TOKEN_RUNTIME_ENV=prod` 时，必须显式注入 runtime store 与 audit store；当前仓库仍未提供持久化 store，因此这两种环境会快速失败，而不是伪装成可上线服务。
- `audit-events` 当前始终保持可查询接口语义；默认内存 audit store 会返回真实事件，未提供查询能力的自定义 emitter 会返回空结果而不是 `501` 占位响应。

## 设计边界

1. 仅支持 `Authorization: Bearer <token>` 入站。
2. 外部 query key（`key`、`api_key`、`token`）一律拒绝。
3. 不在任何响应或审计字段中输出 access token 明文。

## 本地运行

```bash
cd "/home/long/project/立交桥/platform-token-runtime"
go run ./cmd/platform-token-runtime
```

默认监听 `:18081`。可通过以下环境变量覆盖：

```bash
export TOKEN_RUNTIME_ADDR=":18081"
export TOKEN_RUNTIME_ENV="dev"
```

## 验证命令

模块级验证：

```bash
cd "/home/long/project/立交桥/platform-token-runtime"
GOCACHE=/tmp/lijiaoqiao-go-cache-platform-token-runtime go test ./...
```

仓库级统一验证：

```bash
cd "/home/long/project/立交桥"
bash scripts/ci/repo_integrity_check.sh
```

## 关键文件

- `internal/app/bootstrap.go`：环境判断、runtime store / audit store 装配。
- `internal/httpapi/token_api.go`：HTTP 接口与审计查询输出。
- `internal/auth/service/runtime_store.go`：内存 runtime store。
- `internal/auth/service/audit_store.go`：内存 audit store 与审计查询。
