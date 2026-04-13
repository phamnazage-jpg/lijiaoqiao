# 设计文档清理计划

> 日期：2026-04-03
> 目标：清理过期文档，确保文档体系清晰

---

## 一、文档清理原则

1. **保留现行有效文档**：当前项目正在使用的设计文档
2. **归档历史版本**：已过时但可能需要参考的文档移至归档目录
3. **删除无效文档**：已完全被替代且无参考价值的文档

---

## 二、需要清理的文档分类

### 2.1 现行有效文档（保留）

| 文档路径 | 说明 | 状态 |
|---------|------|------|
| `docs/audit_log_enhancement_design_v1_2026-04-02.md` | 审计日志增强设计 | ✅ 现行，需修复P0问题 |
| `reports/audit_system_production_readiness_review_2026-04-03.md` | 本次审核报告 | ✅ 新增 |

---

### 2.2 过期的对齐检查点（归档）

以下文档已过期（最后检查日期2026-03-31），应移至归档目录：

| 文档路径 | 日期 | 建议操作 |
|---------|------|---------|
| `reports/alignment_validation_checkpoint_01~33_2026-03-*.md` | 3月27日-4月1日 | 归档至 `reports/archive/alignment/` |
| `reports/gates/tok005_dryrun_*.md` | 3月30日 | 归档至 `reports/archive/gates/` |
| `reports/gates/tok006_gate_bundle_*.md` | 3月30日 | 归档至 `reports/archive/gates/` |
| `reports/design_drift_daily_2026-03-*.md` | 3月30-31日 | 归档至 `reports/archive/design/` |

---

### 2.3 可直接删除的文档

| 文档路径 | 原因 |
|---------|------|
| `reports/alignment_validation_checkpoint_*.md` (34之后) | 检查点编号已超过最新版本 |
| `reports/design_drift_daily_2026-03-30-debug.md` | debug临时文件 |
| `reports/supply_flaky_budget_2026-03-25.md` | 旧版本预算分析 |
| `reports/supply_gate_preflight_2026-03-25.md` | 旧版本预检 |

---

### 2.4 待确认的文档

| 文档路径 | 说明 | 建议 |
|---------|------|------|
| `docs/supply_technical_design_enhanced_v1_2026-03-25.md` | 供应技术设计 | 与supply-api当前实现需对比确认 |
| `docs/token_auth_middleware_design_v1_2026-03-29.md` | Token中间件设计 | 与gateway实现需对比确认 |

---

## 三、建议的目录结构

```
reports/
├── archive/                          # 归档目录
│   ├── alignment/                    # 对齐检查点归档
│   │   └── checkpoint_2026-03/
│   ├── gates/                        # Gate报告归档
│   │   └── tok005_006_2026-03/
│   └── design/                       # 设计漂移归档
│       └── drift_2026-03/
├── review/                           # 复审报告
├── audit_system_production_readiness_review_2026-04-03.md  # 新增审核报告
└── tdd_execution_summary_2026-04-02.md
```

---

## 四、执行命令

```bash
# 1. 创建归档目录
mkdir -p reports/archive/alignment/checkpoint_2026-03
mkdir -p reports/archive/gates/tok005_006_2026-03
mkdir -p reports/archive/design/drift_2026-03

# 2. 移动对齐检查点归档
mv reports/alignment_validation_checkpoint_*_2026-03-*.md reports/archive/alignment/checkpoint_2026-03/

# 3. 移动Gate报告归档
mv reports/gates/tok005_dryrun_*.md reports/archive/gates/tok005_006_2026-03/
mv reports/gates/tok006_gate_bundle_*.md reports/archive/gates/tok005_006_2026-03/

# 4. 移动设计漂移归档
mv reports/design_drift_daily_2026-03-*.md reports/archive/design/drift_2026-03/

# 5. 删除过期临时文件
rm reports/design_drift_daily_2026-03-30-debug.md
rm reports/supply_flaky_budget_2026-03-25.md
rm reports/supply_gate_preflight_2026-03-25.md
```

---

## 五、清理后验证

清理完成后，reports目录应只包含：

```
reports/
├── archive/                          # 归档目录（不再干扰主流程）
├── review/                           # 复审报告
├── db/                              # 数据库相关
├── dependency/                       # 依赖分析
├── gates/                            # 最新的Gate报告
├── alignment_validation_checkpoint_34_2026-04-03.md  # 最新对齐检查
├── audit_system_production_readiness_review_2026-04-03.md  # 本次审核报告
└── tdd_execution_summary_2026-04-02.md
```

---

**清理执行状态**：✅ 已完成

**设计文档更新状态**：✅ 已完成
- CI/CD Gate脚本已更新（awk + 重试 + 超时）
- JSONB索引已更新（冗余布尔字段 + GIN备选）
- 10K TPS达成路径已添加
