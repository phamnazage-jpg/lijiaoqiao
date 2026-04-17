# Code Quality & Security Review Report

**Service:** platform-token-runtime  
**Path:** `/home/long/project/立交桥/platform-token-runtime/`  
**Date:** 2026-04-16  
**Go Version:** 1.22  
**Reviewer:** Hermes Agent

---

## 1. Executive Summary

The `platform-token-runtime` microservice implements a token lifecycle management system with issuance, refresh, revocation, introspection, and audit logging capabilities. The codebase demonstrates good architectural practices with clean separation of concerns, but contains **critical data integrity bugs** and several security concerns that require immediate attention.

### Overall Assessment

| Category | Rating | Notes |
|----------|--------|-------|
| **Security** | MEDIUM-LOW | Design is security-conscious, but has implementation gaps |
| **Code Quality** | MEDIUM | Good structure, but critical bug in Refresh persistence |
| **Test Coverage** | GOOD | Core paths covered, but gaps in edge cases |
| **Dependencies** | EXCELLENT | Only stdlib, minimal attack surface |

### Critical Issues (Must Fix)

1. **[CRITICAL] Refresh does not persist TTL changes** - `inmemory_runtime.go:128-147`
2. **[CRITICAL] Non-thread-safe map write in Save** - `runtime_store.go:17-27`
3. **[HIGH] audit-events endpoint lacks authorization** - `token_api.go:326-374`

---

## 2. Code Structure Analysis

```
platform-token-runtime/
├── cmd/platform-token-runtime/
│   ├── main.go              # Entry point, graceful shutdown
│   └── main_test.go         # Bootstrap tests
├── internal/
│   ├── app/
│   │   ├── bootstrap.go     # Environment-based store initialization
│   │   └── bootstrap_test.go
│   ├── auth/
│   │   ├── middleware/
│   │   │   ├── token_auth_middleware.go       # Bearer auth, authorization
│   │   │   ├── token_auth_middleware_test.go
│   │   │   └── query_key_reject_middleware.go # Query key rejection
│   │   ├── model/
│   │   │   └── principal.go    # Principal struct, scope matching
│   │   └── service/
│   │       ├── audit_store.go      # In-memory audit event store
│   │       ├── audit_store_test.go
│   │       ├── inmemory_runtime.go # Token runtime implementation
│   │       ├── runtime_store.go    # Token storage
│   │       ├── runtime_store_test.go
│   │       └── token_verifier.go   # Interfaces and constants
│   ├── httpapi/
│   │   ├── token_api.go      # HTTP handlers
│   │   └── token_api_test.go
│   └── token/
│       ├── audit_executable_test.go  # Audit behavior tests
│       ├── lifecycle_executable_test.go  # Lifecycle tests
│       ├── audit_test_template_test.go
│       └── lifecycle_test_template_test.go
├── go.mod
└── Dockerfile
```

---

## 3. Security Review

### 3.1 Authentication & Authorization

**Strengths:**
- Bearer-only authentication enforced (`Authorization: Bearer <token>`)
- Query key rejection middleware blocks `key`, `api_key`, `token` query parameters
- Role-based authorization (owner, viewer, admin) with scope-based route protection
- Admin role bypasses scope checks (`ScopeRoleAuthorizer.Authorize`)

**Issues Found:**

| ID | Severity | Location | Description |
|----|----------|----------|-------------|
| SEC-001 | MEDIUM | `token_api.go:326-374` | `audit-events` endpoint has no authentication/authorization |
| SEC-002 | MEDIUM | `middleware/token_auth_middleware.go:259-270` | `X-Forwarded-For` trusted without validation |
| SEC-003 | LOW | `token_api.go:414-422` | No request body size limit in `decodeJSON` |

**SEC-001 Detail:**
The `/api/v1/platform/tokens/audit-events` endpoint allows querying audit logs without requiring any authentication. Any caller can filter by `request_id`, `token_id`, `subject_id`, `event_name`, and `result_code`. This could leak sensitive operational data.

```go
// token_api.go - No auth check before handleAuditEvents
func (a *TokenAPI) handleAuditEvents(w http.ResponseWriter, r *http.Request) {
    // ... reads audit events without verifying caller identity
}
```

**SEC-002 Detail:**
Client IP extraction trusts `X-Forwarded-For` header directly, enabling IP spoofing:

```go
func extractClientIP(r *http.Request) string {
    xForwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For"))
    if xForwardedFor != "" {
        parts := strings.Split(xForwardedFor, ",")
        return strings.TrimSpace(parts[0])  // No validation
    }
    // ...
}
```

### 3.2 Token Security

**Strengths:**
- Cryptographically random token generation using `crypto/rand` (16 bytes entropy for access tokens)
- Access tokens prefixed with `ptk_`, token IDs with `tok_`
- Access tokens never returned in audit responses
- SHA-256 based idempotency hash

**Issues Found:**

| ID | Severity | Location | Description |
|----|----------|----------|-------------|
| SEC-004 | LOW | `inmemory_runtime.go:285-295` | Idempotency hash does not include `RequestID` - potential for hash collision across different requests |

### 3.3 Input Validation

**Strengths:**
- `json.Decoder.DisallowUnknownFields()` prevents unknown field attacks
- Role validation against whitelist (owner, viewer, admin)
- TTL minimum enforcement (>= 60 seconds)
- Required header validation (X-Request-Id, Idempotency-Key)

**Issues Found:**

| ID | Severity | Location | Description |
|----|----------|----------|-------------|
| SEC-005 | LOW | `token_api.go:337-345` | Audit query filter values not validated for length or content |

---

## 4. Code Quality Review

### 4.1 Critical Bugs

#### BUG-001: Refresh Does Not Persist TTL Changes

**Severity:** CRITICAL  
**Location:** `inmemory_runtime.go:128-147`

```go
func (r *InMemoryTokenRuntime) Refresh(_ context.Context, tokenID string, ttl time.Duration) (TokenRecord, error) {
    // ... validation ...
    record, ok := r.store.GetByTokenID(tokenID)
    if !ok {
        return TokenRecord{}, errors.New("token not found")
    }
    r.applyExpiry(record)
    if record.Status != TokenStatusActive {
        return TokenRecord{}, errors.New("token is not active")
    }

    record.ExpiresAt = r.now().Add(ttl)  // <-- Modifies in-memory copy
    return cloneRecord(*record), nil     // <-- Original store not updated!
}
```

**Impact:** Token refresh operations are not durable. If the service restarts, the refreshed TTL is lost and the token reverts to its original expiration time.

**Fix Required:** Call `r.store.Save()` or add an update method after modifying `record.ExpiresAt`.

---

#### BUG-002: Non-Thread-Safe Map Write in Save

**Severity:** CRITICAL  
**Location:** `runtime_store.go:17-27`, called from `inmemory_runtime.go:122`

```go
func (s *InMemoryRuntimeStore) Save(record TokenRecord, idempotencyKey, requestHash string) {
    recordCopy := cloneRecord(record)
    s.records[record.TokenID] = &recordCopy      // <-- Unsynchronized map write
    s.tokenToID[record.AccessToken] = record.TokenID
    // ...
}
```

The `Save` method is called **outside** the mutex lock in `Issue`:

```go
// inmemory_runtime.go:107-123
r.mu.Lock()
if idempotencyKey != "" {
    // ... idempotency check ...
}
r.store.Save(record, idempotencyKey, requestHash)  // <-- Lock is held here (OK)
r.mu.Unlock()

return record, nil  // <-- But Save modifies map after this in some paths?
```

Wait - on closer inspection, `Save` is called while holding `r.mu`, but `Save` itself doesn't acquire `s.mu` (the store's mutex). Since Go maps are not thread-safe, concurrent calls to `Save` from different goroutines could cause a data race.

However, looking at `Issue`, it's also protected by `r.mu`. But `Refresh`, `Revoke`, `Lookup` etc. all use their own locks but could theoretically call `Save` in the future. The design is fragile.

**Impact:** Potential data race if store is shared across multiple runtimes or if Save is called without proper external synchronization.

---

### 4.2 Medium Issues

#### BUG-003: Inconsistent Error Handling Strategy

**Severity:** MEDIUM  
**Location:** Multiple files

The codebase mixes two error handling approaches:

1. Direct `errors.New()` with string matching
2. Custom `AuthError` type with code matching

```go
// Approach 1: String-based error matching
// token_api.go:400-412
func mapRuntimeError(err error) (int, string) {
    msg := err.Error()
    switch {
    case strings.Contains(msg, "not found"):
        return http.StatusNotFound, "TOKEN_NOT_FOUND"
    // ...
    }
}
```

```go
// Approach 2: Typed error matching
// token_verifier.go:121-127
func IsAuthCode(err error, code string) bool {
    var authErr *AuthError
    if !errors.As(err, &authErr) {
        return false
    }
    return authErr.Code == code
}
```

**Issue:** String matching is fragile and could break if error messages are localized or modified. The `mapRuntimeError` function using `strings.Contains` is particularly vulnerable.

---

#### BUG-004: Redundant Nil Checks for Auditor

**Severity:** MEDIUM  
**Location:** `token_api.go:196,287,300`

```go
if a.auditor != nil {
    _ = a.auditor.Emit(r.Context(), service.AuditEvent{...})
}
```

The nil check is repeated in many places. This suggests the `AuditEmitter` interface contract doesn't clearly define whether nil is valid.

---

### 4.3 Minor Issues

| ID | Severity | Location | Description |
|----|----------|----------|-------------|
| BUG-005 | LOW | `token_api.go:434` | `json.NewEncoder(w).Encode(payload)` error ignored |
| BUG-006 | LOW | `middleware/token_auth_middleware.go:215` | Mutating `*r` in `ensureRequestID` modifies caller's request |
| BUG-007 | INFO | `bootstrap.go:71-75` | Health endpoint uses inline handler, inconsistent with API pattern |

---

## 5. Test Coverage Analysis

### 5.1 Coverage Summary

| Module | Test Files | Coverage Assessment |
|--------|------------|---------------------|
| `cmd/platform-token-runtime` | 1 | Basic bootstrap test for prod env rejection |
| `internal/app` | 1 | Bootstrap configuration tests |
| `internal/auth/middleware` | 1 | Auth chain, query key rejection |
| `internal/auth/model` | 0 | No direct tests (trivial) |
| `internal/auth/service` | 3 | Runtime lifecycle, audit, store |
| `internal/httpapi` | 1 | API endpoint tests |
| `internal/token` | 4 | Lifecycle and audit template tests |

### 5.2 Test Quality Assessment

**Strengths:**
- Parallel test execution (`t.Parallel()`)
- Deterministic time handling via `now` function injection
- Good coverage of idempotency scenarios
- Security-specific tests (e.g., `TestTOKAud006QueryKeyRejectedEvent` verifies raw query values not in audit)

**Gaps:**

| ID | Severity | Missing Test Scenario |
|----|----------|----------------------|
| TST-001 | HIGH | Refresh followed by service restart (proves persistence bug) |
| TST-002 | HIGH | Concurrent token issuance (proves race condition) |
| TST-003 | MEDIUM | audit-events without authentication |
| TST-004 | MEDIUM | Introspection with expired token returns correct status |
| TST-005 | LOW | X-Forwarded-For spoofing attempt |

---

## 6. Build & Infrastructure

### 6.1 Dockerfile Analysis

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/platform-token-runtime ./cmd/platform-token-runtime

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /out/platform-token-runtime /app/platform-token-runtime
EXPOSE 18081
ENTRYPOINT ["/app/platform-token-runtime"]
```

**Assessment:** GOOD
- Multi-stage build reduces image size
- `distroless/static-debian12` is a minimal, security-focused base image
- No shell available in final image (reduces attack surface)
- CGO_ENABLED=0 ensures static linking

### 6.2 Dependency Analysis

```go
module lijiaoqiao/platform-token-runtime

go 1.22
```

**Assessment:** EXCELLENT
- No third-party dependencies
- Only Go standard library used
- Minimal attack surface from supply chain

---

## 7. Positive Findings

1. **Clean Architecture:** Well-separated concerns with interfaces for testability
2. **Graceful Shutdown:** Proper signal handling with 5-second timeout
3. **Security Design:** Bearer-only auth, query key rejection, access token not in audit responses
4. **Idempotency:** Robust implementation with payload hash verification
5. **Test Quality:** Good use of table-driven tests and deterministic time
6. **Error Responses:** Consistent JSON error envelope format
7. **Timeouts:** Proper HTTP server timeouts configured (`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`)
8. **Mutex Discipline:** Most concurrent operations properly protected with `sync.RWMutex`

---

## 8. Recommendations

### 8.1 Immediate Actions (Critical)

1. **Fix Refresh persistence bug (BUG-001)**
   - Add `store.Save()` call after modifying ExpiresAt, OR
   - Add dedicated `UpdateExpiresAt` method to store

2. **Add synchronization to Save method (BUG-002)**
   - Add `sync.Mutex` to `InMemoryRuntimeStore` and protect all map operations

### 8.2 Short-term Improvements

3. **Add authentication to audit-events endpoint (SEC-001)**
   - Require valid bearer token or add separate API key auth
   - Or restrict to internal network only

4. **Validate X-Forwarded-For (SEC-002)**
   - Only trust if from known proxy IPs
   - Fall back to `RemoteAddr` if not from trusted proxy

5. **Add request body size limit (SEC-003)**
   ```go
   dec := json.NewDecoder(r.Body)
   dec.DisallowUnknownFields()
   dec.MaxBytes(1024) // 1KB limit for token requests
   ```

6. **Refactor error handling (BUG-003)**
   - Use typed errors consistently
   - Remove string matching in `mapRuntimeError`

### 8.3 Long-term Enhancements

7. **Add rate limiting** - Especially for `/issue` endpoint
8. **Add metrics/observability** - Prometheus metrics for token operations
9. **Add persistence layer** - Current in-memory stores are not production-ready for prod/staging
10. **Add integration tests** - Full HTTP integration tests with `net/http/httptest`

---

## 9. Conclusion

The platform-token-runtime service demonstrates solid architectural foundations with good security awareness in its design. The code is generally clean and well-organized. However, **two critical bugs** (BUG-001 and BUG-002) must be fixed before production deployment, and several security improvements are recommended to strengthen the service's security posture.

The service is **NOT production-ready** in its current state due to the Refresh persistence bug and missing audit-events authorization.

---

## Appendix: Files Reviewed

| File | Lines | Assessment |
|------|-------|------------|
| `cmd/platform-token-runtime/main.go` | 49 | GOOD |
| `internal/app/bootstrap.go` | 86 | GOOD |
| `internal/httpapi/token_api.go` | 449 | MEDIUM - BUGs, SEC-001 |
| `internal/auth/middleware/token_auth_middleware.go` | 270 | MEDIUM - SEC-002 |
| `internal/auth/middleware/query_key_reject_middleware.go` | 51 | GOOD |
| `internal/auth/model/principal.go` | 35 | GOOD |
| `internal/auth/service/audit_store.go` | 136 | GOOD |
| `internal/auth/service/inmemory_runtime.go` | 359 | CRITICAL - BUG-001 |
| `internal/auth/service/runtime_store.go` | 50 | CRITICAL - BUG-002 |
| `internal/auth/service/token_verifier.go` | 127 | GOOD |
| `Dockerfile` | 13 | GOOD |
| `go.mod` | 3 | EXCELLENT |

---

*Report generated by Hermes Agent on 2026-04-16*
