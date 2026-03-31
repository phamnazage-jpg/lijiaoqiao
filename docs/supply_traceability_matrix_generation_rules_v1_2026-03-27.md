# 供应侧追踪矩阵生成规则（v1.0）

- 日期：2026-03-27
- 适用文件：`reports/supply_traceability_matrix_2026-03-25.csv`
- 目标：保证 Requirement -> API -> Test -> Metric -> Gate 的自动化可追踪与口径一致。

## 1. 字段规范

1. `requirement_id`：唯一且稳定，不得复用。
2. `api`：必须使用 OpenAPI 主路径与精确参数名（如 `{accountId}`、`{packageId}`、`{settlementId}`）。
3. `api_alias`：仅记录历史兼容路径；无兼容值填写 `-`。
4. `test_case`：使用 `|` 连接多个用例 ID，顺序按主路径优先。
5. `metric`：使用 SSOT 中的统一指标名，禁止自造同义词。
6. `gate`：映射 SUP/SEC/XR 门禁，多个值用 `|` 分隔。
7. `status`：`PLANNED/RUNNING/PASS/FAIL/BLOCKED` 五态。

## 2. 生成流程

1. 从按钮级 PRD 抽取需求项并形成 `requirement_id`。
2. 从 OpenAPI 提取接口主路径，填入 `api`。
3. 对历史路径或迁移路径填入 `api_alias`。
4. 绑定测试用例、指标、门禁并指定 owner。
5. 由 QA 执行完整性检查后发布 CSV。

## 3. 校验规则

1. `api` 必须可在 OpenAPI 中检索命中。
2. `api_alias` 不得与 `api` 完全相同。
3. `gate` 必须在任务单中存在对应条目。
4. 每条记录必须有 `evidence_path`。
5. 任一校验失败，`M-019` 计为不通过。

## 4. 变更治理

1. 修改 `api` 视为高风险变更，必须同步更新用例与门禁映射。
2. 新增 alias 必须附迁移原因和下线计划。
3. 每次变更后需执行一次路径一致性检查并留痕。
