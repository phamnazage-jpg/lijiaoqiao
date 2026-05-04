# ai-customer-service 配置契约基线

> 来源：`internal/config/config.go` 当前实现  
> 用途：作为 PM / QA / DevOps / 部署文档的唯一配置事实来源  
> 状态：当前代码事实基线；production 下的关键运行约束已经由 `internal/config/config.go` 执行校验

---

## 0. 重要说明

当前代码已经实现了基础配置解析，并对 production 下的关键约束做了 fail-fast 校验。

这意味着：
- 本文档描述的是**当前代码真实读取和校验的配置契约**
- production 下缺少关键配置时，`Load()` 会直接返回错误
- readiness / 依赖可观测仍需结合运行态和部署层继续完善

---

## 1. 当前代码真实读取的环境变量

### 1.1 HTTP

| 变量名 | 默认值 | 含义 | 当前代码是否校验 | prod 是否应允许默认值 |
|---|---|---|---|---|
| `AI_CS_ADDR` | `:8080` | HTTP 监听地址 | 非空校验 | 视部署环境决定 |
| `AI_CS_READ_HEADER_TIMEOUT_SEC` | `5` | header 读取超时（秒） | 无额外校验 | 可 |
| `AI_CS_READ_TIMEOUT_SEC` | `10` | 请求读取超时（秒） | 无额外校验 | 可 |
| `AI_CS_WRITE_TIMEOUT_SEC` | `15` | 响应写超时（秒） | 无额外校验 | 可 |
| `AI_CS_IDLE_TIMEOUT_SEC` | `60` | 空闲连接超时（秒） | 无额外校验 | 可 |
| `AI_CS_MAX_HEADER_BYTES` | `1048576` | header 大小上限 | 无额外校验 | 可 |
| `AI_CS_MAX_BODY_BYTES` | `1048576` | body 大小上限 | 必须 > 0 | 需结合流量评估 |

### 1.2 Postgres

| 变量名 | 默认值 | 含义 | 当前代码是否校验 | prod 是否应允许默认值 |
|---|---|---|---|---|
| `AI_CS_POSTGRES_ENABLED` | `false` | 是否启用 Postgres store | 解析布尔值 | **不允许** |
| `AI_CS_POSTGRES_DSN` | 空 | Postgres 连接串 | 启用 PG 时必填 | **不允许为空** |
| `AI_CS_POSTGRES_MIGRATION_DIR` | `db/migration` | migration 目录 | 无路径存在性校验 | 需确认可用 |
| `AI_CS_POSTGRES_MAX_OPEN_CONNS` | `20` | 最大打开连接数 | 无额外校验 | 需容量确认 |
| `AI_CS_POSTGRES_MAX_IDLE_CONNS` | `5` | 最大空闲连接数 | 无额外校验 | 需容量确认 |
| `AI_CS_POSTGRES_CONN_MAX_LIFETIME_SEC` | `300` | 连接最大生命周期（秒） | 无额外校验 | 需容量确认 |

### 1.3 Webhook

| 变量名 | 默认值 | 含义 | 当前代码是否校验 | prod 是否应允许默认值 |
|---|---|---|---|---|
| `AI_CS_WEBHOOK_SECRET` | 空 | webhook HMAC secret | production 下必填 | **不允许为空** |
| `AI_CS_WEBHOOK_TIMESTAMP_HEADER` | `X-CS-Timestamp` | 时间戳请求头 | 无额外校验 | 可 |
| `AI_CS_WEBHOOK_SIGNATURE_HEADER` | `X-CS-Signature` | 签名请求头 | 无额外校验 | 可 |
| `AI_CS_WEBHOOK_MAX_SKEW_SECONDS` | `300` | 最大时钟偏差（秒） | 必须 > 0 | 需安全确认 |

---

## 2. 当前代码已经执行的校验

来自 `internal/config/config.go`：

1. `AI_CS_ADDR` 不允许为空
2. `AI_CS_MAX_BODY_BYTES` 必须为正数
3. `AI_CS_POSTGRES_ENABLED=true` 时，`AI_CS_POSTGRES_DSN` 不允许为空
4. `AI_CS_WEBHOOK_MAX_SKEW_SECONDS` 必须为正数
5. `AI_CS_RUNTIME_ENV` 只允许 `production/development/test`
6. `AI_CS_RUNTIME_ENV=production` 时，`AI_CS_POSTGRES_ENABLED` 必须为 `true`
7. `AI_CS_RUNTIME_ENV=production` 时，`AI_CS_WEBHOOK_SECRET` 不允许为空

---

## 3. 当前代码尚未自动保证、但生产必须满足的要求

以下要求目前仍需部署层和运行态共同保证：

1. **readiness 必须反映 DB / migration / 关键配置就绪状态**
2. **migration 目录必须真实可执行，且执行成功才能接流量**
3. **部署文档和环境模板必须只使用真实变量名**

---

## 4. 文档使用规则

后续所有文档若涉及配置、部署、上线前检查，必须以本文档和 `internal/config/config.go` 为唯一事实来源。

### 4.1 禁止继续使用的泛化写法
以下名称若未在代码中真实读取，不应继续写入正式部署文档：
- `DATABASE_URL`
- `POSTGRES_*`
- `WEBHOOK_SECRET`
- `RATE_LIMIT_*`
- `LOG_LEVEL`
- `OPENAI_API_KEY`
- `LLM_PROVIDER`
- `FEISHU_APP_ID`
- `FEISHU_APP_SECRET`
- `TELEGRAM_BOT_TOKEN`

### 4.2 允许的文档表达方式
正确方式：
- 直接写真实变量名
- 标明默认值
- 标明 prod 是否允许默认值
- 标明当前代码是否已强制校验

错误方式：
- 用泛化变量名代替真实变量名
- 把“生产要求”误写成“代码已经自动保证”
- 不区分 dev/test 与 prod 约束

---

## 5. 后续维护要求

若 `internal/config/config.go` 变更，必须同步更新：
1. `docs/CONFIG_CONTRACT_BASELINE.md`
2. `prd/PRODUCTION_CHECKLIST.md`
3. `test/QA_GATE_STATUS.md`

否则视为配置契约漂移。
