# Router Core 接管率统计 SQL 与看板字段定义（v1）

- 版本：v1.0
- 日期：2026-03-17
- 适用阶段：S2（2026-05-16 至 2026-08-15）
- 关联文档：
  - `router_core_takeover_execution_plan_v3_2026-03-17.md`
  - `sub2api_scheduler_billing_flow_deep_dive_v2_2026-03-17.md`

## 1. 口径与前提

S2 验收口径（与 v3 一致）：

1. `overall_takeover >= 60%`
2. `cn_takeover = 100%`

说明：

1. 现有 `usage_logs` 已有 `request_type/openai_ws_mode/inbound_endpoint/upstream_endpoint`，但没有“本请求由谁执行主路径（自研 Router Core vs subapi 路径）”的显式字段。
2. 因此本文件给出两套 SQL：
   - T+0 临时口径：基于 `gateway_route_marks` 标记表（可立即落地）。
   - T+7 验收口径：在 `usage_logs` 增加 `router_engine` 后，直接从事实表统计（推荐作为最终验收口径）。

## 2. T+0 临时口径（可立即执行）

## 2.1 建立路由标记表（一次性 DDL）

```sql
CREATE TABLE IF NOT EXISTS gateway_route_marks (
    request_id      VARCHAR(255) NOT NULL,
    api_key_id      BIGINT NOT NULL,
    router_engine   SMALLINT NOT NULL, -- 1=subapi_path, 2=router_core
    marked_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (request_id, api_key_id),
    CONSTRAINT gateway_route_marks_router_engine_check
        CHECK (router_engine IN (1, 2))
);

CREATE INDEX IF NOT EXISTS idx_gateway_route_marks_marked_at
    ON gateway_route_marks (marked_at DESC);

CREATE INDEX IF NOT EXISTS idx_gateway_route_marks_engine_time
    ON gateway_route_marks (router_engine, marked_at DESC);
```

写入约定（必须执行）：

1. 每个主路径请求在最终完成 usage 记录前，写入一条 `gateway_route_marks`。
2. `router_engine=2` 表示四个关键环节（scheduler/concurrency/failover/billing）均由自研链路执行。
3. `router_engine=1` 表示任一关键环节仍依赖 subapi 路径。

## 2.2 全供应商接管率（overall）

```sql
WITH params AS (
    SELECT
        NOW() - INTERVAL '24 hours' AS start_ts,
        NOW() AS end_ts
),
main_path AS (
    SELECT
        ul.id,
        ul.request_id,
        ul.api_key_id,
        ul.created_at,
        rm.router_engine
    FROM usage_logs ul
    LEFT JOIN gateway_route_marks rm
        ON rm.request_id = ul.request_id
       AND rm.api_key_id = ul.api_key_id
    CROSS JOIN params p
    WHERE ul.created_at >= p.start_ts
      AND ul.created_at < p.end_ts
      AND (
            ul.inbound_endpoint IN (
                '/v1/chat/completions',
                '/v1/messages',
                '/v1/responses'
            )
            OR ul.inbound_endpoint LIKE '/v1beta/%'
          )
)
SELECT
    COUNT(*) AS all_main_path_requests,
    COUNT(*) FILTER (WHERE router_engine = 2) AS self_built_main_path_requests,
    ROUND(
        100.0 * COUNT(*) FILTER (WHERE router_engine = 2)
        / NULLIF(COUNT(*), 0),
        2
    ) AS overall_takeover_pct,
    ROUND(
        100.0 * COUNT(*) FILTER (WHERE router_engine IN (1, 2))
        / NULLIF(COUNT(*), 0),
        2
    ) AS route_mark_coverage_pct
FROM main_path;
```

说明：`cn_platforms` 必须由配置表维护（例如 `gateway_cn_platforms`），禁止在验收 SQL 硬编码固定值。

## 2.3 国内供应商接管率（cn）

```sql
WITH params AS (
    SELECT
        NOW() - INTERVAL '24 hours' AS start_ts,
        NOW() AS end_ts,
        COALESCE(
            (
                SELECT ARRAY_AGG(platform ORDER BY platform)
                FROM gateway_cn_platforms
                WHERE enabled = TRUE
            ),
            ARRAY[]::TEXT[]
        ) AS cn_platforms
),
main_path AS (
    SELECT
        ul.id,
        ul.request_id,
        ul.api_key_id,
        ul.created_at,
        a.platform,
        rm.router_engine
    FROM usage_logs ul
    JOIN accounts a
      ON a.id = ul.account_id
    LEFT JOIN gateway_route_marks rm
      ON rm.request_id = ul.request_id
     AND rm.api_key_id = ul.api_key_id
    CROSS JOIN params p
    WHERE ul.created_at >= p.start_ts
      AND ul.created_at < p.end_ts
      AND a.platform = ANY(p.cn_platforms)
      AND (
            ul.inbound_endpoint IN (
                '/v1/chat/completions',
                '/v1/messages',
                '/v1/responses'
            )
            OR ul.inbound_endpoint LIKE '/v1beta/%'
          )
)
SELECT
    COUNT(*) AS all_cn_provider_requests,
    COUNT(*) FILTER (WHERE router_engine = 2) AS self_built_cn_provider_requests,
    ROUND(
        100.0 * COUNT(*) FILTER (WHERE router_engine = 2)
        / NULLIF(COUNT(*), 0),
        2
    ) AS cn_takeover_pct,
    ROUND(
        100.0 * COUNT(*) FILTER (WHERE router_engine IN (1, 2))
        / NULLIF(COUNT(*), 0),
        2
    ) AS route_mark_coverage_pct
FROM main_path;
```

## 2.4 趋势 SQL（按小时 / 按天）

### 2.4.1 按小时趋势（overall + cn 同图）

```sql
WITH params AS (
    SELECT
        NOW() - INTERVAL '72 hours' AS start_ts,
        NOW() AS end_ts,
        COALESCE(
            (
                SELECT ARRAY_AGG(platform ORDER BY platform)
                FROM gateway_cn_platforms
                WHERE enabled = TRUE
            ),
            ARRAY[]::TEXT[]
        ) AS cn_platforms
),
base AS (
    SELECT
        DATE_TRUNC('hour', ul.created_at) AS bucket,
        a.platform,
        rm.router_engine
    FROM usage_logs ul
    JOIN accounts a
      ON a.id = ul.account_id
    LEFT JOIN gateway_route_marks rm
      ON rm.request_id = ul.request_id
     AND rm.api_key_id = ul.api_key_id
    CROSS JOIN params p
    WHERE ul.created_at >= p.start_ts
      AND ul.created_at < p.end_ts
      AND (
            ul.inbound_endpoint IN (
                '/v1/chat/completions',
                '/v1/messages',
                '/v1/responses'
            )
            OR ul.inbound_endpoint LIKE '/v1beta/%'
          )
)
SELECT
    bucket,
    ROUND(
        100.0 * COUNT(*) FILTER (WHERE router_engine = 2)
        / NULLIF(COUNT(*), 0),
        2
    ) AS overall_takeover_pct,
    ROUND(
        100.0 * COUNT(*) FILTER (
            WHERE platform = ANY((SELECT cn_platforms FROM params))
              AND router_engine = 2
        )
        / NULLIF(
            COUNT(*) FILTER (WHERE platform = ANY((SELECT cn_platforms FROM params))),
            0
        ),
        2
    ) AS cn_takeover_pct,
    COUNT(*) AS total_requests,
    COUNT(*) FILTER (WHERE platform = ANY((SELECT cn_platforms FROM params))) AS cn_requests
FROM base
GROUP BY bucket
ORDER BY bucket;
```

### 2.4.2 按天趋势（验收期）

```sql
WITH params AS (
    SELECT
        NOW() - INTERVAL '30 days' AS start_ts,
        NOW() AS end_ts,
        COALESCE(
            (
                SELECT ARRAY_AGG(platform ORDER BY platform)
                FROM gateway_cn_platforms
                WHERE enabled = TRUE
            ),
            ARRAY[]::TEXT[]
        ) AS cn_platforms
),
base AS (
    SELECT
        DATE_TRUNC('day', ul.created_at) AS bucket,
        a.platform,
        rm.router_engine
    FROM usage_logs ul
    JOIN accounts a
      ON a.id = ul.account_id
    LEFT JOIN gateway_route_marks rm
      ON rm.request_id = ul.request_id
     AND rm.api_key_id = ul.api_key_id
    CROSS JOIN params p
    WHERE ul.created_at >= p.start_ts
      AND ul.created_at < p.end_ts
      AND (
            ul.inbound_endpoint IN (
                '/v1/chat/completions',
                '/v1/messages',
                '/v1/responses'
            )
            OR ul.inbound_endpoint LIKE '/v1beta/%'
          )
)
SELECT
    bucket,
    ROUND(100.0 * COUNT(*) FILTER (WHERE router_engine = 2) / NULLIF(COUNT(*), 0), 2) AS overall_takeover_pct,
    ROUND(
        100.0 * COUNT(*) FILTER (
            WHERE platform = ANY((SELECT cn_platforms FROM params))
              AND router_engine = 2
        )
        / NULLIF(
            COUNT(*) FILTER (WHERE platform = ANY((SELECT cn_platforms FROM params))),
            0
        ),
        2
    ) AS cn_takeover_pct
FROM base
GROUP BY bucket
ORDER BY bucket;
```

## 3. T+7 验收口径（推荐）

## 3.1 对 `usage_logs` 做最小字段扩展

```sql
ALTER TABLE usage_logs
    ADD COLUMN IF NOT EXISTS router_engine SMALLINT NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS router_version VARCHAR(32);

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'usage_logs_router_engine_check'
    ) THEN
        ALTER TABLE usage_logs
            ADD CONSTRAINT usage_logs_router_engine_check
            CHECK (router_engine IN (0, 1, 2));
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_usage_logs_router_engine_created_at
    ON usage_logs (router_engine, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_usage_logs_created_at_account_id
    ON usage_logs (created_at DESC, account_id);
```

字段语义：

1. `0`：未知（历史数据或未标记）。
2. `1`：subapi 路径。
3. `2`：自研 Router Core 路径（验收口径中的“接管请求”）。

## 3.2 用标记表回填（可选，建议分批）

```sql
WITH cte AS (
    SELECT ul.id, rm.router_engine
    FROM usage_logs ul
    JOIN gateway_route_marks rm
      ON rm.request_id = ul.request_id
     AND rm.api_key_id = ul.api_key_id
    WHERE ul.router_engine = 0
    ORDER BY ul.id
    LIMIT 5000
)
UPDATE usage_logs ul
SET router_engine = cte.router_engine
FROM cte
WHERE ul.id = cte.id;
```

## 3.3 验收 SQL（不依赖标记表）

```sql
WITH params AS (
    SELECT
        NOW() - INTERVAL '24 hours' AS start_ts,
        NOW() AS end_ts,
        COALESCE(
            (
                SELECT ARRAY_AGG(platform ORDER BY platform)
                FROM gateway_cn_platforms
                WHERE enabled = TRUE
            ),
            ARRAY[]::TEXT[]
        ) AS cn_platforms
),
base AS (
    SELECT
        ul.created_at,
        ul.router_engine,
        a.platform
    FROM usage_logs ul
    JOIN accounts a
      ON a.id = ul.account_id
    CROSS JOIN params p
    WHERE ul.created_at >= p.start_ts
      AND ul.created_at < p.end_ts
      AND (
            ul.inbound_endpoint IN (
                '/v1/chat/completions',
                '/v1/messages',
                '/v1/responses'
            )
            OR ul.inbound_endpoint LIKE '/v1beta/%'
          )
)
SELECT
    ROUND(100.0 * COUNT(*) FILTER (WHERE router_engine = 2) / NULLIF(COUNT(*), 0), 2) AS overall_takeover_pct,
    ROUND(
        100.0 * COUNT(*) FILTER (
            WHERE platform = ANY((SELECT cn_platforms FROM params))
              AND router_engine = 2
        )
        / NULLIF(
            COUNT(*) FILTER (WHERE platform = ANY((SELECT cn_platforms FROM params))),
            0
        ),
        2
    ) AS cn_takeover_pct,
    ROUND(100.0 * COUNT(*) FILTER (WHERE router_engine IN (1, 2)) / NULLIF(COUNT(*), 0), 2) AS route_mark_coverage_pct
FROM base;
```

说明：验收 SQL 的 `cn_platforms` 以配置表 `gateway_cn_platforms` 为唯一来源，避免平台清单变更导致口径漂移。

## 4. 看板字段定义（S2）

## 4.1 KPI 卡片

| 字段键 | 展示名 | 公式 | 阈值/目标 | 数据来源 |
|---|---|---|---|---|
| `overall_takeover_pct` | 全供应商接管率 | `self_built_main_path_requests / all_main_path_requests * 100` | `>= 60%` | 上述 SQL |
| `cn_takeover_pct` | 国内供应商接管率 | `self_built_cn_provider_requests / all_cn_provider_requests * 100` | `= 100%` | 上述 SQL |
| `route_mark_coverage_pct` | 路由标记覆盖率 | `marked_requests / all_main_path_requests * 100` | `>= 99.9%` | 上述 SQL |
| `billing_error_rate_pct` | 账务差错率 | `billing_error_requests / billed_requests * 100` | `<= 0.1%` | 账务核对任务/报表 |
| `billing_conflict_rate_pct` | 幂等冲突率 | `billing_dedup_conflicts / billed_requests * 100` | `<= 0.01%` | 扣费幂等审计计数器 |
| `gateway_added_latency_p95_ms` | 网关附加时延P95 | 网关处理时延分位数 | `<= 60ms` | `ops_system_metrics` 或 APM |
| `gateway_5xx_delta_pct` | 5xx 相对基线增量 | `current_5xx - baseline_5xx` | `<= +0.1%` | 统一错误指标 |

## 4.2 维度拆分

看板必须支持以下维度切片：

1. `platform`（`anthropic/openai/gemini/antigravity/sora`）
2. `group_id`
3. `inbound_endpoint`
4. `upstream_endpoint`
5. `request_type`（sync/stream/openai_ws）
6. `api_key_id`（用于租户级排障）

## 4.3 图表建议

1. 折线：`overall_takeover_pct` 按小时（72h）+ 按天（30d）。
2. 折线：`cn_takeover_pct` 按小时（72h）+ 按天（30d）。
3. 堆叠柱：主路径请求量按 `router_engine` 拆分（自研/subapi/未知）。
4. 热力图：`platform x inbound_endpoint` 的接管率。
5. 散点：`takeover_pct` vs `gateway_added_latency_p95_ms`，用于识别“接管提升但延迟恶化”的点位。

## 5. 告警规则（与 S2 门槛一致）

1. `cn_takeover_pct < 100` 持续 5 分钟：`P0`。
2. `overall_takeover_pct < 60` 且当前处于 Wave-Global-3：`P1`。
3. `route_mark_coverage_pct < 99.9`：`P1`（口径不可信，阻断升级）。
4. `billing_conflict_rate_pct > 0.01`：`P0`（立即停止继续灰度）。
5. `billing_error_rate_pct > 0.1`：`P0`。
6. `gateway_added_latency_p95_ms > 60` 持续 10 分钟：`P1`。
7. `gateway_5xx_delta_pct > 0.1` 持续 5 分钟：`P0`。

## 6. 落地顺序（建议）

1. 先落 `gateway_route_marks` + 临时 SQL，保证本周就有可观测接管率。
2. 再加 `usage_logs.router_engine`，切换到验收口径。
3. 验收和周报统一只读“验收口径”看板，避免双口径冲突。
