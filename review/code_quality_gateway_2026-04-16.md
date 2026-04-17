# Gateway Microservice Code Quality & Security Review Report

**Project:** lijiaoqiao/gateway  
**Review Date:** 2026-04-16  
**Reviewer:** Hermes Agent (Code Quality & Security Audit)  
**Go Version:** 1.21  

---

## 1. Executive Summary

The gateway microservice is an OpenAI-compatible API gateway handling authentication, rate limiting, upstream routing, and audit logging. Overall code quality is **acceptable** with some security concerns that need addressing. All tests pass (17 packages).

**Key Findings:**
- **Critical Issues:** 2 (hardcoded encryption key, CORS wildcard in production)
- **High Issues:** 4 (request ID trust, trusted proxies config, error message exposure, missing security headers)
- **Medium Issues:** 5
- **Low Issues:** 4

---

## 2. Module Analysis

### 2.1 Configuration Module (`internal/config/config.go`)

**Strengths:**
- Proper separation of concerns with distinct config structs
- Environment variable-based configuration
- Validation for auth config (prevents `inmemory` mode in prod/staging)
- Password encryption with AES-GCM

**Issues:**

| Severity | Issue | Location | Description |
|----------|-------|----------|-------------|
| 🔴 Critical | Hardcoded encryption key | Line 18 | `encryptionKey = []byte(getEnv("PASSWORD_ENCRYPTION_KEY", "default-key-32-bytes-long!!!!!!!"))` - Falls back to an insecure default. In production, if `PASSWORD_ENCRYPTION_KEY` is not set, encrypted passwords can be trivially decrypted. |
| 🟡 Medium | Weak default provider credentials | Lines 205-215 | Default OpenAI provider is created with potentially empty API key from environment variable `OPENAI_API_KEY` |

**Recommendations:**
1. Fail startup if `PASSWORD_ENCRYPTION_KEY` is not explicitly set in non-dev environments
2. Add validation to reject empty API keys for providers

---

### 2.2 Handler Module (`internal/handler/handler.go`)

**Strengths:**
- Request body size limiting via `maxBytesReader` (1MB limit)
- Proper request ID generation and propagation
- Context usage for request tracking
- Separate handling for streaming vs non-streaming responses

**Issues:**

| Severity | Issue | Location | Description |
|----------|-------|----------|-------------|
| 🟡 Medium | Request ID header trusted | Line 61 | `requestID := r.Header.Get("X-Request-ID")` - User-supplied request ID is used directly without sanitization, which could lead to log injection attacks |
| 🟡 Medium | Error message leakage | Lines 77-78 | `err.Error()` exposed in `COMMON_INVALID_REQUEST` - Internal error details leaked to clients |
| 🟢 Low | Missing security headers | Throughout | No `X-Content-Type-Options`, `X-Frame-Options`, `Content-Security-Policy` headers |

**Recommendations:**
1. Sanitize or regenerate request IDs instead of trusting client-supplied values
2. Use generic error messages for internal errors; log detailed errors server-side only

---

### 2.3 Router Module (`internal/router/router.go`)

**Strengths:**
- Multiple load balancing strategies (latency, round-robin, weighted, availability)
- Thread-safe with proper mutex usage
- Health tracking with exponential moving average for latency
- Graceful failure recovery

**Issues:**

| Severity | Issue | Location | Description |
|----------|-------|----------|-------------|
| 🟡 Medium | Global random source not CSPRNG | Line 16 | `rand.New(rand.NewSource(time.Now().UnixNano()))` - Using time-based seed for weighted selection; not cryptographically secure, though not critical for load balancing |
| 🟢 Low | Hardcoded "primary" provider name | Lines 203-204 | `if name == "primary"` - Hardcoded logic that could silently skip fallback providers if named differently |

**Recommendations:**
1. Use `crypto/rand` for weighted selection if used for security-sensitive decisions
2. Make fallback selection configurable rather than hardcoded

---

### 2.4 Middleware Module

#### 2.4.1 CORS (`internal/middleware/cors.go`)

**Strengths:**
- Proper preflight handling
- Domain wildcard matching with `*.example.com` support
- Configurable credentials and max age

**Issues:**

| Severity | Issue | Location | Description |
|----------|-------|----------|-------------|
| 🔴 Critical | CORS wildcard allowed | Line 23 | `AllowOrigins: []string{"*"}` in `DefaultCORSConfig()` - Wildcard origin allows any website to make cross-origin requests. Comment says "生产环境应限制具体域名" but default allows all. |

**Recommendations:**
1. Default to restrictive CORS (no wildcard) and require explicit configuration for allowed origins
2. Add validation to reject wildcard in production environments

---

#### 2.4.2 Authentication Chain (`internal/middleware/chain.go`)

**Strengths:**
- Comprehensive auth flow: verification → status resolution → authorization
- Proper audit event emission for all auth outcomes
- Query key rejection middleware (prevents API key in URL)
- IP extraction with trusted proxy support

**Issues:**

| Severity | Issue | Location | Description |
|----------|-------|----------|-------------|
| 🔴 High | Trusted proxies not configured | Line 58 (bootstrap.go) | `TrustedProxies` is never set in `BuildServer()`, meaning `extractClientIP()` will always use `RemoteAddr` even when behind a reverse proxy. If deployed behind a proxy without setting trusted proxies, client IPs are untrusted. |
| 🟡 Medium | Context value mutation | Line 242 | `*r = *r.WithContext(ctx)` - Mutates the request object directly, which can cause issues in concurrent scenarios |

---

#### 2.4.3 Remote Token Runtime (`internal/middleware/remote_runtime.go`)

**Strengths:**
- Proper token caching with expiration tracking
- Context-aware HTTP calls

**Issues:**

| Severity | Issue | Location | Description |
|----------|-------|----------|-------------|
| 🟡 Medium | Missing timeout on HTTP client | Lines 70-73 | HTTP request to token runtime has no explicit timeout; uses `http.DefaultClient` with default timeout |

---

### 2.5 Rate Limit Module (`internal/ratelimit/ratelimit.go`)

**Strengths:**
- Multiple algorithms (token bucket, sliding window)
- Proper cleanup of inactive buckets
- Context-aware limiting

**Issues:**

| Severity | Issue | Location | Description |
|----------|-------|----------|-------------|
| 🟢 Low | Rate limit by token may be bypassed | Line 233-235 | `AllowToken` falls back to `Allow` for sliding window, ignoring actual token count for TPM limiting |

---

### 2.6 Compliance Rules Engine (`internal/compliance/rules/engine.go`)

**Strengths:**
- Regex caching for performance
- Thread-safe with double-checked locking
- Clear error types

**Issues:**

| Severity | Issue | Location | Description |
|----------|-------|----------|-------------|
| 🟢 Low | No regex complexity limit | Lines 97, 128 | `regexp.Compile(pattern)` without complexity limits; vulnerable to ReDoS if user-provided patterns are used |

---

### 2.7 App Bootstrap (`internal/app/bootstrap.go`)

**Strengths:**
- Proper server lifecycle management
- Clean separation of concerns
- Default value normalization

**Issues:**

| Severity | Issue | Location | Description |
|----------|-------|----------|-------------|
| 🟡 Medium | Missing TrustedProxies config | Line 58 | `TrustedProxies` is never populated, breaking IP-based rate limiting in proxy environments |

---

### 2.8 Adapter Module (`internal/adapter/`)

**Strengths:**
- Clean abstraction with `ProviderAdapter` interface
- Proper error mapping to provider-specific errors
- Streaming support with proper context cancellation

**Issues:**

| Severity | Issue | Location | Description |
|----------|-------|----------|-------------|
| 🟡 Medium | Hardcoded 60s timeout | Line 28 (openai_adapter.go) | HTTP client timeout is hardcoded to 60s; should be configurable |
| 🟡 Medium | Response body not limited | Line 76 (openai_adapter.go) | `io.ReadAll(resp.Body)` reads entire response into memory; no limit for large responses |
| 🟡 Medium | Custom contains implementation | Lines 280-291 | Uses custom `contains()` instead of `strings.Contains()`, which is less readable and potentially slower |

---

### 2.9 Error Package (`pkg/error/error.go`)

**Strengths:**
- Well-organized error codes by category
- Structured error with details and request ID
- Comprehensive error definitions with HTTP status codes

**Issues:** None significant

---

## 3. Security Assessment

### 3.1 Authentication & Authorization
- ✅ Bearer token authentication implemented
- ✅ Token introspection with status tracking
- ✅ Scope-based authorization
- ⚠️ `inmemory` mode blocked for prod/staging (good)
- ⚠️ Trusted proxies configuration missing

### 3.2 Input Validation
- ✅ Request body size limited to 1MB
- ✅ Query key rejection prevents API keys in URLs
- ⚠️ Request ID trusted from client
- ⚠️ No input sanitization for log injection

### 3.3 Secrets Management
- 🔴 Hardcoded encryption key fallback
- ⚠️ No KMS integration
- ⚠️ API keys passed via environment variables (acceptable)

### 3.4 CORS
- 🔴 Default allows all origins (`*`)
- ✅ Preflight handling correct
- ⚠️ No production restriction enforcement

### 3.5 Rate Limiting
- ✅ Implemented per-client
- ✅ Multiple algorithms available
- ⚠️ Token-based limiting incomplete

### 3.6 Audit Logging
- ✅ Comprehensive audit events
- ✅ PostgreSQL and memory backends
- ⚠️ DSN password masked in logs (but password passed directly in connection string)

### 3.7 Transport Security
- ⚠️ No TLS configuration shown
- ⚠️ Hardcoded HTTPS base URLs not enforced

---

## 4. Test Coverage

All 17 packages pass tests:
```
ok  lijiaoqiao/gateway/cmd/gateway        0.003s
ok  lijiaoqiao/gateway/internal/adapter    10.013s
ok  lijiaoqiao/gateway/internal/alert     0.004s
ok  lijiaoqiao/gateway/internal/app        0.003s
ok  lijiaoqiao/gateway/internal/compliance/rules  0.006s
ok  lijiaoqiao/gateway/internal/config     0.002s
ok  lijiaoqiao/gateway/internal/handler    0.011s
ok  lijiaoqiao/gateway/internal/middleware 0.005s
ok  lijiaoqiao/gateway/internal/ratelimit 0.003s
ok  lijiaoqiao/gateway/internal/router     0.003s
ok  lijiaoqiao/gateway/internal/router/engine  0.003s
ok  lijiaoqiao/gateway/internal/router/fallback 0.003s
ok  lijiaoqiao/gateway/internal/router/metrics  0.003s
ok  lijiaoqiao/gateway/internal/router/scoring  0.003s
ok  lijiaoqiao/gateway/internal/router/strategy 0.003s
ok  lijiaoqiao/gateway/pkg/error           0.003s
ok  lijiaoqiao/gateway/pkg/model           0.002s
```

**Coverage Assessment:** Tests exist for all major modules, but security-focused test files (`*_security_test.go`) should be reviewed to ensure they cover edge cases.

---

## 5. Summary of Issues by Severity

### 🔴 Critical (Must Fix)
1. **Hardcoded encryption key fallback** (`config.go:18`) - Undermines entire password encryption mechanism
2. **CORS wildcard default** (`cors.go:23`) - Allows any origin to make cross-origin requests

### 🟠 High (Should Fix)
3. **TrustedProxies not configured** (`bootstrap.go`) - Breaks IP-based controls behind proxies
4. **RemoteAddr trust in production** (`chain.go`) - IP extraction needs proper proxy configuration

### 🟡 Medium (Consider Fixing)
5. Request ID from client not sanitized (`handler.go:61`)
6. Internal error messages leaked to clients (`handler.go:78`)
7. HTTP client missing timeout for token introspection (`remote_runtime.go`)
8. Hardcoded 60s adapter timeout (`openai_adapter.go:28`)
9. Response body reading without size limit (`openai_adapter.go:76`)

### 🟢 Low (Nice to Have)
10. Custom `contains()` instead of `strings.Contains()`
11. Time-based random seed for weighted routing
12. Missing security headers (X-Content-Type-Options, etc.)
13. ReDoS potential in regex patterns

---

## 6. Recommendations

### Immediate Actions Required:
1. **Remove hardcoded encryption key fallback** - Fail startup if `PASSWORD_ENCRYPTION_KEY` is not set in non-dev environments
2. **Fix CORS default** - Default to restrictive config, require explicit allowed origins list
3. **Add TrustedProxies configuration** - Ensure proper IP extraction behind reverse proxies

### Short-term Improvements:
4. Sanitize or regenerate request IDs instead of trusting client input
5. Add request/response size limits to adapter
6. Configure HTTP timeouts for all outbound calls
7. Add security headers to all responses

### Long-term Enhancements:
8. Integrate with proper KMS for secret management
9. Add comprehensive integration tests for security flows
10. Implement request/response logging with PII handling
11. Add metrics and alerting for security events

---

## 7. Conclusion

The gateway microservice demonstrates good architectural decisions with proper separation of concerns, comprehensive error handling, and multiple redundancy strategies. However, **two critical security issues require immediate attention** (hardcoded encryption key and CORS wildcard) before production deployment. The missing TrustedProxies configuration will cause issues in typical production environments behind load balancers.

Overall assessment: **NOT PRODUCTION-READY** without fixes for critical issues.

---

*Report generated by Hermes Agent on 2026-04-16*
