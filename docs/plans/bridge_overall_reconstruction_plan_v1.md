# Bridge 项目整体完全重构方案 v1.0

> **项目**: 立交桥 / Bridge Gateway
> **主代码库**: `/home/long/project/立交桥/`
> **漂移目录 A**: `/home/long/hermes-agent/bridge/` (规划/前端/部署版)
> **漂移目录 B**: `/home/long/hermes-agent-official/bridge/backend/` (精简架构蓝本)
> **编制日期**: 2026-04-26
> **状态**: 待执行

---

## 一、现状诊断

### 1.1 三个代码库关系

```
主项目 (立交桥)          漂移目录 A                漂移目录 B
┌────────────────────┐    ┌────────────────────┐    ┌──────────────────┐
│ gateway/              │    │ docs/plans/           │    │ internal/         │
│ supply-api/           │    │ web/apps/             │    │   api/gateway/    │
│ platform-token-runtime/ │  │ docker-compose.yml    │    │   route/          │
│ review/ (大量报告)   │    │ backend/ (嵌在官方仓)│    │   service/        │
│ sql/                  │    │                       │    │   upstream/       │
└────────────────────┘    └────────────────────┘    └──────────────────┘
  → 实际生产代码            → 规划文档+前端+        → 目标架构蓝图
                              部署配置
```

- **主项目**：唯一能够真实启动、测试、落库的代码库。但缺陷严重，前端缺失。
- **A目录**：包含完整产品规格、技术架构、实施计划，以及 Next.js 前端设计（admin-console + user-console）。但 `backend/` 是 `hermes-agent` 官方仓库的子目录，非独立模块。
- **B目录**：精简的独立 Go 模块（约 1,085 行），采用更干净的分层架构（api → service → upstream → route），是理想的后端架构蓝图。

### 1.2 主项目关键缺陷

#### P0 阻塞上线（4个待修复）

| ID | 模块 | 问题 | 工时 | 状态 |
|----|------|------|------|------|
| P0-3 | token-runtime | Refresh TTL 不持久化，仅修改内存未调用 store.Save() | 1h | ⚪ 待修 |
| P0-4 | token-runtime | 并发写 Map 非线程安全，Save 方法在 mutex 外写 map | 1h | ⚪ 待修 |
| P0-5 | token-runtime | `/v1/audit-events` 端点无鉴权可直接查询 | 1h | ⚪ 待修 |
| P0-1/2 | gateway | 硬编码密钥/宽松 CORS 仅在 bootstrap 中添加验证，未根除默认值 | 1h | ⚪ 待彻底修复 |

#### P1 强烈建议（6个待修复）

| ID | 模块 | 问题 | 工时 | 状态 |
|----|------|------|------|------|
| P1-1 | supply-api | KMS 使用 SHA-256(concat) 简单哈希派生，固定盐值 | 2h | ⚪ 待修 |
| P1-2 | supply-api | JWT 空 alg 时回退到 HS256，可能签名绕过 | 1h | ⚪ 待修 |
| P1-3 | supply-api | adapter 层测试覆盖率 **0%** | 4h | ⚪ 待修 |
| P1-4 | supply-api | repository 层覆盖率 **3.1%** | 8h | ⚪ 待修 |
| P1-5 | gateway | TrustedProxies 未设置，反向代理环境下始终用 RemoteAddr | 1h | ⚪ 待修 |
| P1-6 | gateway | 请求 ID 直接信任用户输入，日志注入风险 | 0.5h | ⚪ 待修 |
| P1-7 | gateway | 内部错误信息直接暴露给客户端 | 1h | ⚪ 待修 |

#### 真实环境验证确定性缺陷（6个）

| 模块 | 问题 |
|------|------|
| token-runtime | PostgreSQL 刷新/撤销路径存在缺陷 |
| supply-api | 幂等锁写入路径存在缺陷 |
| supply-api | 套餐创建 SQL 存在问题 |
| IAM | 初始化 DDL 存在问题 |
| IAM | DB-backed 查询空值扫描 |
| 全局 | audit_events 表结构与审计仓储实现不一致 |

### 1.3 架构和工程问题

1. **代码分散**：三个目录各自为政，规划、实现、部署不在同一代码库。
2. **前端缺失**：主项目无前端源码，A 目录有前端设计但未与后端对接。
3. **架构不一致**：三个服务的包结构、错误处理、日志规范、配置管理各有差异。
4. **测试薄弱**：adapter 0%、repository 3.1%、多个关键路径无覆盖。
5. **CI 缺失**：无持续集成门禁，缺陷发现和修复趁于被动。
6. **配置管理混乱**：各服务配置格式、加载方式不统一，敏感配置缺乏加密保护。

---

## 二、重构目标

### 2.1 总体目标

将分散在三个目录中的 Bridge 项目合并为一个**统一的、生产级的、前后端完整的** 单代码库。

### 2.2 分层目标

| 维度 | 目标 | 验收标准 |
|------|------|---------|
| 安全 | P0 + P1 完全清零 | Bandit 高危+中危为 0，安全测试通过 |
| 稳定性 | 核心路径无确定性缺陷 | 真实环境验证报告中所有确定性缺陷修复 |
| 可观测性 | 结构化日志 + 健康检查 + 指标 | 三套服务统一日志格式，/健康端点可用 |
| 测试 | 关键路径覆盖 | adapter → 80%、repository → 70%、domain → 70% |
| 架构 | 三服务统一风格 | 包结构、错误码、日志、配置一致 |
| 产品 | 前后端完整对接 | 运营后台 + 用户控制台可启动、可登录、可操作 |
| 部署 | 一键部署 | `docker compose up -d` 可启动全部服务 |

---

## 三、合并策略

### 3.1 代码库结构重组

```
bridge/                               # 新的统一代码库根
├── README.md
├── docker-compose.yml              # 从 A 目录合并，整合主项目配置
├── Makefile                        # 统一构建、测试、部署
├── .github/workflows/              # 新增 CI/CD
│   ├── ci.yml                      # lint / test / security / build
│   └── release.yml                 # 镜像构建与发布
├── docs/                           # 从 A 目录合并
│   ├── prd/                        # 产品规格
│   ├── architecture/               # 架构设计
│   └── ops/                        # 运维手册
├── web/                            # 从 A 目录合并
│   ├── apps/
│   │   ├── admin-console/          # 运营后台
│   │   └── user-console/           # 用户控制台
│   └── packages/
│       ├── ui/                     # 组件库
│       └── api-client/             # API 客户端
├── backend/                        # 主项目代码作为基线 + B 架构改进
│   ├── go.work                     # 统一 Go workspace
│   ├── shared/                     # 新增：三服务共享代码
│   │   ├── pkg/
│   │   │   ├── error/            # 统一错误码（参考 B 的 error设计）
│   │   │   ├── crypto/           # AES-256-GCM, bcrypt（参考 B 的 crypto实现）
│   │   │   ├── logging/          # 统一结构化日志
│   │   │   ├── config/           # 统一配置加载框架
│   │   │   └── middleware/       # 共享中间件
│   │   └── proto/                  # 内部通信协议（可选）
│   ├── gateway/                    # 原主项目 gateway
│   │   ├── cmd/
│   │   ├── internal/
│   │   └── go.mod
│   ├── supply-api/                 # 原主项目 supply-api
│   │   ├── cmd/
│   │   ├── internal/
│   │   └── go.mod
│   └── platform-token-runtime/     # 原主项目 token-runtime
│       ├── cmd/
│       ├── internal/
│       └── go.mod
├── sql/                            # 从主项目合并
│   └── postgresql/
└── deploy/                         # 从 A 目录合并
    ├── nginx/
    └── monitoring/
```

### 3.2 合并原则

| 来源 | 处理方式 | 说明 |
|------|---------|------|
| 主项目后端代码 | **作为基线保留** | 唯一能够真实启动、落库、通过部分测试的实现 |
| A 目录 docs/plans | **合并到 docs/** | 产品规格、架构设计、运维文档是现有资产，需与代码对齐 |
| A 目录 web/ | **合并到 web/** | 前端设计已完整，需与后端 API 对接 |
| A 目录 docker-compose.yml | **合并为根级** | 整合三套后端服务 + 前端 + DB + Redis + Nginx |
| B 目录 internal/ | **架构参考 + 部分合并** | B 的分层更干净（api→service→upstream→route），作为架构改进目标 |
| B 目录 crypto/ | **合并到 shared/pkg/crypto/** | B 的 AES-256-GCM 实现更完整，替换主项目中的弱加密 |
| B 目录 upstream/ | **参考并部分合并** | B 的上游客户端有更好的测试覆盖 |

---

## 四、分阶段重构路线图

### 阶段一：安全清零与基线修复（第 1-2 周）

**目标**: P0 + P1 完全清零，真实环境验证的 6 个确定性缺陷修复。

| 任务 | 模块 | 工时 | 验收 |
|------|------|------|------|
| S1-T1 | token-runtime: Refresh 持久化 | 2h | 单元测试 + 真实数据库验证 |
| S1-T2 | token-runtime: 并发安全修复 | 2h | 并发测试通过 |
| S1-T3 | token-runtime: audit-events 鉴权 | 2h | 未鉴权请求返回 401 |
| S1-T4 | gateway: 硬编码密钥根除 | 4h | 生产环境缺少配置时服务拒绝启动 |
| S1-T5 | gateway: CORS 根除任意来源 | 4h | 生产环境 `*` 时拒绝启动 |
| S1-T6 | supply-api: KMS 升级 HKDF | 4h | 密钥派生算法更新，旧数据兼容 |
| S1-T7 | supply-api: JWT 算法回退禁用 | 2h | 空 alg 时拒绝验证 |
| S1-T8 | gateway: TrustedProxies 配置 | 2h | XFF 可配置，非代理环境默认不信任 |
| S1-T9 | gateway: 请求 ID 校验/重生 | 2h | 用户输入过长或非法字符时重生 |
| S1-T10 | gateway: 错误信息脱敏 | 4h | 内部错误不暴露给客户端 |
| S1-T11 | 全局: audit_events schema 一致性 | 4h | DDL、代码、文档三者一致 |
| S1-T12 | IAM: 初始化 DDL 修复 | 4h | 数据库迁移可执行 |
| S1-T13 | 幂等锁 + 套餚 SQL 修复 | 4h | 真实数据库验证通过 |

**里程碑**: CI 新增 `go test ./...` + `go vet ./...` + 安全扫描，全绿通过。

### 阶段二：代码合并与架构统一（第 3-4 周）

**目标**: 完成三个目录的物理合并，建立统一的工程基座。

| 任务 | 说明 | 工时 |
|------|------|------|
| S2-T1 | 创建统一代码库 `bridge/`，初始化 `go.work` | 4h |
| S2-T2 | 将主项目三服务移入 `backend/` | 4h |
| S2-T3 | 将 A 目录 `docs/` 、`web/` 移入根目录 | 4h |
| S2-T4 | 新建 `backend/shared/` 共享包，移入统一 error、crypto、logging | 8h |
| S2-T5 | 以 B 目录架构为参考，重构 gateway 的 adapter/service 分层 | 16h |
| S2-T6 | 统一三服务的配置加载方式（采用 Viper 或 koanf） | 8h |
| S2-T7 | 统一错误码规范（`{SOURCE}_{CATEGORY}_{CODE}`） | 8h |
| S2-T8 | 统一日志格式（结构化 JSON） | 8h |
| S2-T9 | 整合 docker-compose.yml（DB + Redis + 三后端 + Nginx） | 8h |

**里程碑**: `docker compose up -d` 可启动全部后端服务 + 数据库 + Redis，健康检查通过。

### 阶段三：测试补强与质量门禁（第 5-6 周）

**目标**: 关键路径测试覆盖达标，CI 全线通过。

| 任务 | 说明 | 工时 | 验收 |
|------|------|------|------|
| S3-T1 | supply-api adapter 层 mock 测试 | 16h | 覆盖率 → 80% |
| S3-T2 | supply-api repository 层 sqlmock 测试 | 24h | 覆盖率 → 70% |
| S3-T3 | gateway adapter 层测试 | 16h | 覆盖率 → 70% |
| S3-T4 | gateway handler 层测试 | 16h | 覆盖率 → 75% |
| S3-T5 | token-runtime 存储层测试 | 12h | 覆盖率 → 70% |
| S3-T6 | e2e 测试补强（订单流程、幂等、审计） | 16h | 关键业务流程通过 |
| S3-T7 | CI/CD 搭建（GitHub Actions） | 8h | PR 合并前必须绿通 |
| S3-T8 | 安全扫描自动化（Bandit / gosec / trivy） | 8h | 高危+中危为 0 |

**里程碑**: CI 绿通率 100%，代码覆盖率门禁：合并前 adapter ≥ 70%、repository ≥ 60%、domain ≥ 60%。

### 阶段四：前端对接与产品完整性（第 7-8 周）

**目标**: 前后端完整对接，运营后台和用户控制台可用。

| 任务 | 说明 | 工时 |
|------|------|------|
| S4-T1 | 完善 web/apps/admin-console/运营后台 | 40h |
| S4-T2 | 完善 web/apps/user-console/用户控制台 | 40h |
| S4-T3 | API 客户端封装（packages/api-client） | 16h |
| S4-T4 | 前后端联调：认证、套餚、订单、审计 | 24h |
| S4-T5 | Nginx 反向代理配置（前端 + API 路由） | 8h |

**里程碑**: `docker compose up -d` 启动后，可通过浏览器访问运营后台和用户控制台，完成一条完整业务流程。

### 阶段五：性能优化与生产准备（第 9-10 周）

**目标**: 生产环境可部署，性能基准建立。

| 任务 | 说明 | 工时 |
|------|------|------|
| S5-T1 | 数据库连接池优化（pgx 参数调优） | 8h |
| S5-T2 | Redis 缓存策略实施 | 16h |
| S5-T3 | 压力测试（k6 戓 Vegeta） | 16h |
| S5-T4 | 监控与告警（Prometheus + Grafana） | 16h |
| S5-T5 | 日志聚合（Loki 戓 ELK） | 16h |
| S5-T6 | 安全响应头（X-Content-Type-Options 等） | 4h |
| S5-T7 | 生产部署文档与检查清单 | 8h |

**里程碑**: 通过生产环境部署演练，支撑 100 QPS 以上。

---

## 五、漂移目录清理

重构完成后，漂移目录应被清理以避免未来混淆：

```bash
# 重构完成后执行
rm -rf /home/long/hermes-agent/bridge/
rm -rf /home/long/hermes-agent-official/bridge/

# 如需保留历史，则移动到归档目录
mv /home/long/hermes-agent/bridge /home/long/archives/bridge-plan-2026-04-24
mv /home/long/hermes-agent-official/bridge /home/long/archives/bridge-blueprint-2026-04-26
```

---

## 六、风险与回退策略

| 风险 | 影响 | 回退策略 |
|------|------|---------|
| 代码合并引入回归 | 主链路故障 | 每个合并 PR 单独评审，保持原仓库 tag 可回滚 |
| 前端开发延期 | 整体进度拖后 | 阶段四可与阶段三并行，先保证 API 稳定 |
| 安全修复突破兼容性 | 旧数据无法使用 | KMS 升级时实施双向兼容，逐步迁移 |
| 测试补齐耗时 | 进度超预期 | 采用渐进式覆盖，先保证核心路径 80% |
| 团队人手不足 | 无法按期完成 | 优先完成阶段一和阶段二，阶段三五可分批外包 |

---

## 七、验收标准汇总

| 检查项 | 通过标准 |
|--------|---------|
| 安全扫描 | `gosec -fmt sarif ./...` 高危+中危 = 0 |
| 单元测试 | `go test ./...` 全绿 |
| 覆盖率 | adapter ≥ 70%、repository ≥ 60%、domain ≥ 60% |
| 真实环境 | `docker compose up -d` 启动后三套服务健康检查通过 |
| 前端对接 | 可通过浏览器完成登录、订单、查询三个核心流程 |
| 性能基准 | 100 QPS 下 P99 < 500ms |
| 文档完整 | README 、API 文档 、部署文档 与代码一致 |

---

## 八、立即执行的下一步

1. 创建统一代码库 `bridge/` 并初始化 `go.work`
2. 封装现有三个目录（主项目、A、B）为只读，确保基线可回滚
3. 开启阶段一：按 S1-T1~S1-T13 顺序修复 P0/P1 缺陷
4. 每日 standup 跟踪安全清零进度

**小龙，请确认：**
- 是否立即启动阶段一（安全清零）？
- 是否需要我先深入分析 B 目录的架构差异，输出具体的代码合并对照表？
- 是否需要先创建统一代码库并完成物理合并？
