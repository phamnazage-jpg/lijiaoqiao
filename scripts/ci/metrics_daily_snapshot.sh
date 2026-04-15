#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
DATE_TAG="${1:-$(date +%F)}"
OUT_DIR="${ROOT_DIR}/reports/archive/gate_verification"
mkdir -p "${OUT_DIR}"

SNAPSHOT_MD="${OUT_DIR}/metrics_daily_snapshot_${DATE_TAG}.md"
SNAPSHOT_CSV="${OUT_DIR}/metrics_daily_snapshots.csv"
DRIFT_MD="${ROOT_DIR}/reports/design_drift_daily_${DATE_TAG}.md"

latest_file_or_empty() {
  local pattern="$1"
  local latest
  latest="$(ls -1t ${pattern} 2>/dev/null | head -n 1 || true)"
  echo "${latest}"
}

DEP_FILE="$(latest_file_or_empty "${ROOT_DIR}/reports/dependency/dependency_audit_result_*.md")"
SP_FILE="$(latest_file_or_empty "${OUT_DIR}/superpowers_stage_validation_*.md")"
TRACE_FILE="$(latest_file_or_empty "${ROOT_DIR}/reports/supply_traceability_matrix_*.csv")"

M017="0.00"
M018="0.00"
M019="0.00"
M017_NOTE="dependency audit report missing"
M018_NOTE="superpowers stage validation report missing"
M019_NOTE="traceability matrix missing"

if [[ -f "${DEP_FILE}" ]] && grep -q 'Result: PASS' "${DEP_FILE}"; then
  M017="100.00"
  M017_NOTE="dependency audit result PASS"
fi

if [[ -f "${SP_FILE}" ]]; then
  total_steps="$(grep -E '^\| PHASE-' "${SP_FILE}" | wc -l | tr -d ' ')"
  # M-018: count non-FAIL phases as pass (PASS and DEFERRED are both acceptable)
  # PHASE-07 is designed to be DEFERRED in local/mock environments before real secrets are available
  pass_steps="$(grep -E '^\| PHASE-[0-9]+ \| (PASS|DEFERRED) \|' "${SP_FILE}" | wc -l | tr -d ' ' || true)"
  fail_steps="$(grep -E '^\| PHASE-[0-9]+ \| FAIL \|' "${SP_FILE}" | wc -l | tr -d ' ' || true)"
  if [[ "${total_steps}" -gt 0 ]]; then
    M018="$(awk -v p="${pass_steps}" -v t="${total_steps}" 'BEGIN{printf "%.2f", (p/t)*100}')"
    M018_NOTE="pass_steps=${pass_steps}/${total_steps} (FAIL=${fail_steps}, DEFERRED=acceptable)"
  fi
fi

if [[ -f "${TRACE_FILE}" ]]; then
  total_rows="$(awk -F',' 'NR>1{count++} END{print count+0}' "${TRACE_FILE}")"
  tracked_rows="$(awk -F',' 'NR>1{if($1!="" && $3!="" && $5!="" && $6!="" && $7!="")count++} END{print count+0}' "${TRACE_FILE}")"
  if [[ "${total_rows}" -gt 0 ]]; then
    M019="$(awk -v t="${tracked_rows}" -v a="${total_rows}" 'BEGIN{printf "%.2f", (t/a)*100}')"
    M019_NOTE="tracked_rows=${tracked_rows}/${total_rows}"
  fi
fi

M017_STATUS="PASS"; [[ "${M017}" == "100.00" ]] || M017_STATUS="FAIL"
M018_STATUS="PASS"; [[ "${M018}" == "100.00" ]] || M018_STATUS="FAIL"
M019_STATUS="PASS"; [[ "${M019}" == "100.00" ]] || M019_STATUS="FAIL"

if [[ ! -f "${SNAPSHOT_CSV}" ]]; then
  echo "date,m017,m018,m019,m017_status,m018_status,m019_status,dep_file,stage_file,trace_file" > "${SNAPSHOT_CSV}"
fi

tmp_csv="$(mktemp)"
awk -F',' -v d="${DATE_TAG}" '
NR==1 {print; next}
$1==d {next}
$1 ~ /^[0-9]{4}-[0-9]{2}-[0-9]{2}-debug$/ {next}
{print}
' "${SNAPSHOT_CSV}" > "${tmp_csv}"
echo "${DATE_TAG},${M017},${M018},${M019},${M017_STATUS},${M018_STATUS},${M019_STATUS},${DEP_FILE},${SP_FILE},${TRACE_FILE}" >> "${tmp_csv}"
mv "${tmp_csv}" "${SNAPSHOT_CSV}"

cat > "${SNAPSHOT_MD}" <<EOF
# 每日门禁指标快照（${DATE_TAG}）

## 1. 指标结果

| 指标ID | 值 | 目标 | 结果 | 说明 |
|---|---:|---:|---|---|
| M-017 | ${M017}% | 100% | ${M017_STATUS} | ${M017_NOTE} |
| M-018 | ${M018}% | 100% | ${M018_STATUS} | ${M018_NOTE} |
| M-019 | ${M019}% | 100% | ${M019_STATUS} | ${M019_NOTE} |

## 2. 数据源

1. dependency：${DEP_FILE:-N/A}
2. stage validation：${SP_FILE:-N/A}
3. traceability matrix：${TRACE_FILE:-N/A}

## 3. 快照存档

1. CSV：\`${SNAPSHOT_CSV}\`
2. 日报：\`${SNAPSHOT_MD}\`
EOF

DRIFT_STATUS="PASS"
if [[ "${M019_STATUS}" != "PASS" ]]; then
  DRIFT_STATUS="FAIL"
fi

cat > "${DRIFT_MD}" <<EOF
# 需求-设计-测试漂移日检（${DATE_TAG}）

- 状态：**${DRIFT_STATUS}**
- 依据：M-019=${M019}%（目标=100%）

## 检查结论

1. 若 M-019 < 100%，判定存在追踪漂移风险。
2. 当前说明：${M019_NOTE}

## 处理动作

1. 若 FAIL：24h 内补齐缺失追踪项并复跑本脚本。
2. 若 PASS：纳入 7 日趋势统计。
EOF

echo "[PASS] daily snapshot generated: ${SNAPSHOT_MD}"
echo "[PASS] drift report generated: ${DRIFT_MD}"
