# AI-Customer-Service 实施计划

> 状态说明：本文件原先采用 `MVP-proto` 口径，已不再作为生产上线判断依据。生产执行以 `PRODUCTION_EXECUTION_PLAN.md` 为准。

> 历史说明：以下内容保留为原型阶段记录，不代表当前生产目标已达成。

## 1. 选择该项目的理由

AI-Customer-Service 是当前三个项目里最适合优先实施的对象：
- 文档结构最完整，且章节一致性最好。
- 业务主链路最短：Webhook 接入 → Session → Intent → Reply/Handoff → Audit。
- 风险可控，适合作为从文档到实现的第一条样板链路。
- 相比 AI-Ops 和 Supply-Intelligence，外部依赖与状态机复杂度更低，更容易做最小闭环验证。

## 2. 实施目标

第一阶段只交付“最小生产可运行版本”，包含：
1. 独立运行模式 HTTP 服务。
2. 健康检查端点：`/actuator/health`、`/actuator/health/live`、`/actuator/health/ready`。
3. Webhook 接口：最小文本消息接入。
4. Session 管理：内存版会话存储。
5. Intent 识别：规则版最小实现（不用真实 LLM）。
6. Reply 生成：规则版 FAQ / fallback 回复。
7. Handoff：敏感意图或低置信度转人工。
8. Audit：内存版审计日志记录。
9. OpenAPI 占位文档。
10. 最小测试：主路径 + 失败路径。

非目标：
- 不在第一阶段实现 PostgreSQL / Redis / 向量数据库。
- 不在第一阶段实现真正 RAG 检索。
- 不在第一阶段实现多渠道适配，只做单 webhook 文本入口。
- 不在第一阶段实现完整 RBAC 后台。

## 3. 推荐工程结构

```text
ai-customer-service/
  go.mod
  cmd/ai-customer-service/main.go
  internal/app/app.go
  internal/http/router.go
  internal/http/handlers/health_handler.go
  internal/http/handlers/webhook_handler.go
  internal/domain/message/message.go
  internal/domain/session/session.go
  internal/domain/intent/intent.go
  internal/domain/audit/audit.go
  internal/service/dialog/service.go
  internal/service/intent/service.go
  internal/service/reply/service.go
  internal/service/handoff/service.go
  internal/store/memory/session_store.go
  internal/store/memory/audit_store.go
  internal/store/memory/knowledge_store.go
  internal/openapi/openapi.json
  test/e2e/webhook_e2e_test.go
  test/integration/dialog_service_test.go
  Makefile
  Dockerfile
```

## 4. 分阶段任务清单

### Phase 1：工程初始化
1. 创建 Go module。
2. 建立 `cmd/` + `internal/` 目录结构。
3. 创建最小 `main.go`，支持 HTTP 启动。
4. 增加 health handler。
5. 增加基础 router。
6. 写启动 smoke test。

### Phase 2：主链路实现
1. 定义 `UnifiedMessage`、`Session`、`IntentResult`、`AuditEvent`。
2. 实现 webhook handler：接收最小 JSON 文本消息。
3. 实现 session store（memory）。
4. 实现 intent service（规则匹配：quota/token/error/handoff/general）。
5. 实现 reply service（规则回复/fallback）。
6. 实现 handoff service（敏感词或低置信度转人工）。
7. 实现 audit store（memory）。
8. 打通主链路：receive → parse → intent → reply/handoff → audit。

### Phase 3：测试与门禁
1. 单元测试：intent service。
2. 单元测试：handoff service。
3. 集成测试：dialog service。
4. E2E 测试：webhook 主路径。
5. E2E 测试：敏感词转人工失败路径。
6. 验证 health/readiness 端点。
7. 生成最小 OpenAPI 占位文档。

### Phase 4：运行工件
1. 编写 Dockerfile。
2. 编写最小 Makefile。
3. 本地运行验证：`go test ./...`。
4. 本地运行验证：启动服务并 curl health/webhook。

## 5. 阶段门禁

### Gate A：进入实现前
- [x] PRD / HLD / TEST_DESIGN / INTERFACE 已存在。
- [x] 文档中门禁、威胁建模、阻断条件已补齐。
- [x] 工程目录已创建。

### Gate B：主链路完成
- [x] 独立运行服务可启动。
- [x] Webhook 能接收消息并返回应答。
- [x] 敏感意图能够转人工。
- [x] 审计事件会记录。

### Gate C：可交付最小版本
- [x] `go test ./...` 全通过。
- [x] health/live/ready 通过。
- [x] 至少 1 条主路径 + 1 条失败路径 + 1 条转人工路径验证通过。
- [x] Dockerfile 可构建。

## 6. 验证命令

```bash
go test ./...
go test ./test/e2e -v
curl -i http://127.0.0.1:8080/actuator/health/live
curl -i http://127.0.0.1:8080/actuator/health/ready
curl -i -X POST http://127.0.0.1:8080/api/v1/customer-service/webhook \
  -H 'Content-Type: application/json' \
  -d '{"message_id":"m1","channel":"widget","open_id":"u1","content":"查询额度"}'
```

## 7. 风险与控制

1. 当前没有真实 LLM/RAG，先用规则实现，防止卡死在外部依赖。
2. 先做内存存储，防止过早引入数据库和 Redis 增加噪声。
3. 先独立运行，不先做集成模式，等主链路稳定后再补 IntegrationPlugin。
4. 严禁把 demo 规则实现误标为生产完成；本计划交付的是“最小生产可运行原型”，不是最终版。
