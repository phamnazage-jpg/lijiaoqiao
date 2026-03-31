# 依赖版本兼容性审计基线（v1.0）

- 版本：v1.0
- 日期：2026-03-27
- 状态：生效（发布前强制 Gate）
- 目标：把“依赖可用”升级为“依赖可审计、可回滚、可阻断”
- 关联文档：
  - `docs/technical_architecture_optimized_v2_2026-03-18.md`
  - `docs/acceptance_gate_single_source_v1_2026-03-18.md`
  - `docs/subapi_integration_risk_controls_execution_tasks_v1_2026-03-17.md`

---

## 1. 审计对象与冻结策略

| 层 | 对象 | 冻结规则 |
|---|---|---|
| Runtime | Go / Node / JDK / Python | 仅允许 LTS 或已验证小版本 |
| Data | PostgreSQL / Redis | 生产固定主版本，升级必须灰度 |
| 服务依赖 | subapi / provider SDK | 固定精确版本（`X.Y.Z`） |
| 第三方库 | go mod / npm / maven | 锁文件变更必须触发兼容测试 |
| OS 镜像 | 基础镜像 digest | 必须可追溯到 SBOM |

---

## 2. 必交付证据

每次发布候选版本必须提供：

1. `SBOM`：`reports/dependency/sbom_<date>.spdx.json`
2. `锁文件差异`：`reports/dependency/lockfile_diff_<date>.md`
3. `兼容矩阵`：`reports/dependency/compat_matrix_<date>.md`
4. `风险清单`：`reports/dependency/risk_register_<date>.md`

无上述四项，发布门禁直接阻断。

---

## 3. 兼容性审计流程（分阶段）

### 3.1 Pre-Merge（开发合并前）

1. 检查 `go.mod/go.sum`、`package-lock.json/pnpm-lock.yaml`、`pom.xml` 变化。
2. 依赖变更自动分类：Patch/Minor/Major。
3. Major 变更必须附“兼容影响评估 + 回滚预案”。

### 3.2 Nightly（每日）

1. 运行依赖漏洞扫描（CVE/SCA）。
2. 运行契约回归（Schema/Behavior）。
3. 生成依赖健康趋势（新增高危漏洞数）。

### 3.3 Pre-Release（发布前）

1. 运行完整兼容回归（兼容三重 Gate + SUP Gate）。
2. 校验运行时与数据层版本匹配矩阵。
3. 通过后冻结候选构建包与镜像 digest。

### 3.4 Post-Release（发布后 24h）

1. 监控新增依赖告警、崩溃、性能回退。
2. 若触发 P0/P1 依赖事故，执行自动回滚到上一稳定版本。

---

## 4. 阻断规则（必须）

1. `dependency_compat_audit_pass_pct < 100%`：阻断发布。
2. 新增 Critical CVE 且无缓解：阻断发布。
3. Major 依赖变更无回滚演练记录：阻断发布。
4. subapi/provider SDK 精确版本未锁定：阻断发布。
5. 依赖清单与运行镜像不一致：阻断发布。

---

## 5. 推荐版本兼容矩阵（首版）

| 组件 | 基线版本 | 兼容范围 | 备注 |
|---|---|---|---|
| Go | 1.21.x | 1.21.x | 不跨主版本 |
| PostgreSQL | 15.x | 15.x | SQL 与索引以 PG15 语法为准 |
| Redis | 7.x | 7.x | 限流与缓存行为基于 Redis7 验证 |
| subapi | 精确 `X.Y.Z` | 同 patch | Minor 升级需完整回归 |
| Node（前端） | 20.x LTS | 20.x | 锁文件必须纳入审计 |

---

## 6. 与发布门禁对齐

1. 依赖兼容审计结果接入 `acceptance_gate_single_source` 指标 `M-017`。
2. 分阶段测试质量接入指标 `M-018`。
3. 任一未达标，不得进入 `GO` 结论。

