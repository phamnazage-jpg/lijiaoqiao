#!/bin/bash
# 每日报告生成器 - 小龙多角色协同工作流

cd "$(dirname "$0")"

# 默认输出到 reports 目录
REPORTS_DIR="./data/reports"
mkdir -p "$REPORTS_DIR"

DATE=$(date +%Y%m%d)
REPORT_FILE="$REPORTS_DIR/daily_${DATE}.md"

echo "📊 生成每日汇报: $DATE"
python3 task_manager.py daily > "$REPORT_FILE"

if [ $? -eq 0 ]; then
    echo "✅ 报告已生成: $REPORT_FILE"
    cat "$REPORT_FILE"
else
    echo "❌ 报告生成失败"
    exit 1
fi
