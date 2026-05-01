# Supply-Intelligence 供应链智能增强 — 竞品分析报告

## 1. 竞品范围

| 竞品 | 项目地址 | 技术栈 | 相关能力 |
|-------|---------|--------|---------|
| **LiteLLM** | berriai/litellm | Python/FastAPI | 模型定价数据库、自动路由、新模型告警、部署冷却、容灾切换 |
| **Sub2API** | Wei-Shaw/sub2api | Go/Gin/Ent | 模型定价镜像、代理管理、账号/订阅管理、用量统计、公告系统 |
| **NewAPI / OneAPI** | Calcium-Ion/new-api | Go/Gin/GORM | 渠道管理、模型配置、上游状态监控 |

---

## 2. 核心能力对标

### 2.1 模型定价与供应商数据库

#### LiteLLM Model Prices Database
LiteLLM 维护了行业内最完整的模型定价数据库 `model_prices_and_context_window_backup.json`：

**关键特征**:
- 覆盖 100+ 供应商、1000+ 模型
- 每个模型包含：input_cost_per_token, output_cost_per_token, context_window, max_tokens, supports_vision, supports_function_calling 等
- 支持分层定价（tiered_pricing）：如 >128k tokens 时使用不同单价
- 支持批量定价（batch pricing）
- 支持音频 token 定价
- 支持自定义成本覆盖

**更新机制**:
- 主数据库内置在代码中，通过版本发布更新
- 支持远程拉取更新（可配置镜像源）
- Sub2API 就是从 LiteLLM 上游镜像此文件

#### Sub2API Pricing Service
Sub2API 的定价服务是被动消费型的（从上游获取）：

**关键设计**:
- 远程拉取 LiteLLM 镜像 `model_prices_and_context_window.json`
- 本地 fallback 文件缓存
- SHA256 hash 验证更新
- 模型家族回退算法：未知模型按命名规则回退到已知模型
  - 例如：gpt-5.3 未知 → 回退到 gpt-5.1
  - 例如：claude-unknown → 回退到 claude-sonnet
- 动态价格字段优先级配置

**缺陷**:
- 被动获取，无主动发现新模型能力
- 无模型质量探针（仅依赖定价数据）
- 无自动测试和准入检查

### 2.2 供应商/渠道管理

#### Sub2API Proxy & Account Management
Sub2API 提供了完整的上游管理能力：

**代理管理** (`Proxy` schema):
```go
type Proxy struct {
    name     string   // 代理名称
    protocol string   // 协议
    host     string   // 主机
    port     int      // 端口
    username string   // 用户名（可选）
    password string   // 密码（可选）
    status   string   // active / inactive
}
```

**账号管理** (`Account` schema):
- 支持多个上游供应商
- 每个账号关联一个代理（Proxy）
- 支持账号分组（AccountGroup）
- 软删除机制

**用量统计** (`UsageLog`):
- 详细记录每次请求的模型、token数、成本、时间戳
- `UsageCleanupTask`: 定期清理过期用量数据

#### NewAPI/OneAPI 渠道管理
- 支持多个上游渠道配置
- 渠道状态监控（可用/不可用）
- 支持渠道优先级和权重
- 支持渠道购买次数限制

### 2.3 自动路由与容灾

#### LiteLLM Router & Auto-Router
LiteLLM 的路由系统是其核心竞争力：

**路由策略**:
- **lowest_latency**: 选择响应最快的部署
- **lowest_cost**: 选择成本最低的部署
- **lowest_tpm_rpm**: TPM/RPM 最低
- **least_busy**: 负载最低
- **auto_router**: 语义路由（基于请求内容匹配最适模型）
- **budget_limiter**: 按 key/team 限制预算

**容灾机制**:
- **Cooldown**: 连续失败的部署自动进入 cooldown，暂时从路由池移除
- **Fallback**: 主模型失败时自动切换到备用模型
- **Retries**: 可配置重试次数和策略

**新模型告警** (`new_model_added`):
- 当新模型上线时发送 Slack 告警
- 但仅限于通知，无结构化的准入测试流程

### 2.4 用户与订阅管理

#### Sub2API 用户体系
- `User`: 基础用户信息
- `UserSubscription`: 订阅计划、配额、到期时间
- `UserAttributeDefinition` / `UserAttributeValue`: 用户自定义属性
- `PromoCode` / `RedeemCode`: 营销代码系统
- `SecuritySecret`: 安全凭证管理

---

## 3. 差距分析（我们的机会）

| 能力维度 | 竞品现状 | 我们的机会 |
|---------|---------|---------|
| **模型发现** | LiteLLM 被动维护定价库，Sub2API 被动镜像 | 主动全网扫描发现新模型（爬取供应商 API、HN、Twitter、官方文档） |
| **准入测试** | 竞品均不具备 | 自动化准入测试流程，含功能、性能、成本、安全等维度 |
| **质量探针** | LiteLLM 仅有基础 cooldown，无深度探针 | 多维度品质探针：连通性、配额、延迟、错误率、响应质量 |
| **自动注册** | 竞品均不支持 | 自动在供应商后台注册账号、申请 API Key |
| **账号生命周期** | Sub2API 有基础账号管理，无自动更新 | 自动轮换密钥、检测过期、自动补充账号 |
| **供应商健康大盘** | Sub2API 有用量统计，无综合健康视图 | 统一供应商健康大盘，实时可视化 |
| **模型比价** | LiteLLM 有定价库，但无比价能力 | 同类模型多供应商价格对比，智能推荐最优供应商 |
| **运营工作台** | 竞品均为散点式管理 | 统一运营工作台，支持干预操作（暂停、强制切换、测试触发） |
| **模型下线预测** | LiteLLM 有新模型告警，但无下线预测 | 基于用量趋势和供应商动态预测模型下线 |
| **自动化闭环** | 竞品均为人工配置 | 发现 → 测试 → 准入 → 上线 → 监控 → 下线 全自动化 |

---

## 4. 对产品规划的影响

### 强化方向

1. **模型定价数据库参考 LiteLLM**：
   - 维护标准化的模型定价数据库，支持 input/output cost、context window、功能支持等字段
   - 支持远程更新和本地 fallback
   - 支持模型家族回退

2. **供应商账号管理参考 Sub2API**：
   - 代理（Proxy）管理：协议、主机、端口、状态
   - 账号分组：AccountGroup
   - 软删除机制
   - 安全凭证管理

3. **用量统计参考 Sub2API**：
   - 详细 UsageLog 记录
   - 定期清理机制
   - 用户-订阅-用量关联

4. **路由策略参考 LiteLLM**：
   - 多种路由策略（latency、cost、load、semantic）
   - 容灾切换机制
   - 部署冷却

### 新增差异化能力

5. **主动全网模型发现**：竞品均为被动维护，我们应主动扫描
6. **自动准入测试**：竞品不具备，是核心差异化
7. **自动账号注册**：竞品不支持，是核心差异化
8. **智能推荐**：基于价格、质量、位置的供应商推荐
9. **预测性分析**：模型下线预测、供应商变动预测

---

## 5. 对技术规划的影响

### 应引入的设计模式

| 设计模式 | 来源 | 应用场景 |
|---------|------|---------|
| **Model Prices Database** | LiteLLM | 模型定价数据库，支持远程更新和本地 fallback |
| **SHA256 Hash 验证** | Sub2API | 定价数据更新的完整性验证 |
| **模型家族回退** | Sub2API | 未知模型的智能回退 |
| **Proxy + Account 关联** | Sub2API | 上游代理与账号的关联管理 |
| **UsageLog + CleanupTask** | Sub2API | 用量记录与定期清理 |
| **路由策略抽象** | LiteLLM | 支持多种路由策略的插件化设计 |
| **Cooldown + Fallback** | LiteLLM | 故障部署的自动处理 |

### 技术避坑

1. **不重复造轮子**: 定价数据库可以直接复用 LiteLLM 的开源数据，不需要自己维护
2. **发现与测试解耦**: 模型发现和准入测试应该解耦，支持独立触发和组合触发
3. **注册模块的可扩展性**: 每个供应商的注册流程不同，需要抽象接口 + 具体实现
4. **测试隔离**: 准入测试不得影响生产环境，必须使用独立账号或模拟环境
