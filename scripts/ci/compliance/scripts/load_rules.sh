#!/usr/bin/env bash
# compliance/scripts/load_rules.sh - Bash规则加载脚本
# 功能：加载和验证YAML规则配置文件

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
COMPLIANCE_BASE="$(cd "$SCRIPT_DIR/.." && pwd)"

# 默认值
VERBOSE=false
RULES_FILE=""

# 使用说明
usage() {
    cat << EOF
使用说明: $(basename "$0") [选项]

选项:
    -f, --file <文件>    规则YAML文件路径
    -v, --verbose        详细输出
    -h, --help           显示帮助信息

示例:
    $(basename "$0") --file rules.yaml
    $(basename "$0") -f rules.yaml -v

EOF
    exit 0
}

# 解析命令行参数
parse_args() {
    while [[ $# -gt 0 ]]; do
        case $1 in
            -f|--file)
                RULES_FILE="$2"
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

# 验证YAML文件存在
validate_file() {
    if [ -z "$RULES_FILE" ]; then
        echo "ERROR: 必须指定规则文件 (--file)"
        exit 1
    fi

    if [ ! -f "$RULES_FILE" ]; then
        echo "ERROR: 文件不存在: $RULES_FILE"
        exit 1
    fi
}

# 验证YAML语法
validate_yaml_syntax() {
    if command -v python3 >/dev/null 2>&1; then
        # 使用Python进行YAML验证
        if ! python3 -c "import yaml; yaml.safe_load(open('$RULES_FILE'))" 2>/dev/null; then
            echo "ERROR: YAML语法错误: $RULES_FILE"
            exit 1
        fi
    elif command -v yq >/dev/null 2>&1; then
        # 使用yq进行YAML验证
        if ! yq '.' "$RULES_FILE" >/dev/null 2>&1; then
            echo "ERROR: YAML语法错误: $RULES_FILE"
            exit 1
        fi
    else
        # 如果没有验证工具，进行基本检查
        if ! grep -q "^rules:" "$RULES_FILE"; then
            echo "ERROR: 缺少 'rules:' 根元素"
            exit 1
        fi
    fi
}

# 验证规则ID格式
validate_rule_id_format() {
    local id="$1"
    # 格式: {Category}-{SubCategory}[-{Detail}]
    if ! [[ "$id" =~ ^[A-Z]{2,4}-[A-Z]{2,10}(-[A-Z0-9]{1,20})?$ ]]; then
        echo "ERROR: 无效的规则ID格式: $id"
        echo "       期望格式: {Category}-{SubCategory}[-{Detail}]"
        return 1
    fi
    return 0
}

# 验证必需字段
validate_required_fields() {
    local rule_json="$1"
    local rule_id

    # 使用Python提取规则ID
    if command -v python3 >/dev/null 2>&1; then
        rule_id=$(python3 -c "import yaml; rules = yaml.safe_load(open('$RULES_FILE')); print('none')" 2>/dev/null || echo "none")
    fi

    # 基本验证：检查rules数组存在
    if ! grep -q "^- " "$RULES_FILE"; then
        echo "ERROR: 缺少规则定义"
        exit 1
    fi
}

# 加载规则
load_rules() {
    local count=0

    if [ "$VERBOSE" = true ]; then
        echo "[DEBUG] 加载规则文件: $RULES_FILE"
    fi

    # 验证YAML语法
    validate_yaml_syntax

    # 使用Python解析YAML并验证
    if command -v python3 >/dev/null 2>&1; then
        python3 << 'PYTHON_SCRIPT'
import sys
import yaml
import re

try:
    with open('$RULES_FILE', 'r') as f:
        config = yaml.safe_load(f)

    if not config or 'rules' not in config:
        print("ERROR: 缺少 'rules' 根元素")
        sys.exit(1)

    rules = config['rules']
    if not isinstance(rules, list):
        print("ERROR: 'rules' 必须是数组")
        sys.exit(1)

    # 规则ID格式验证
    pattern = re.compile(r'^[A-Z]{2,4}-[A-Z]{2,10}(-[A-Z0-9]{1,20})?$')

    for i, rule in enumerate(rules):
        if 'id' not in rule:
            print(f"ERROR: 规则[{i}]缺少必需字段: id")
            sys.exit(1)
        if 'name' not in rule:
            print(f"ERROR: 规则[{i}]缺少必需字段: name")
            sys.exit(1)
        if 'severity' not in rule:
            print(f"ERROR: 规则[{i}]缺少必需字段: severity")
            sys.exit(1)
        if 'matchers' not in rule or not rule['matchers']:
            print(f"ERROR: 规则[{i}]缺少必需字段: matchers")
            sys.exit(1)
        if 'action' not in rule or 'primary' not in rule['action']:
            print(f"ERROR: 规则[{i}]缺少必需字段: action.primary")
            sys.exit(1)

        rule_id = rule['id']
        if not pattern.match(rule_id):
            print(f"ERROR: 无效的规则ID格式: {rule_id}")
            print(f"       期望格式: {{Category}}-{{SubCategory}}[{{-Detail}}]")
            sys.exit(1)

        # 验证正则表达式
        for j, matcher in enumerate(rule['matchers']):
            if 'type' not in matcher:
                print(f"ERROR: 规则[{i}].matchers[{j}]缺少type字段")
                sys.exit(1)
            if 'pattern' not in matcher:
                print(f"ERROR: 规则[{i}].matchers[{j}]缺少pattern字段")
                sys.exit(1)
            try:
                re.compile(matcher['pattern'])
            except re.error as e:
                print(f"ERROR: 规则[{i}].matchers[{j}]正则表达式错误: {e}")
                sys.exit(1)

    print(f"Loaded {len(rules)} rules")
    for rule in rules:
        print(f"  - {rule['id']}: {rule['name']} (Severity: {rule['severity']})")

    sys.exit(0)

except yaml.YAMLError as e:
    print(f"ERROR: YAML解析错误: {e}")
    sys.exit(1)
except Exception as e:
    print(f"ERROR: {e}")
    sys.exit(1)
PYTHON_SCRIPT
    else
        # 备选方案：使用grep和基本验证
        count=$(grep -c "^- id:" "$RULES_FILE" || echo "0")
        echo "Loaded $count rules (basic mode, install python3 for full validation)"

        if [ "$VERBOSE" = true ]; then
            grep "^- id:" "$RULES_FILE" | sed 's/^- id: //' | while read -r id; do
                echo "  - $id"
            done
        fi
    fi
}

# 主函数
main() {
    parse_args "$@"
    validate_file
    load_rules
}

# 运行
main "$@"
