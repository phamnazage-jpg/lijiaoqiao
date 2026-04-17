# Architectural Analysis Report - Go Code Duplication & Architectural Issues
**Date:** 2026-04-16
**Project:** /home/long/project/立交桥
**Scope:** supply-api, gateway, platform-token-runtime, llm-gateway-competitors

---

## Executive Summary

The codebase exhibits significant architectural problems including:
1. **Duplicate error definitions** across services using incompatible error systems
2. **Duplicated middleware logic** with inconsistent patterns
3. **Inconsistent repository patterns** for similar data access patterns
4. **Fragmented SMS provider implementations** with code duplication
5. **Inconsistent batch processing abstractions** for similar high-throughput requirements
6. **Duplicate domain logic** for compensation and outbox patterns

---

## 1. Duplicate Error Definition Systems

### Problem: Two Completely Different Error Packages

#### `supply-api/pkg/error/errors.go` (133 lines)
```go
// Format: {DOMAIN}_{CODE}
type CodeError struct {
    Code    string  // e.g., "SUP_ACC_4001"
    Message string
    Err     error
}
```

#### `gateway/pkg/error/error.go` (254 lines)
```go
// Format: {CATEGORY}_{NUMBER}
type ErrorCode string  // e.g., "AUTH_001", "COMMON_003"
type GatewayError struct {
    Code      ErrorCode
    Message   string
    Details   map[string]interface{}
    RequestID string
    Internal  error
}
```

### Issues:
- **No shared error library**: Two services in the same project use completely different error handling approaches
- **Incompatible error codes**: `SUP_*` vs `AUTH_*` formats
- **Duplicated ErrorInfo mapping**: Both services maintain separate error definition maps
- **No unified error type**: Cannot easily wrap or propagate errors between services

### Files Affected:
- `/home/long/project/立交桥/supply-api/pkg/error/errors.go`
- `/home/long/project/立交桥/gateway/pkg/error/error.go`

---

## 2. Duplicated Repository Error Handling

### Problem: Inconsistent Duplicate Key Detection

#### `supply-api/internal/iam/repository/iam_repository.go` (lines 111-113)
```go
if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
    return ErrDuplicateRoleCode
}
```

#### `supply-api/internal/iam/repository/iam_repository.go` (lines 422-424)
```go
if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique constraint") {
    return ErrDuplicateAssignment
}
```

### Issues:
- **String-based error detection**: Using `strings.Contains()` instead of proper PostgreSQL error code checking
- **Inconsistent handling**: Should use `pgconn.PgError.Code == "23505"` (unique violation) instead of parsing error messages
- **Risk of i18n issues**: String matching breaks if database messages change or use different languages

### Files Affected:
- `/home/long/project/立交桥/supply-api/internal/iam/repository/iam_repository.go`

---

## 3. SMS Provider Code Duplication

### Problem: Nearly Identical SMS Service Implementations

#### `supply-api/internal/sms/aliyun_sms.go` (204 lines)
```go
type AliyunSMSService struct {
    accessKeyID     string
    accessKeySecret string
    signName        string
    endpoint        string
    enabled         bool
}
```

#### `supply-api/internal/sms/tencent_sms.go` (151 lines)
```go
type TencentSMSService struct {
    secretID      string
    secretKey     string
    appID         string
    signName      string
    region        string
    enabled       bool
}
```

### Duplicated Logic:
1. Both implement the same `SMSService` interface
2. Both have identical `IsEnabled()`, `SendVerificationCode()`, `VerifyCode()` method signatures
3. Both use nearly identical code structure for `BuildRequest()` and `SendRequest()`
4. Both have `InMemoryCodeStore` usage with `sync.RWMutex`

### Files Affected:
- `/home/long/project/立交桥/supply-api/internal/sms/sms.go` (interface definition)
- `/home/long/project/立交桥/supply-api/internal/sms/aliyun_sms.go`
- `/home/long/project/立交桥/supply-api/internal/sms/tencent_sms.go`

### Recommendation:
Create a common `BaseSMSService` with shared HTTP logic, leaving only provider-specific signature/authentication in concrete implementations.

---

## 4. Duplicate Middleware Patterns

### Problem: Separate Auth Middleware Implementations

#### `supply-api/internal/middleware/auth.go` (884 lines)
- `AuthMiddleware` with JWT verification, brute force protection, token caching
- `TokenClaims` struct with `SubjectID`, `Role`, `Scope`, `TenantID`
- `AuditEvent` struct with `EventName`, `RequestID`, `TokenID`, `SubjectID`, `Route`, `ResultCode`, `ClientIP`, `CreatedAt`

#### `gateway/internal/middleware/runtime.go`
- Separate `VerifiedToken` struct
- Separate token verification logic
- No shared audit event structure

### Duplicated Features:
1. JWT parsing and validation
2. Token status checking (revocation)
3. IP extraction (getClientIP)
4. Error response formatting

### Files Affected:
- `/home/long/project/立交桥/supply-api/internal/middleware/auth.go`
- `/home/long/project/立交桥/gateway/internal/middleware/runtime.go`

---

## 5. Inconsistent Batch Processing Patterns

### Problem: Two Different Batch Processing Abstractions

#### `supply-api/internal/audit/service/batch_buffer.go` (239 lines)
```go
type BatchBuffer struct {
    buffer       []*AuditEvent
    bufferSize   int
    flushSize    int
    flushTimeout time.Duration
    mu           sync.Mutex
    notEmpty     chan struct{}
    stopCh       chan struct{}
}
```

#### `supply-api/internal/domain/compensation.go` (392 lines)
```go
type CompensationProcessor struct {
    store           CompensationStore
    operationExecutor OperationExecutor
    stats            CompensationStats
    workerCancel     context.CancelFunc
}
```

### Issues:
- **Different batch trigger patterns**: `BatchBuffer` uses size+time triggers; `CompensationProcessor` uses ticker-based polling
- **Different flush strategies**: Buffer-based vs single-item processing with retry
- **Duplicated error handling**: Both have retry logic with backoff, max retries
- **No shared batch processing interface**

### Files Affected:
- `/home/long/project/立交桥/supply-api/internal/audit/service/batch_buffer.go`
- `/home/long/project/立交桥/supply-api/internal/domain/compensation.go`

---

## 6. Duplicate Domain Models

### Problem: Similar Outbox and Compensation Patterns

#### `supply-api/internal/domain/outbox.go` (54 lines)
```go
type OutboxRetryConfig struct {
    MaxRetries            int
    InitialBackoffSeconds int
    MaxBackoffSeconds     int
    BatchSize             int
}
```

#### `supply-api/internal/domain/compensation.go` (lines 106-110)
```go
type CompensationConfig struct {
    MaxRetries   int
    RetryInterval time.Duration
}
```

### Issues:
- **Similar retry configurations**: Both have `MaxRetries`, `InitialBackoff`, `MaxBackoff`
- **Different backoff calculation**: `CalculateOutboxBackoff()` in outbox.go vs inline retry logic in compensation
- **No shared retry/backoff abstraction**

### Files Affected:
- `/home/long/project/立交桥/supply-api/internal/domain/outbox.go`
- `/home/long/project/立交桥/supply-api/internal/domain/compensation.go`

---

## 7. Alert System Duplication

### Problem: Two Nearly Identical Alert Stores

#### `supply-api/internal/audit/service/alert_service.go` (275 lines)
```go
type InMemoryAlertStore struct {
    alerts map[string]*Alert
    mu     sync.RWMutex
    nextID int64
}
```

#### `supply-api/internal/audit/repository/alert_repository.go` (459 lines)
```go
type PostgresAlertRepository struct {
    pool *pgxpool.Pool
}
```

### Issues:
- Both implement `AlertService` with nearly identical CRUD operations
- Inconsistent method naming: `GetByID()` vs `GetAlertByID()`
- Duplicate validation logic in service layer

### Files Affected:
- `/home/long/project/立交桥/supply-api/internal/audit/service/alert_service.go`
- `/home/long/project/立交桥/supply-api/internal/audit/repository/alert_repository.go`

---

## 8. Architecture Chaos Indicators

### Service Structure Issues:

| Service | Import Path | Package Structure |
|---------|-------------|-------------------|
| supply-api | `lijiaoqiao/supply-api` | Internal packages not exported |
| gateway | `lijiaoqiao/gateway` | Internal packages not exported |
| platform-token-runtime | `lijiaoqiao/platform-token-runtime` | Mixed internal/exported |
| llm-gateway-competitors | `lijiaoqiao/llm-gateway-competitors` | Unknown structure |

### Cross-Service Communication Issues:
- No shared protobuf/gRPC contracts
- REST-based communication with no standardized error format
- No shared middleware library

---

## 9. Summary Table of Issues

| Category | Location | Issue | Impact |
|----------|----------|-------|--------|
| Error System | `supply-api/pkg/error/` vs `gateway/pkg/error/` | Two incompatible error packages | High |
| Error Handling | `iam_repository.go` | String-based duplicate key detection | Medium |
| SMS | `sms/*.go` | 3 near-identical service implementations | High |
| Auth Middleware | `supply-api/middleware/auth.go` vs `gateway/middleware/` | Duplicated auth logic | High |
| Batch Processing | `batch_buffer.go` vs `compensation.go` | Different batch abstractions | Medium |
| Domain Models | `outbox.go` vs `compensation.go` | Similar retry configs | Low |
| Alert System | `alert_service.go` vs `alert_repository.go` | Duplicate alert stores | Medium |

---

## 10. Recommendations

### Priority 1 (Critical):
1. **Unify error handling**: Create a shared `pkg/errors` package with common error types
2. **Fix duplicate key detection**: Use `pgconn.PgError.Code` instead of string matching
3. **Consolidate SMS providers**: Extract common HTTP logic to base class

### Priority 2 (High):
1. **Share middleware library**: Create `lijiaoqiao/middleware` package for common auth, rate limiting, CORS
2. **Unify batch processing**: Create common `BatchProcessor` interface with configurable strategies
3. **Extract audit events**: Share `AuditEvent` struct across services

### Priority 3 (Medium):
1. **Standardize repository patterns**: Use consistent error handling and naming
2. **Extract retry logic**: Create shared exponential backoff utility
3. **Document service contracts**: Define clear interfaces between services

---

## Files Referenced in Analysis

### supply-api/
- `pkg/error/errors.go` - Error handling (133 lines)
- `internal/iam/repository/iam_repository.go` - IAM repository (610 lines)
- `internal/sms/sms.go` - SMS interface (201 lines)
- `internal/sms/aliyun_sms.go` - Aliyun SMS (204 lines)
- `internal/sms/tencent_sms.go` - Tencent SMS (151 lines)
- `internal/middleware/auth.go` - Auth middleware (884 lines)
- `internal/audit/service/alert_service.go` - Alert service (275 lines)
- `internal/audit/service/batch_buffer.go` - Batch buffer (239 lines)
- `internal/domain/compensation.go` - Compensation (392 lines)
- `internal/domain/outbox.go` - Outbox (54 lines)
- `internal/app/runtime.go` - App runtime (533 lines)

### gateway/
- `pkg/error/error.go` - Error handling (254 lines)
- `internal/middleware/runtime.go` - Runtime middleware

### platform-token-runtime/
- `internal/httpapi/token_api.go` - Token API (449 lines)

---

*Report generated by architectural analysis tool*
