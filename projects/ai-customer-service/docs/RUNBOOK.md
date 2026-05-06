# DO-P1-2：运行与回滚 Runbook

> 状态：✅ 已交付
> 负责人：DevOps（宰相代填）
> 基准：P0 完成 Gate B 预生产验证
> 日期：2026-05-04

---

## 0. Gate B 推荐入口

预生产 Gate B 不再建议靠零散手工命令拼接验证。优先使用：

- [scripts/verify_preprod_gate_b.sh](/home/long/project/立交桥/projects/ai-customer-service/scripts/verify_preprod_gate_b.sh)
- 最近一次实测记录：[PREPROD_VERIFICATION_RECORD.md](/home/long/project/立交桥/projects/ai-customer-service/docs/PREPROD_VERIFICATION_RECORD.md)
- Gate C 回滚演练入口：[scripts/verify_gate_c_rollback.sh](/home/long/project/立交桥/projects/ai-customer-service/scripts/verify_gate_c_rollback.sh)
- 最近一次回滚演练记录：[ROLLBACK_DRILL_RECORD.md](/home/long/project/立交桥/projects/ai-customer-service/docs/ROLLBACK_DRILL_RECORD.md)

脚本会完成：

1. 环境变量完整性检查
2. PostgreSQL 连通性检查
3. migration 基线检查
4. 当前源码构建与服务启动
5. `live` / `ready` 探针检查
6. signed webhook 联调
7. dedup 入库验证
8. ticket / audit 入库闭环验证

推荐执行方式：

```bash
AI_CS_RUNTIME_ENV=production \
AI_CS_ADDR=127.0.0.1:18080 \
AI_CS_POSTGRES_ENABLED=true \
AI_CS_POSTGRES_DSN='host=localhost port=5434 user=ai_cs password=ai_cs_secret dbname=ai_customer_service sslmode=disable' \
AI_CS_POSTGRES_MIGRATION_DIR="$PWD/db/migration" \
AI_CS_WEBHOOK_SECRET='replace-with-real-secret' \
AI_CS_WEBHOOK_TIMESTAMP_HEADER='X-CS-Timestamp' \
AI_CS_WEBHOOK_SIGNATURE_HEADER='X-CS-Signature' \
AI_CS_WEBHOOK_MAX_SKEW_SECONDS=300 \
scripts/verify_preprod_gate_b.sh
```

通过标准：

- 脚本退出码为 `0`
- 输出末尾出现 `summary: pass=... fail=0`
- 产物目录中保留 `summary.txt`、`service.log`、`webhook_response.json`

---

## 一、部署前检查清单（Pre-flight）

```bash
# 1. 确认环境变量完整
echo "AI_CS_RUNTIME_ENV=$AI_CS_RUNTIME_ENV"
echo "AI_CS_POSTGRES_ENABLED=$AI_CS_POSTGRES_ENABLED"
echo "AI_CS_POSTGRES_DSN=${AI_CS_POSTGRES_DSN:+[SET]}"
echo "AI_CS_WEBHOOK_SECRET=${AI_CS_WEBHOOK_SECRET:+[SET]}"
echo "AI_CS_LOG_LEVEL=$AI_CS_LOG_LEVEL"

# 2. 确认 PostgreSQL 可连
PGPASSWORD=ai_cs_secret psql -h localhost -p 5434 -U ai_cs -d ai_customer_service -c "SELECT 1" || exit 1

# 3. 确认 migration 已执行
PGPASSWORD=ai_cs_secret psql -h localhost -p 5434 -U ai_cs -d ai_customer_service -c "SELECT table_name FROM information_schema.tables WHERE table_schema='public' ORDER BY table_name;" | grep -q cs_sessions || { echo "MIGRATION MISSING"; exit 1; }

# 4. 启动服务（后台）
nohup ./ai-customer-service > /var/log/ai-cs.log 2>&1 &
sleep 3

# 5. 验证 ready probe
curl -s http://localhost:8080/actuator/health/ready | grep -q '"status":"UP"' || { echo "READY FAILED"; cat /var/log/ai-cs.log; exit 1; }
```

---

## 二、启动失败排查

| 症状 | 原因 | 解决方案 |
|------|------|----------|
| `memory fallback is not allowed` ERROR | Env=production 但 `AI_CS_POSTGRES_ENABLED≠true` | 设置 `AI_CS_POSTGRES_ENABLED=true` 并重启 |
| `AI_CS_POSTGRES_DSN is required` ERROR | Env=production 但 DSN 未配置 | 配置完整 DSN：`postgres://user:pass@host:5434/db?sslmode=disable` |
| `listen tcp :8080: bind: address already in use` | 8080 端口被占用 | `pkill -f ai-customer-service` 或改 `AI_CS_ADDR=:8081` |
| `pq: connection refused` | PostgreSQL 不可达 | 检查 PG 主机/端口/防火墙，确认 `psql` 可连 |
| `pq: password authentication failed` | 密码错误 | 核对 `AI_CS_POSTGRES_DSN` 中的密码 |
| 启动成功但 `/actuator/health/ready` 返回 `postgres:DOWN` | PG 连通但 health check 失败 | 检查 PG 是否在 `AI_CS_POSTGRES_DSN` 指定端口响应 |

---

## 三、Migration 失败排查

| 症状 | 原因 | 解决方案 |
|------|------|----------|
| `pq: relation "cs_sessions" does not exist` | migration 未执行 | 手动执行 `psql -f db/migration/0001_init.up.sql` |
| `pq: duplicate key value violates unique constraint` | 表已存在但 migration 重跑 | migration 已幂等（`CREATE TABLE IF NOT EXISTS`），忽略即可 |
| `pq: permission denied` | PG 用户无建表权限 | 确认 `ai_cs` 用户是 superuser 或拥有 `ai_customer_service` 库 |

```bash
# 手动执行 migration
psql "postgres://ai_cs:ai_cs_secret@localhost:5434/ai_customer_service?sslmode=disable" -f db/migration/0001_init.up.sql
```

---

## 四、数据库不可用时的行为

- **Env=production**：启动时 config.go 会检查 `AI_CS_POSTGRES_ENABLED=true`，若 DSN 不可达或认证失败，服务**拒绝启动**（不会 fallback 到 memory）
- **Env=test/development**：可设置 `AI_CS_POSTGRES_ENABLED=false` 使用 memory store（测试用）

---

## 五、Webhook 签名认证联调失败排查

| 症状 | 原因 | 解决方案 |
|------|------|----------|
| `CS_AUTH_4034 invalid webhook signature` | HMAC secret 不匹配 | 确认上游使用与 `AI_CS_WEBHOOK_SECRET` 相同的密钥 |
| `CS_AUTH_4031 missing webhook signature` | 上游未传 `X-CS-Signature` header | 检查上游 webhook 发送逻辑 |
| `CS_AUTH_4033 stale webhook request` | 请求时间戳 > MaxSkew（默认 300s） | 确认服务器时间同步（NTP），或调整 `AI_CS_WEBHOOK_MAX_SKEW_SECONDS` |
| 偶发性 403 | 时钟漂移超过 300s | 检查服务器时区与 NTP 配置 |

```bash
# 验证签名算法（本地测试）
TS=$(date +%s)
BODY='{"test":"payload"}'
SIG=$(echo -n "${TS}.${BODY}" | openssl dgst -sha256 -hmac "test-secret-123" | awk '{print $2}')
curl -v -X POST http://localhost:8080/api/v1/customer-service/webhook \
  -H "Content-Type: application/json" \
  -H "X-CS-Timestamp: $TS" \
  -H "X-CS-Signature: $SIG" \
  -d "$BODY"
```

---

## 六、回滚操作流程

### 6.1 版本回滚（从 v1.1.0 回滚到 v1.0.0）

```bash
# 1. 记录当前版本
echo "Rolling back from $(./ai-customer-service --version) to v1.0.0"

# 2. 停止当前服务
pkill -f "ai-customer-service"
sleep 2

# 3. 备份当前数据库（可选，建议先备份）
PGPASSWORD=ai_cs_secret pg_dump -h localhost -p 5434 -U ai_cs ai_customer_service > /tmp/ai_cs_backup_$(date +%Y%m%d_%H%M%S).sql

# 4. 拉取旧版本镜像 / 二进制
# Docker: docker pull ai-customer-service:v1.0.0
# Binary: 从备份位置获取 v1.0.0 二进制

# 5. 重启服务
nohup ./ai-customer-service-v1.0.0 > /var/log/ai-cs-v1.0.0.log 2>&1 &
sleep 3

# 6. 验证
curl -s http://localhost:8080/actuator/health/ready
curl -s http://localhost:8080/actuator/health
```

### 6.2 配置回滚

```bash
# 若新配置有问题，恢复环境变量
export AI_CS_POSTGRES_ENABLED=true
export AI_CS_POSTGRES_DSN="postgres://ai_cs:ai_cs_secret@localhost:5434/ai_customer_service?sslmode=disable"
export AI_CS_WEBHOOK_SECRET="previous-secret"
pkill -f "ai-customer-service"
sleep 2
nohup ./ai-customer-service > /var/log/ai-cs.log 2>&1 &
```

### 6.3 数据库回滚（Migration 不支持向下回滚，需手动处理）

```sql
-- 紧急情况：清空所有数据重建（仅 development）
TRUNCATE cs_audit_logs, cs_tickets, cs_messages, cs_sessions, cs_message_dedup CASCADE;
-- 然后重启服务，让 migration 重新初始化
```

---

## 七、健康状态快速诊断

```bash
#!/bin/bash
# 60s 快速诊断脚本

echo "=== AI-CS Health Diagnostic ==="
echo ""

echo "[1/5] Service process:"
ps aux | grep "ai-customer-service" | grep -v grep || echo "  NOT RUNNING ❌"

echo ""
echo "[2/5] HTTP endpoints:"
for endpoint in "/actuator/health/live" "/actuator/health/ready" "/actuator/health"; do
  status=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080$endpoint)
  echo "  $endpoint → HTTP $status $([ "$status" = "200" ] && echo '✅' || echo '❌')"
done

echo ""
echo "[3/5] PostgreSQL:"
PGPASSWORD=ai_cs_secret psql -h localhost -p 5434 -U ai_cs -d ai_customer_service -c "SELECT count(*) as tickets FROM cs_tickets;" 2>&1 | grep -v "^Password" | tail -1

echo ""
echo "[4/5] Recent errors in log:"
tail -50 /var/log/ai-cs.log 2>/dev/null | grep "ERROR" | tail -5 || echo "  No recent errors ✅"

echo ""
echo "[5/5] Webhook test:"
TS=$(date +%s)
BODY='{"channel":"widget","message_id":"diag-001","open_id":"diag-open","content":"health check","timestamp":"2026-05-04T00:00:00Z"}'
SIG=$(echo -n "${TS}.${BODY}" | openssl dgst -sha256 -hmac "test-secret-123" | awk '{print $2}')
curl -s -X POST http://localhost:8080/api/v1/customer-service/webhook \
  -H "Content-Type: application/json" \
  -H "X-CS-Timestamp: $TS" \
  -H "X-CS-Signature: $SIG" \
  -d "$BODY" | head -c 200

echo ""
echo "=== Diagnostic complete ==="
```
