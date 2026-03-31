# platform-token-runtime（TOK-002/003/004 开发实现）

本目录用于承载 token 运行态的开发阶段实现，不依赖真实 staging 参数。

## 文件说明

1. `cmd/platform-token-runtime/main.go`：可执行服务入口（HTTP + 健康检查）。
2. `internal/httpapi/token_api.go`：`issue/refresh/revoke/introspect` 接口处理。
3. `internal/httpapi/token_api_test.go`：HTTP 接口单测。
4. `internal/auth/middleware/*`：TOK-002 中间件与单测。
2. `internal/auth/service/token_verifier.go`：鉴权依赖接口、错误码、审计事件常量。
3. `internal/auth/service/inmemory_runtime.go`：开发阶段最小可运行内存实现（签发/续期/吊销/introspect + 鉴权接口实现）。
4. `internal/token/*_template_test.go`：TOK-003/004 测试模板（按 `TOK-LIFE-*`/`TOK-AUD-*` 对齐）。
5. `internal/token/*_executable_test.go`：已转可执行用例（`TOK-LIFE-001~008`、`TOK-AUD-001~007`）。
6. `Dockerfile`：运行时镜像构建工件。

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
