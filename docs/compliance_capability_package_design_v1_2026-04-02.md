# P2 合规能力包详细设计

> 本文档为 P2 阶段合规能力包的增强设计，基于 `tos_compliance_engine_design_v1_2026-03-18.md` 的 S4 合规引擎架构，扩展以满足 M-013~M-017 指标的自动化合规检查与报告需求。

---

## 1. 概述与背景

### 1.1 目的

P2 合规能力包旨在扩展现有 ToS 合规引擎的能力，实现：

1. **合规规则库扩展**：支持 M-013~M-016 指标的规则化定义与执行
2. **自动化合规检查**：将合规检查嵌入 CI/CD 流水线，实时检测违规事件
3. **合规报告生成**：自动生成符合 M-017 要求的依赖兼容审计四件套报告

### 1.2 指标映射

| 指标ID | 指标名称 | 目标值 | 阻断阈值 | P2 能力要求 |
|--------|----------|--------|----------|-------------|
| M-013 | supplier_credential_exposure_events | 0 | >0 即 P0 | 凭证泄露检测规则 + 实时告警 |
| M-014 | platform_credential_ingress_coverage_pct | 100% | <100% 即阻断 | 入站凭证校验 + 覆盖率统计 |
| M-015 | direct_supplier_call_by_consumer_events | 0 | >0 即 P0 | 直连检测规则 + 阻断机制 |
| M-016 | query_key_external_reject_rate_pct | 100% | <100% 即阻断 | 外部 query key 拒绝规则 |
| M-017 | dependency_compatibility_audit | PASS | FAIL 即阻断 | SBOM + 锁文件 diff + 兼容矩阵 + 风险登记册 |

### 1.3 与现有设计的关系

```
tos_compliance_engine_design_v1_2026-03-18.md (S4 设计)
           │
           ▼
┌─────────────────────────────────────────────┐
│         P2 合规能力包扩展                     │
├─────────────────────────────────────────────┤
│  1. 合规规则库扩展（M-013~M-016 指标规则化）    │
│  2. 自动化合规检查（CI 流水线集成）             │
│  3. 合规报告生成（M-017 四件套）               │
└─────────────────────────────────────────────┘
```

---

## 2. 合规规则库扩展

### 2.1 M-013 凭证泄露检测规则

#### 2.1.1 规则定义

> **重要**：所有事件命名遵循 `audit_log_enhancement_design_v1_2026-04-02.md` 规范，格式为 `{Category}-{SubCategory}[-{Detail}]`，以确保与审计日志系统兼容。

| 规则ID | 事件名称 | 匹配条件 | 动作 | 严重级别 |
|--------|----------|----------|------|----------|
| R01 | CRED-EXPOSE-RESPONSE | 响应包含 `sk-`、`ak-`、`api_key` 等可复用凭证片段 | block + alert | P0 |
| R02 | CRED-EXPOSE-LOG | 日志输出包含完整凭证格式 | block + alert | P0 |
| R03 | CRED-EXPOSE-EXPORT | 导出功能返回可还原凭证 | block + alert | P0 |
| R04 | CRED-EXPOSE-WEBHOOK | 回调请求携带供应商凭证 | block + alert | P0 |

> **注**：原 `C013-R01~R04` 格式已废弃，统一使用 `CRED-EXPOSE-*` 格式与审计日志对齐。

#### 2.1.2 规则配置示例

```yaml
# compliance/rules/m013_credential_exposure.yaml
rules:
  - id: "CRED-EXPOSE-RESPONSE"
    name: "响应体凭证泄露检测"
    description: "检测 API 响应中是否包含可复用的供应商凭证片段"
    severity: "P0"
    matchers:
      - type: "regex_match"
        pattern: "(sk-|ak-|api_key|secret|token).*[a-zA-Z0-9]{20,}"
        target: "response_body"
        scope: "all"
    action:
      primary: "block"
      secondary: "alert"
    notification:
      channels: ["slack", "email"]
      template: "credential_exposure_alert"
    audit:
      log_level: "critical"
      retention_days: 1825  # 5年
      # 审计日志事件名称（与 audit_log_enhancement_design_v1_2026-04-02.md 对齐）
      event_name: "CRED-EXPOSE-RESPONSE"
      event_category: "CRED"
      event_sub_category: "EXPOSE"
```

#### 2.1.3 检测算法

```
凭证泄露检测算法 (CRED-EXPOSE-D01)

输入: HTTP 响应体内容
输出: 泄露检测结果 {is_leaked: bool, matches: []Match}

步骤:
1. 预编译凭证正则模式库
2. 对响应体进行多模式并行匹配
3. 过滤误报 (测试数据、示例数据)
4. 若匹配, 提取匹配片段并脱敏后记录审计日志
   - 审计事件名称: CRED-EXPOSE-RESPONSE
   - 事件分类: CRED
   - 事件子分类: EXPOSE
5. 触发阻断或告警流程
```

### 2.2 M-014 入站凭证覆盖率规则

#### 2.2.1 规则定义

| 规则ID | 事件名称 | 匹配条件 | 动作 | 严重级别 |
|--------|----------|----------|------|----------|
| R01 | CRED-INGRESS-PLATFORM | 请求头不包含 `Authorization: Bearer ptk_*` | block + alert | P0 |
| R02 | CRED-INGRESS-FORMAT | 平台凭证格式不符合规范 | block + alert | P1 |
| R03 | CRED-INGRESS-EXPIRED | 平台凭证已过期或被吊销 | block | P0 |

> **注**：原 `C014-R01~R03` 格式已废弃，统一使用 `CRED-INGRESS-*` 格式与审计日志对齐。

#### 2.2.2 覆盖率统计

```yaml
# compliance/rules/m014_ingress_coverage.yaml
coverage_tracking:
  metric: "platform_credential_ingress_coverage_pct"
  calculation: "(使用有效平台凭证的请求数 / 总请求数) * 100"
  target: 100
  blocking_threshold: 100
  window: "rolling_1h"
  aggregation: "percentile"
```

### 2.3 M-015 直连检测规则

#### 2.3.1 规则定义

| 规则ID | 事件名称 | 匹配条件 | 动作 | 严重级别 |
|--------|----------|----------|------|----------|
| R01 | CRED-DIRECT-SUPPLIER | 请求目标为已知供应商 IP/域名 | block + alert | P0 |
| R02 | CRED-DIRECT-API | 请求路径匹配 `*/v1/chat/completions` 等上游端点 | block | P0 |
| R03 | CRED-DIRECT-UNAUTH | 调用未经审批的供应商 | block + alert | P0 |

> **注**：原 `C015-R01~R03` 格式已废弃，统一使用 `CRED-DIRECT-*` 格式与审计日志对齐。

#### 2.3.2 检测方法

M-015 直连检测通过以下多层检测机制实现：

| 检测方法 | 描述 | 检测点 |
|----------|------|--------|
| **蜜罐检测** | 在 API Gateway 层部署蜜罐端点，检测是否有直接访问上游 API 的请求 | API Gateway |
| **网络流量分析** | 监控出站连接，识别绕过平台代理的直接连接 | 出网防火墙 |
| **API 日志分析** | 分析请求日志，检测异常的上游 API 调用模式 | 审计中间件 |
| **DNS 解析监控** | 监控 DNS 解析，检测是否有应用直接解析供应商域名 | 网络层 |
| **代理层检测** | 检查请求是否经过平台代理层，未经过则标记为直连 | 负载均衡器 |

> **检测流程**：蜜罐触发 -> 网络流量分析 -> API 日志复核 -> 确认直连事件

#### 2.3.2 供应商白名单配置

```yaml
# compliance/config/allowed_suppliers.yaml
allowed_suppliers:
  direct_access:
    # 禁止直连，全部通过平台代理
    enabled: false

  approved_providers:
    - name: "openai"
      base_urls:
        - "api.openai.com"
        - "api.openai.azure.com"
      requires_approval: true

    - name: "anthropic"
      base_urls:
        - "api.anthropic.com"
      requires_approval: true

    - name: "minimax"
      base_urls:
        - "api.minimax.chat"
      requires_approval: false
```

### 2.4 M-016 外部 Query Key 拒绝规则

#### 2.4.1 规则定义

| 规则ID | 事件名称 | 匹配条件 | 动作 | 严重级别 |
|--------|----------|----------|------|----------|
| R01 | AUTH-QUERY-KEY | 来自外部的 query key 请求进入平台北向入口 | reject (403) | P0 |
| R02 | AUTH-QUERY-INJECT | 请求参数包含 `key=`、`api_key=`、`token=` 等外部 key | reject (403) | P0 |
| R03 | AUTH-QUERY-AUDIT | 内部处理 query key 时记录全量审计 | alert | P1 |

> **注**：原 `C016-R01~R03` 格式已废弃，统一使用 `AUTH-QUERY-*` 格式与审计日志对齐。

#### 2.4.2 拒绝模式配置

```yaml
# compliance/rules/m016_query_key_reject.yaml
query_key_rejection:
  enabled: true
  default_action: "reject"

  patterns:
    # 拒绝所有包含以下模式的外部请求
    reject_patterns:
      - "key=.*"
      - "api_key=.*"
      - "token=.*"
      - "bearer=.*"
      - "authorization=.*"

    # 允许内部白名单模式
    allow_patterns:
      - "^internal-.*"
      - "^platform-.*"

  response:
    status_code: 403
    message: "External query keys are not allowed"
    include_request_id: true
```

---

## 3. 自动化合规检查

### 3.1 架构设计

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        自动化合规检查系统                                     │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌────────────────┐    ┌────────────────┐    ┌────────────────┐            │
│  │  合规规则引擎   │───▶│  实时检测器    │───▶│  告警发送器    │            │
│  │  (Rule Engine) │    │ (Real-time)   │    │  (Notifier)    │            │
│  └────────────────┘    └────────────────┘    └────────────────┘            │
│           │                      │                      │                   │
│           ▼                      ▼                      ▼                   │
│  ┌─────────────────────────────────────────────────────────────────────┐  │
│  │                      合规指标存储层                                    │  │
│  │  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐            │  │
│  │  │ M-013    │ │ M-014    │ │ M-015    │ │ M-016    │            │  │
│  │  │ 泄露事件  │ │ 入站覆盖  │ │ 直连事件  │ │ 拒绝率    │            │  │
│  │  └───────────┘ └───────────┘ └───────────┘ └───────────┘            │  │
│  └─────────────────────────────────────────────────────────────────────┘  │
│                                    │                                        │
│                                    ▼                                        │
│  ┌─────────────────────────────────────────────────────────────────────┐  │
│  │                      CI/CD 流水线集成                                 │  │
│  │  ┌───────────┐ ┌───────────┐ ┌───────────┐ ┌───────────┐            │  │
│  │  │ Pre-Commit │ │  Build   │ │  Deploy  │ │  Monitor  │            │  │
│  │  └───────────┘ └───────────┘ └───────────┘ └───────────┘            │  │
│  └─────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 规则执行引擎

#### 3.2.1 核心组件

| 组件 | 职责 | 性能要求 |
|------|------|----------|
| **规则编译器** | 将 YAML 规则编译为可执行格式 | 启动时编译，不影响运行时 |
| **规则匹配器** | 根据请求上下文匹配适用规则 | P95 < 2ms |
| **策略执行器** | 执行 block/alert/reject 动作 | P95 < 1ms |
| **审计记录器** | 记录所有合规决策 | 异步，不阻塞主流程 |

#### 3.2.2 规则执行流程

```
规则执行流程 (CMP-FLOW-01)

1. 请求进入合规检查拦截点
         │
         ▼
2. 提取请求上下文
   - 请求头 (Authorization, X-Request-Id)
   - 请求路径
   - 请求参数
   - 源 IP
         │
         ▼
3. 并行匹配所有启用规则
         │
         ▼
4. 聚合匹配结果
   - 若存在 P0 匹配 → 立即阻断
   - 若存在 P1 匹配 → 告警 + 继续
   - 若仅 P2/P3 匹配 → 记录但不阻断
         │
         ▼
5. 执行动作
   - block: 返回错误响应
   - alert: 发送告警通知
   - reject: 返回 403
         │
         ▼
6. 记录审计日志
   - 规则 ID
   - 匹配结果
   - 执行动作
   - 时间戳
```

### 3.3 CI/CD 流水线集成

#### 3.3.1 集成点

| 阶段 | 检查项 | 阻断条件 | 超时时间 |
|------|--------|----------|----------|
| **Pre-Commit** | 本地凭证泄露扫描 | M-013 > 0 | 30s |
| **Build** | 依赖兼容审计 (M-017) | 四件套任一 FAIL | 120s |
| **Deploy-Staging** | M-013~M-016 实时检测 | 任一 P0 | N/A (实时) |
| **Deploy-Production** | M-013~M-016 实时检测 | 任一 P0 | N/A (实时) |
| **Monitor** | 7x24 指标监控 | 阈值突破 | N/A |

#### 3.3.2 CI 脚本集成

```bash
# compliance/ci/compliance_gate.sh

#!/bin/bash
# 合规门禁 CI 脚本

set -e

# 使用环境变量或相对路径，避免硬编码
COMPLIANCE_BASE="${COMPLIANCE_BASE:-$(cd "$(dirname "$0")/.." && pwd)}"
RULES_DIR="${COMPLIANCE_BASE}/rules"
REPORTS_DIR="${COMPLIANCE_BASE}/reports"

# M-013: 凭证泄露扫描
echo "[COMPLIANCE] Running M-013 credential exposure scan..."
if ! bash "${COMPLIANCE_BASE}/ci/m013_credential_scan.sh"; then
    echo "[COMPLIANCE] M-013 FAILED: Credential exposure detected"
    exit 1
fi

# M-014: 入站覆盖率检查
echo "[COMPLIANCE] Running M-014 ingress coverage check..."
if ! bash "${COMPLIANCE_BASE}/ci/m014_ingress_coverage.sh"; then
    echo "[COMPLIANCE] M-014 FAILED: Ingress coverage below 100%"
    exit 1
fi

# M-015: 直连检测
echo "[COMPLIANCE] Running M-015 direct access check..."
if ! bash "${COMPLIANCE_BASE}/ci/m015_direct_access_check.sh"; then
    echo "[COMPLIANCE] M-015 FAILED: Direct supplier access detected"
    exit 1
fi

# M-016: Query Key 拒绝率
echo "[COMPLIANCE] Running M-016 query key rejection check..."
if ! bash "${COMPLIANCE_BASE}/ci/m016_query_key_reject.sh"; then
    echo "[COMPLIANCE] M-016 FAILED: Query key rejection rate below 100%"
    exit 1
fi

# M-017: 依赖兼容审计
echo "[COMPLIANCE] Running M-017 dependency audit..."
if ! bash "${COMPLIANCE_BASE}/ci/m017_dependency_audit.sh"; then
    echo "[COMPLIANCE] M-017 FAILED: Dependency compatibility issue"
    exit 1
fi

echo "[COMPLIANCE] All checks PASSED"
```

> **注意**：以下 CI 脚本处于**待实现**状态，依赖于 `compliance/` 目录的创建：
> - `m013_credential_scan.sh` - 待实现
> - `m014_ingress_coverage.sh` - 待实现
> - `m015_direct_access_check.sh` - 待实现
> - `m016_query_key_reject.sh` - 待实现
> - `m017_dependency_audit.sh` - 待实现

### 3.4 实时监控

#### 3.4.1 监控指标

| 指标 | 描述 | 告警阈值 |
|------|------|----------|
| m013_exposure_events_total | 凭证泄露事件总数 | > 0 |
| m014_ingress_coverage_pct | 入站凭证覆盖率 | < 100 |
| m015_direct_access_events_total | 直连事件总数 | > 0 |
| m016_query_key_reject_rate_pct | query key 拒绝率 | < 100 |
| compliance_rules_triggered_total | 规则触发总数 | N/A |

#### 3.4.2 告警规则

```yaml
# compliance/monitoring/alerts.yaml
alerts:
  - name: "m013_credential_exposure_p0"
    condition: "m013_exposure_events_total > 0"
    severity: "P0"
    channels: ["slack_critical", "pagerduty", "email"]
    message: "P0: Credential exposure event detected"

  - name: "m014_ingress_coverage_degraded"
    condition: "m014_ingress_coverage_pct < 100"
    severity: "P0"
    channels: ["slack_critical", "pagerduty"]
    message: "P0: Platform credential ingress coverage below 100%"

  - name: "m015_direct_access_detected"
    condition: "m015_direct_access_events_total > 0"
    severity: "P0"
    channels: ["slack_critical", "pagerduty", "email"]
    message: "P0: Direct supplier access detected"

  - name: "m016_reject_rate_degraded"
    condition: "m016_query_key_reject_rate_pct < 100"
    severity: "P1"
    channels: ["slack", "email"]
    message: "P1: Query key rejection rate below 100%"
```

---

## 4. 合规报告生成

### 4.1 M-017 依赖兼容审计四件套

根据 `supply_gate_command_playbook_v1_2026-03-25.md` 第7章要求，M-017 需生成以下四件套：

| 报告 | 文件名模式 | 内容要求 |
|------|------------|----------|
| **SBOM** | `sbom_{date}.spdx.json` | 软件物料清单，SPDX 2.3 格式 |
| **锁文件 Diff** | `lockfile_diff_{date}.md` | 依赖版本变更对比 |
| **兼容矩阵** | `compat_matrix_{date}.md` | 组件版本兼容性矩阵 |
| **风险登记册** | `risk_register_{date}.md` | 发现的安全与合规风险 |

### 4.2 四件套生成流程

```
依赖兼容审计流程 (M017-FLOW-01)

1. 执行时间: 每日 00:00 UTC (CI Build 阶段自动触发)
         │
         ▼
2. SBOM 生成
   - 使用 syft/spdx-syft 生成项目 SPDX 2.3 SBOM
   - 覆盖语言: Go (go.mod), Node (package.json), Python (requirements.txt)
         │
         ▼
3. 锁文件 Diff 生成
   - 对比当前 lock 文件与 baseline
   - 提取新增/升级/降级/删除依赖
   - 变更影响评估
         │
         ▼
4. 兼容矩阵生成
   - 读取兼容矩阵模板
   - 填充当前版本信息
   - 标注已知不兼容项
         │
         ▼
5. 风险登记册生成
   - 汇总 CVSS >= 7.0 的漏洞
   - 汇总许可证合规风险
   - 汇总过期依赖风险
         │
         ▼
6. 报告输出
   - 生成日期标注的报告文件
   - 更新趋势数据库
   - 发送摘要邮件
```

### 4.3 四件套详细规格

#### 4.3.1 SBOM (软件物料清单)

```json
{
  "spdxVersion": "SPDX-2.3",
  "dataLicense": "CC0-1.0",
  "SPDXID": "SPDXRef-DOCUMENT",
  "name": "llm-gateway",
  "documentNamespace": "https://llm-gateway.example.com/spdx/2026-04-02",
  "creationInfo": {
    "created": "2026-04-02T00:00:00Z",
    "creators": ["Tool: syft-0.100.0"]
  },
  "packages": [
    {
      "SPDXID": "SPDXRef-Package-go-github-com-openai",
      "name": "github.com/openai/openai-go",
      "versionInfo": "v0.2.0",
      "supplier": "Organization: OpenAI",
      "downloadLocation": "https://github.com/openai/openai-go",
      "licenseConcluded": "Apache-2.0"
    }
  ]
}
```

#### 4.3.2 锁文件 Diff

```markdown
# Lockfile Diff Report - 2026-04-02

## Summary
| 变更类型 | 数量 |
|----------|------|
| 新增依赖 | 3 |
| 升级依赖 | 7 |
| 降级依赖 | 0 |
| 删除依赖 | 1 |

## New Dependencies
| 名称 | 版本 | 用途 | 风险评估 |
|------|------|------|----------|
| github.com/acme/newpkg | v1.2.0 | 新功能 | LOW |

## Upgraded Dependencies
| 名称 | 旧版本 | 新版本 | 风险评估 |
|------|--------|--------|----------|
| github.com/acme/existing | v1.0.0 | v1.1.0 | LOW |

## Deleted Dependencies
| 名称 | 旧版本 | 原因 |
|------|--------|------|
| github.com/acme/unused | v0.9.0 | 功能下线 |

## Breaking Changes
None detected.
```

#### 4.3.3 兼容矩阵

```markdown
# Dependency Compatibility Matrix - 2026-04-02

## Go Dependencies
| 组件 | 版本 | Go 1.21 | Go 1.22 | Go 1.23 |
|------|------|----------|----------|----------|
| github.com/acme/pkg | v1.2.0 | PASS | PASS | PASS |

## Known Incompatibilities
None detected.
```

#### 4.3.4 风险登记册

```markdown
# Risk Register - 2026-04-02

## Summary
| 风险级别 | 数量 |
|----------|------|
| CRITICAL | 0 |
| HIGH | 1 |
| MEDIUM | 2 |
| LOW | 5 |

## High Risk Items
| ID | 描述 | CVSS | 组件 | 修复建议 |
|----|------|------|------|----------|
| RISK-001 | CVE-2024-XXXXX | 8.1 | github.com/acme/vuln-pkg | 升级到 v1.3.0 |

## Medium Risk Items
| ID | 描述 | CVSS | 组件 | 修复建议 |
|----|------|------|------|----------|
| RISK-002 | License: GPL-3.0 conflict | N/A | github.com/acme/gpl-pkg | 评估许可证合规 |

## Mitigation Status
| ID | 状态 | 负责人 | 截止日期 |
|----|------|--------|----------|
| RISK-001 | IN_PROGRESS | @security | 2026-04-05 |
```

### 4.4 自动化报告生成脚本

```bash
#!/bin/bash
# compliance/reports/m017_dependency_audit.sh

#!/usr/bin/env bash
set -e

REPORT_DATE="${1:-$(date +%Y-%m-%d)}"
# 使用环境变量或相对路径，避免硬编码
REPORT_DIR="${COMPLIANCE_REPORT_DIR:-${PROJECT_ROOT}/reports/dependency}"
PROJECT_ROOT="${PROJECT_ROOT:-$(cd "$(dirname "$0")/../.." && pwd)}"

mkdir -p "${REPORT_DIR}"

echo "[M017] Starting dependency audit for ${REPORT_DATE}"

# 1. Generate SBOM
echo "[M017] Generating SBOM..."
if command -v syft >/dev/null 2>&1; then
    syft "${PROJECT_ROOT}" -o spdx-json > "${REPORT_DIR}/sbom_${REPORT_DATE}.spdx.json"
    # 验证 SBOM 包含有效包
    if ! grep -q '"packages"' "${REPORT_DIR}/sbom_${REPORT_DATE}.spdx.json" || \
       [ "$(grep -c '"SPDXRef' "${REPORT_DIR}/sbom_${REPORT_DATE}.spdx.json" || echo 0)" -eq 0 ]; then
        echo "[M017] FAIL: syft generated invalid SBOM (no packages found)"
        exit 1
    fi
    echo "[M017] SBOM generated successfully with syft"
else
    echo "[M017] ERROR: syft is required but not found. Please install syft first."
    echo "[M017] See: https://github.com/anchore/syft#installation"
    exit 1
fi

# 2. Generate Lockfile Diff
echo "[M017] Generating lockfile diff..."
LOCKFILE_DIFF_SCRIPT="${PROJECT_ROOT}/scripts/ci/lockfile_diff.sh"
if [ -x "$LOCKFILE_DIFF_SCRIPT" ]; then
    bash "$LOCKFILE_DIFF_SCRIPT" "${REPORT_DATE}" > "${REPORT_DIR}/lockfile_diff_${REPORT_DATE}.md"
else
    echo "[M017] ERROR: lockfile_diff.sh not found or not executable at $LOCKFILE_DIFF_SCRIPT"
    exit 1
fi

# 3. Generate Compatibility Matrix
echo "[M017] Generating compatibility matrix..."
COMPAT_MATRIX_SCRIPT="${PROJECT_ROOT}/scripts/ci/compat_matrix.sh"
if [ -x "$COMPAT_MATRIX_SCRIPT" ]; then
    bash "$COMPAT_MATRIX_SCRIPT" "${REPORT_DATE}" > "${REPORT_DIR}/compat_matrix_${REPORT_DATE}.md"
else
    echo "[M017] ERROR: compat_matrix.sh not found or not executable at $COMPAT_MATRIX_SCRIPT"
    exit 1
fi

# 4. Generate Risk Register
echo "[M017] Generating risk register..."
RISK_REGISTER_SCRIPT="${PROJECT_ROOT}/scripts/ci/risk_register.sh"
if [ -x "$RISK_REGISTER_SCRIPT" ]; then
    bash "$RISK_REGISTER_SCRIPT" "${REPORT_DATE}" > "${REPORT_DIR}/risk_register_${REPORT_DATE}.md"
else
    echo "[M017] ERROR: risk_register.sh not found or not executable at $RISK_REGISTER_SCRIPT"
    exit 1
fi

# 5. Validate all artifacts exist
echo "[M017] Validating artifacts..."
ARTIFACTS=(
    "sbom_${REPORT_DATE}.spdx.json"
    "lockfile_diff_${REPORT_DATE}.md"
    "compat_matrix_${REPORT_DATE}.md"
    "risk_register_${REPORT_DATE}.md"
)

ALL_PASS=true
for artifact in "${ARTIFACTS[@]}"; do
    if [ -f "${REPORT_DIR}/${artifact}" ] && [ -s "${REPORT_DIR}/${artifact}" ]; then
        echo "[M017] ${artifact}: OK"
    else
        echo "[M017] ${artifact}: MISSING OR EMPTY"
        ALL_PASS=false
    fi
done

# 6. Generate summary
if [ "$ALL_PASS" = true ]; then
    echo "[M017] PASS: All 4 artifacts generated successfully"
    exit 0
else
    echo "[M017] FAIL: One or more artifacts missing"
    exit 1
fi
```

### 4.5 四件套生成脚本详细设计

> **重要**：以下脚本均为**待实现**状态，需要在 P2-CMP-006 阶段完成开发。

#### 4.5.1 Lockfile Diff 生成脚本

```bash
#!/bin/bash
# scripts/ci/lockfile_diff.sh
# 功能：生成依赖版本变更对比报告
# 输入：REPORT_DATE (可选，默认为昨天)
# 输出：lockfile_diff_{date}.md

set -e

REPORT_DATE="${1:-$(date -d '1 day ago' +%Y-%m-%d)}"
PROJECT_ROOT="${PROJECT_ROOT:-$(cd "$(dirname "$0")/../.." && pwd)}"

echo "# Lockfile Diff Report - ${REPORT_DATE}"

# 获取当前 lockfile
LOCKFILE="${PROJECT_ROOT}/go.sum"
BASELINE_DIR="${PROJECT_ROOT}/.compliance/baseline"

# 对比逻辑
echo "## Summary"
echo "| 变更类型 | 数量 |"
echo "|----------|------|"
echo "| 新增依赖 | TBD |"
echo "| 升级依赖 | TBD |"
echo "| 降级依赖 | TBD |"
echo "| 删除依赖 | TBD |"

# 待实现：实际的对比逻辑
```

#### 4.5.2 兼容矩阵生成脚本

```bash
#!/bin/bash
# scripts/ci/compat_matrix.sh
# 功能：生成组件版本兼容性矩阵
# 输入：REPORT_DATE (可选)
# 输出：compat_matrix_{date}.md

set -e

REPORT_DATE="${1:-$(date -d '1 day ago' +%Y-%m-%d)}"
PROJECT_ROOT="${PROJECT_ROOT:-$(cd "$(dirname "$0")/../.." && pwd)}"

echo "# Dependency Compatibility Matrix - ${REPORT_DATE}"

# 读取 Go 版本信息
GO_VERSION=$(go version 2>/dev/null | grep -oP 'go\d+\.\d+' || echo "unknown")

echo "## Go Dependencies (${GO_VERSION})"
echo "| 组件 | 版本 | 兼容性 |"
echo "|------|------|--------|"
echo "| TBD | TBD | TBD |"

# 待实现：实际的兼容性检查逻辑
```

#### 4.5.3 风险登记册生成脚本

```bash
#!/bin/bash
# scripts/ci/risk_register.sh
# 功能：生成安全与合规风险登记册
# 输入：REPORT_DATE (可选)
# 输出：risk_register_{date}.md

set -e

REPORT_DATE="${1:-$(date -d '1 day ago' +%Y-%m-%d)}"
PROJECT_ROOT="${PROJECT_ROOT:-$(cd "$(dirname "$0")/../.." && pwd)}"

echo "# Risk Register - ${REPORT_DATE}"

echo "## Summary"
echo "| 风险级别 | 数量 |"
echo "|----------|------|"
echo "| CRITICAL | 0 |"
echo "| HIGH | 0 |"
echo "| MEDIUM | 0 |"
echo "| LOW | 0 |"

echo "## High Risk Items"
echo "| ID | 描述 | CVSS | 组件 | 修复建议 |"
echo "|----|------|------|------|----------|"
echo "| - | 无高风险项 | - | - | - |"

# 待实现：实际的漏洞扫描和风险评估逻辑
# 建议集成：grype (漏洞扫描)、license-check (许可证检查)
```

---

## 5. 与现有安全机制联动

### 5.1 联动矩阵

| 源机制 | 目标机制 | 联动方式 | 触发条件 |
|--------|----------|----------|----------|
| ToS 合规引擎 | 告警系统 | 事件推送 | 违规事件触发 |
| Token Runtime | 合规规则引擎 | 凭证验证 | Token 校验时 |
| Rate Limit | 合规规则引擎 | 流量检测 | 限流触发时 |
| Audit Middleware | 合规报告 | 日志聚合 | 审计事件写入 |
| Secret Scanner | 合规规则引擎 | 凭证检测 | 扫描结果输出 |

### 5.2 联动设计

#### 5.2.1 告警系统联动

```
合规事件 ──┬──▶ 告警通道 (Slack/PagerDuty/Email)
           │
           └──▶ 事件存储 (审计数据库)
                    │
                    └──▶ 趋势分析 ──▶ M-013~M-016 指标更新
```

#### 5.2.2 Token Runtime 联动

```
Token 校验请求
     │
     ├──▶ CRED-INGRESS-PLATFORM: 验证平台凭证存在
     │
     ├──▶ CRED-INGRESS-FORMAT: 验证凭证格式
     │
     └──▶ CRED-INGRESS-EXPIRED: 验证凭证状态 (通过 Token Runtime)
```

#### 5.2.3 Audit Middleware 联动

```
HTTP 请求
     │
     ├──▶ Audit Middleware (记录请求)
     │
     ├──▶ 合规规则引擎 (执行检查)
     │         │
     │         ├──▶ CRED-EXPOSE-* 凭证泄露检测
     │         │
     │         └──▶ CRED-DIRECT-* 直连检测
     │
     └──▶ 合规报告生成 (聚合日志)
```

---

## 6. 目录结构

```
compliance/                        # [待创建] 合规能力包根目录
├── rules/                          # 合规规则定义
│   ├── m013_credential_exposure.yaml
│   ├── m014_ingress_coverage.yaml
│   ├── m015_direct_access.yaml
│   └── m016_query_key_reject.yaml
│
├── config/                         # 合规配置
│   ├── allowed_suppliers.yaml
│   ├── alert_channels.yaml
│   └── rule_sets.yaml
│
├── engine/                         # 合规规则引擎
│   ├── compiler.go                 # 规则编译器
│   ├── matcher.go                  # 规则匹配器
│   ├── executor.go                 # 策略执行器
│   └── audit.go                    # 审计记录器
│
├── reports/                        # 合规报告 (M-017)
│   ├── m017_dependency_audit.sh    # [待实现] 四件套生成脚本
│   └── templates/                  # 报告模板
│
├── ci/                             # CI 集成
│   ├── compliance_gate.sh          # 合规门禁主脚本
│   ├── m013_credential_scan.sh    # [待实现]
│   ├── m014_ingress_coverage.sh    # [待实现]
│   ├── m015_direct_access_check.sh # [待实现]
│   ├── m016_query_key_reject.sh    # [待实现]
│   └── m017_dependency_audit.sh   # [待实现]
│
├── monitoring/                     # 监控配置
│   ├── alerts.yaml                # 告警规则
│   └── dashboards/                # 监控面板
│
└── docs/                          # 文档
    ├── compliance_capability_package_design_v1_2026-04-02.md
    └── compliance_rules_reference.md

scripts/ci/                        # [已存在] 现有 CI 脚本目录
├── lockfile_diff.sh               # [待实现] Lockfile Diff 生成
├── compat_matrix.sh               # [待实现] 兼容矩阵生成
└── risk_register.sh               # [待实现] 风险登记册生成
```

---

## 7. 实施计划

### 7.1 P2 阶段任务分解

> **工期修正说明**：根据评审意见，原设计工期（26d）低估了CI脚本实现工作量。实际工作量需要 **35-40d**，主要原因是：
> 1. 所有 CI 脚本（m013~m017）均需新实现
> 2. M-017 四件套生成脚本需要独立开发
> 3. 与现有审计日志系统的集成需要额外协调

| 任务ID | 任务名称 | 依赖 | 设计工期 | 修正工期 | 交付物 |
|--------|----------|------|---------|---------|--------|
| P2-CMP-001 | 合规规则引擎核心开发 | - | 5d | 6d | engine/*.go |
| P2-CMP-002 | M-013 凭证泄露规则实现 | P2-CMP-001 | 3d | 4d | rules/m013_*.yaml + ci/m013_*.sh |
| P2-CMP-003 | M-014 入站覆盖规则实现 | P2-CMP-001 | 2d | 3d | rules/m014_*.yaml + ci/m014_*.sh |
| P2-CMP-004 | M-015 直连检测规则实现 | P2-CMP-001 | 2d | 4d | rules/m015_*.yaml + ci/m015_*.sh + 蜜罐配置 |
| P2-CMP-005 | M-016 Query Key 拒绝规则实现 | P2-CMP-001 | 2d | 3d | rules/m016_*.yaml + ci/m016_*.sh |
| P2-CMP-006 | M-017 依赖审计四件套 | - | 3d | 6d | 四件套生成脚本 + 模板 |
| P2-CMP-007 | CI 流水线集成 | P2-CMP-002~006 | 2d | 5d | ci/compliance_gate.sh |
| P2-CMP-008 | 监控告警配置 | P2-CMP-001 | 2d | 3d | monitoring/alerts.yaml |
| P2-CMP-009 | 安全机制联动实现 | P2-CMP-001 | 3d | 4d | 联动代码 |
| P2-CMP-010 | 端到端测试与验证 | P2-CMP-007 | 2d | 4d | 测试报告 |
| **总计** | | | **26d** | **38d** | |

### 7.2 里程碑

| 里程碑 | 完成条件 | 设计日期 | 修正日期 |
|--------|----------|----------|----------|
| **M1: 规则引擎完成** | P2-CMP-001 通过单元测试 | 2026-04-07 | 2026-04-08 |
| **M2: 四大规则就绪** | P2-CMP-002~005 在 staging 通过 | 2026-04-11 | 2026-04-15 |
| **M3: CI 集成完成** | P2-CMP-007 合规门禁在 CI 通过 | 2026-04-13 | 2026-04-20 |
| **M4: 监控告警就绪** | P2-CMP-008 告警通道验证通过 | 2026-04-15 | 2026-04-22 |
| **M5: P2 交付完成** | E2E 测试通过 + 文档完备 | 2026-04-17 | 2026-04-26 |

---

## 8. 验收标准

### 8.1 M-013~M-016 验收

| 指标 | 验收条件 | 测试方法 |
|------|----------|----------|
| M-013 | 凭证泄露事件 = 0 | 自动化扫描 + 渗透测试 |
| M-014 | 入站覆盖率 = 100% | 日志分析覆盖率 |
| M-015 | 直连事件 = 0 | 蜜罐检测 + 日志分析 |
| M-016 | 拒绝率 = 100% | 外部 query key 构造测试 |

### 8.2 M-017 验收

| 报告 | 验收条件 |
|------|----------|
| SBOM | SPDX 2.3 格式有效, 包含所有直接依赖 |
| Lockfile Diff | 变更条目完整, 影响评估准确 |
| 兼容矩阵 | 版本对应关系正确 |
| 风险登记册 | CVSS >= 7.0 漏洞已收录 |

### 8.3 集成验收

| 场景 | 验收条件 |
|------|----------|
| CI 流水线 | 合规门禁在 build 阶段可执行 |
| 告警通道 | 告警可实时送达 (延迟 < 30s) |
| 报告生成 | 四件套在 CI 中自动生成 |
| 规则热更新 | 规则变更无需重启服务 |

---

## 9. 附录

### 9.1 参考文档

- `docs/tos_compliance_engine_design_v1_2026-03-18.md` - ToS 合规引擎设计
- `docs/llm_gateway_subapi_evolution_plan_v4_2_2026-03-24.md` - M-013~M-016 指标定义
- `docs/supply_gate_command_playbook_v1_2026-03-25.md` - M-017 依赖审计要求

### 9.2 术语表

| 术语 | 定义 |
|------|------|
| SBOM | Software Bill of Materials, 软件物料清单 |
| SPDX | Software Package Data Exchange, 软件包数据交换标准 |
| CVSS | Common Vulnerability Scoring System, 通用漏洞评分系统 |
| ToS | Terms of Service, 服务条款 |
| CI | Continuous Integration, 持续集成 |

---

**文档状态**: 已修订
**版本**: v1.1
**日期**: 2026-04-02
**关联任务**: P2 合规能力包设计
**修订说明**:
- 统一事件命名格式与 audit_log_enhancement_design_v1_2026-04-02.md 对齐
- 修复硬编码路径问题，改为环境变量或相对路径
- 补充 M-015 直连检测方法（蜜罐、网络流量分析等）
- 修复 syft 缺失时生成无效 SBOM 问题（改为必需检查）
- 补充 M-017 四件套生成脚本详细设计（待实现状态）
- 修正实施工期从 26d 到 38d
