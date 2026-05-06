# Platform Callback Runbook

> 适用范围：`sub2api / newapi` 平台适配层的出站 callback 投递  
> 当前实现事实来源：`internal/store/postgres/platform_event_store.go`、`internal/service/platformdelivery/worker.go`

---

## 1. 快速判断

平台回调链路分三层状态：

1. **主链成功，outbox 已入库**
   表：`cs_platform_event_outbox`
2. **callback 尝试记录**
   表：`cs_platform_event_delivery_attempts`
3. **重试耗尽进入死信**
   表：`cs_platform_event_dead_letters`

如果用户反馈“平台没收到回调”，先按这个顺序查，不要直接看应用日志猜。

---

## 1.1 关键运行参数

当前 callback worker 已支持通过环境变量外显这些参数：

| 变量 | 默认值 | 说明 |
|---|---|---|
| `AI_CS_PLATFORM_SUB2API_CALLBACK_TIMEOUT_MS` | `3000` | 单次 callback HTTP 超时 |
| `AI_CS_PLATFORM_SUB2API_CALLBACK_MAX_RETRIES` | `5` | 最大重试次数 |
| `AI_CS_PLATFORM_SUB2API_CALLBACK_POLL_INTERVAL_MS` | `5000` | worker 轮询间隔 |
| `AI_CS_PLATFORM_SUB2API_CALLBACK_BATCH_SIZE` | `20` | 单轮最大拉取事件数 |
| `AI_CS_PLATFORM_SUB2API_CALLBACK_RETRY_SCHEDULE_SEC` | `10,30,60,300,900` | 重试退避序列 |

`newapi` 侧使用同构变量名：
- `AI_CS_PLATFORM_NEWAPI_CALLBACK_TIMEOUT_MS`
- `AI_CS_PLATFORM_NEWAPI_CALLBACK_MAX_RETRIES`
- `AI_CS_PLATFORM_NEWAPI_CALLBACK_POLL_INTERVAL_MS`
- `AI_CS_PLATFORM_NEWAPI_CALLBACK_BATCH_SIZE`
- `AI_CS_PLATFORM_NEWAPI_CALLBACK_RETRY_SCHEDULE_SEC`

---

## 2. 常用查询

### 2.1 查看待投递事件

```sql
SELECT id, platform, event_type, callback_target, status, attempt_count, next_attempt_at, last_error
FROM cs_platform_event_outbox
WHERE status IN ('pending', 'retrying')
ORDER BY next_attempt_at ASC, created_at ASC
LIMIT 100;
```

### 2.2 查看最近投递尝试

```sql
SELECT event_id, attempt_no, response_status, error_message, created_at
FROM cs_platform_event_delivery_attempts
ORDER BY created_at DESC
LIMIT 100;
```

### 2.3 查看死信事件

```sql
SELECT event_id, platform, event_type, callback_target, attempt_count, final_error, created_at
FROM cs_platform_event_dead_letters
ORDER BY created_at DESC
LIMIT 100;
```

---

## 3. 故障分类

### 3.1 平台回调失败

表现：
- `cs_platform_event_outbox.status` 为 `retrying` 或 `dead_letter`
- `cs_platform_event_delivery_attempts` 有记录

说明：
- 主链已经处理成功
- 失败点在平台 callback 出站链路

### 3.2 主链失败

表现：
- 平台入口直接返回 `500`
- `cs_platform_event_outbox` 没有对应事件

说明：
- 失败点在 webhook 入站、dialog 主链或 outbox 写入
- 这不属于 callback worker 故障

---

## 4. 手动重放

当前版本没有单独重放脚本，最小操作方式是把死信或重试事件改回可投递状态：

```sql
UPDATE cs_platform_event_outbox
SET status = 'pending',
    next_attempt_at = NOW(),
    last_error = NULL,
    updated_at = NOW()
WHERE id = '<event_id>';
```

如果事件已经在 `dead_letters`：

```sql
DELETE FROM cs_platform_event_dead_letters
WHERE event_id = '<event_id>';
```

再等待 worker 下一轮拉取。

---

## 5. 处理原则

1. 不要手工删除 `outbox` 主记录，除非已经确认平台侧不需要这条事件。
2. 优先保留 `delivery_attempts` 和 `dead_letters`，它们是排障证据。
3. 如果同一平台持续大量 `retrying`，优先检查 callback 地址、签名 secret 和平台上游可用性。
