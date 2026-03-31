#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
END_DATE="${1:-$(date +%F)}"
OUT_DIR="${ROOT_DIR}/reports/gates"
SNAPSHOT_CSV="${OUT_DIR}/minimax_upstream_daily_snapshots.csv"
OUT_MD="${OUT_DIR}/minimax_upstream_trend_7d_${END_DATE}.md"

if [[ ! -f "${SNAPSHOT_CSV}" ]]; then
  echo "[FAIL] missing minimax snapshot csv: ${SNAPSHOT_CSV}"
  exit 1
fi

tmp_rows="$(mktemp)"
{
  head -n 1 "${SNAPSHOT_CSV}"
  tail -n +2 "${SNAPSHOT_CSV}" \
    | awk -F',' '$1 ~ /^[0-9]{4}-[0-9]{2}-[0-9]{2}$/' \
    | sort -t, -k1,1 \
    | tail -n 7
} > "${tmp_rows}"

data_count="$(tail -n +2 "${tmp_rows}" | wc -l | tr -d ' ')"
if [[ "${data_count}" -eq 0 ]]; then
  echo "[FAIL] no minimax snapshot rows found"
  rm -f "${tmp_rows}"
  exit 1
fi

pass_days="$(awk -F',' 'NR>1 && $2=="PASS"{c++} END{print c+0}' "${tmp_rows}")"
conditional_days="$(awk -F',' 'NR>1 && $2=="CONDITIONAL_PASS"{c++} END{print c+0}' "${tmp_rows}")"
fail_days="$(awk -F',' 'NR>1 && $2=="FAIL"{c++} END{print c+0}' "${tmp_rows}")"

trend_status="INSUFFICIENT_DATA"
trend_note="less than 7 days of minimax snapshots"
if [[ "${data_count}" -ge 7 ]]; then
  trend_status="NOT_READY"
  trend_note="need 7 PASS days to mark stable upstream trend"
  if [[ "${pass_days}" -eq 7 ]]; then
    trend_status="PASS_7D"
    trend_note="7 consecutive PASS days reached"
  elif [[ "${fail_days}" -eq 0 && "${conditional_days}" -gt 0 ]]; then
    trend_status="CONDITIONAL_7D"
    trend_note="no FAIL but contains CONDITIONAL_PASS days"
  fi
fi

{
  echo "# Minimax 上游 7 日趋势报告（截至 ${END_DATE}）"
  echo
  echo "## 1. 汇总"
  echo
  echo "- 采样天数：${data_count}"
  echo "- PASS 天数：${pass_days}"
  echo "- CONDITIONAL_PASS 天数：${conditional_days}"
  echo "- FAIL 天数：${fail_days}"
  echo "- 趋势状态：**${trend_status}**"
  echo "- 说明：${trend_note}"
  echo
  echo "## 2. 明细"
  echo
  echo "| 日期 | 状态 | overall | base_http | active_http | run_active_smoke | 报告 |"
  echo "|---|---|---|---:|---:|---:|---|"
  awk -F',' 'NR>1{printf "| %s | %s | %s | %s | %s | %s | %s |\n",$1,$2,$3,$4,$5,$6,$7}' "${tmp_rows}"
  echo
  echo "## 3. 数据源"
  echo
  echo "1. \`${SNAPSHOT_CSV}\`"
  echo "2. 本报告仅用于 Minimax 上游可达性趋势，不替代 SUP 发布门禁结论。"
} > "${OUT_MD}"

rm -f "${tmp_rows}"
echo "[PASS] minimax trend report generated: ${OUT_MD}"

