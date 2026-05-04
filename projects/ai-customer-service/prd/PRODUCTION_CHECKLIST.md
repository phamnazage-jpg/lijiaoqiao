# 生产一期上线前清单（整改版）

> 版本：v2.2  
> 日期：2026-05-04  
> 负责人：PM（小龙团队）  
> 范围：ai-customer-service 生产一期（Phase 1）  
> 依据：`docs/RECTIFICATION_REVIEW_REPORT_V2.md`、`test/QA_GATE_STATUS.md`、当前代码配置契约

---

## 0. 使用说明

本清单不再把“仓库内测试通过”直接等同于“生产可上线”。

本项目当前必须分三层判断：
1. **代码级门禁**：代码主链存在，仓库内测试通过
2. **预生产门禁**：真实依赖、真实配置、真实联调完成
3. **生产放行门禁**：灰度、监控、回滚、运行基线闭环

**当前真实结论：**
- 代码级门禁：已通过
- 预生产门禁：未通过
- 生产放行门禁：未通过

因此：**当前仅可进入预生产整改与联调准备，不可按“生产已具备上线条件”放行。**

---

## 1. 当前已确认事实

### 1.1 代码主链状态
当前已确认具备：
- webhook 收消息
- dialog 意图识别
- handoff 创建工单
- ticket assign / resolve / close / get
- feedback / manual handoff / stats 能力
- Webhook HMAC / timestamp / dedup / body limit / rate limit 基础安全入口
- Postgres 持久化路径

### 1.2 仓库内验证状态
已执行/已确认的关键验证包括：

```bash
go test ./internal/config ./internal/app ./test/integration -count=1
go test ./... -count=1
go vet ./...
```

**当前解释口径：**
- 这些结果说明：仓库内关键测试与静态检查已通过
- 这些结果**不等于**：生产依赖、配置、部署和运行门禁已闭环

### 1.3 本轮已完成的代码级整改
1. prod 下不再允许依赖 memory fallback 启动
2. prod 下要求 `AI_CS_WEBHOOK_SECRET` 非空
3. `AI_CS_RUNTIME_ENV` / `AI_CS_ENV` 契约已明确，且有测试覆盖
4. readiness 语义已校准：
   - production 缺关键配置时直接启动失败
   - non-prod memory 模式可正常 ready
5. 测试初始化配置已同步到 `test` runtime，保证测试语义清晰

---

## 2. 当前仍阻断生产放行的事项

### 2.1 剩余 P0/P1 阻断项
1. **真实环境 DB / migration / webhook / audit / ticket 入库验证未闭环**
2. **部署侧关键配置 fail-fast、监控、回滚 runbook 未落地**
3. **灰度观察项与放量证据尚未形成**

### 2.2 当前明确不能下的结论
- 不能说“上线门禁全部通过”
- 不能说“允许上线”
- 不能把“代码主链通过”写成“生产 ready”

---

## 3. 代码真实配置契约（当前基线）

> 以下内容以 `internal/config/config.go` 当前实现为准。PM / QA / DevOps 文档均应以此为唯一来源。

### 3.1 Runtime / 环境模式
| 变量名 | 默认值 | 说明 | 是否允许 prod 使用默认值 |
|---|---|---|---|
| `AI_CS_RUNTIME_ENV` | `development` | 运行环境模式，支持 `development` / `production` / `test`（兼容旧 `AI_CS_ENV` 读法） | **不允许依赖默认值** |
| `AI_CS_ENV` | 无 | 兼容旧变量；仅当 `AI_CS_RUNTIME_ENV` 未设置时回退使用 | 不建议继续作为正式新配置入口 |

### 3.2 HTTP 相关
| 变量名 | 默认值 | 说明 | 是否允许 prod 使用默认值 |
|---|---|---|---|
| `AI_CS_ADDR` | `:8080` | HTTP 监听地址 | 视部署环境决定 |
| `AI_CS_READ_HEADER_TIMEOUT_SEC` | `5` | header 读取超时 | 可 |
| `AI_CS_READ_TIMEOUT_SEC` | `10` | 请求读取超时 | 可 |
| `AI_CS_WRITE_TIMEOUT_SEC` | `15` | 响应写超时 | 可 |
| `AI_CS_IDLE_TIMEOUT_SEC` | `60` | 空闲连接超时 | 可 |
| `AI_CS_MAX_HEADER_BYTES` | `1048576` | 最大 header 大小 | 可 |
| `AI_CS_MAX_BODY_BYTES` | `1048576` | 最大 body 大小 | 需按生产流量评估 |

### 3.3 Postgres 相关
| 变量名 | 默认值 | 说明 | 是否允许 prod 使用默认值 |
|---|---|---|---|
| `AI_CS_POSTGRES_ENABLED` | `false` | 是否启用 PG store | **production 不允许默认/不允许 false** |
| `AI_CS_POSTGRES_DSN` | 空 | PG 连接串 | **启用 PG 时不允许为空** |
| `AI_CS_POSTGRES_MIGRATION_DIR` | `db/migration` | migration 目录 | 需确认可用 |
| `AI_CS_POSTGRES_MAX_OPEN_CONNS` | `20` | 最大打开连接数 | 需容量确认 |
| `AI_CS_POSTGRES_MAX_IDLE_CONNS` | `5` | 最大空闲连接数 | 需容量确认 |
| `AI_CS_POSTGRES_CONN_MAX_LIFETIME_SEC` | `300` | 连接最大生命周期 | 需容量确认 |

### 3.4 Webhook 安全相关
| 变量名 | 默认值 | 说明 | 是否允许 prod 使用默认值 |
|---|---|---|---|
| `AI_CS_WEBHOOK_SECRET` | 空 | webhook HMAC secret | **production 不允许为空** |
| `AI_CS_WEBHOOK_TIMESTAMP_HEADER` | `X-CS-Timestamp` | 时间戳 header | 通常可 |
| `AI_CS_WEBHOOK_SIGNATURE_HEADER` | `X-CS-Signature` | 签名 header | 通常可 |
| `AI_CS_WEBHOOK_MAX_SKEW_SECONDS` | `300` | 最大时钟偏差 | 需安全确认 |

---

## 4. 预生产前必须确认的配置与依赖

### 4.1 必须由 TechLead / DevOps 共同确认
| 项目 | 当前要求 | 责任角色 |
|---|---|---|
| runtime env 已明确 | `AI_CS_RUNTIME_ENV=production` | TechLead / DevOps |
| Postgres 已启用 | `AI_CS_POSTGRES_ENABLED=true` | TechLead / DevOps |
| Postgres 连接串有效 | `AI_CS_POSTGRES_DSN` 非空且可连通 | DevOps |
| migration 目录可执行 | `AI_CS_POSTGRES_MIGRATION_DIR` 可访问且脚本可执行 | DevOps |
| webhook secret 已配置 | `AI_CS_WEBHOOK_SECRET` 与上游一致 | TechLead |
| body limit 配置已评估 | `AI_CS_MAX_BODY_BYTES` 满足真实流量 | TechLead |
| webhook skew 已评估 | `AI_CS_WEBHOOK_MAX_SKEW_SECONDS` 满足时钟偏差策略 | TechLead |

### 4.2 当前文档中不再使用的泛化变量写法
以下写法不再作为正式部署基线：
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

原因：**这些名称不是 `internal/config/config.go` 当前真实读取项，继续使用会造成部署误配。**

---

## 5. 预生产门禁（Gate B）

以下全部完成前，不得进入“可灰度”结论：

### 5.1 真实环境验证
- [ ] 使用真实环境变量启动一次服务
- [ ] 确认 `AI_CS_RUNTIME_ENV=production` 下启动行为符合预期
- [ ] 确认 Postgres 可连通
- [ ] 确认 migration 执行成功
- [ ] 确认 webhook 签名联调成功
- [ ] 确认 ticket 实际入库成功
- [ ] 确认 audit 实际入库成功
- [ ] 确认实例重启后数据仍然存在

### 5.2 运行门禁验证
- [ ] 确认缺关键配置时启动直接失败
- [x] 确认 non-prod memory 模式不会被误判为 not ready
- [ ] 确认生产 Postgres 模式的 readiness 能反映真实依赖状态
- [ ] 确认缺少 DB / secret 时不会以“假成功”状态进入流量

### 5.3 文档一致性验证
- [x] QA 文档与当前代码状态一致
- [x] PM checklist 与配置契约一致
- [x] 整改执行表状态已同步

---

## 6. 生产灰度门禁（Gate C）

以下全部完成前，不得进入“生产可放量”结论：

### 6.1 灰度准备
- [ ] 已有真实部署基线文档
- [ ] 已有监控大盘 / 告警项
- [ ] 已有回滚 runbook
- [ ] 已保留上一版本回滚路径

### 6.2 灰度观察项
- [ ] handoff 比率正常
- [ ] ticket 创建量正常
- [ ] audit 写入持续正常
- [ ] 5xx / reject 未异常飙升
- [ ] ready down 时长在可接受范围内

### 6.3 放量条件
- [ ] 5% 灰度稳定
- [ ] 30% 灰度稳定
- [ ] 回滚演练已验证
- [ ] PM / QA / TechLead / DevOps 共同签字确认

---

## 7. 角色责任分工

| 角色 | 当前必须完成的动作 |
|---|---|
| 小龙 | 统一阶段口径，禁止无证据放行 |
| PM | 修正上线口径、配置契约表达、观察指标和失败线 |
| TechLead | 禁止 prod fallback、校准 runtime env/readiness、输出配置契约基线 |
| QA | 维护分层门禁结论，防止状态漂移 |
| DevOps | 建立部署 fail-fast、监控、回滚、runbook |

---

## 8. 当前正式结论

**ai-customer-service 当前应定义为：**

> **第5件事已完成；代码级门禁已通过，适合进入预生产联调与部署基线整改阶段；但尚不应被标记为生产可直接上线。**

因此：
- **允许继续预生产整改和联调准备**
- **不允许按“上线门禁全部通过”口径对外宣称可上线**
