# ai-customer-service 整改版审查报告 v2

**角色框架：小龙 / PM / TechLead / QA / DevOps**  
**审查目标：从“可跑通”提升到“生产可控、可灰度、可追责、不可默默降级”**

---

## 0. 阶段门控结论

**当前结论：REQUEST_CHANGES**  
**是否可直接按“生产已具备上线条件”放行：否**

### 当前真实状态
- **代码主链**：已基本打通
- **关键测试**：已实跑通过
- **生产落地控制**：仍有明显缺口
- **团队流程一致性**：存在文档漂移与门禁失真

### 本轮阻塞上线的核心原因
1. **默认允许 memory store 启动，存在生产级降级失控**
2. **readiness 不能证明“生产依赖已就绪”**
3. **QA / 上线文档与真实代码状态存在漂移**
4. **环境变量与部署文档口径不一致，存在实施误配风险**
5. **后台操作鉴权与运维级控制尚未形成完整闭环**

---

## 1. 本次复核依据与已验证证据

### 1.1 已实际读取的关键实现
重点核查：
- `internal/app/app.go`
- `internal/config/config.go`
- `internal/http/router.go`
- `internal/http/handlers/webhook_handler.go`
- `internal/http/handlers/webhook_security.go`
- `internal/http/handlers/health_handler.go`
- `internal/http/handlers/session_handler.go`
- `internal/http/handlers/ticket_handler.go`
- `internal/service/dialog/service.go`
- `internal/service/handoff/service.go`
- `internal/store/postgres/*`
- `internal/store/memory/*`
- `db/migration/0001_init.up.sql`

文档对照：
- `prd/PRODUCTION_CHECKLIST.md`
- `test/QA_GATE_STATUS.md`

### 1.2 已实际执行的验证
已通过 ASCII symlink 规避中文路径工具限制后执行：

```bash
go test -count=1 ./...
go test -count=1 ./test/e2e -run 'TestFullTicketFlow_E2E|TestSecurity_.*' -v
go test -count=1 ./test/integration -run 'TestHealthCheck_.*|TestDialogService_.*|TestTicketAssignResolve.*|TestSessionHandler.*' -v
```

### 实测结论
- 全仓测试通过
- 工单 E2E 主链通过
- Webhook 安全 E2E 通过
- Health/readiness 集成测试通过
- Session / Dialog / Ticket 关键集成测试通过

---

## 2. 审查后的总判断

### 2.1 已落实部分
#### A. 功能主链已存在
- webhook 收消息
- dialog 识别意图
- handoff 创建 ticket
- ticket assign / resolve / close / get
- stats / feedback / manual handoff 基本能力已落地

#### B. 基础安全入口已存在
- webhook HMAC
- timestamp 窗口校验
- dedup 幂等
- body limit
- rate limit

#### C. Postgres 持久化路径已接通
- `AI_CS_POSTGRES_ENABLED=true` 时走 postgres store
- migration 启动执行
- session / ticket / audit / dedup 均有 PG 实现

### 2.2 当前不能判定“生产可放心上线”的核心原因
#### A. 降级失控
**生产默认可退化为 memory store，且服务仍能成功启动并 ready。**

#### B. readiness 语义过宽
**只能证明“进程能收请求”，不能证明“生产依赖与安全前置条件已满足”。**

#### C. 文档与代码状态漂移
**团队已有“测试失败/允许上线”等结论与当前代码真实状态不一致。**

#### D. 部署契约未收敛
**文档写的环境变量名与代码真实读取项不完全一致。**

#### E. 后台操作的真实性边界不足
**ticket / session 相关后台接口仍偏内网占位实现，缺少真正的操作鉴权闭环。**

---

## 3. 角色化整改方案

### 3.1 小龙（CEO / 统筹者）整改责任

#### 核心问题
当前团队最大问题不是“没人干活”，而是：
- 修了代码但没同步门禁文档
- 跑通了链路但仍以“允许上线”模糊放行
- 角色产出没有被强制做事实校准

#### 小龙必须承担的整改动作

##### XL-P0-1：建立“代码事实高于报告”的门禁
今后任何“已完成 / 可上线 / 已通过”的结论，必须满足：
1. 有实际文件证据
2. 有实际命令输出
3. 有当前版本时间点的校准
4. 有至少一次小龙抽样自验

##### XL-P0-2：重写阶段状态口径
把当前项目阶段结论统一收敛成三层：
- **代码主链状态**
- **预生产验证状态**
- **生产上线状态**

禁止再用单句“允许上线”覆盖全部层次。

##### XL-P1-1：强制角色交付模板
后续 PM / TechLead / QA / DevOps 输出必须固定带：
- 结论
- 证据
- 阻塞项
- 下一阶段条件
- 责任人
- 时间要求

##### XL-P1-2：增加“实施漂移复核点”
每次关键修复后，小龙必须做 3 件事：
1. 复跑最小必要测试
2. 复核关键配置契约
3. 更新门禁文档状态

#### 小龙验收标准
- [ ] 所有“完成/通过”结论都有命令或文件证据
- [ ] 文档状态与当前代码状态一致
- [ ] 不再使用“允许上线”作为模糊总括结论
- [ ] 每个整改项都有明确责任角色和验收人

### 3.2 PM（产品经理）整改责任

#### 当前 PM 问题
不是没有文档，而是**文档覆盖面和交付口径不够硬**：
- 上线检查项有，但和代码契约未完全对齐
- 对“Phase1 可上线”表述偏乐观
- 对“dev fallback 与 prod 要求差异”没有明确写成产品/交付边界
- 对“上线前必须真实环境验证”的门槛定义不够强

#### PM 必须承担的整改动作

##### PM-P0-1：修正文档中的上线口径
将现有文档中的“允许上线”改成分层表述：
- 代码级门禁通过
- 仓库内测试门禁通过
- 真实环境门禁未闭环
- 仅允许进入预生产/灰度准备

##### PM-P0-2：补“运行模式约束”
在 PRD / checklist / status 中明确写入：
- `memory mode` 仅用于开发 / 测试
- `prod` 环境不允许无持久化运行
- 若缺少 DB / secret / 关键依赖，系统应 fail-fast，不得 silent degrade

##### PM-P1-1：补齐“上线运营口径与观察指标”
新增明确观察项：
- 工单创建量是否异常偏低/偏高
- handoff 比率是否异常
- audit 写入是否持续
- dedup 是否稳定
- readiness 是否真实反映依赖状态
- 实例重启后数据是否仍在

##### PM-P1-2：统一环境变量文档契约
所有面向部署的文档，必须统一写成代码真实读取的变量名，例如：
- `AI_CS_POSTGRES_ENABLED`
- `AI_CS_POSTGRES_DSN`
- `AI_CS_POSTGRES_MIGRATION_DIR`
- `AI_CS_WEBHOOK_SECRET`
- `AI_CS_WEBHOOK_MAX_SKEW_SECONDS`

禁止再用泛化口径替代真实配置契约。

#### PM 验收标准
- [ ] 文档中不再出现“仅凭仓库内测试即可认定生产可上线”的表述
- [ ] 文档中的环境变量名与 `config.go` 完全一致
- [ ] 明确区分 dev/test 与 prod 运行要求
- [ ] 上线观察指标、失败判定线、回滚触发条件都已写清

### 3.3 TechLead（技术经理）整改责任

#### 当前 TechLead 问题
TechLead 已把主链做起来，但**没有把“生产默认安全”做成系统约束**。

#### 必须整改的技术项

##### TL-P0-1：禁止生产默认退化到 memory store
目标：
- 生产模式下，不允许 `Postgres.Enabled=false` 仍正常启动

建议实现方向：
1. 增加运行模式，例如：
   - `AI_CS_RUNTIME_MODE=dev|test|prod`
2. 在 `prod` 模式下强制校验：
   - `AI_CS_POSTGRES_ENABLED=true`
   - `AI_CS_POSTGRES_DSN` 非空
   - migration dir 可用
3. 不满足则 `app.New()` 直接返回错误

**验收标准**
- [ ] prod 下未启用 Postgres 时服务启动失败
- [ ] 错误信息明确说明缺失项
- [ ] 有对应测试覆盖

##### TL-P0-2：收紧 readiness 语义
当前 `probe.SetReady(true)` 太早，必须改。

建议：
- 启动完成后不直接 ready
- ready 的条件至少包含：
  - DB 已连接
  - migration 已成功
  - 关键配置已完整
  - 运行模式合法
  - 如启用 webhook auth，则 secret 已配置

可选策略：
- `health` 保持诊断信息
- `ready` 专门作为流量门禁

**验收标准**
- [ ] 缺 DB / 缺 secret / 缺关键配置时 ready=DOWN
- [ ] ready 不再仅因为进程启动成功就返回 UP
- [ ] 有集成测试覆盖关键失败场景

##### TL-P0-3：统一配置契约与部署文档
TechLead 要输出一份**代码视角的配置契约基线**，作为 PM / DevOps / QA 的唯一来源。

至少包括：
- 变量名
- 默认值
- 是否允许默认值出现在 prod
- 是否阻断启动
- 对应组件
- 风险等级

示例字段：
- `AI_CS_POSTGRES_ENABLED`
- `AI_CS_POSTGRES_DSN`
- `AI_CS_POSTGRES_MIGRATION_DIR`
- `AI_CS_WEBHOOK_SECRET`
- `AI_CS_MAX_BODY_BYTES`

**验收标准**
- [ ] 有单独配置契约表
- [ ] 与 `config.go` 实际实现一致
- [ ] 明确哪些默认值仅限 dev/test

##### TL-P1-1：补后台接口鉴权设计
当前：
- `actor_id` 主要来自 query param
- 更接近内部占位实现，而不是正式后台控制面接口

需明确：
- 是仅内网可调
- 还是后台服务调用
- 还是运营台使用
- 对应认证方式是什么

至少补设计：
- 内部 token / service auth / gateway auth
- 操作审计字段真实性
- actor 来源不可伪造

**验收标准**
- [ ] ticket/session 后台接口有明确 auth 模式
- [ ] actor_id 不再只是前端随便传
- [ ] 权限边界写入设计文档

##### TL-P1-2：补多实例与恢复场景验证设计
需要明确验证：
- dedup 在多实例下是否稳定
- ticket / session / audit 在重启后是否一致
- migration 重复执行是否幂等
- 故障恢复后 ready 恢复逻辑是否正确

### 3.4 QA（质量经理）整改责任

#### 当前 QA 问题
QA 不是没工作，而是**结论闭环不够硬**：
- 文档中存在过时结论
- “允许上线”没有严格区分代码门禁与生产门禁
- 没把“memory fallback 风险”上升为真正阻断项

#### QA 必须承担的整改动作

##### QA-P0-1：重做上线门禁文档
重写 `QA_GATE_STATUS`，按以下结构：
1. 当前代码事实
2. 实测命令
3. 通过项
4. 未通过项
5. 文档漂移项
6. 生产阻断项
7. 下一阶段建议结论

必须明确区分：
- **仓库内验证通过**
- **真实环境未验证**
- **生产阻断未解除**

##### QA-P0-2：把“降级失控”列为 Critical
以下情形必须判定为 Critical：
- prod 可在 memory mode 启动
- ready 不能区分关键依赖缺失
- 部署文档与配置契约不一致
- 文档已允许上线，但真实环境门禁未验证

##### QA-P1-1：建立“文档漂移检测”检查项
今后每次 QA 审查必须加一栏：
- 代码状态 vs status 文档是否一致
- 测试状态 vs 报告状态是否一致
- 配置项 vs checklist 是否一致

##### QA-P1-2：增加真实环境前置门禁
上线前 QA 必须强制检查：
- 使用真实环境变量启动一次
- ready / health 返回符合预期
- Postgres migration 执行成功
- webhook 签名真实联调成功
- audit / ticket 实际入库成功

#### QA 验收标准
- [ ] QA 报告明确区分代码门禁 / 生产门禁
- [ ] 文档漂移项被单独列出
- [ ] memory fallback 风险被列为 Critical 直到修复
- [ ] 不再用“允许上线”掩盖真实环境未验证

### 3.5 DevOps（运维 / SRE）整改责任

#### 当前 DevOps 问题
仓库中有上线清单，但还不是实际运维闭环。

#### DevOps 必须承担的整改动作

##### DO-P0-1：形成真实部署基线
需要明确：
- 启动命令
- 必填环境变量
- secret 注入方式
- Postgres 连通性检查
- migration 执行方式
- readiness / liveness 探针路径
- 灰度方式
- 回滚方式

##### DO-P0-2：把“关键配置缺失即启动失败”纳入部署标准
即使代码修完，部署侧也要加保护：
- 若 prod 缺少 `AI_CS_POSTGRES_DSN` / `AI_CS_WEBHOOK_SECRET`
- CI/CD 或启动脚本应直接 fail

##### DO-P1-1：补监控与告警
最少补这些：
- 5xx rate
- webhook reject rate
- handoff rate
- ticket create rate
- audit write error
- DB connect / migration error
- ready down duration

##### DO-P1-2：补 runbook
必须有：
- 启动失败排查
- migration 失败回滚
- DB 不可用处理
- webhook auth 失败联调
- 实例重启后数据一致性检查

#### DevOps 验收标准
- [ ] 有真实部署基线文档
- [ ] prod 关键配置缺失时不会“假成功启动”
- [ ] 有最小监控告警集
- [ ] 有回滚与故障 runbook

---

## 4. P0 / P1 / P2 总整治清单

### P0：上线前必须完成
1. **禁止 prod 退化为 memory mode**
2. **收紧 readiness，改成真实依赖门禁**
3. **修正文档：状态、测试、环境变量口径统一**
4. **QA 重做门禁结论，撤销过宽“允许上线”表述**
5. **建立部署侧关键配置 fail-fast 机制**

### P1：灰度前应完成
1. 后台操作接口鉴权边界明确
2. 真实环境 DB / migration / webhook 联调
3. 监控告警最小闭环
4. 文档漂移检测纳入 QA 常规项
5. runbook 与回滚路径补齐

### P2：全量上线后持续补
1. 更完整威胁建模
2. 多实例一致性与恢复测试
3. store/app 层覆盖率继续补齐
4. 敏感字段脱敏、审计治理、保留策略完善
5. 更细粒度容量与可观测性建设

---

## 5. 整改后阶段门禁定义

### Gate A：代码级通过
满足：
- 主链测试通过
- 安全测试通过
- prod 不允许 memory fallback
- readiness 逻辑收紧
- 配置契约与代码一致

### Gate B：预生产通过
满足：
- 真实 Postgres 联通
- migration 成功
- webhook 签名联调成功
- audit / ticket 入库成功
- ready / live 行为符合预期
- 最小监控已接通

### Gate C：生产灰度通过
满足：
- 5% 灰度稳定
- handoff / ticket / audit 指标正常
- 无异常 5xx / reject 激增
- 回滚演练已通过

---

## 6. 最终整改版结论

**ai-customer-service 当前应被定义为：**  
**“代码主链可用，适合进入生产整改与预生产验证阶段；但尚不应被标记为生产可直接放心上线。”**

更准确地说：
- **不是没做成**
- **也不是 demo 空壳**
- **但现在离生产级放心放量，还差最后一层关键控制：禁止隐式降级、收紧 readiness、统一配置契约、修正文档漂移、补部署门禁。**
