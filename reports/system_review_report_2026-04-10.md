> 归档状态：已归档
> 归档批次：ARCHIVE-HISTORY-2026-04-14-A01
> 归档标识：AHR-20260414-022
> 当前用途：历史快照，仅供追溯；不得作为现行门禁、发布决议或实现状态事实源。
> 当前事实源：`docs/plans/2026-04-14-repo-integrity-refactor-plan.md`、`reports/archive/gate_verification/` 与仍在维护的签署/门禁文档。

# 立交桥项目系统性 Review 报告

## 1. Review 范围

- 目标模块：`supply-api`、`gateway`、`platform-token-runtime`
- 检查维度：项目结构、主入口、配置装载、认证鉴权、幂等、审计、CORS、测试覆盖、构建与运行验证
- 验证方法：
  - 静态审查关键入口与核心中间件
  - 运行 `go test ./...`
  - 运行 `go build ./...`
  - 对关键服务做最小启动与健康探测

## 2. 总体结论

当前仓库处于“部分模块可编译、部分模块可测试，但主运行路径与安全边界仍存在阻断性问题”的状态，不满足生产发布标准。

结论分级如下：

- `supply-api`：模块测试通过，但主程序入口存在阻断性问题，无法提供真实 HTTP 服务
- `gateway`：可以构建，但测试不全绿，且认证保护范围与公开路由不一致，存在安全绕过风险
- `platform-token-runtime`：构建与测试通过，但定位仍是开发阶段最小可运行实现，不应直接视为生产级组件

## 3. 真实验证结果

### 3.1 gateway

- `go build ./...`：通过
- `go test ./...`：失败
- 失败点集中在 `internal/middleware/cors_test.go`
- 说明当前主干并非持续绿灯状态

### 3.2 supply-api

- `go build ./...`：通过
- `go test ./...`：通过
- `go run ./cmd/supply-api -env=dev`：日志显示初始化完成，但健康检查端口不可达
- `curl http://127.0.0.1:18082/actuator/health`：连接失败
- 说明该服务“可编译、可单测”，但“不可真正启动对外服务”

### 3.3 platform-token-runtime

- `go build ./...`：通过
- `go test ./...`：通过
- 质量状态相对最好，但文档明确说明其为开发阶段内存实现

## 4. 关键问题列表

### P0-01 supply-api 主程序未真正启动 HTTP 服务

现象：

- `http.Server` 被创建，但入口代码未调用 `ListenAndServe`
- 程序随后直接进入信号等待与关闭逻辑

影响：

- 服务进程无法监听端口
- 所有 HTTP API 在真实运行态不可用

证据：

- `supply-api/cmd/supply-api/main.go`

### P0-02 gateway 公开 API 可绕过鉴权

现象：

- `/v1/chat/completions` 与 `/v1/completions` 被包装了认证链
- 但认证链默认只保护 `/api/v1/supply` 与 `/api/v1/platform`
- `/api/v1/chat/completions` 兼容路径甚至未经过认证链与限流链

影响：

- 公开 API 在默认配置下可能被未授权访问
- 属于真实安全边界缺失

证据：

- `gateway/cmd/gateway/main.go`
- `gateway/internal/middleware/chain.go`

### P1-01 gateway CORS 语义与测试预期不一致

现象：

- 不允许来源的实际请求未被拒绝，只是未设置 CORS 头
- 对应单测期望返回 `403`
- 当前主程序也未把 CORS 中间件接入主请求链

影响：

- 测试持续失败
- 运行态与安全预期不一致

证据：

- `gateway/internal/middleware/cors.go`
- `gateway/internal/middleware/cors_test.go`

### P1-02 supply-api 的配置入口与请求上下文处理不完整

现象：

- CLI 解析了 `-config` 参数，但加载配置时未真正使用该参数
- 幂等中间件降级路径使用 `context.Background()`，会丢失请求上下文、超时与取消信号

影响：

- 自定义配置入口失效
- 请求链路与资源控制信息在降级路径上丢失

证据：

- `supply-api/cmd/supply-api/main.go`
- `supply-api/internal/httpapi/supply_api.go`

### P1-03 自动化验证未覆盖关键真实路径

现象：

- `supply-api` E2E 测试基本为 `Skip`
- `gateway` 鉴权测试覆盖了 `/api/v1/supply` 类路径，但没有覆盖实际暴露的 `/v1/*` 路由

影响：

- 主程序无法启动、公开路由绕过鉴权这类问题不会被现有测试及时发现

证据：

- `supply-api/e2e/e2e_test.go`
- `gateway` 现有认证相关测试

## 5. 次级风险

### P2-01 gateway 使用默认加密密钥回退

- `PASSWORD_ENCRYPTION_KEY` 缺失时回退到硬编码默认值
- 不符合生产环境密钥管理要求

### P2-02 supply-api 数据库连接默认 `sslmode=disable`

- 对本地开发可接受
- 对跨主机部署存在传输安全风险

### P2-03 审计批处理失败路径仍未闭环

- flush 失败处只有 TODO，没有重试、降级落盘或告警动作

## 6. 正向观察

- `supply-api` 的领域、IAM、审计、安全相关模块已有较多单测
- `platform-token-runtime` 代码边界清晰，适合作为开发态最小运行基线
- `gateway` 的路由、限流、适配器、中间件已经具备结构化基础

## 7. 当前状态判断

如果以“是否可上线”为标准，结论是：

- 当前不可直接上线

如果以“是否完全不可用”为标准，结论是：

- 不是完全不可用，但距离生产就绪仍有关键收口工作

更准确的状态应描述为：

- 研发实现已经形成主体
- 文档与历史评审较多
- 但运行态与安全态仍存在明显落差
- 当前最需要的是对入口、边界和自动化门禁进行收口

## 8. 建议整改方向

优先顺序建议如下：

1. 修复 `supply-api` 启动阻断问题
2. 修复 `gateway` 公开路由认证缺口
3. 修复 `gateway` CORS 语义与接入问题
4. 修复 `supply-api` 配置入口与上下文降级问题
5. 补齐关键运行路径的自动化验证
6. 清理仓库构建产物与工程治理问题
