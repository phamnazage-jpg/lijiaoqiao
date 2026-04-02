# SSO/SAML集成技术调研报告

> 版本：v1.1
> 日期：2026-04-02
> 目的：为LLM Gateway项目提供SSO/SAML技术选型参考
> 状态：已修复（根据2026-04-02评审意见）

---

## 1. 执行摘要

### 1.1 调研范围

本报告针对以下SSO/SAML方案进行技术调研和对比分析：
- Keycloak（开源）
- Auth0（商业，现属Okta）
- Okta（商业）
- Casdoor（开源，中国团队）
- Ory（开源）
- Azure AD / Microsoft Entra ID（商业，微软）

### 1.2 关键结论

| 优先级 | 场景 | 推荐方案 | 理由 |
|--------|------|----------|------|
| **P0** | 快速上线 + 成本敏感 | **Casdoor** | 轻量级部署、中文文档、中国合规 |
| **P1** | 企业级功能 + 长期演进 | **Keycloak** | 功能最全面、社区活跃、定制能力强 |
| **P2** | 国际化企业客户 | **Okta/Auth0** | 品牌信任、全球化合规 |
| **后续** | Microsoft 365生态客户 | **Azure AD/Entra ID** | 企业市场领导者，世纪互联运营合规版本 |

### 1.3 行动建议

**近期（1-2个月）**：采用 Casdoor 作为MVP方案，满足快速上线需求
**中期（3-6个月）**：评估 Keycloak 迁移路径，支持更复杂的企业需求
**长期（6个月+）**：根据客户群体，决定是否迁移到 Okta/Auth0 或 Azure AD/Entra ID

---

## 2. 供应商详细对比

### 2.1 Keycloak

#### 2.1.1 基本信息

| 属性 | 值 |
|------|-----|
| 类型 | 开源 (Apache 2.0) |
| 官网 | https://www.keycloak.org |
| 最新版本 | 26.x (2025) |
| GitHub Stars | ~28k |
| 主要语言 | Java |

#### 2.1.2 功能特性

**协议支持**：
- SAML 2.0 (完整实现)
- OpenID Connect / OAuth 2.0
- LDAP / Active Directory
- Social Login (Google, GitHub, etc.)
- 离线令牌

**企业级功能**：
- 多租户支持 (Realm级别隔离)
- 细粒度RBAC + ABAC
- 身份代理 (Identity Brokering)
- 用户联盟
- 审计日志
- 密码策略
- MFA (TOTP, WebAuthn, SMS)
- 客户端_credentials_flow (机器对机器)

#### 2.1.3 Go集成方案

**推荐库**：
- `github.com/keycloak/keycloak-go` (社区维护)
- `github.com/coreos/go-oidc` (通用OIDC库)

**集成复杂度**：中等
- 需要部署Keycloak服务器
- 提供标准OIDC/SAML接口
- 官方提供 Helm Chart / Operator

**优势**：
- 功能最全面的开源方案
- 活跃的社区和丰富的文档
- Red Hat支持 (JBoss/WildFly生态)
- 大量生产环境验证

**劣势**：
- 资源占用较高 (建议4C8G+)
- Java技术栈，学习曲线
- 默认配置安全性需加强
- 中国区无原生CDN加速

#### 2.1.4 成本分析

| 成本项 | 费用 |
|--------|------|
| 软件本身 | 免费 |
| 自托管服务器 | ¥500-2000/月 (4C8G云主机) |
| 运维人力 | 0.5-1 FTE |
| 商业支持 | Red Hat SSO (约 $40k/年) |

---

### 2.2 Auth0

#### 2.2.1 基本信息

| 属性 | 值 |
|------|-----|
| 类型 | 商业SaaS |
| 官网 | https://auth0.com |
| 母公司 | Okta (2021年收购) |
| 估值 | $340亿+ (Okta) |

#### 2.2.2 功能特性

**协议支持**：
- SAML 2.0 / SAML Proxy
- OpenID Connect / OAuth 2.0
- WS-Federation
- 所有主流Social Login

**企业级功能**：
-  universal login (单页登录)
-  异常检测 (Anomaly Detection)
-  机器对机器认证
-  无密码认证 (Passwordless)
-  实时日志流
-  99.99% SLA

#### 2.2.3 Go集成方案

**推荐库**：
- `github.com/auth0/go-auth0` (官方SDK)
- `goth` (社区Social Login库)

**集成复杂度**：低
- 提供SDK，集成简单
- 丰富的API和Webhook
- 详细的开发者文档

**优势**：
- 零运维负担
- 业界最佳实践
- 快速集成 (通常1-2周)
- 信用卡PCI合规

**劣势**：
- **数据必须传到境外服务器**
- 成本较高 (按MAU计费)
- 中国区访问慢/不稳定
- vendor lock-in风险

#### 2.2.4 成本分析

| 成本项 | 费用 |
|--------|------|
| 免费额度 | 7,000 MAU |
| Growth Plan | $165/月起 或 $0.020/MAU |
| Enterprise | 需询价 ($50k+/年) |
| **实际成本** | **¥5-50万/年** |

---

### 2.3 Okta

#### 2.3.1 基本信息

| 属性 | 值 |
|------|-----|
| 类型 | 商业SaaS |
| 官网 | https://www.okta.com |
| 上市 | NASDAQ: OKTA |
| 市值 | ~$340亿 |

#### 2.3.2 功能特性

**协议支持**：
- SAML 2.0 (核心功能)
- OpenID Connect / OAuth 2.0
- SCIM 2.0 (用户配置)
- 所有主流企业应用集成

**企业级功能**：
-  Application Network (预集成7k+应用)
-  Lifecycle Management
-  API Access Management
-  Advanced Server Access
-  Privileged Access
-  99.99% SLA

#### 2.3.3 Go集成方案

**推荐库**：
- `github.com/okta/okta-sdk-golang` (官方SDK)
- `github.com/okta/okta-jwt-verifier-go`

**集成复杂度**：低

**优势**：
- 企业市场领导者
- 最广泛的集成生态
- 成熟的治理功能
- 强大的合规认证 (SOC2, ISO27001, FedRAMP)

**劣势**：
- **数据必须传到境外服务器**
- 成本最高
- 中国区访问问题严重
- vendor lock-in风险

#### 2.3.4 成本分析

| 成本项 | 费用 |
|--------|------|
| IAM | $3-6/人/月 |
| SSO | $4-8/人/月 |
| Lifecycle | $3/人/月 |
| **Enterprise套餐** | **$100k+/年** |

---

### 2.4 Casdoor

#### 2.4.1 基本信息

| 属性 | 值 |
|------|-----|
| 类型 | 开源 (Apache 2.0) |
| 官网 | https://casdoor.org |
| GitHub Stars | ~9k |
| 主要语言 | Go |
| 维护团队 | 中国团队 |

#### 2.4.2 功能特性

**协议支持**：
- SAML 2.0 (完整实现)
- OpenID Connect / OAuth 2.0
- LDAP (部分)
- 社交登录 (微信、钉钉等中国平台)
- CAS 1.0/2.0

**企业级功能**：
- 多租户支持
- 基于组织的访问控制 (Org-based RBAC)
- WebAuthn / FIDO2
- 微信/钉钉/飞书集成 (中国特色)
- 轻量级设计 (~50MB内存)

#### 2.4.3 Go集成方案

**推荐库**：
- `github.com/casdoor/casdoor-go-sdk` (官方SDK)
- 直接调用API

**集成复杂度**：低
- 纯Go实现，与项目技术栈一致
- 提供 Helm Chart / Docker Compose
- 中文文档完善

**优势**：
- **Go语言原生，与Gateway技术栈一致**
- **内置中国社交登录（微信、钉钉）**
- **轻量级，资源占用低**
- **中文社区，文档完善**
- **可以完全自托管，数据不出境**

**劣势**：
- 社区规模较小
- 国际化程度较低
- 生产验证案例少于Keycloak
- 部分功能仍在完善

#### 2.4.4 成本分析

| 成本项 | 费用 |
|--------|------|
| 软件本身 | 免费 |
| 自托管服务器 | ¥100-500/月 (2C4G即可) |
| 运维人力 | 0.25-0.5 FTE |
| 商业支持 | 暂无官方商业支持 |

---

### 2.5 Ory

#### 2.5.1 基本信息

| 属性 | 值 |
|------|-----|
| 类型 | 开源 (Apache 2.0) |
| 官网 | https://www.ory.sh |
| GitHub Stars | ~15k (Keto) |
| 主要语言 | Go |

#### 2.5.2 功能特性

**组件**：
- **Ory Kratos**: 身份与用户管理
- **Ory OAuth2/OIDC**: OAuth2/OIDC实现
- **Ory Keto**: 权限管理 (Zanzibar)
- **Ory Hydra**: OAuth2授权服务器
- **Ory Oathkeeper**: 零信任代理

**协议支持**：
- OpenID Connect / OAuth 2.0
- OAuth 2.0 (Hydra)
- WebAuthn / FIDO2 (Kratos)
- **不支持SAML** (重要！)

**企业级功能**：
- 微服务友好
- 云原生架构 (Kubernetes Native)
- 可扩展的权限模型
- 低延迟 (Go实现)

#### 2.5.3 Go集成方案

**推荐库**：
- `github.com/ory/kratos-client-go`
- `github.com/ory/hydra-client-go`

**集成复杂度**：中
- 需要组合多个组件
- 无SAML支持
- 文档质量参差不齐

**优势**：
- **Go语言原生，性能优异**
- 云原生友好
- 现代架构设计
- 权限模型强大

**劣势**：
- **不支持SAML** (如有SAML需求则排除)
- 组件较多，集成复杂
- 社区较小
- 文档不够完善

#### 2.5.4 成本分析

| 成本项 | 费用 |
|--------|------|
| 软件本身 | 免费 (开源) |
| Ory Cloud | $25/月起 (托管服务) |
| 自托管服务器 | ¥500-1500/月 |
| 商业支持 | Ory Enterprise (询价) |

---

### 2.6 Azure AD / Microsoft Entra ID

#### 2.6.1 基本信息

| 属性 | 值 |
|------|-----|
| 类型 | 商业SaaS |
| 官网 | https://www.microsoft.com/en-us/security/business/microsoft-entra-id |
| 原名 | Azure Active Directory (Azure AD) |
| 最新名称 | Microsoft Entra ID |
| 市值 | 微软市值 ~$2.8万亿 |

#### 2.6.2 中国运营版本

| 版本 | 运营方 | 合规优势 |
|------|--------|----------|
| Global版 | 微软（境外） | 全球覆盖，中国大陆访问受限 |
| 世纪互联版 | 世纪互联（境内） | **数据存储在中国大陆**，满足《网络安全法》要求 |

> **重要**：中国区企业客户可申请**世纪互联运营的Entra ID版本**，数据存储在境内数据中心，合规风险显著低于其他境外SaaS方案。

#### 2.6.3 功能特性

**协议支持**：
- SAML 2.0 (完整实现)
- OpenID Connect / OAuth 2.0
- WS-Federation
- SCIM 2.0 (用户配置)
- 所有主流企业应用集成

**企业级功能**：
- Microsoft 365 / Teams / SharePoint / Dynamics 365 原生集成
- Application Network (预集成900+应用)
- Conditional Access (条件访问)
- Identity Protection (身份保护)
- Privileged Identity Management (特权身份管理)
- API Access Management
- 99.99% SLA

#### 2.6.4 Go集成方案

**推荐库**：
- `github.com/microsoftgraph/msgraph-sdk-go` (官方SDK)
- `github.com/AzureAD/microsoft-authentication-library-for-go` (MSAL Go)

**集成复杂度**：中
- 标准OIDC/SAML接口，集成友好
- Microsoft Graph API功能丰富
- 文档完善，但中国版可能有差异

**优势**：
- **企业市场领导者**，品牌信任度高
- **Microsoft 365生态原生集成**
- **世纪互联版本数据境内存储**，合规风险低
- 企业客户已有订阅的情况多
- 合规认证最全（SOC2, ISO27001, FedRAMP, 中国等保）

**劣势**：
- 境外Global版数据出境风险高
- 成本较高
- 部分功能仅限Global版
- 技术栈绑定微软生态

#### 2.6.5 成本分析

| 成本项 | 费用 |
|--------|------|
| Free Tier | 免费（基础功能） |
| P1 (每用户/月) | $6 |
| P2 (每用户/月) | $9 |
| Enterprise套餐 | $100k+/年 |
| **实际成本** | **¥300-600/人/年（基础版）** |

---

## 3. 综合对比表

### 3.1 功能维度

| 特性 | Keycloak | Auth0 | Okta | Casdoor | Ory | Azure AD |
|------|----------|-------|------|---------|-----|----------|
| SAML 2.0 | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
| OIDC/OAuth2 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 多租户 | ✅ | ✅ | ✅ | ✅ | ⚠️ | ✅ |
| MFA | ✅ | ✅ | ✅ | ✅ | ⚠️ | ✅ |
| 中国社交登录 | ⚠️ | ❌ | ❌ | ✅ | ❌ | ❌ |
| 用户 federation | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | ✅ |
| 轻量级 | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Go SDK | ⚠️ | ✅ | ✅ | ✅ | ✅ | ⚠️ |
| 社区活跃度 | 高 | 高 | 高 | 中 | 中 | 高 |
| Microsoft 365集成 | ⚠️ | ❌ | ❌ | ❌ | ❌ | ✅ |
| 审计报表 | ✅ | ✅ | ✅ | ⚠️ | ⚠️ | ✅ |

### 3.2 成本维度

| 成本项 | Keycloak | Auth0 | Okta | Casdoor | Ory | Azure AD |
|--------|----------|-------|------|---------|-----|----------|
| 软件成本 | $0 | $0-$50k+/年 | $50k+/年 | $0 | $0 | ¥300-600/人/年 |
| 基础设施/月 | ¥500-2000 | $0 | $0 | ¥100-500 | ¥500-1500 | $0 |
| 集成复杂度 | 中 | 低 | 低 | 低 | 中 | 中 |
| 维护成本 | 中 | 低 | 低 | 低 | 中 | 低 |

### 3.3 合规维度

| 合规要求 | Keycloak | Auth0 | Okta | Casdoor | Ory | Azure AD (Global) | Azure AD (世纪互联) |
|----------|----------|-------|------|---------|-----|---------------------|---------------------|
| 中国数据不出境 | ✅ | ❌ | ❌ | ✅ | ✅ | ❌ | **✅** |
| GDPR | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| SOC2 | ⚠️ | ✅ | ✅ | ❌ | ⚠️ | ✅ | ✅ |
| ISO27001 | ⚠️ | ✅ | ✅ | ❌ | ⚠️ | ✅ | ✅ |
| 中国等保 | ⚠️ | ❌ | ❌ | ⚠️ | ⚠️ | ⚠️ | **待定** |
| FedRAMP | ❌ | ✅ | ✅ | ❌ | ❌ | ✅ | ❌ |

---

## 4. 中国区合规分析

### 4.1 数据本地化要求

根据《网络安全法》《数据安全法》《个人信息保护法》：
- **重要数据必须存储在境内**
- 个人信息出境需安全评估
- 金融、医疗等行业有额外要求

### 4.2 等保合规分析

#### 4.2.1 等保认证状态对比

| 方案 | 等保认证状态 | 认证级别 | 说明 |
|------|-------------|---------|------|
| Keycloak (自托管) | **可满足等保** | 需自行认证 | 本身不具备认证，需通过部署配置满足要求 |
| Casdoor (自托管) | **待验证** | 无官方认证 | 需通过部署配置和额外安全加固满足要求 |
| Ory (自托管) | **待验证** | 无官方认证 | 需通过部署配置满足要求 |
| Azure AD (世纪互联) | **待定** | 暂无等保认证 | 微软未公开等保认证情况 |
| Auth0 | **不可行** | 无 | 境外服务，数据出境 |
| Okta | **不可行** | 无 | 境外服务，数据出境 |

#### 4.2.2 等保合规验证清单

**自托管方案等保满足路径**：

1. **网络安全等级保护（等保2.0）基本要求**：
   - 身份鉴别：实现强密码策略、多因素认证 ✅ (Keycloak/Casdoor均支持)
   - 访问控制：细粒度RBAC/ABAC ✅ (Keycloak最强，Casdoor支持Org-based RBAC)
   - 安全审计：日志记录、留存、查询 ✅ (均支持，但报表能力有差异)
   - 入侵防范：Web应用防火墙、日志监控 ⚠️ 需额外配置
   - 数据保密性：传输加密、存储加密 ✅ (TLS+数据库加密)

2. **各方案合规满足度评估**：

| 等保要求项 | Keycloak | Casdoor | Ory |
|-----------|----------|---------|-----|
| 身份鉴别 (8.1.3) | ✅ 完全满足 | ✅ 满足 | ⚠️ 部分满足 |
| 访问控制 (8.1.4) | ✅ 完全满足 | ✅ 满足 | ⚠️ 部分满足 |
| 安全审计 (8.1.5) | ✅ 完整审计日志 | ⚠️ 基础日志 | ⚠️ 基础日志 |
| 审计报表导出 | ✅ 支持 | ❌ 不支持 | ❌ 不支持 |
| 数据保密性 (8.1.9) | ✅ 满足 | ✅ 满足 | ✅ 满足 |
| **等保合规风险** | **低** | **中** | **中** |

#### 4.2.3 行业特定合规建议

| 行业 | 额外要求 | 合规建议 |
|------|---------|----------|
| 政府/国企 | 等保三级、系统国产化 | **Keycloak**（功能最全面，可定制） |
| 金融 | 等保三级、PCI DSS、数据加密 | **Keycloak** + 额外安全加固 |
| 医疗 | 等保二级、HIPAA合规 | **Keycloak** 或 **Casdoor** |
| 教育 | 等保二级 | **Casdoor**（轻量、微信集成） |

> **重要结论**：Casdoor和Ory均未取得等保认证，在政府/金融/医疗行业作为IdP使用时可能存在准入障碍。建议在高合规要求行业中使用**Keycloak自托管**方案。

### 4.3 合规结论

| 方案 | 数据出境风险 | 等保合规 | 建议 |
|------|-------------|----------|------|
| Keycloak (自托管) | **无风险** | **可满足（需自行认证）** | 推荐 ✅ |
| Auth0 | **高风险** | 不可行 | 不推荐 ❌ |
| Okta | **高风险** | 不可行 | 不推荐 ❌ |
| Casdoor (自托管) | **无风险** | **待验证（存在风险）** | 推荐（谨慎）⚠️ |
| Ory (自托管) | **无风险** | **待验证（存在风险）** | 慎选 ⚠️ |
| Azure AD (世纪互联) | **低风险** | **待定** | 可考虑（Microsoft生态） |

**关键结论**：
1. 如需满足中国合规要求，必须选择自托管方案（Keycloak/Casdoor/Ory）或世纪互联版Azure AD
2. 高合规要求行业（政府/金融/医疗）建议使用**Keycloak**，Casdoor/Ory可能存在准入障碍
3. Microsoft 365生态客户可考虑**Azure AD世纪互联版**，但需确认等保认证状态

### 4.4 审计报表能力评估

审计报表是企业版首批必含能力之一，以下是各方案的审计能力对比：

#### 4.4.1 审计能力对比

| 审计能力 | Keycloak | Auth0 | Okta | Casdoor | Ory | Azure AD |
|---------|----------|-------|------|---------|-----|----------|
| 登录日志 | ✅ | ✅ | ✅ | ✅ | ✅ | ✅ |
| 操作审计日志 | ✅ | ✅ | ✅ | ⚠️ 基础 | ⚠️ 基础 | ✅ |
| 自定义报表 | ✅ | ✅ | ✅ | ❌ | ❌ | ✅ |
| 合规报告模板 | ✅ (SOX等) | ✅ | ✅ | ❌ | ❌ | ✅ |
| 日志导出格式 | JSON/SYSLOG | JSON | JSON | JSON | JSON | JSON |
| 日志留存周期 | 可配置 | 可配置 | 可配置 | 依赖DB | 依赖DB | 可配置 |
| 实时日志流 | ⚠️ | ✅ | ✅ | ❌ | ⚠️ | ✅ |
| 用户行为分析 | ⚠️ | ✅ | ✅ | ❌ | ❌ | ✅ |
| 异常检测 | ⚠️ | ✅ | ✅ | ❌ | ❌ | ✅ |

#### 4.4.2 审计报表能力分析

**Keycloak**：
- 完整的审计事件日志（登录、登出、操作）
- 支持导出为JSON格式
- 可对接外部SIEM系统（ELK、Splunk）
- 自定义报表需借助第三方工具
- 提供事件监听器接口，可扩展

**Auth0/Okta**：
- 最完善的审计报表能力
- 内置异常检测和实时告警
- 丰富的合规报告模板（SOX、HIPAA、GDPR）
- 99.99% SLA保障

**Casdoor**：
- 基础审计日志功能
- 支持登录/登出事件记录
- **不支持自定义报表**
- **不支持合规报告模板**
- 日志依赖数据库存储，需自行实现导出

**Ory**：
- 基础审计能力
- 通过Ory Keto可追踪权限变更
- **不支持自定义报表**
- 微服务架构，日志分散

**Azure AD**：
- 完整的审计日志
- Azure Monitor集成
- 合规报告模板丰富
- Microsoft Sentinel可选

#### 4.4.3 审计报表能力结论

| 场景 | 推荐方案 | 说明 |
|------|---------|------|
| 基础审计需求 | Casdoor | MVP阶段可用，需自行扩展报表 |
| 企业级审计 | Keycloak + SIEM | 可对接外部系统实现完整审计 |
| 高合规要求 | Okta/Auth0/Azure AD | 内置完整审计和合规报表 |

---

## 5. 技术选型建议

### 5.1 场景分析

#### 场景A：快速上线 + 成本敏感 + 中国市场

**推荐：Casdoor**

理由：
1. Go语言原生，集成成本最低
2. 内置微信/钉钉/飞书登录，中国市场刚需
3. 轻量级，2C4G即可运行
4. 部署简单，有Docker Compose
5. 数据完全自托管，满足合规

风险：
- 社区较小，生产案例有限
- 部分功能（如SAML IdP）稳定性待验证

#### 场景B：企业级需求 + 多客户 + 长期演进

**推荐：Keycloak**

理由：
1. 功能最全面，生产验证最充分
2. 社区活跃，问题解决快
3. 支持SAML和OIDC，兼容性最好
4. 多租户能力强
5. Red Hat商业支持可选

风险：
- 资源消耗较高
- Java技术栈，与Go项目风格差异

#### 场景C：国际化企业客户为主

**推荐：Okta/Auth0**

理由：
1. 品牌认可度高
2. 预集成7k+应用
3. 合规认证最全
4. 零运维

风险：
- 数据出境问题
- 成本高昂
- vendor lock-in

### 5.2 推荐架构

```
                    ┌─────────────────────────────────────┐
                    │           LLM Gateway                │
                    │  ┌─────────────────────────────┐   │
                    │  │     Token Auth Middleware   │   │
                    │  └──────────────┬──────────────┘   │
                    │                 │                  │
                    │                 ▼                  │
                    │  ┌─────────────────────────────┐   │
                    │  │    SSO/SAML Integration     │   │
                    │  │  (Go OIDC/SAML Client Lib)  │   │
                    │  └──────────────┬──────────────┘   │
                    └─────────────────┼──────────────────┘
                                      │
                                      ▼
                    ┌─────────────────────────────────────┐
                    │      身份提供商 (IdP)                 │
                    │                                      │
                    │  MVP阶段: Casdoor (自托管)            │
                    │  演进阶段: Keycloak (可选)            │
                    │  企业客户: Okta/Auth0 (可选)          │
                    │  Microsoft生态: Azure AD/Entra ID    │
                    └─────────────────────────────────────┘
```

---

## 6. 集成方案设计

### 6.1 Casdoor集成方案

#### 6.1.1 部署架构

```yaml
# docker-compose.yml (Casdoor)
version: '3.8'
services:
  casdoor:
    image: casbin/casdoor:latest
    ports:
      - "8000:8000"
    environment:
      RUNNING_IN_DOCKER: "true"
    volumes:
      - ./conf:/conf
      - ./data:/data
    networks:
      - casdoor-network

  # 可选：MySQL/PostgreSQL 存储
  db:
    image: postgres:15
    environment:
      POSTGRES_DB: casdoor
      POSTGRES_USER: casdoor
      POSTGRES_PASSWORD: secret
    volumes:
      - db-data:/var/lib/postgresql/data
    networks:
      - casdoor-network

networks:
  casdoor-network:
    driver: bridge

volumes:
  db-data:
```

#### 6.1.2 Go集成代码

```go
// internal/middleware/sso.go
package middleware

import (
    "context"
    "encoding/json"
    "net/http"
    "net/url"

    "github.com/casdoor/casdoor-go-sdk/casdoorsdk"
)

type SSOConfig struct {
    Endpoint string // e.g., "http://localhost:8000"
    ClientID string
    ClientSecret string
    Certificate string
    OrganizationName string
    ApplicationName string
}

type SSOHandler struct {
    config *SSOConfig
}

func NewSSOHandler(cfg *SSOConfig) *SSOHandler {
    casdoorsdk.InitConfig(
        cfg.ClientID,
        cfg.ClientSecret,
        cfg.Certificate,
        cfg.Endpoint,
        cfg.OrganizationName,
        cfg.ApplicationName,
    )
    return &SSOHandler{config: cfg}
}

// HandleCallback 处理SSO回调
func (h *SSOHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
    code := r.URL.Query().Get("code")
    if code == "" {
        http.Error(w, "missing code", http.StatusBadRequest)
        return
    }

    // 获取token
    token, err := casdoorsdk.GetOAuthToken(code)
    if err != nil {
        http.Error(w, "failed to get token", http.StatusInternalServerError)
        return
    }

    // 获取用户信息
    claims, err := casdoorsdk.ParseJwtToken(token.AccessToken)
    if err != nil {
        http.Error(w, "failed to parse token", http.StatusInternalServerError)
        return
    }

    // 生成内部token或session
    internalToken := generateInternalToken(claims)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "token": internalToken,
        "user": claims,
    })
}

// RedirectToSSO 重定向到SSO登录
func (h *SSOHandler) RedirectToSSO(w http.ResponseWriter, r *http.Request, state string) {
    authURL := casdoorsdk.GetOAuthLoginURL(state)
    http.Redirect(w, r, authURL, http.StatusFound)
}
```

#### 6.1.3 SAML集成配置

Casdoor支持标准SAML 2.0，可作为SP或IdP：

```json
// Casdoor SAML配置示例
{
  "saml": {
    "enable": true,
    "certificate": "-----BEGIN CERTIFICATE-----...",
    "privateKey": "-----BEGIN PRIVATE KEY-----...",
    "signMetadata": true,
    "wantAssertionSigned": true,
    "assertionConsumerServiceURL": "http://your-app/saml/callback",
    "entityID": "urn:casdoor:your-app"
  }
}
```

### 6.2 Keycloak集成方案

#### 6.2.1 部署架构

```yaml
# keycloak-clustered.yaml
apiVersion: v1
kind: StatefulSet
metadata:
  name: keycloak
spec:
  serviceName: keycloak
  replicas: 2
  selector:
    matchLabels:
      app: keycloak
  template:
    spec:
      containers:
        - name: keycloak
          image: quay.io/keycloak/keycloak:26.0
          args: ["start-clustered", "--db=postgres", "--db-url=jdbc:postgresql://postgres:5432/keycloak"]
          env:
            - name: KEYCLOAK_ADMIN
              value: "admin"
            - name: KEYCLOAK_ADMIN_PASSWORD
              valueFrom:
                secretKeyRef:
                  name: keycloak-admin
                  key: password
          resources:
            requests:
              cpu: "1"
              memory: "2Gi"
            limits:
              cpu: "2"
              memory: "4Gi"
```

#### 6.2.2 Go OIDC集成

```go
// internal/middleware/keycloak_oidc.go
package middleware

import (
    "context"
    "crypto/rsa"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "math/big"
    "net/http"
    "sync"
    "time"

    "github.com/coreos/go-oidc/v3/oidc"
)

type KeycloakConfig struct {
    IssuerURL    string
    ClientID     string
    ClientSecret string
}

type KeycloakVerifier struct {
    provider *oidc.Provider
    verifier *oidc.IDTokenVerifier
    config   *KeycloakConfig
}

func NewKeycloakVerifier(ctx context.Context, cfg *KeycloakConfig) (*KeycloakVerifier, error) {
    provider, err := oidc.NewProvider(ctx, cfg.IssuerURL)
    if err != nil {
        return nil, fmt.Errorf("failed to create provider: %w", err)
    }

    verifier := provider.Verifier(&oidc.Config{
        ClientID: cfg.ClientID,
    })

    return &KeycloakVerifier{
        provider: provider,
        verifier: verifier,
        config:   cfg,
    }, nil
}

func (v *KeycloakVerifier) VerifyToken(ctx context.Context, rawToken string) (*Claims, error) {
    idToken, err := v.verifier.Verify(ctx, rawToken)
    if err != nil {
        return nil, err
    }

    var claims Claims
    if err := idToken.Claims(&claims); err != nil {
        return nil, err
    }

    return &claims, nil
}

type Claims struct {
    Subject       string   `json:"sub"`
    Email         string   `json:"email"`
    EmailVerified bool     `json:"email_verified"`
    Name          string   `json:"name"`
    PreferredUsername string `json:"preferred_username"`
    Groups        []string `json:"groups"`
    Roles         []string `json:"realm_access"`
}
```

---

## 7. 风险评估

### 7.1 各方案风险矩阵

| 风险项 | Keycloak | Casdoor | Okta/Auth0 |
|--------|----------|---------|------------|
| 数据泄露风险 | 低 (自托管) | 低 (自托管) | **高 (境外存储)** |
| 服务中断风险 | 中 (自运维) | 中 (自运维) | 低 (商业SLA) |
| 供应商锁定 | 低 | 低 | **高** |
| 技术支持 | 中 (社区) | 低 (小社区) | 高 (商业支持) |
| 合规风险 | 低 | 低 | **高** |
| 性能问题 | 中 | 低 | 低 |

### 7.2 风险缓解措施

#### 风险1：Casdoor社区较小
**缓解措施**：
- 保持对Keycloak的兼容性预留
- 监控社区发展，适时评估迁移
- 考虑雇佣/咨询Casbin团队

#### 风险2：Keycloak资源消耗高
**缓解措施**：
- 使用Keycloak Operator管理集群
- 配置适当的缓存策略
- 监控资源使用，及时扩容

#### 风险3：中国区网络访问IdP
**缓解措施**：
- Casdoor部署在境内
- Keycloak可选择境内云托管
- 使用CDN加速静态资源

---

## 8. 实施计划

### 8.1 阶段一：MVP快速上线 (1-2个月)

> **注意**：微信/钉钉OAuth对接需考虑企业资质审批时间，建议MVP周期预留1-2个月

**目标**：满足基本SSO需求，快速验证

**任务**：
1. 部署Casdoor实例 (1-2天)
2. 配置OIDC集成 (3-5天)
3. 实现Token中间件 (3-5天)
4. 对接微信/钉钉登录 (1-2周，含企业资质审批)
5. SAML 2.0支持 (1周，如客户需要)
6. 测试和文档 (1周)
7. 缓冲时间 (1周，应对集成问题)

**交付物**：
- Casdoor部署配置
- SSO集成代码
- 测试用例
- 运维文档

**成本估算**：
- 人力投入：1-1.5 FTE
- 基础设施：¥100-500/月

### 8.2 阶段二：企业级增强 (1-2个月)

**目标**：支持更复杂的企业需求

**任务**：
1. 多租户隔离强化 (1-2周)
2. MFA集成 (1周)
3. 审计日志完善 (1周)
4. 审计报表功能扩展 (1-2周)
5. Keycloak迁移路径设计 (可选)

**交付物**：
- 多租户设计文档
- MFA集成方案
- 审计报表扩展方案

### 8.3 阶段三：可选迁移评估 (根据需要)

**触发条件**：
- 企业客户明确要求Okta/Auth0/Azure AD
- 运维成本超出承受范围
- 目标行业需要更高级别合规认证

**评估内容**：
- 迁移成本 vs 收益
- 数据出境合规影响
- 供应商锁定风险
- Keycloak vs Azure AD vs Okta 选型

---

## 9. 参考资料

### 9.1 官方文档

- Keycloak: https://www.keycloak.org/documentation
- Casdoor: https://casdoor.org/docs/
- Auth0: https://auth0.com/docs
- Okta: https://developer.okta.com/docs/
- Ory: https://www.ory.sh/docs/
- Azure AD / Microsoft Entra ID: https://www.microsoft.com/en-us/security/business/microsoft-entra-id

### 9.2 Go SDK

- `github.com/casdoor/casdoor-go-sdk`
- `github.com/coreos/go-oidc`
- `github.com/okta/okta-sdk-golang`
- `github.com/keycloak/keycloak-go`
- `github.com/microsoftgraph/msgraph-sdk-go`
- `github.com/AzureAD/microsoft-authentication-library-for-go`

### 9.3 社区资源

- Keycloak GitHub: https://github.com/keycloak/keycloak
- Casdoor GitHub: https://github.com/casdoor/casdoor
- Ory GitHub: https://github.com/ory
- Microsoft Entra ID GitHub: https://github.com/microsoftgraph/msgraph-sdk-go

---

## 10. 附录

### 附录A：术语表

| 术语 | 说明 |
|------|------|
| SSO | Single Sign-On，单点登录 |
| SAML | Security Assertion Markup Language，安全性断言标记语言 |
| OIDC | OpenID Connect，开放ID连接 |
| IdP | Identity Provider，身份提供商 |
| SP | Service Provider，服务提供商 |
| MFA | Multi-Factor Authentication，多因素认证 |
| RBAC | Role-Based Access Control，基于角色的访问控制 |
| ABAC | Attribute-Based Access Control，基于属性的访问控制 |
| SCIM | System for Cross-domain Identity Management，跨域身份管理系统 |

### 附录B：决策树

```
开始
  │
  ├─ 中国市场优先？
  │    │
  │    ├─ 是 ──► Casdoor (MVP) 或 Keycloak (企业)
  │    │
  │    └─ 否 ──► Microsoft 365客户？
  │               │
  │               ├─ 是 ──► Azure AD/Entra ID (世纪互联版)
  │               │
  │               └─ 否 ──► 企业客户？
  │                          │
  │                          ├─ 是 ──► Okta/Auth0 或 Keycloak
  │                          │
  │                          └─ 否 ──► Casdoor 或 Keycloak
  │
  └─ 预算有限？
       │
       ├─ 是 ──► Casdoor (自托管)
       │
       └─ 否 ──► Okta/Auth0 (SaaS) 或 Azure AD (世纪互联版)
```

---

**文档信息**：
- 作者：Claude AI
- 版本：v1.1
- 日期：2026-04-02
- 状态：已修复（根据评审意见）
- 修复内容：补充Azure AD评估、深化等保合规分析、补充审计报表能力评估、修正实施周期估算
