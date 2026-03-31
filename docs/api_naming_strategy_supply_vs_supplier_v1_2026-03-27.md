# API 命名策略：`/supply` vs `/supplier`（v1.0）

- 日期：2026-03-27
- 决策类型：命名规范与兼容策略
- 适用范围：供应侧控制台与平台账务相关 API

## 1. 决策

1. 规范主路径统一采用：`/api/v1/supply/*`。
2. 历史兼容路径 `/api/v1/supplier/*` 保留为 alias，并标记 `deprecated`。
3. 新增接口禁止使用 `/supplier` 前缀。

## 2. 兼容策略

1. 别名路径只做兼容，不扩展新字段。
2. 响应体增加迁移提示字段（如 `deprecation_notice`）或在文档标注迁移窗口。
3. S2 阶段评估 alias 下线时间，提前至少一个版本公告。

## 3. 验收标准

1. OpenAPI 同时存在 canonical 路径与 alias 路径声明。
2. alias 路径标记 `deprecated: true`。
3. 追踪矩阵 `api_alias` 字段可定位所有 alias 使用点。
