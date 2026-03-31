#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
END_DATE="${1:-$(date +%F)}"
OUT_DIR="${ROOT_DIR}/reports/gates"
SNAPSHOT_CSV="${OUT_DIR}/metrics_daily_snapshots.csv"
OUT_MD="${OUT_DIR}/metrics_trend_7d_${END_DATE}.md"

if [[ ! -f "${SNAPSHOT_CSV}" ]]; then
  echo "[FAIL] missing snapshot csv: ${SNAPSHOT_CSV}"
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
  echo "[FAIL] no snapshot rows found"
  rm -f "${tmp_rows}"
  exit 1
fi

all_pass_days="$(awk -F',' 'NR>1{if($5=="PASS" && $6=="PASS" && $7=="PASS")c++} END{print c+0}' "${tmp_rows}")"
trend_status="NOT_READY"
trend_note="need 7 all-pass days to satisfy continuous trend requirement"
if [[ "${data_count}" -ge 7 && "${all_pass_days}" -eq 7 ]]; then
  trend_status="PASS_7D"
  trend_note="7 consecutive days all PASS"
fi

{
  echo "# M-017/M-018/M-019 7日趋势报告（截至 ${END_DATE}）"
  echo
  echo "## 1. 汇总"
  echo
  echo "- 采样天数：${data_count}"
  echo "- 全通过天数：${all_pass_days}"
  echo "- 趋势状态：**${trend_status}**"
  echo "- 说明：${trend_note}"
  echo
  echo "## 2. 明细"
  echo
  echo "| 日期 | M-017 | M-018 | M-019 | M-017状态 | M-018状态 | M-019状态 |"
  echo "|---|---:|---:|---:|---|---|---|"
  awk -F',' 'NR>1{printf "| %s | %s%% | %s%% | %s%% | %s | %s | %s |\n",$1,$2,$3,$4,$5,$6,$7}' "${tmp_rows}"
  echo
  echo "## 3. 数据源"
  echo
  echo "1. \`${SNAPSHOT_CSV}\`"
} > "${OUT_MD}"

rm -f "${tmp_rows}"
echo "[PASS] trend report generated: ${OUT_MD}"
