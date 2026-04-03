#!/usr/bin/env bash
# scripts/ci/m017_dependency_audit.sh - M-017 依赖审计四件套主脚本
# 功能：生成SBOM、Lockfile Diff、兼容矩阵、风险登记册
# 输入：REPORT_DATE
# 输出：四个报告文件

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="${PROJECT_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"

REPORT_DATE="${1:-$(date +%Y-%m-%d)}"
REPORT_DIR="${2:-${PROJECT_ROOT}/reports/dependency}"

mkdir -p "$REPORT_DIR"

echo "[M017] Starting dependency audit for ${REPORT_DATE}"
echo "[M017] Report directory: ${REPORT_DIR}"

# 1. 生成SBOM
echo "[M017] Step 1/4: Generating SBOM..."
if bash "${SCRIPT_DIR}/m017_sbom.sh" "$REPORT_DATE" "$REPORT_DIR"; then
    echo "[M017] SBOM generation: SUCCESS"
else
    echo "[M017] SBOM generation: FAILED"
fi

# 2. 生成Lockfile Diff
echo "[M017] Step 2/4: Generating lockfile diff..."
if bash "${SCRIPT_DIR}/m017_lockfile_diff.sh" "$REPORT_DATE" "$REPORT_DIR"; then
    echo "[M017] Lockfile diff generation: SUCCESS"
else
    echo "[M017] Lockfile diff generation: FAILED"
fi

# 3. 生成兼容矩阵
echo "[M017] Step 3/4: Generating compatibility matrix..."
if bash "${SCRIPT_DIR}/m017_compat_matrix.sh" "$REPORT_DATE" "$REPORT_DIR"; then
    echo "[M017] Compatibility matrix generation: SUCCESS"
else
    echo "[M017] Compatibility matrix generation: FAILED"
fi

# 4. 生成风险登记册
echo "[M017] Step 4/4: Generating risk register..."
if bash "${SCRIPT_DIR}/m017_risk_register.sh" "$REPORT_DATE" "$REPORT_DIR"; then
    echo "[M017] Risk register generation: SUCCESS"
else
    echo "[M017] Risk register generation: FAILED"
fi

# 验证所有artifacts存在
echo "[M017] Validating artifacts..."
ARTIFACTS=(
    "sbom_${REPORT_DATE}.spdx.json"
    "lockfile_diff_${REPORT_DATE}.md"
    "compat_matrix_${REPORT_DATE}.md"
    "risk_register_${REPORT_DATE}.md"
)

ALL_PASS=true
for artifact in "${ARTIFACTS[@]}"; do
    if [ -f "${REPORT_DIR}/${artifact}" ] && [ -s "${REPORT_DIR}/${artifact}" ]; then
        echo "[M017] ${artifact}: OK"
    else
        echo "[M017] ${artifact}: MISSING OR EMPTY"
        ALL_PASS=false
    fi
done

# 输出摘要
echo ""
echo "========================================"
if [ "$ALL_PASS" = true ]; then
    echo "[M017] PASS: All 4 artifacts generated successfully"
    echo "========================================"
    exit 0
else
    echo "[M017] FAIL: One or more artifacts missing"
    echo "========================================"
    exit 1
fi
