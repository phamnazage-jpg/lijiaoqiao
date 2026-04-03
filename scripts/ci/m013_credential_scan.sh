#!/usr/bin/env bash
# scripts/ci/m013_credential_scan.sh - M-013凭证泄露扫描脚本
# 功能：扫描响应体、日志、导出文件中的凭证泄露
# 输出：JSON格式结果

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="${PROJECT_ROOT:-$(cd "$SCRIPT_DIR/.." && pwd)}"

# 默认值
INPUT_FILE=""
INPUT_TYPE="auto"  # auto, json, log, export, webhook
OUTPUT_FORMAT="text"  # text, json
VERBOSE=false

# 使用说明
usage() {
    cat << EOF
使用说明: $(basename "$0") [选项]

选项:
    -i, --input <文件>    输入文件路径 (必需)
    -t, --type <类型>     输入类型: auto, json, log, export, webhook (默认: auto)
    -o, --output <格式>   输出格式: text, json (默认: text)
    -v, --verbose         详细输出
    -h, --help            显示帮助信息

示例:
    $(basename "$0") --input response.json
    $(basename "$0") --input logs/app.log --type log

退出码:
    0 - 无凭证泄露
    1 - 发现凭证泄露
    2 - 错误

EOF
    exit 0
}

# 解析命令行参数
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -i|--input)
                INPUT_FILE="$2"
                shift 2
                ;;
            -t|--type)
                INPUT_TYPE="$2"
                shift 2
                ;;
            -o|--output)
                OUTPUT_FORMAT="$2"
                shift 2
                ;;
            -v|--verbose)
                VERBOSE=true
                shift
                ;;
            -h|--help)
                usage
                ;;
            *)
                echo "未知选项: $1"
                usage
                ;;
        esac
    done
}

# 验证输入文件
validate_input() {
    if [ -z "$INPUT_FILE" ]; then
        echo "ERROR: 必须指定输入文件 (--input)" >&2
        exit 2
    fi

    if [ ! -f "$INPUT_FILE" ]; then
        if [ "$OUTPUT_FORMAT" = "json" ]; then
            echo "{\"status\": \"error\", \"message\": \"file not found: $INPUT_FILE\"}" >&2
        else
            echo "ERROR: 文件不存在: $INPUT_FILE" >&2
        fi
        exit 2
    fi
}

# 检测输入类型
detect_input_type() {
    if [ "$INPUT_TYPE" != "auto" ]; then
        return
    fi

    # 根据文件扩展名检测
    case "$INPUT_FILE" in
        *.json)
            INPUT_TYPE="json"
            ;;
        *.log)
            INPUT_TYPE="log"
            ;;
        *.csv)
            INPUT_TYPE="export"
            ;;
        *)
            # 尝试检测是否为JSON
            if head -c 10 "$INPUT_FILE" 2>/dev/null | grep -q '{'; then
                INPUT_TYPE="json"
            else
                INPUT_TYPE="log"
            fi
            ;;
    esac
}

# 扫描JSON内容
scan_json() {
    local content="$1"

    if ! command -v python3 >/dev/null 2>&1; then
        # 没有Python，使用grep
        local found=0
        for pattern in \
            "sk-[a-zA-Z0-9]\{20,\}" \
            "sk-ant-[a-zA-Z0-9-]\{20,\}" \
            "AKIA[0-9A-Z]\{16\}" \
            "api[_-]key" \
            "bearer" \
            "secret" \
            "token"; do
            if grep -qE "$pattern" "$INPUT_FILE" 2>/dev/null; then
                found=$((found + $(grep -cE "$pattern" "$INPUT_FILE" 2>/dev/null || echo 0)))
            fi
        done
        echo "$found"
        return
    fi

    # 使用Python进行JSON解析和凭证扫描
    python3 << 'PYTHON_SCRIPT'
import sys
import re
import json

patterns = [
    r"sk-[a-zA-Z0-9]{20,}",
    r"sk-ant-[a-zA-Z0-9-]{20,}",
    r"AKIA[0-9A-Z]{16}",
    r"api_key",
    r"bearer",
    r"secret",
    r"token",
]

try:
    content = sys.stdin.read()
    data = json.loads(content)

    def search_strings(obj, path=""):
        results = []
        if isinstance(obj, str):
            for pattern in patterns:
                if re.search(pattern, obj, re.IGNORECASE):
                    results.append(pattern)
            return results
        elif isinstance(obj, dict):
            result = []
            for key, value in obj.items():
                result.extend(search_strings(value, f"{path}.{key}"))
            return result
        elif isinstance(obj, list):
            result = []
            for i, item in enumerate(obj):
                result.extend(search_strings(item, f"{path}[{i}]"))
            return result
        return []

    all_matches = search_strings(data)
    # 去重
    unique_patterns = list(set(all_matches))
    print(len(unique_patterns))

except Exception:
    print("0")
PYTHON_SCRIPT
}

# 执行扫描
run_scan() {
    local credentials_found

    case "$INPUT_TYPE" in
        json|webhook)
            credentials_found=$(scan_json "$(cat "$INPUT_FILE")")
            ;;
        log)
            credentials_found=$(scan_json "$(cat "$INPUT_FILE")")
            ;;
        export)
            credentials_found=$(scan_json "$(cat "$INPUT_FILE")")
            ;;
        *)
            credentials_found=$(scan_json "$(cat "$INPUT_FILE")")
            ;;
    esac

    # 确保credentials_found是数字
    credentials_found=${credentials_found:-0}

    # 输出结果
    if [ "$OUTPUT_FORMAT" = "json" ]; then
        if [ "$credentials_found" -gt 0 ] 2>/dev/null; then
            echo "{\"status\": \"failed\", \"credentials_found\": $credentials_found, \"rule_id\": \"CRED-EXPOSE-RESPONSE\"}"
            return 1
        else
            echo "{\"status\": \"passed\", \"credentials_found\": 0}"
            return 0
        fi
    else
        if [ "$credentials_found" -gt 0 ] 2>/dev/null; then
            echo "[M-013] FAILED: 发现 $credentials_found 个凭证泄露"
            return 1
        else
            echo "[M-013] PASSED: 无凭证泄露"
            return 0
        fi
    fi
}

# 主函数
main() {
    parse_args "$@"
    validate_input
    detect_input_type

    run_scan
}

# 运行
main "$@"
