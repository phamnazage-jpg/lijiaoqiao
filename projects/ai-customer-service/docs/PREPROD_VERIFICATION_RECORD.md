# PREPROD_VERIFICATION_RECORD.md

> 状态：已建立  
> 最近一次更新：2026-05-04  
> 目标：沉淀 Gate B 预生产验证的可复跑证据，而不是口头结论

---

## 1. 验证范围

本记录对应 Task 5 的 Gate B 验证脚本：

- [scripts/verify_preprod_gate_b.sh](/home/long/project/立交桥/projects/ai-customer-service/scripts/verify_preprod_gate_b.sh)

脚本覆盖的检查项：

1. 环境变量完整性与 production 约束
2. PostgreSQL 连通性
3. migration 账本与基线版本检查
4. 当前源码构建与服务启动
5. `/actuator/health/live`
6. `/actuator/health/ready`
7. 带签名 webhook 请求
8. dedup 入库与重复消息抑制
9. ticket 创建 / 分配 / 解决 / 关闭
10. audit 入库验证

---

## 2. 最近一次实测记录

- 时间：2026-05-04 18:50 CST
- 环境：本机容器化/本地 PostgreSQL 联调环境
- 基线提交：`65e48bc`
- 说明：本次验证基于当前工作区源码重新编译执行，不依赖仓库内旧二进制
- 运行 ID：`gateb-20260504185024`
- 产物目录：`/tmp/ai-customer-service-preprod-gate-b/gateb-20260504185024`

执行命令：

```bash
AI_CS_RUNTIME_ENV=production \
AI_CS_ADDR=127.0.0.1:18080 \
AI_CS_POSTGRES_ENABLED=true \
AI_CS_POSTGRES_DSN='host=localhost port=5434 user=ai_cs password=ai_cs_secret dbname=ai_customer_service sslmode=disable' \
AI_CS_POSTGRES_MIGRATION_DIR='/home/long/project/立交桥/projects/ai-customer-service/db/migration' \
AI_CS_WEBHOOK_SECRET='gate-b-secret-20260504' \
AI_CS_WEBHOOK_TIMESTAMP_HEADER='X-CS-Timestamp' \
AI_CS_WEBHOOK_SIGNATURE_HEADER='X-CS-Signature' \
AI_CS_WEBHOOK_MAX_SKEW_SECONDS=300 \
scripts/verify_preprod_gate_b.sh
```

结果摘要：

- PASS 总数：`30`
- FAIL 总数：`0`
- 生成 ticket：`0806e91f-f50a-4942-b263-f14a4ed5285e`
- 生成 session：`9a468320-81c3-44fb-9707-9819dba16e94`
- 验证 message_id：`gateb-20260504185024-message`
- 服务日志：`/tmp/ai-customer-service-preprod-gate-b/gateb-20260504185024/service.log`

关键通过项：

1. 当前源码可成功构建并启动为 production + postgres 模式
2. `live` / `ready` 探针均返回成功
3. 带 HMAC 签名的 webhook 请求返回 `200`
4. 首次 webhook 成功创建 `ticket` 与 `message_processed audit`
5. 相同 `message_id` 的重复 webhook 被 dedup，且 dedup 表中仅保留一条记录
6. `assign -> resolve -> close` 工单闭环在 PostgreSQL 中成功落库
7. `assign / resolve / close` 两层 audit 都成功入库

---

## 3. 本次验证中暴露并修复的问题

在脚本首次联调过程中，暴露并修复了两个真实问题：

1. Gate B 脚本最初使用仓库内旧二进制，无法代表当前源码行为  
   已修复为：脚本默认先构建当前源码，再启动服务。

2. handler 层 audit 事件 ID 不是合法 UUID，导致 PostgreSQL audit 写入静默失败  
   已修复文件：
   - [audit_helper.go](/home/long/project/立交桥/projects/ai-customer-service/internal/http/handlers/audit_helper.go)
   - [audit_helper_test.go](/home/long/project/立交桥/projects/ai-customer-service/internal/http/handlers/audit_helper_test.go)

这两项修复后，Gate B 本地/容器化预演已全部通过。

---

## 4. 当前结论

### 已确认

- **本地/容器化 Gate B 预演：通过**
- **脚本化验证入口：已建立**
- **ticket / audit / dedup / health / migration：已有可复跑证据**

### 仍未确认

- **真实共享预生产环境 Gate B：尚未执行同脚本复跑**
- **Gate C 灰度监控 / 回滚演练：未完成**

因此当前正确结论是：

> **Gate B 脚本与本地/容器化联调证据已经建立并通过，但还不能把这直接等同于“真实预生产环境已经放行”。**

