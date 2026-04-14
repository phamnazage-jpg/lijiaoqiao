# Gateway

> OpenAI 兼容入口网关，负责请求接入、鉴权、限流、上游路由和基础审计。

## 当前真实状态

- 服务入口是 [main.go](/home/long/project/立交桥/gateway/cmd/gateway/main.go)。
- 当前对外暴露的主要接口是 `/v1/chat/completions`、`/v1/completions`、`/v1/models`，以及对应的 `/api/v1/*` 兼容路径。
- 鉴权运行时支持两种模式：
  - `inmemory`
  - `remote_introspection`
- 当前 provider 注册仍是代码内装配，不是配置驱动。默认只在入口里注册 OpenAI adapter。
- 审计发射器支持 PostgreSQL 与内存实现；数据库未配置时会回退到内存实现。

## 目录结构

```text
gateway/
├── cmd/gateway/main.go              # 进程入口与优雅关闭
├── internal/adapter/                # 上游 provider 适配
├── internal/config/                 # 环境变量配置
├── internal/handler/                # OpenAI 兼容 HTTP handler
├── internal/middleware/             # 鉴权、CORS、远程 introspection
├── internal/ratelimit/              # 令牌桶 / 滑动窗口限流
├── internal/router/                 # 路由、打分、fallback
└── pkg/                             # 通用模型与错误码
```

## 与其他服务的边界

- `gateway` 自己不签发业务 token。
- `gateway` 在 `remote_introspection` 模式下依赖 `platform-token-runtime` 提供 token introspection。
- `gateway` 保护 `supply-api` 和 `platform-token-runtime` 的受保护路径，但不承载它们的业务逻辑。

## 运行

### 必要环境变量

```bash
export OPENAI_API_KEY="..."
export GATEWAY_ENV="dev"
export GATEWAY_TOKEN_RUNTIME_MODE="inmemory"
```

如果要走远程 token 校验：

```bash
export GATEWAY_TOKEN_RUNTIME_MODE="remote_introspection"
export GATEWAY_TOKEN_RUNTIME_URL="http://127.0.0.1:18081"
```

### 本地启动

```bash
cd "/home/long/project/立交桥/gateway"
go run ./cmd/gateway
```

默认监听 `0.0.0.0:8080`。

## 测试

模块级验证：

```bash
cd "/home/long/project/立交桥/gateway"
go test ./...
```

仓库级统一验证：

```bash
cd "/home/long/project/立交桥"
bash scripts/ci/repo_integrity_check.sh
```

## 已知限制

- `internal/config.Config.Providers` 已存在，但当前入口尚未按该配置动态注册 provider。
- 生产级 provider 装配和更细的 bootstrap 拆分，仍在后续重构计划中。
