#!/usr/bin/env bash
# scripts/ci/staging_release_pipeline.sh
# Staging 发布流水线 — 生成 manifest.json 作为硬门禁载体
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
SCRIPT_DIR="${ROOT_DIR}/scripts/ci"
OUT_DIR="${ROOT_DIR}/reports/archive/gate_verification"
RELEASES_DIR="${ROOT_DIR}/reports/releases"
LIB_FILE="${SCRIPT_DIR}/lib/manifest_lib.sh"
mkdir -p "${OUT_DIR}" "${RELEASES_DIR}"

TS="$(date +%F_%H%M%S)"
PIPELINE_LOG="${OUT_DIR}/staging_release_pipeline_${TS}.log"
PIPELINE_REPORT="${OUT_DIR}/staging_release_pipeline_${TS}.md"

# shellcheck disable=SC1091
source "${LIB_FILE}"

log() {
  echo "$1" | tee -a "${PIPELINE_LOG}"
}

# ──────────────────────────────────────────────────────────────
# 步骤 0：生成 manifest（run_id + created_at + environment）
# ──────────────────────────────────────────────────────────────
STEP=0
log "[STEP-00] 生成 manifest..."

RUN_ID="staging_${TS}"
MANIFEST_FILE="${RELEASES_DIR}/${RUN_ID}/manifest.json"
MANIFEST_DIR="${RELEASES_DIR}"

manifest_generate --run-id "${RUN_ID}" --staging
manifest_validate "${MANIFEST_FILE}" || {
  log "[FAIL] manifest 验证失败"
  exit 1
}

manifest_set "pipeline_log" "${PIPELINE_LOG}" "${MANIFEST_FILE}"

log "[STEP-00] DONE: manifest=${MANIFEST_FILE} run_id=${RUN_ID}"

# ──────────────────────────────────────────────────────────────
# 步骤 1：repo_integrity_check（含 contract gate）
# 门禁：任何非零退出码 → 整个 pipeline 失败
# ──────────────────────────────────────────────────────────────
STEP=1
log ""
log "[STEP-01] repo_integrity_check（含 Phase 1 contract gate）..."

R1_LOG="${OUT_DIR}/repo_integrity_${TS}.log"
R1_REPORT="${OUT_DIR}/repo_integrity_${TS}.md"

# repo_integrity_check.sh 执行顺序：
#   STEP-01~04: 服务单元+集成测试
#   STEP-R: contract gate（四个场景）
if bash "${SCRIPT_DIR}/repo_integrity_check.sh" \
  > >(tee "${R1_LOG}") 2>&1; then
  manifest_set "decision_inputs.repo_integrity" "PASS" "${MANIFEST_FILE}"
  manifest_set "artifact_paths.repo_integrity_log" "${R1_LOG}" "${MANIFEST_FILE}"
  manifest_set "contract_results.repo_integrity" "PASS" "${MANIFEST_FILE}"
  log "[STEP-01] PASS"
else
  manifest_set "decision_inputs.repo_integrity" "FAIL" "${MANIFEST_FILE}"
  manifest_set "artifact_paths.repo_integrity_log" "${R1_LOG}" "${MANIFEST_FILE}"
  manifest_set "contract_results.repo_integrity" "FAIL" "${MANIFEST_FILE}"
  log "[STEP-01] FAIL — repo_integrity_check 非零退出"
  log "[FAIL] staging pipeline aborted at STEP-01"
  exit 1
fi

# manifest 硬门禁：run_id 不能为空
manifest_hard_gate_run_id "${MANIFEST_FILE}" || {
  log "[FAIL] run_id hard gate failed"
  exit 1
}

# ──────────────────────────────────────────────────────────────
# 步骤 2：superpowers_stage_validate（硬门禁）
# 门禁：NO_GO → 失败；CONDITIONAL_GO → 失败（不再放行）
# ──────────────────────────────────────────────────────────────
STEP=2
log ""
log "[STEP-02] superpowers_stage_validate（staging 硬门禁）..."

SP_LOG="${OUT_DIR}/superpowers_stage_validation_${TS}.log"
SP_REPORT="${OUT_DIR}/superpowers_stage_validation_${TS}.md"

if bash "${SCRIPT_DIR}/superpowers_stage_validate.sh" \
  > >(tee "${SP_LOG}") 2>&1; then
  # stage_validate.sh 只在 NO_GO 时 exit 1，这里补充对 CONDITIONAL_GO 的处理
  # 从 report 中读取实际决策
  SP_DECISION="$(grep -E '^- (机判结论|决策)：\*\*' "${SP_REPORT}" 2>/dev/null | \
    sed -E 's/.*\*\*([^*]+)\*\*/\1/' | tr -d ' ' || echo 'UNKNOWN')"
  if [[ "${SP_DECISION}" == "CONDITIONAL_GO" ]]; then
    manifest_set "decision_inputs.stage_validation" "CONDITIONAL_GO" "${MANIFEST_FILE}"
    manifest_set "artifact_paths.stage_validation_report" "${SP_REPORT}" "${MANIFEST_FILE}"
    log "[STEP-02] CONDITIONAL_GO detected — blocking pipeline"
    log "[FAIL] staging pipeline aborted at STEP-02 (CONDITIONAL_GO not allowed)"
    exit 1
  fi
  manifest_set "decision_inputs.stage_validation" "PASS" "${MANIFEST_FILE}"
  manifest_set "artifact_paths.stage_validation_report" "${SP_REPORT}" "${MANIFEST_FILE}"
  log "[STEP-02] PASS"
else
  manifest_set "decision_inputs.stage_validation" "FAIL" "${MANIFEST_FILE}"
  manifest_set "artifact_paths.stage_validation_report" "${SP_REPORT}" "${MANIFEST_FILE}"
  log "[STEP-02] FAIL — superpowers_stage_validate 非零退出"
  log "[FAIL] staging pipeline aborted at STEP-02"
  exit 1
fi

# ──────────────────────────────────────────────────────────────
# 步骤 3：cross_service_smoke（纳入发布链）
# ──────────────────────────────────────────────────────────────
STEP=3
log ""
log "[STEP-03] cross_service_smoke..."

SMOKE_LOG="${OUT_DIR}/cross_service_smoke_${TS}.log"
SMOKE_REPORT="${OUT_DIR}/cross_service_smoke_${TS}.md"

# 调用 cross_service_smoke.sh
# 环境变量传入服务 URL
TOK_RUNTIME_URL="${TOK_RUNTIME_URL:-http://127.0.0.1:18081}" \
GATEWAY_URL="${GATEWAY_URL:-http://127.0.0.1:18080}" \
SUPPLY_API_URL="${SUPPLY_API_URL:-http://127.0.0.1:18082}" \
bash "${SCRIPT_DIR}/cross_service_smoke.sh" \
  > >(tee "${SMOKE_LOG}") 2>&1
SMOKE_RC=$?

if [[ "${SMOKE_RC}" -eq 0 ]]; then
  manifest_set "smoke_results.cross_service" "PASS" "${MANIFEST_FILE}"
  manifest_set "artifact_paths.cross_service_smoke_log" "${SMOKE_LOG}" "${MANIFEST_FILE}"
  log "[STEP-03] PASS"
elif [[ "${SMOKE_RC}" -eq 2 ]]; then
  # exit 2 = SKIP_LOCAL_PLACEHOLDER（本地 mock，不计入通过）
  manifest_set "smoke_results.cross_service" "SKIP_LOCAL_PLACEHOLDER" "${MANIFEST_FILE}"
  manifest_set "artifact_paths.cross_service_smoke_log" "${SMOKE_LOG}" "${MANIFEST_FILE}"
  log "[STEP-03] SKIP_LOCAL_PLACEHOLDER — not counted as pass"
  # 这种情况下 staging 不能算真正完成，但不一定 abort pipeline（取决于 DEFERRED 策略）
else
  manifest_set "smoke_results.cross_service" "FAIL" "${MANIFEST_FILE}"
  manifest_set "artifact_paths.cross_service_smoke_log" "${SMOKE_LOG}" "${MANIFEST_FILE}"
  log "[STEP-03] FAIL — cross_service_smoke 非零退出"
  log "[FAIL] staging pipeline aborted at STEP-03"
  exit 1
fi

# ──────────────────────────────────────────────────────────────
# 步骤 4：生成最终 release manifest
# ──────────────────────────────────────────────────────────────
STEP=4
log ""
log "[STEP-04] 生成最终 release manifest..."

# 收集所有结果
REPO_INT="$(manifest_get "decision_inputs.repo_integrity" "${MANIFEST_FILE}")"
STAGE_VAL="$(manifest_get "decision_inputs.stage_validation" "${MANIFEST_FILE}")"
SMOKE_RES="$(manifest_get "smoke_results.cross_service" "${MANIFEST_FILE}")"

# 最终决策
OVERALL="PASS"
if [[ "${REPO_INT}" == "FAIL" || "${STAGE_VAL}" == "FAIL" || "${SMOKE_RES}" == "FAIL" ]]; then
  OVERALL="FAIL"
elif [[ "${SMOKE_RES}" == "SKIP_LOCAL_PLACEHOLDER" ]]; then
  # smoke 未真实运行，不算 staging 完成
  if [[ "${STAGE_VAL}" == "PASS" ]]; then
    OVERALL="CONDITIONAL_PASS"
  fi
fi

manifest_set "decision_inputs.overall_decision" "${OVERALL}" "${MANIFEST_FILE}"

log "[STEP-04] overall_decision=${OVERALL}"

# 生成 pipeline 报告
cat > "${PIPELINE_REPORT}" <<EOF
# Staging Release Pipeline 报告

- 时间戳：${TS}
- run_id：${RUN_ID}
- manifest：${MANIFEST_FILE}

## 步骤结果

| 步骤 | 门禁 | 结果 |
|---|---|---|
| STEP-01 repo_integrity | 必须 PASS | ${REPO_INT} |
| STEP-02 stage_validate | NO_GO/CONDITIONAL_GO → FAIL | ${STAGE_VAL} |
| STEP-03 cross_smoke | FAIL → FAIL；SKIP → 警告 | ${SMOKE_RES} |

## 最终决策

- 整体结论：**${OVERALL}**
- manifest：\`${MANIFEST_FILE}\`

## manifest 内容摘要

EOF

jq '.' "${MANIFEST_FILE}" >> "${PIPELINE_REPORT}" 2>/dev/null || true

log ""
log "=========================================="
log "[RESULT] staging pipeline: ${OVERALL}"
log "[INFO]  manifest: ${MANIFEST_FILE}"
log "[INFO]  report:   ${PIPELINE_REPORT}"
log "=========================================="

if [[ "${OVERALL}" == "FAIL" ]]; then
  exit 1
fi
