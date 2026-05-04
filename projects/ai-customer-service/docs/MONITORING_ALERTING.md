# DO-P1-1：最小监控与告警闭环

> 状态：✅ 已交付
> 负责人：DevOps（宰相代填）
> 基准：P0 完成 Gate B 预生产验证
> 日期：2026-05-04

---

## 一、监控覆盖矩阵

| 告警项 | 监控端点 | 阈值/判定条件 | 动作 |
|--------|----------|---------------|------|
| **5xx 错误激增** | `GET /actuator/health` 中 status≠UP，或日志 level=ERROR | 5xx 占比 > 5% 持续 1min | 触发 PagerDuty / 日志告警 |
| **签名拒绝** | 业务日志中 `CS_AUTH_4031/4033/4034` code 出现 | 10 次 / 5min 窗口 | 记录安全事件，暂不阻塞 |
| **Handoff 异常** | `GET /api/v1/customer-service/webhook` 返回 `handoff=true` 率 | handoff=true 突增 3x 历史均值 | 记录人工介入事件 |
| **Ticket 未创建** | refund intent 触发后 10s 内 cs_tickets 无对应记录 | refund intent 但 ticket_id="" | 告警并记录异常 |
| **Audit 未写入** | ticket 创建后 5s 内 cs_audit_logs 无 `object_type=ticket` 记录 | audit_count 增量=0 | 告警 DB 写入问题 |
| **PostgreSQL 不可用** | `GET /ready` 中 postgres check ≠UP | postgres status= DOWN | 立即告警，影响 ready |
| **服务未就绪** | `GET /ready` 返回 non-200 或超时 3s | ready != 200 | 服务 restart 触发 |
| **服务挂了** | `GET /live` 返回 non-200 或超时 3s | live != 200 | K8s/Supervisor restart |

---

## 二、监控接入方式

### 2.1 Kubernetes Probe（存活 + 就绪）

```yaml
livenessProbe:
  httpGet:
    path: /live
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
  failureThreshold: 3
```

### 2.2 Prometheus 指标暴露（可选，v1.1+）

```
# 暴露端点
GET /metrics

# 关键指标
ai_cs_webhook_requests_total{status="success|reject|5xx"}
ai_cs_tickets_created_total
ai_cs_audit_logs_written_total
ai_cs_handoff_total
ai_cs_postgres_errors_total
ai_cs_session_active_gauge
```

### 2.3 日志聚合（ELK/Loki）

关键日志字段抓取：
```
level=ERROR AND msg="webhook request rejected"
level=ERROR AND msg="audit log write failed"
level=WARN  AND msg="handoff ticket missing"
```

---

## 三、告警阈值配置（Prometheus AlertManager 风格）

```yaml
groups:
  - name: ai-customer-service
    rules:
      - alert: HighErrorRate
        expr: rate(ai_cs_webhook_requests_total{status="5xx"}[1m]) / rate(ai_cs_webhook_requests_total[1m]) > 0.05
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "AI-CS 5xx 错误率超过 5%"

      - alert: PostgresDown
        expr: ai_cs_postgres_errors_total > 0
        for: 30s
        labels:
          severity: critical

      - alert: TicketCreationDrop
        expr: rate(ai_cs_tickets_created_total[5m]) == 0 AND rate(ai_cs_webhook_requests_total[5m]) > 0.1
        for: 2m
        labels:
          severity: warning

      - alert: AuditLogWriteFailure
        expr: increase(ai_cs_audit_logs_written_total[5m]) == 0 AND increase(ai_cs_tickets_created_total[5m]) > 0
        for: 1m
        labels:
          severity: critical
```

---

## 四、最小化监控检查清单（部署时必检）

- [ ] **就绪探针**：`curl http://localhost:8080/ready` → 200 + `postgres:UP`
- [ ] **存活探针**：`curl http://localhost:8080/live` → 200
- [ ] **日志告警**：ERROR level 日志出现时触发监控告警
- [ ] **PG 连接**：每分钟 check `/ready` 中 postgres status
- [ ] **Handoff 率**：每 5 分钟比对 `webhook_count` vs `handoff_count`
- [ ] **Ticket 漏单**：refund intent 触发后 10s 内查 DB 确认 ticket 存在
- [ ] **Audit 漏写**：ticket 创建后 5s 内查 `cs_audit_logs` 确认记录

---

## 五、故障自愈策略

| 故障 | 自动处理 | 人工介入 |
|------|----------|----------|
| `/ready` 失败 3 次 | K8s 重启 Pod | 如果 5min 内仍失败，发告警 |
| PG 连接断开 | 服务 graceful shutdown，等待 PG 恢复后自动重连 | 若 >10min 无自动恢复，发告警 |
| OOM / 内存泄漏 | OOMKiller 杀掉后，K8s 重启 | 分析 heap profile |
| 磁盘满（审计日志） | — | 立即告警，人工清理 |