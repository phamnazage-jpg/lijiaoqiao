# SQL Schema Code Quality Review Report

**Date**: 2026-04-16  
**Reviewer**: Hermes Agent  
**Scope**: `/home/long/project/立交桥` SQL schema files

---

## Executive Summary

This report reviews SQL schema files across two subprojects:
- `supply-api/sql/postgresql/` (3 files)
- `sql/postgresql/` (7 files)
- `llm-gateway-competitors/sub2api-tar/backend/migrations/` (74 migration files)

Overall assessment: **Good** with some concerns noted below.

---

## 1. Security Issues

### 1.1 Credentials Storage - PASS (Good)

| File | Finding |
|------|---------|
| `supply_core_schema_v2.sql` | `encrypted_credentials TEXT NOT NULL DEFAULT ''` - Credentials stored encrypted |
| `supply_schema_v1.sql` | `encrypted_credentials TEXT NOT NULL` - Credentials field present |
| `iam_schema_v1.sql` | `password_hash` not exposed in plaintext |
| `001_init.sql` (llm-gateway) | `password VARCHAR(100)` in proxies table - **WARNING: Plain text password field** |

**Note**: The `proxies` table in llm-gateway stores `username` and `password` directly. While acceptable for proxy authentication credentials (as proxy servers may require them), ensure these are properly encrypted at rest in production.

### 1.2 Sensitive Data - PASS (Good)

- No hardcoded secrets or API keys found in schema files
- No SQL injection vectors (no dynamic SQL with user input concatenated)
- `key_hash` in `auth_platform_api_keys` uses SHA-256 HMAC - good practice

### 1.3 IP Address Logging - PASS (Good)

- `created_ip INET`, `updated_ip INET` fields properly typed as INET in supply tables
- `client_ip INET` in audit_events - good practice for IP storage

---

## 2. Index Coverage Analysis

### 2.1 Supply Core Schema (supply_core_schema_v2.sql)

| Table | Indexes | Coverage |
|-------|---------|----------|
| `supply_accounts` | 3 indexes | Good: user+status+created_at, request_id (partial), idempotency_key (partial) |
| `supply_packages` | 3 indexes | Good: user+status+created_at, account_id, request_id (partial) |
| `supply_settlements` | 3 indexes | Good: user+status+created_at, idempotency_key (partial), request_id (partial) |

**Good pattern**: Partial indexes on `request_id` and `idempotency_key` with `WHERE <> ''` condition - efficient for sparse columns.

### 2.2 Audit Tables

| Table | Indexes | Coverage |
|-------|---------|----------|
| `audit_alerts` | 3 indexes | Good: tenant+status+created_at, tenant+type+level, supplier+created_at (partial) |
| `audit_events` (partitioned) | 4 indexes | Good: tenant+domain+time, request_id, trace_id, result_code |

### 2.3 Platform Core Schema (platform_core_schema_v1.sql)

| Table | Indexes | Coverage |
|-------|---------|----------|
| `core_tenants` | 2 indexes | Adequate |
| `core_projects` | 1 index + UNIQUE | Adequate (tenant_id, project_code) |
| `iam_users` | 2 indexes + UNIQUE | Good (tenant, email unique constraint) |
| `auth_platform_api_keys` | 4 indexes | Good |
| `billing_accounts` | 1 index | Adequate |
| `billing_ledger_entries` | 4 indexes | Good |
| `routing_policies` | 2 indexes | Good |
| `security_kms_key_registry` | 1 index | Adequate |
| `audit_events` | 4 indexes | Good |

### 2.4 IAM Schema (iam_schema_v1.sql)

| Table | Indexes | Coverage |
|-------|---------|----------|
| `iam_roles` | 5 indexes | Excellent: code, type, parent, level, is_active |
| `iam_scopes` | 3 indexes | Good |
| `iam_role_scopes` | 2 indexes | Good |
| `iam_user_roles` | 5 indexes + UNIQUE | Excellent |
| `iam_role_hierarchy` | 2 indexes + constraints | Good |

### 2.5 Outbox Pattern (outbox_pattern_v1.sql)

| Table | Indexes | Coverage |
|-------|---------|----------|
| `outbox_events` | 4 indexes | Good: status+next_retry (partial), aggregate, created_at, event_id |
| `outbox_dead_letter` | 2 indexes | Good: unhandled (partial), original_event_id |
| `supply_batch_compensation` | 2 indexes | Good: batch+status, status+created_at |

**Good**: Partial indexes used for `status IN ('pending', 'failed')` - efficient for selective queries.

### 2.6 Token Runtime (token_runtime_schema_v1.sql)

| Table | Indexes | Coverage |
|-------|---------|----------|
| `auth_platform_tokens` | 4 indexes | Good: fingerprint (unique), subject+status, expires_at, tenant+project |
| `auth_token_audit_events` | 4 indexes | Good: token+time, request_id, subject+time, event_name |

---

## 3. Constraint Completeness

### 3.1 Foreign Key Constraints

| Schema | Foreign Keys | Assessment |
|--------|-------------|------------|
| `supply_core_schema_v2.sql` | None explicitly | **NOTE**: Comment states "当前仓库仍使用应用层外键校验，因此这里先不引入数据库级外键" - intentionally deferred |
| `supply_schema_v1.sql` | Yes - `supply_packages.supply_account_id REFERENCES supply_accounts(id)` | Good |
| `platform_core_schema_v1.sql` | Yes - proper FK relationships | Good |
| `iam_schema_v1.sql` | Yes - proper FK with CASCADE/SET NULL | Good |
| `llm-gateway 001_init.sql` | Yes - proper FK with ON DELETE behavior | Good |

### 3.2 CHECK Constraints

| Schema | CHECK Constraints | Assessment |
|--------|------------------|------------|
| `supply_core_schema_v2.sql` | None visible | **CONCERN**: Status values not constrained at DB level |
| `iam_schema_v1.sql` | Yes - role level >= 0, role_code format | Good |
| `outbox_pattern_v1.sql` | Yes - status IN list, direction IN ('dr','cr') | Good |
| `partition_strategy_v1.sql` | Yes - severity IN list, direction IN ('dr','cr') | Good |
| `token_runtime_schema_v1.sql` | Yes - role_code IN list, status IN list | Good |

**Issue Found**: `supply_core_schema_v2.sql` does NOT have CHECK constraints on `status` fields for:
- `supply_accounts.status`
- `supply_packages.status`
- `supply_settlements.status`

Compare to `supply_schema_v1.sql` which DOES have CHECK constraints:
```sql
status VARCHAR(20) NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending', 'active', 'suspended', 'disabled'))
```

### 3.3 NOT NULL Constraints

Most critical columns properly marked NOT NULL:
- Primary keys
- Amount fields in financial tables
- Status fields
- Timestamp fields with defaults

### 3.4 Unique Constraints

| Table | Unique Constraint | Assessment |
|-------|------------------|------------|
| `supply_settlements` (v2) | `settlement_no UNIQUE` | Good |
| `supply_orders` | `order_no UNIQUE` | Good |
| `iam_roles` | `code UNIQUE` | Good |
| `iam_scopes` | `code UNIQUE` | Good |
| `core_tenants` | `tenant_code UNIQUE` | Good |

### 3.5 Default Values

Good patterns observed:
- `version BIGINT NOT NULL DEFAULT 0` for optimistic locking - consistent
- `created_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP`
- `updated_at TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP`

---

## 4. Migration Safety Analysis

### 4.1 Idempotent Migrations - GOOD

Most migration files use `IF NOT EXISTS` patterns:

```sql
-- Good pattern examples
CREATE TABLE IF NOT EXISTS xxx (...);
CREATE INDEX IF NOT EXISTS idx_xxx ON xxx (...);
ALTER TABLE xxx ADD COLUMN IF NOT EXISTS col_name ...;
```

### 4.2 Backup Before Dangerous Operations

**Good** - `audit_events_migration_v1_to_v2.sql` includes backup:
```sql
CREATE TABLE IF NOT EXISTS audit_events_backup AS
SELECT * FROM audit_events WHERE 1=0;
INSERT INTO audit_events_backup SELECT * FROM audit_events;
```

### 4.3 Transaction Safety

| File | Transaction Handling |
|------|---------------------|
| `iam_schema_v1.sql` | Uses `BEGIN/COMMIT` - Good |
| `outbox_pattern_v1.sql` | No transaction wrapper (functions are autonomous) - Acceptable |
| `partition_strategy_v1.sql` | No transaction wrapper - **CONCERN**: DROP TABLE operations |
| `supply_schema_v1_patch_2026-03-27.sql` | Uses `BEGIN/COMMIT` - Good |

### 4.4 Dangerous Patterns Found

**partition_strategy_v1.sql (lines 8, 88)**:
```sql
DROP TABLE IF EXISTS audit_events CASCADE;
DROP TABLE IF EXISTS billing_ledger_entries CASCADE;
```

While `IF EXISTS` provides safety, `CASCADE` can silently drop dependent objects. Ensure application code is checked before running this migration.

**audit_events_migration_v1_to_v2.sql (line 21)**:
```sql
DROP TABLE IF EXISTS audit_events CASCADE;
```

Same concern - ensure no dependent objects rely on this table.

### 4.5 Schema Consistency Issue

**CONCERN**: Two different `audit_events` schemas exist:

1. `partition_strategy_v1.sql` - Uses columns: `tenant_id`, `project_id`, `actor_user_id`, `domain_code`, `object_type`, etc.

2. `audit_events_migration_v1_to_v2.sql` - Uses columns: `event_id`, `event_name`, `event_category`, `timestamp`, `source_ip`, etc.

3. `platform_core_schema_v1.sql` (line 174) - Defines `audit_events` with old schema

These schemas are **incompatible**. The `audit_events_migration_v1_to_v2.sql` drops and recreates with a completely different column set.

---

## 5. Specific Issues

### 5.1 Critical Issues

| ID | File | Issue | Severity |
|----|------|-------|----------|
| C1 | `supply_core_schema_v2.sql` | No CHECK constraint on `status` fields unlike v1 schema | Medium |
| C2 | `partition_strategy_v1.sql` | `DROP TABLE` without transactional safety | Medium |
| C3 | Multiple files | Inconsistent `audit_events` schema definitions | High |

### 5.2 Recommendations

1. **Add CHECK constraints** to `supply_core_schema_v2.sql` status fields:
   ```sql
   status VARCHAR(32) NOT NULL CHECK (status IN ('pending', 'active', 'suspended', 'disabled'))
   ```

2. **Review schema conflicts**: Reconcile the three different `audit_events` definitions before production deployment.

3. **Wrap partition_strategy_v1.sql in transaction** or ensure it's run as a isolated migration.

4. **Consider adding missing indexes**:
   - `supply_settlements.user_id` alone (without status/created_at combination)
   - `supply_accounts.platform` alone

### 5.3 Good Practices Observed

1. **Partial indexes** for sparse columns (idempotency_key, request_id)
2. **Optimistic locking** via `version` column consistently applied
3. **Audit fields** (request_id, trace_id, created_ip, updated_ip) consistently present
4. **Idempotent migrations** using `IF NOT EXISTS`
5. **Proper data typing** (INET for IPs, JSONB for metadata, NUMERIC for money)
6. **Soft delete pattern** via `deleted_at` in llm-gateway schema

---

## 6. File Summary

| File | Lines | Schema Version | Assessment |
|------|-------|----------------|------------|
| `supply_core_schema_v2.sql` | 144 | v2 | Good - missing CHECK constraints |
| `audit_alerts_v1.sql` | 41 | v1 | Excellent |
| `audit_events_migration_v1_to_v2.sql` | 101 | Migration | Good - includes backup |
| `iam_schema_v1.sql` | 168 | v1 | Excellent |
| `outbox_pattern_v1.sql` | 332 | v1 | Excellent |
| `partition_strategy_v1.sql` | 251 | v1 | Good - minor safety concerns |
| `platform_core_schema_v1.sql` | 204 | v1 | Excellent |
| `supply_schema_v1.sql` | 196 | v1 | Good |
| `supply_schema_v1_patch_2026-03-27.sql` | 112 | Patch | Excellent |
| `token_runtime_schema_v1.sql` | 65 | v1 | Excellent |
| llm-gateway migrations (74 files) | ~3000 total | Various | Generally Good |

---

## 7. Conclusion

The SQL schemas reviewed are generally well-designed with:
- Good index coverage for common query patterns
- Proper use of partial indexes for sparse columns
- Consistent audit field patterns
- Appropriate use of constraints

Main concerns are:
1. Inconsistent `audit_events` schema definitions across files
2. Missing CHECK constraints in v2 schema (compared to v1)
3. Transaction safety for partition DDL

**Recommended Actions**:
1. Resolve audit_events schema conflicts
2. Add CHECK constraints to supply_core_schema_v2.sql status fields
3. Document migration run order carefully

---

*Report generated: 2026-04-16*
