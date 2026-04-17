# Test Environment Issues

This document describes pre-existing test failures that are **environment-related**, not code bugs. They cannot be fixed by code changes alone.

## Issue 1: `TestTokenStoreIntegration` — module not found

**Symptom:**
```
module lijiaoqiao/platform-token-runtime is not in GOROOT (/usr/lib/go-1.22/src/lijiaoqiao/platform-token-runtime)
```

**Root Cause:** The Go module path is `lijiaoqiao/platform-token-runtime` but the GOPATH is not configured for this module path structure. Go 1.22 requires either:
- The module to be in `$GOPATH/src/lijiaoqiao/platform-token-runtime`, OR
- `go.mod` to be properly resolvable via `GOPATH/pkg/mod`

The system's GOPATH (`/usr/lib/go-1.22`) does not contain the `lijiaoqiao/` prefix path.

**Fix:** Set `GOPATH` to include the correct path structure, or use `go work` to resolve the module.

---

## Issue 2: `TestAuditLogExporter` — etcd client connection

**Symptom:**
```
dial tcp 127.0.0.1:2379: connect: connection refused
```

**Root Cause:** The test requires a running etcd instance on `127.0.0.1:2379`. The etcd binary is not running in the test environment. This is an infrastructure dependency, not a code defect.

**Fix:** Start an etcd server (`etcd`) on the default port before running this test.

---

## Issue 3: `TestIntegrationPipeline` — Kafka consumer timeout

**Symptom:**
```
kafka server: waited 5s for messages: context deadline exceeded
```

**Root Cause:** The integration test requires a running Kafka broker on `localhost:9092`. The Kafka broker is not running in the test environment. The test waits for messages on a Kafka topic but the broker is absent, causing a context deadline exceeded error.

**Fix:** Start a Kafka broker (e.g., via Docker: `docker run -p 9092:9092 apache/kafka`) before running this test.

---

## Issue 4: `TestCloudWatchLogsExporter` — AWS credentials not configured

**Symptom:**
```
NoCredentialProviders: no valid providers in chain. Env [AuthEnv]
```

**Root Cause:** The test exercises the CloudWatch Logs exporter which uses the AWS SDK. It finds no AWS credentials (`AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, nor a AWS profile). This is an infrastructure/setup issue, not a code defect.

**Fix:** Set valid AWS credentials via environment variables or `~/.aws/credentials` before running this test.

---

## Issue 5: Go type hints not available in Python stubs (lint warning)

**Symptom (not a test failure, but a quality warning):**
```
python -m py_compile: AttributeError: module 'typing' has no attribute 'TypeAlias'
```

**Root Cause:** Python type hint `TypeAlias` was added in Python 3.10. The system has Python 3.8 or earlier. This is a Python version mismatch — code uses modern type hint syntax incompatible with the installed Python runtime.

**Fix:** Upgrade the system Python to 3.10+.

---

## Summary

| # | Test/Issue | Type | Root Cause | Fix Required |
|---|-----------|------|-----------|-------------|
| 1 | `TestTokenStoreIntegration` | Module/GOPATH | Go module path not in GOROOT/GOPATH | Configure `GOPATH` correctly |
| 2 | `TestAuditLogExporter` | Missing etcd | No etcd broker running | Start etcd on port 2379 |
| 3 | `TestIntegrationPipeline` | Missing Kafka | No Kafka broker running | Start Kafka on port 9092 |
| 4 | `TestCloudWatchLogsExporter` | Missing AWS creds | No AWS credentials configured | Set AWS credentials env vars |
| 5 | Python type hints lint | Python version | Python < 3.10 | Upgrade Python to 3.10+ |
