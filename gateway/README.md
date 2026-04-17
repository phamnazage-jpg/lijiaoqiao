# Gateway

> OpenAI 兼容入口网关，负责接入、鉴权、限流、上游路由与基础审计。

## 当前真实状态

- 服务入口是 `cmd/gateway/main.go`。
- 当前对外暴露的主要接口是 `/v1/chat/completions`、`/v1/completions`、`/v1/models`，以及对应的 `/api/v1/*` 兼容路径。
- `/v1/models` 现在返回当前已注册 provider 的模型并集，按模型 ID 去重后排序，不再返回硬编码静态列表。
- 鉴权运行时支持两种模式：
  - `inmemory`
  - `remote_introspection`
- provider 注册已经从配置装配；如果未显式配置 provider，启动时会基于环境变量生成默认 OpenAI provider。
- 审计发射器支持 PostgreSQL 与内存实现；数据库未配置时会显式回退到内存实现。
- 生产环境必须显式提供 `PASSWORD_ENCRYPTION_KEY` 与 `GATEWAY_CORS_ALLOW_ORIGINS`；缺省密钥和 `*` CORS 只允许在开发态使用。

## 与其他服务的边界

- `gateway` 不签发业务 token。
- `gateway` 在 `remote_introspection` 模式下依赖 `platform-token-runtime` 提供 token introspection。
- `gateway` 只承载入口控制，不承载 `supply-api` 或 `platform-token-runtime` 的业务逻辑。

## 本地运行

```bash
cd "/home/long/project/立交桥/gateway"
export OPENAI_API_KEY="..."
export OPENAI_BASE_URL="https://api.openai.com"
export OPENAI_MODELS="gpt-4,gpt-3.5-turbo"
export GATEWAY_ENV="dev"
export GATEWAY_TOKEN_RUNTIME_MODE="inmemory"
go run ./cmd/gateway
```

如果要切到远程 token 校验：

```bash
export GATEWAY_TOKEN_RUNTIME_MODE="remote_introspection"
export GATEWAY_TOKEN_RUNTIME_URL="http://127.0.0.1:18081"
```

默认监听 `0.0.0.0:8080`。

生产环境额外要求：

```bash
export GATEWAY_ENV="production"
export PASSWORD_ENCRYPTION_KEY="32-byte-production-secret........"
export GATEWAY_CORS_ALLOW_ORIGINS="https://console.example.com,https://app.example.com"
```

## 验证命令

模块级验证：

```bash
cd "/home/long/project/立交桥/gateway"
go test -count=1 ./...
```

仓库级统一验证：

```bash
cd "/home/long/project/立交桥"
bash scripts/ci/repo_integrity_check.sh
```

## 关键目录

- `internal/config/`：环境变量配置与 provider 配置加载。
- `internal/handler/`：OpenAI 兼容 HTTP handler。
- `internal/middleware/`：鉴权、CORS、远程 introspection。
- `internal/router/`：provider 路由、打分与 fallback。

## 路由策略说明

- 主启动链路当前只接入 `latency`、`round_robin`、`weighted`、`availability` 四种策略。
- `internal/router/strategy/cost_based.go`、`internal/router/strategy/cost_aware.go` 与 `internal/router/fallback/` 仍属于实验性模块，没有接入 `BuildServer` 的运行时装配。
- 如果配置里填入未接入的策略名，启动链路会回退到 `latency`。
