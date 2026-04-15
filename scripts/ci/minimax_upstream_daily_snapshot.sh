#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
DATE_TAG="${1:-$(date +%F)}"
ENV_FILE_REL="${2:-scripts/supply-gate/.env.minimax-dev}"
if [[ "${ENV_FILE_REL}" == /* ]]; then
  ENV_FILE="${ENV_FILE_REL}"
else
  ENV_FILE="${ROOT_DIR}/${ENV_FILE_REL}"
fi

OUT_DIR="${ROOT_DIR}/reports/archive/gate_verification"
mkdir -p "${OUT_DIR}"

SNAPSHOT_CSV="${OUT_DIR}/minimax_upstream_daily_snapshots.csv"
SNAPSHOT_MD="${OUT_DIR}/minimax_upstream_daily_snapshot_${DATE_TAG}.md"
RUN_ACTIVE_SMOKE="${RUN_ACTIVE_SMOKE:-0}"

extract_overall() {
  local file="$1"
  if [[ ! -f "${file}" ]]; then
    echo "UNKNOWN"
    return
  fi
  grep -E '^- 总体结论：\*\*' "${file}" | head -n 1 | sed -E 's/^- 总体结论：\*\*([^*]+)\*\*$/\1/' || true
}

find_latest_smoke_report() {
  local choose_real_only="$1"
  local candidate=""
  local first_any=""
  for candidate in $(ls -1t "${OUT_DIR}"/minimax_upstream_smoke_*.md 2>/dev/null || true); do
    if [[ -z "${first_any}" ]]; then
      first_any="${candidate}"
    fi
    if [[ "${choose_real_only}" == "1" ]]; then
      overall="$(extract_overall "${candidate}")"
      if [[ "${overall}" != "PASS_DRY_RUN" ]]; then
        echo "${candidate}"
        return
      fi
    else
      echo "${candidate}"
      return
    fi
  done
  echo "${first_any}"
}

if [[ "${RUN_ACTIVE_SMOKE}" == "1" ]]; then
  bash "${ROOT_DIR}/scripts/supply-gate/minimax_upstream_smoke.sh" "${ENV_FILE}"
fi

LATEST_REPORT="$(find_latest_smoke_report "1")"
if [[ -z "${LATEST_REPORT}" || ! -f "${LATEST_REPORT}" ]]; then
  echo "[FAIL] no minimax smoke report found under ${OUT_DIR}"
  exit 1
fi

OVERALL="$(extract_overall "${LATEST_REPORT}")"
BASE_HTTP="$(grep -E '^- http_code：' "${LATEST_REPORT}" | sed -n '1p' | sed -E 's/^- http_code：([0-9]+)$/\1/' || true)"
ACTIVE_HTTP="$(grep -E '^- http_code：' "${LATEST_REPORT}" | sed -n '2p' | sed -E 's/^- http_code：([0-9]+)$/\1/' || true)"

if [[ -z "${OVERALL}" ]]; then
  OVERALL="UNKNOWN"
fi
STATUS="FAIL"
if [[ "${OVERALL}" == "PASS" || "${OVERALL}" == "PASS_AUTH_REACHED" ]]; then
  STATUS="PASS"
elif [[ "${OVERALL}" == "PASS_DRY_RUN" ]]; then
  STATUS="CONDITIONAL_PASS"
fi

NOTE="latest_report=${LATEST_REPORT}"
if [[ "${RUN_ACTIVE_SMOKE}" != "1" ]]; then
  NOTE="${NOTE}; run_active_smoke=0(use latest report only)"
fi

if [[ ! -f "${SNAPSHOT_CSV}" ]]; then
  echo "date,status,overall,base_http,active_http,run_active_smoke,report,note" > "${SNAPSHOT_CSV}"
fi

tmp_csv="$(mktemp)"
awk -F',' -v d="${DATE_TAG}" '
NR==1 {print; next}
$1==d {next}
{print}
' "${SNAPSHOT_CSV}" > "${tmp_csv}"
echo "${DATE_TAG},${STATUS},${OVERALL},${BASE_HTTP:-N/A},${ACTIVE_HTTP:-N/A},${RUN_ACTIVE_SMOKE},${LATEST_REPORT},${NOTE}" >> "${tmp_csv}"
mv "${tmp_csv}" "${SNAPSHOT_CSV}"

{
  echo "# Minimax 上游每日快照（${DATE_TAG}）"
  echo
  echo "- 运行模式：RUN_ACTIVE_SMOKE=${RUN_ACTIVE_SMOKE}"
  echo "- 环境文件：\`${ENV_FILE_REL}\`"
  echo "- 快照结果：**${STATUS}**"
  echo "- overall：\`${OVERALL}\`"
  echo "- base_http：\`${BASE_HTTP:-N/A}\`"
  echo "- active_http：\`${ACTIVE_HTTP:-N/A}\`"
  echo "- 证据：\`${LATEST_REPORT}\`"
  echo
  echo "## 说明"
  echo
  echo "1. RUN_ACTIVE_SMOKE=0 时仅汇总最新 smoke 报告，不触发外部请求。"
  echo "2. RUN_ACTIVE_SMOKE=1 时会执行一次实时 smoke，并更新快照。"
  echo "3. 该快照用于上游可达性监控，不替代 SUP 发布门禁结论。"
  echo
  echo "## 存档"
  echo
  echo "1. CSV：\`${SNAPSHOT_CSV}\`"
  echo "2. 日报：\`${SNAPSHOT_MD}\`"
} > "${SNAPSHOT_MD}"

echo "[PASS] minimax daily snapshot generated: ${SNAPSHOT_MD}"
