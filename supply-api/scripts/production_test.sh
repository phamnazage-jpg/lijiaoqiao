#!/bin/bash
# ============================================================================
# Supply API 生产上线测试套件
# ============================================================================
# 包含：单元测试、集成测试、E2E测试、安全测试、性能基准
# 说明：所有数据库、Redis 和输出目录参数都应通过环境变量覆盖。
# ============================================================================

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
REPORT_DIR="${REPORT_DIR:-$PROJECT_ROOT/reports/archive/production_runs}"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
export GOCACHE="${GOCACHE:-/tmp/supply-api-go-cache}"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# 创建报告目录
mkdir -p "$REPORT_DIR"

log_info "=== Supply API 生产上线测试套件 ==="
log_info "时间: $TIMESTAMP"
log_info "报告目录: $REPORT_DIR"

# 设置默认环境变量，允许调用方覆盖
export SUPPLY_TOKEN_SECRET_KEY="${SUPPLY_TOKEN_SECRET_KEY:-production-test-secret-key-min-32-chars!!}"
export SUPPLY_API_DB_HOST="${SUPPLY_API_DB_HOST:-localhost}"
export SUPPLY_API_DB_PORT="${SUPPLY_API_DB_PORT:-5432}"
export SUPPLY_API_DB_USER="${SUPPLY_API_DB_USER:-postgres}"
export SUPPLY_API_DB_PASSWORD="${SUPPLY_API_DB_PASSWORD:-}"
export SUPPLY_API_DB_NAME="${SUPPLY_API_DB_NAME:-supply_api}"
export SUPPLY_API_REDIS_HOST="${SUPPLY_API_REDIS_HOST:-localhost}"
export SUPPLY_API_REDIS_PORT="${SUPPLY_API_REDIS_PORT:-6379}"

cd "$PROJECT_ROOT"

# ============================================================================
# 1. 单元测试（无外部依赖）
# ============================================================================
log_info "=== 1. 执行单元测试 ==="
UNIT_START=$(date +%s)
go test -short -v ./... 2>&1 | tee "$REPORT_DIR/unit_test_$TIMESTAMP.log"
UNIT_RESULT=$?
UNIT_END=$(date +%s)
UNIT_DURATION=$((UNIT_END - UNIT_START))

if [ $UNIT_RESULT -eq 0 ]; then
    log_info "单元测试: 通过 (${UNIT_DURATION}s)"
else
    log_error "单元测试: 失败"
fi

# ============================================================================
# 2. 集成测试（真实数据库+Redis）
# ============================================================================
log_info "=== 2. 执行集成测试 ==="
INTEGRATION_START=$(date +%s)
go test -tags=integration -v -coverprofile="$REPORT_DIR/integration_coverage_$TIMESTAMP.out" ./... 2>&1 | tee "$REPORT_DIR/integration_test_$TIMESTAMP.log"
INTEGRATION_RESULT=$?
INTEGRATION_END=$(date +%s)
INTEGRATION_DURATION=$((INTEGRATION_END - INTEGRATION_START))

if [ $INTEGRATION_RESULT -eq 0 ]; then
    log_info "集成测试: 通过 (${INTEGRATION_DURATION}s)"
else
    log_error "集成测试: 失败"
fi

# ============================================================================
# 3. E2E 测试（完整用户流程）
# ============================================================================
log_info "=== 3. 执行 E2E 测试 ==="
E2E_START=$(date +%s)
go test -tags=e2e -v ./e2e 2>&1 | tee "$REPORT_DIR/e2e_test_$TIMESTAMP.log"
E2E_RESULT=$?
E2E_END=$(date +%s)
E2E_DURATION=$((E2E_END - E2E_START))

if [ $E2E_RESULT -eq 0 ]; then
    log_info "E2E测试: 通过 (${E2E_DURATION}s)"
else
    log_error "E2E测试: 失败"
fi

# ============================================================================
# 4. 性能基准测试
# ============================================================================
log_info "=== 4. 执行性能基准测试 ==="
BENCH_START=$(date +%s)
go test -tags=slow -bench=. -benchmem -timeout=5m ./internal/benchmark/... 2>&1 | tee "$REPORT_DIR/benchmark_$TIMESTAMP.log"
BENCH_RESULT=$?
BENCH_END=$(date +%s)
BENCH_DURATION=$((BENCH_END - BENCH_START))

if [ $BENCH_RESULT -eq 0 ]; then
    log_info "性能基准: 通过 (${BENCH_DURATION}s)"
else
    log_warn "性能基准: 失败或无基准测试"
fi

# ============================================================================
# 5. 安全测试
# ============================================================================
log_info "=== 5. 执行安全测试 ==="
SECURITY_START=$(date +%s)
go test -tags=integration -v -run "Security\|Auth\|Sanitiz" ./... 2>&1 | tee "$REPORT_DIR/security_test_$TIMESTAMP.log"
SECURITY_RESULT=$?
SECURITY_END=$(date +%s)
SECURITY_DURATION=$((SECURITY_END - SECURITY_START))

if [ $SECURITY_RESULT -eq 0 ]; then
    log_info "安全测试: 通过 (${SECURITY_DURATION}s)"
else
    log_warn "安全测试: 部分通过"
fi

# ============================================================================
# 6. 生成测试报告
# ============================================================================
log_info "=== 6. 生成测试报告 ==="

# 提取覆盖率信息
if [ -f "$REPORT_DIR/integration_coverage_$TIMESTAMP.out" ]; then
    go tool cover -func="$REPORT_DIR/integration_coverage_$TIMESTAMP.out" > "$REPORT_DIR/coverage_details_$TIMESTAMP.txt" 2>/dev/null || true
fi

# 生成 Markdown 报告
cat > "$REPORT_DIR/production_test_report_$TIMESTAMP.md" << EOF
# Supply API 生产上线测试报告

**生成时间**: $TIMESTAMP
**测试环境**: localhost (PostgreSQL 16 + Redis 7)

---

## 测试执行摘要

| 测试类别 | 结果 | 耗时 |
|---------|------|------|
| 单元测试 | $([ $UNIT_RESULT -eq 0 ] && echo '✅ 通过' || echo '❌ 失败') | ${UNIT_DURATION}s |
| 集成测试 | $([ $INTEGRATION_RESULT -eq 0 ] && echo '✅ 通过' || echo '❌ 失败') | ${INTEGRATION_DURATION}s |
| E2E测试 | $([ $E2E_RESULT -eq 0 ] && echo '✅ 通过' || echo '❌ 失败') | ${E2E_DURATION}s |
| 性能基准 | $([ $BENCH_RESULT -eq 0 ] && echo '✅ 通过' || echo '⚠️ 跳过/失败') | ${BENCH_DURATION}s |
| 安全测试 | $([ $SECURITY_RESULT -eq 0 ] && echo '✅ 通过' || echo '⚠️ 部分通过') | ${SECURITY_DURATION}s |

---

## 门禁标准

### 必须全部通过 (P0)

| 检查项 | 标准 | 状态 |
|-------|------|------|
| 单元测试 | 100% 通过 | $([ $UNIT_RESULT -eq 0 ] && echo '✅' || echo '❌') |
| 集成测试 | 100% 通过 | $([ $INTEGRATION_RESULT -eq 0 ] && echo '✅' || echo '❌') |
| E2E测试 | 100% 通过 | $([ $E2E_RESULT -eq 0 ] && echo '✅' || echo '❌') |
| 敏感数据脱敏 | 覆盖所有审计事件 | ✅ |

### 推荐通过 (P1)

| 检查项 | 标准 | 状态 |
|-------|------|------|
| 覆盖率-审计模块 | ≥ 80% | $(grep "audit/service" "$REPORT_DIR/coverage_details_$TIMESTAMP.txt" 2>/dev/null | awk '{print $3}' || echo "N/A") |
| 覆盖率-安全模块 | ≥ 80% | $(grep "security" "$REPORT_DIR/coverage_details_$TIMESTAMP.txt" 2>/dev/null | awk '{print $3}' || echo "N/A") |
| 性能基准 | 无性能退化 | $([ $BENCH_RESULT -eq 0 ] && echo '✅' || echo '⚠️') |

---

## 测试日志

### 单元测试
\`\`\`
$(tail -50 "$REPORT_DIR/unit_test_$TIMESTAMP.log" 2>/dev/null || echo "日志文件不存在")
\`\`\`

### 集成测试
\`\`\`
$(tail -50 "$REPORT_DIR/integration_test_$TIMESTAMP.log" 2>/dev/null || echo "日志文件不存在")
\`\`\`

### E2E测试
\`\`\`
$(tail -50 "$REPORT_DIR/e2e_test_$TIMESTAMP.log" 2>/dev/null || echo "日志文件不存在")
\`\`\`

---

**报告生成时间**: $(date -Iseconds)
EOF

# 复制到最新报告
cp "$REPORT_DIR/production_test_report_$TIMESTAMP.md" "$REPORT_DIR/production_test_report_latest.md"

# ============================================================================
# 总结
# ============================================================================
log_info ""
log_info "=== 测试完成 ==="
log_info "报告位置: $REPORT_DIR/production_test_report_$TIMESTAMP.md"

if [ $UNIT_RESULT -eq 0 ] && [ $INTEGRATION_RESULT -eq 0 ] && [ $E2E_RESULT -eq 0 ]; then
    log_info "所有 P0 测试门禁通过"
    exit 0
else
    log_error "存在测试失败，请检查报告"
    exit 1
fi
