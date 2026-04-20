# Architecture Review: 立交桥 Project
**Date:** 2026-04-18  
**Reviewer:** Hermes Agent  
**Scope:** Service Architecture, Layering, Module Boundaries, Interface Consistency, Configuration Management, Extensibility

---

## 1. Service Architecture Overview

### 1.1 Services Identified

| Service | Module Path | Port | Go Version | Primary Responsibility |
|---------|------------|------|------------|----------------------|
| gateway | `lijiaoqiao/gateway` | 8080 | 1.21 | OpenAI-compatible API gateway, request routing, rate limiting |
| supply-api | `lijiaoqiao/supply-api` | 18082 | 1.21 | Supply chain business operations (accounts, packages, settlements, earnings) |
| platform-token-runtime | `lijiaoqiao/platform-token-runtime` | 18081 | 1.22 | Token lifecycle management (issue, refresh, revoke, introspect) |

### 1.2 Service Communication Pattern

```
                    ┌─────────────────┐
                    │   Client/SDK    │
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │    gateway      │  (Port 8080)
                    │  - Auth verify  │
                    │  - Rate limit   │
                    │  - Proxy to LLM │
                    └────────┬────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
┌───────▼───────┐  ┌────────▼────────┐  ┌──────▼──────┐
│platform-token │  │   supply-api    │  │  LLM Provider│
│   -runtime   │  │   -18082        │  │  (External)  │
│   -18081     │  │  Business logic │  └──────────────┘
│ Token mgmt   │  │  Domain-driven  │
└───────────────┘  └─────────────────┘
```

**Key Observation:** Services communicate via HTTP over localhost. No message queue or gRPC observed in current architecture.

---

## 2. Service Layering Analysis

### 2.1 gateway - Thin Proxy Layer

**Layering:** 2-tier (HTTP Handler → Backend Proxy)

```
HTTP Handler (handler.go)
    │
    ▼
Middleware Chain (Auth → RateLimit → CORS)
    │
    ▼
Router (router.go) → Provider Adapters (adapter/)
```

**Strengths:**
- Minimal business logic, focused on cross-cutting concerns
- Clean middleware chain composition
- Strategy pattern for load balancing

**Issues:**
- `validateStartupSecurity()` in bootstrap.go uses string comparison for env detection (`isProductionEnv`) - potential edge cases
- Default CORS allows `*` in non-production check (`usesWildcardCORS`) - security gap in validation logic

### 2.2 supply-api - Rich Domain Layer

**Layering:** 4-tier (HTTP → Service → Repository → Storage)

```
HTTP Handler (httpapi/) ──────► Domain Service (domain/)
        │                            │
        ▼                            ▼
Middleware Chain (middleware/)  Repository (repository/)
        │                            │
        ▼                            ▼
  Health Handler              PostgreSQL / Redis
```

**Strengths:**
- Clean domain-driven design with bounded contexts
- Outbox pattern for eventual consistency (`outbox/`)
- Event-driven compensation for failed transactions (`compensation/`)
- Audit trail with DB-backed store (`audit/`)
- Idempotency middleware preventing duplicate operations

**Issues:**
- Large internal package count (21 subdirectories) suggests potential over-structuring
- Coupling between `runtime.go` and many internal packages

### 2.3 platform-token-runtime - Token Operations Layer

**Layering:** 3-tier (HTTP API → Runtime Service → Store)

```
HTTP Handler (httpapi.TokenAPI)
    │
    ▼
Runtime (service.InMemoryTokenRuntime)
    │
    ▼
Store (RuntimeStore / AuditStore interface)
```

**Strengths:**
- Interface-based store abstraction enables different backends
- Clear separation between token operations and persistence
- Audit events emitted consistently

**Issues:**
- Health endpoint uses different path: `/actuator/health` vs `/health` in other services

---

## 3. Module Boundary Analysis

### 3.1 Module Independence

| Module | Depends On | Coupling Type |
|--------|-----------|---------------|
| gateway | None (uses HTTP calls to upstream) | Low - runtime dependency only |
| supply-api | None explicit | Low - self-contained |
| platform-token-runtime | None explicit | Low - self-contained |

**Observation:** No compile-time inter-module dependencies. Each service is independently deployable.

### 3.2 Internal Package Organization

**supply-api (most mature structure):**
```
internal/
├── adapter/       # External service adapters (SMS, etc.)
├── app/          # Application bootstrap and wiring
├── audit/        # Audit logging (store + service)
├── cache/        # Redis cache abstraction
├── compensation/ # Saga compensation logic
├── config/       # Configuration loading
├── domain/       # Domain models and service interfaces
├── httpapi/      # HTTP handlers
├── iam/          # Identity & access management
├── messaging/    # Message queue integration
├── middleware/   # HTTP middleware chain
├── outbox/       # Transactional outbox pattern
├── pkg/          # Shared utilities (logging, etc.)
├── repository/   # Data access layer
├── security/     # Security utilities
├── sms/          # SMS verification
└── storage/      # Storage abstractions
```

**gateway:**
```
internal/
├── adapter/      # LLM provider adapters
├── app/          # Bootstrap
├── compliance/   # Compliance checks
├── config/       # Configuration
├── handler/      # HTTP handlers
├── middleware/   # Auth, CORS, rate limiting
├── ratelimit/    # Rate limiting implementation
└── router/       # Load balancing router
```

**Issues:**
- `internal/pkg/logging` in supply-api vs no shared logging in gateway - inconsistency
- Domain package in supply-api contains both models and service interfaces - potential SOLID violation (ISP)

---

## 4. Interface Consistency Analysis

### 4.1 API Response Format

**gateway:**
```json
{
  "request_id": "uuid",
  "data": { ... }
}
```

**supply-api:**
```json
{
  "request_id": "uuid",
  "data": { ... },
  "pagination": { ... }  // for list endpoints
}
```

**platform-token-runtime:**
```json
{
  "request_id": "uuid",
  "data": { ... }
}
```

**Finding:** Inconsistent pagination response structure. supply-api has dedicated pagination block, others don't.

### 4.2 Error Response Format

**supply-api (httpapi/errors.go pattern):**
```json
{
  "error": {
    "code": "CODE_NAME",
    "message": "human readable"
  }
}
```

**platform-token-runtime:**
```json
{
  "error": {
    "code": "CODE",
    "message": "message"
  }
}
```

**Finding:** Error envelope structure is consistent, but error code naming conventions differ (`CodeName` vs `CODE`).

### 4.3 Health Endpoint Inconsistency

| Service | Health Path | Probe Type |
|---------|-------------|-----------|
| gateway | `/health`, `/healthz`, `/readyz` | Simple 200 OK |
| supply-api | `/health` (custom handler) | DB + Redis checks |
| platform-token-runtime | `/actuator/health` | Simple 200 OK |

**Issue:** Token runtime uses Spring Boot-style path, others use Kubernetes-style paths.

### 4.4 Route Registration Pattern

**supply-api:**
```go
func (a *SupplyAPI) Register(mux *http.ServeMux)
```

**platform-token-runtime:**
```go
func (a *TokenAPI) Register(mux *http.ServeMux)
```

**Finding:** Consistent `Register(*http.ServeMux)` pattern across services.

---

## 5. Configuration Management Analysis

### 5.1 Configuration Strategy

| Service | Format | Loading Method | Env Support |
|---------|--------|----------------|-------------|
| gateway | YAML | Direct parse | Partial (env vars) |
| supply-api | YAML + Viper | `config.LoadFromPath()` | Full (env vars via Viper) |
| platform-token-runtime | Code defaults | Direct struct | Minimal |

### 5.2 Configuration Issues

**CRITICAL BUG - supply-api/internal/config/config.go line 112-113:**

```go
return fmt.Sprintf("postgres://%s:***@%s:%d/%s?sslmode=disable",
    d.User, d.Password, d.Host, d.Port, d.Database)
```

**Problem:** Format string has 4 placeholders (`%s`, `%s`, `%s`, `%d`, `%s`) but 5 arguments. `Host` and `Port` are swapped in order vs format specifiers. This produces malformed DSN in logs.

**Expected:** 5 placeholders for 5 arguments.

### 5.3 Security Configuration

**Production Validation (supply-api):**
- `ValidateForProduction()` enforces:
  - SMS must be configured before withdrawals
  - RSA public key required for RS256+
  - HMAC algorithms (HS256/HS384/HS512) explicitly rejected

**gateway:**
- `validateStartupSecurity()` checks:
  - `PASSWORD_ENCRYPTION_KEY` must be explicitly set
  - CORS origins cannot be wildcard in production

**Finding:** Good security-conscious configuration validation, but slightly different approaches across services.

### 5.4 Configuration Schema Inconsistency

**gateway config:**
```yaml
server:
  host: "0.0.0.0"
  port: 8080
providers: [...]
auth: {...}
ratelimit: {...}
```

**supply-api config:**
```yaml
server: {...}
database: {...}
redis: {...}
token: {...}
settlement: {...}
sms: {...}
```

**Finding:** No shared configuration schema pattern. Each service reinvents its own structure.

---

## 6. Extensibility Analysis

### 6.1 Extension Points

**provider/plugin pattern (gateway):**
```go
type Provider interface {
    ChatComplete(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}
r.RegisterProvider(name, provider)
```

**Runtime interface (token-runtime):**
```go
type Runtime interface {
    IssueAndAudit(ctx context.Context, input IssueTokenInput, auditor AuditEmitter) (TokenRecord, error)
    Refresh(ctx context.Context, tokenID string, ttl time.Duration) (TokenRecord, error)
    RevokeAndAudit(ctx context.Context, tokenID, reason, requestID, subjectID string, auditor AuditEmitter) (TokenRecord, error)
    Introspect(ctx context.Context, accessToken string) (TokenRecord, error)
    Lookup(ctx context.Context, tokenID string) (TokenRecord, error)
}
```

**Store interface (token-runtime):**
```go
type RuntimeStore interface {
    Store(ctx context.Context, record TokenRecord) error
    Lookup(ctx context.Context, tokenID string) (TokenRecord, error)
    Revoke(ctx context.Context, tokenID string) error
    // ...
}
```

**Finding:** Good interface-based design enabling testability and alternate implementations.

### 6.2 Domain Model Extensibility (supply-api)

- Domain services are interface-based
- Repository layer uses interfaces
- Event-driven architecture via outbox pattern
- Compensation logic for saga pattern support

### 6.3 Middleware Chain Extensibility

**gateway:**
```go
BuildMux → CORSMiddleware → RateLimitHandler → TokenAuthChain → Handler
```

**supply-api:**
```go
buildMiddlewareChain: Recovery → Logging → Tracing → RateLimit → Auth → BearerExtract → QueryKeyReject
```

**Finding:** Flexible middleware composition, but different implementations across services (no shared middleware package).

---

## 7. Summary Assessment

### 7.1 Strengths

1. **Clean service decomposition** - Independent deployables with clear responsibilities
2. **Interface-based design** - Testability and extensibility via abstractions
3. **Domain-driven structure in supply-api** - Well-organized internal packages
4. **Security-conscious configuration** - Production validation in place
5. **Event-driven patterns** - Outbox and compensation for reliability
6. **Audit trail** - Consistent event logging across services

### 7.2 Issues and Risks

| Priority | Issue | Location | Impact |
|----------|-------|----------|--------|
| P1 | SafeDSN format bug | supply-api/config.go:112 | Debug logging produces incorrect output |
| P2 | Health endpoint inconsistency | token-runtime | Monitoring/telemetry gaps |
| P3 | No shared config schema | Project-wide | Inconsistent operational experience |
| P2 | CORS validation gap | gateway/bootstrap.go:293-297 | Potential misconfiguration |
| P3 | No shared middleware package | Project-wide | Code duplication |
| P3 | Error code naming inconsistency | Project-wide | Client SDK complexity |
| P3 | Go module GOPATH dependency | Project setup | Developer onboarding friction |

### 7.3 Recommendations

1. **Fix SafeDSN bug** in supply-api configuration
2. **Standardize health endpoints** to `/health`, `/readyz`, `/healthz` across all services
3. **Create shared `internal/pkg/common`** for logging, middleware, config utilities
4. **Document configuration schema** with JSON Schema or similar
5. **Standardize error code format** (e.g., `ERROR_CODE_FORMAT` across all services)
6. **Add vendor directory** or document exact Go version and module requirements

---

## 8. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Multi-service deployment complexity | Medium | High | Document deployment runbook |
| Config drift between environments | Medium | Medium | Infrastructure-as-code for config |
| Token runtime bottleneck | Low | High | Monitor and scale horizontally |
| Supply-api domain logic complexity | High | Medium | Refactor into smaller bounded contexts |

---

*Report generated by Hermes Agent Architecture Review Subagent*
*Review completed: 2026-04-18*
