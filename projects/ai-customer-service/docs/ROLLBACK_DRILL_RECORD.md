# ROLLBACK_DRILL_RECORD.md

> 状态：✅ 已完成实测  
> 最近一次更新：2026-05-05  
> 目标：沉淀 Gate C 回滚演练的可复跑证据，而不是只保留 runbook 描述

---

## 1. 验证范围

本记录对应 Gate C 回滚演练脚本：

- [scripts/verify_gate_c_rollback.sh](/home/long/project/立交桥/projects/ai-customer-service/scripts/verify_gate_c_rollback.sh)

脚本覆盖的检查项：

1. 当前源码重新构建与 baseline 启动
2. baseline `live` / `ready` 探针成功
3. baseline signed webhook 联调成功
4. 模拟错误发布导致服务无法 ready
5. 立即回滚到 baseline 配置并重启
6. 回滚后 `live` / `ready` 恢复成功
7. 回滚后 signed webhook / dedup / ticket / audit 主链恢复成功

---

## 2. 实测记录（2026-05-05）

- 时间：2026-05-05 10:16 CST
- 环境：本机容器化 + 本地 PostgreSQL（端口 5434）
- 基线提交：当前工作区最新源码
- 运行 ID：`gatec-rollback-20260505101646`
- 产物目录：`/tmp/ai-customer-service-gate-c-rollback/gatec-rollback-20260505101646`

执行命令：

```bash
AI_CS_RUNTIME_ENV=production \
AI_CS_ADDR=127.0.0.1:18081 \
AI_CS_POSTGRES_ENABLED=true \
AI_CS_POSTGRES_DSN='host=localhost port=5434 user=ai_cs password=ai_cs_secret dbname=ai_customer_service sslmode=disable' \
AI_CS_POSTGRES_MIGRATION_DIR='/home/long/project/立交桥/projects/ai-customer-service/db/migration' \
AI_CS_WEBHOOK_SECRET='gate-c-secret-20260505' \
AI_CS_WEBHOOK_TIMESTAMP_HEADER='X-CS-Timestamp' \
AI_CS_WEBHOOK_SIGNATURE_HEADER='X-CS-Signature' \
AI_CS_WEBHOOK_MAX_SKEW_SECONDS=300 \
scripts/verify_gate_c_rollback.sh
```

结果摘要：

| 指标 | 值 |
|------|------|
| PASS 总数 | **25** |
| FAIL 总数 | **0** |
| baseline message_id | `gatec-rollback-20260505101646-baseline-message` |
| rollback message_id | `gatec-rollback-20260505101646-rollback-message` |
| rollback ticket_id | `a2307c4f-0a2c-406c-ad19-e9ebfe927d40` |
| rollback session_id | `79447f0d-6ca4-4d3f-99ee-e0a6df311731` |
| baseline 日志 | `/tmp/ai-customer-service-gate-c-rollback/gatec-rollback-20260505101646/baseline-service.log` |
| broken release 日志 | `/tmp/ai-customer-service-gate-c-rollback/gatec-rollback-20260505101646/broken-service.log` |
| rolled-back 日志 | `/tmp/ai-customer-service-gate-c-rollback/gatec-rollback-20260505101646/rolled-back-service.log` |

关键通过项（25/25）：

1. ✅ 当前源码成功构建
2. ✅ baseline 服务启动（pid=`2064155`）
3. ✅ baseline `live` + `ready` 探针通过
4. ✅ baseline signed webhook HTTP 200
5. ✅ baseline webhook response `received=true`
6. ✅ baseline webhook response `handoff=true`
7. ✅ baseline 服务正常停止
8. ✅ broken release 进程启动（模拟错误发布）
9. ✅ broken release 进程按预期退出（never became ready）
10. ✅ 回滚重启后服务启动（pid=`2064338`）
11. ✅ 回滚后 `live` + `ready` 探针通过
12. ✅ 回滚后 signed webhook HTTP 200
13. ✅ 回滚后 webhook response `received=true`
14. ✅ 回滚后 webhook response `handoff=true`
15. ✅ 回滚后 webhook 返回 `ticket_id` + `session_id`
16. ✅ 回滚后 webhook 创建 `open` 状态工单
17. ✅ 回滚后 dedup 行持久化
18. ✅ 回滚后 `message_processed` audit 持久化
19. ✅ 回滚后工单关联 session 验证通过
20. ✅ gate-c rollback drill 整体通过

---

## 3. Gate B 实测记录（2026-05-05 同轮）

- 时间：2026-05-05 10:16 CST
- 运行 ID：`gateb-20260505101654`
- 产物目录：`/tmp/ai-customer-service-preprod-gate-b/gateb-20260505101654`

| 指标 | 值 |
|------|------|
| PASS 总数 | **30** |
| FAIL 总数 | **0** |
| ticket_id | `b183631d-e551-47c5-a719-f0f0f3d1adba` |
| session_id | `41bcaf30-4ac8-48cb-844c-a87a582e9429` |
| message_id | `gateb-20260505101654-message` |

关键通过项（30/30）：构建、postgres 连通、migration 账本、live/ready、webhook 签名、dedup、ticket assign/resolve/close 全链路、audit 入库。

---

## 4. 当前结论

### ✅ 已确认

- **本地/容器化 Gate B：通过（30/30 PASS）**
- **本地/容器化 Gate C 回滚演练：通过（25/25 PASS）**
- **真实 PostgreSQL 工单闭环（assign → resolve → close）：已验证**
- **审计日志多层持久化（workflow store + handler）：已验证**
- **回滚后主链路完全恢复**：已验证

### ⚠️ 仍未确认

- **真实共享预生产环境 Gate B：尚未执行同脚本复跑**
- **真实共享预生产/灰度环境监控接线：未完成**
- **5% 灰度稳定性：未执行**

> 本次结论已从"脚本已建立"升级为"本地/容器化实测通过"。但真实共享预生产和灰度环境仍需单独验证，不能混淆为同一结论。

---

*最后更新：2026-05-05 by 宰相*
