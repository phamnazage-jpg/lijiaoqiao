#!/bin/bash
#===============================================================================
# Review快速执行入口
#===============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 加载配置
source "$SCRIPT_DIR/auto_review_config.sh"

# 执行review
"$SCRIPT_DIR/auto_review.sh" "$@"
