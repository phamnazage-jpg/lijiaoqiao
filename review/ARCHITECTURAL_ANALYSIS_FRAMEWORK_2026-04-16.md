# 架构问题分析报告

**项目**: /home/long/project/立交桥/
**日期**: 2026-04-16
**分析范围**: supply-api, gateway, platform-token-runtime, llm-gateway-competitors/sub2api

---

## 1. 框架使用不一致问题

### 1.1 HTTP 框架使用矩阵

| 服务 | 框架 | Go版本 | 路由方式 |
|------|------|--------|----------|
| supply-api | 标准库 net/http | 1.21 | http.ServeMux + 手动注册 |
| gateway | 标准库 net/http | 1.21 | http.ServeMux + 手动注册 |
| platform-token-runtime | 标准库 net/http | 1.22 | http.ServeMux + 手动注册 |
| sub2api | Gin | 1.26.1 | gin.Engine 路由组 |

### 1.2 问题详情

#### 问题 1.2.1: platform-token-runtime 依赖管理异常
**严重程度**: 🔴 严重

`platform-token-runtime/go.mod` 仅包含:
```
module lijiaoqiao/platform-token-runtime

go 1.22
```

**问题**: 该服务源码存在 (cmd/, internal/)，但 `go.mod` 没有任何业务依赖声明。这表明:
1. 可能从未执行 `go mod tidy`
2. 或依赖被错误删除
3. 与其他服务 (pgx, redis, viper 等) 无法共享

#### 问题 1.2.2: Gin vs 标准库混用
**严重程度**: 🟡 中等

- `sub2api` 使用 Gin 框架 (`github.com/gin-gonic/gin`)
- 其他三个服务使用纯标准库 `net/http`

**影响**:
1. 开发者需要熟悉两套不同的 Web 开发范式
2. 中间件无法跨服务复用
3. 测试方式不同 (Gin 测试助手 vs 标准库 httptest)

#### 问题 1.2.3: 路由注册模式差异

**supply-api** (supply_api.go):
```go
func (api *SupplyAPI) Register(mux *http.ServeMux) {
    mux.HandleFunc(tokenBasePath+"/issue", a.handleIssue)
    // ...
}
```

**token-runtime** (token_api.go):
```go
func (a *TokenAPI) Register(mux *http.ServeMux) {
    mux.HandleFunc(tokenBasePath+"/issue", a.handleIssue)
    // ...
}
```

**gateway** (bootstrap.go):
```go
func BuildMux(...) http.Handler {
    mux := http.NewServeMux()
    mux.HandleFunc("/v1/chat/completions", ...)
    // ...
}
```

**问题**: 三种不同的注册模式，但本质相同却无法抽象。

---

## 2. 依赖管理混乱问题

### 2.1 直接依赖版本对比

| 依赖 | supply-api | gateway | token-runtime | sub2api |
|------|-------------|---------|---------------|---------|
| pgx | v5.5.1 | v5.5.0 | ❌ | - |
| redis | v9.4.0 | - | ❌ | v9.17.2 |
| viper | v1.18.2 | - | ❌ | v1.18.2 |
| jwt | v5.2.0 | - | ❌ | v5.2.2 |
| testify | v1.8.4 | v1.8.1 | ❌ | v1.11.1 |
| Gin | ❌ | ❌ | ❌ | v1.9.1 |
| go-zero | ❌ | ❌ | ❌ | v1.9.4 |
| zap | ❌ | ❌ | ❌ | v1.24.0 |

### 2.2 问题详情

#### 问题 2.2.1: 相同依赖版本不一致
**严重程度**: 🟡 中等

- `pgx/v5`: supply-api 用 v5.5.1，gateway 用 v5.5.0
- `jwt/v5`: supply-api 用 v5.2.0，sub2api 用 v5.2.2
- `viper`: supply-api 和 sub2api 都是 v1.18.2 (一致)
- `testify`: 三种版本 1.8.4, 1.8.1, 1.11.1

#### 问题 2.2.2: platform-token-runtime 依赖完全缺失
**严重程度**: 🔴 严重

该服务 `go.mod` 缺少所有必要依赖:
- 无数据库驱动
- 无 Redis
- 无配置管理 (viper)
- 无日志库
- 无 JWT 库

实际源码使用了 `http` 标准库，但没有声明任何外部依赖。

#### 问题 2.2.3: sub2api 依赖膨胀
**严重程度**: 🟡 中等

`sub2api` 依赖了 44 个直接依赖，包括:
- `entgo.io/ent` (ORM)
- `github.com/aws/aws-sdk-go-v2/*` (AWS SDK)
- `github.com/zeromicro/go-zero` (微服务框架)
- `github.com/testcontainers/testcontainers-go` (测试容器)

其他三个服务都没有使用 ORM 或 AWS SDK，依赖规模差异巨大。

---

## 3. 共享库缺失问题

### 3.1 当前代码组织

```
/home/long/project/立交桥/
├── supply-api/
│   └── internal/
│       ├── pkg/logging/      # 自定义结构化日志
│       ├── middleware/       # 认证、限流、幂等、追踪
│       ├── httpapi/          # HTTP handlers
│       └── ...
├── gateway/
│   └── internal/
│       ├── middleware/       # 认证、限流、CORS、审计
│       ├── handler/          # HTTP handlers
│       └── ...
├── platform-token-runtime/
│   └── internal/
│       ├── httpapi/          # HTTP handlers
│       ├── auth/             # 认证服务
│       └── ...
└── llm-gateway-competitors/sub2api/
    └── internal/
        ├── middleware/       # Gin 中间件
        ├── handler/         # Gin handlers
        └── pkg/logger/      # slog 封装
```

### 3.2 问题详情

#### 问题 3.2.1: 无共享中间件库
**严重程度**: 🟡 中等

三个服务都有类似的中间件功能，但实现分散:

| 功能 | supply-api | gateway | token-runtime |
|------|-------------|---------|---------------|
| 请求ID注入 | ✅ | ✅ | ❌ |
| Trace Context | ✅ | ❌ | ❌ |
| 日志中间件 | ✅ (结构化JSON) | ❌ (标准log) | ❌ (标准log) |
| CORS | ❌ | ✅ | ❌ |
| 认证 | JWT | Token验证 | Token服务 |
| 限流 | ✅ | ✅ | ❌ |
| 幂等 | ✅ | ❌ | ❌ |

**重复实现**:
- supply-api: `internal/middleware/auth.go` (JWT认证)
- gateway: `internal/middleware/chain.go` (Token认证链)
- token-runtime: `internal/auth/middleware/` (Token认证)

#### 问题 3.2.2: 无共享日志库
**严重程度**: 🟡 中等

- **supply-api**: 自定义 `internal/pkg/logging` - 结构化 JSON 日志，支持 trace_id, request_id
- **sub2api**: 使用 `internal/pkg/logger` - 基于 slog 封装
- **gateway**: 使用标准库 `log`
- **token-runtime**: 使用标准库 `log`

**影响**:
1. 日志格式不统一，难以集中分析
2. 无法实现跨服务的请求追踪
3. 重复实现日志工具

#### 问题 3.2.3: 无共享错误处理
**严重程度**: 🟢 低

- **supply-api**: CLAUDE.md 定义了错误码规范 `SUP_SET_4001`
- **gateway**: `pkg/error/error.go` 独立实现
- **token-runtime**: 内联错误处理
- **sub2api**: 无统一错误处理

#### 问题 3.2.4: 无共享配置管理
**严重程度**: 🟡 中等

- **supply-api**: 使用 `spf13/viper`
- **gateway**: 使用 `gopkg.in/yaml.v3` 直接解析
- **token-runtime**: 使用 `os.Getenv`
- **sub2api**: 使用 `viper` + wire 依赖注入

**问题**: 四种不同的配置加载方式。

---

## 4. 服务架构模式差异

### 4.1 Bootstrap 模式对比

**supply-api** (工厂模式):
```go
runtime, err := app.BuildRuntime(app.RuntimeOptions{...})
srv, err := runtime.BuildServer()
```

**gateway** (一步构建):
```go
server, err := app.BuildServer(cfg)
```

**token-runtime** (简单构建):
```go
srv, err := app.BuildServer(app.Config{...})
```

**问题**: 三种不同的运行时构建模式，无法互换组件。

### 4.2 健康检查端点不统一

| 服务 | 端点路径 |
|------|----------|
| supply-api | `/actuator/health` |
| gateway | `/health`, `/healthz`, `/readyz` |
| token-runtime | `/actuator/health` |

---

## 5. 建议的共享库提取机会

### 5.1 高优先级

1. **共享中间件库** (`lijiaoqiao/shared/middleware`)
   - Recovery
   - RequestID
   - Logging (结构化)
   - Tracing (W3C Trace Context)
   - CORS

2. **共享日志库** (`lijiaoqiao/shared/logging`)
   - 统一 JSON 格式
   - trace_id, request_id 支持
   - 多输出目标

3. **共享错误处理** (`lijiaoqiao/shared/errors`)
   - 错误码规范
   - 错误序列化和反序列化
   - gRPC status 转换

### 5.2 中等优先级

4. **共享配置库** (`lijiaoqiao/shared/config`)
   - 环境变量绑定
   - YAML/JSON 配置解析
   - 配置验证

5. **共享 HTTP 工具** (`lijiaoqiao/shared/http`)
   - JSON 编码/解码
   - 分页响应
   - 标准化错误响应

6. **共享审计接口** (`lijiaoqiao/shared/audit`)
   - AuditEvent 定义
   - AuditEmitter 接口
   - 常见审计事件类型

### 5.3 低优先级

7. **共享限流器** (`lijiaoqiao/shared/ratelimit`)
8. **共享指标定义** (`lijiaoqiao/shared/metrics`)
9. **共享健康检查** (`lijiaoqiao/shared/health`)

---

## 6. 修复建议优先级

### 🔴 紧急 (影响功能)

1. **修复 platform-token-runtime 的 go.mod**
   ```bash
   cd /home/long/project/立交桥/platform-token-runtime
   go mod init lijiaoqiao/platform-token-runtime
   go mod tidy
   ```

2. **统一健康检查端点**
   - 建议统一为 `/actuator/health`

### 🟡 重要 (影响维护)

3. **创建共享日志库**
   - 将 supply-api 的 `internal/pkg/logging` 提取为独立模块
   - gateway 和 token-runtime 采用

4. **统一 gateway 日志**
   - 当前使用标准库 `log`
   - 应改用结构化日志

5. **统一依赖版本**
   - pgx: 统一到 v5.5.1
   - viper: 统一到 v1.18.2
   - jwt: 统一到 v5.2.x

### 🟢 建议 (长期改进)

6. **提取共享中间件库**
7. **统一 Bootstrap 模式**
8. **统一错误码规范**
9. **考虑 Gin vs 标准库决策** (建议保持标准库一致性)

---

## 7. 总结

当前项目存在以下核心架构问题:

| 类别 | 问题数 | 严重程度 |
|------|--------|----------|
| 框架不一致 | 3 | 🟡 中等 |
| 依赖管理 | 3 | 🔴 严重 (token-runtime) |
| 共享库缺失 | 4 | 🟡 中等 |
| 架构模式 | 2 | 🟢 低 |

**最大问题**: `platform-token-runtime` 的依赖管理完全缺失，需要立即修复。

**最大改进机会**: 提取共享日志库和中间件库，可显著减少代码重复并提高一致性。

---

*报告生成时间: 2026-04-16*
