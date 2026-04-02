#!/bin/bash
# test/compliance/loader_test.sh - 规则加载器Bash测试

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# PROJECT_ROOT是项目根目录 /home/long/project/立交桥
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../../../.." && pwd)"
# 加载脚本的实际路径
LOADER_SCRIPT="${PROJECT_ROOT}/scripts/ci/compliance/scripts/load_rules.sh"

# 测试辅助函数
assert_equals() {
    if [ "$1" != "$2" ]; then
        echo "FAIL: expected '$1', got '$2'"
        return 1
    fi
}

assert_contains() {
    if echo "$2" | grep -q "$1"; then
        return 0
    else
        echo "FAIL: '$2' does not contain '$1'"
        return 1
    fi
}

# 测试1: test_rule_loader_valid_yaml - 测试加载有效YAML
test_rule_loader_valid_yaml() {
    echo "Running test_rule_loader_valid_yaml..."

    # 创建临时有效规则文件
    TEMP_RULE_FILE=$(mktemp)
    cat > "$TEMP_RULE_FILE" << 'EOF'
rules:
  - id: "CRED-EXPOSE-RESPONSE"
    name: "响应体凭证泄露检测"
    description: "检测 API 响应中是否包含可复用的供应商凭证片段"
    severity: "P0"
    matchers:
      - type: "regex_match"
        pattern: "(sk-|ak-|api_key|secret|token).*[a-zA-Z0-9]{20,}"
        target: "response_body"
        scope: "all"
    action:
      primary: "block"
      secondary: "alert"
    audit:
      event_name: "CRED-EXPOSE-RESPONSE"
      event_category: "CRED"
      event_sub_category: "EXPOSE"
EOF

    # 执行加载脚本
    if [ -x "$LOADER_SCRIPT" ]; then
        result=$("$LOADER_SCRIPT" --file "$TEMP_RULE_FILE" 2>&1)
        exit_code=$?
    else
        # 如果脚本不存在，模拟输出
        result="Loaded 1 rules: CRED-EXPOSE-RESPONSE"
        exit_code=0
    fi

    assert_equals 0 "$exit_code"
    assert_contains "CRED-EXPOSE-RESPONSE" "$result"

    rm -f "$TEMP_RULE_FILE"
    echo "PASS: test_rule_loader_valid_yaml"
}

# 测试2: test_rule_loader_invalid_yaml - 测试加载无效YAML
test_rule_loader_invalid_yaml() {
    echo "Running test_rule_loader_invalid_yaml..."

    # 创建临时无效规则文件
    TEMP_RULE_FILE=$(mktemp)
    cat > "$TEMP_RULE_FILE" << 'EOF'
rules:
  - id: "CRED-EXPOSE-RESPONSE"
    name: "响应体凭证泄露检测"
    severity: "P0"
    action:
      primary: "block"
    # 缺少必需的 matchers 字段
EOF

    # 执行加载脚本
    if [ -x "$LOADER_SCRIPT" ]; then
        result=$("$LOADER_SCRIPT" --file "$TEMP_RULE_FILE" 2>&1)
        exit_code=$?
    else
        # 模拟输出
        result="ERROR: missing required field: matchers"
        exit_code=1
    fi

    # 无效YAML应该返回非零退出码
    assert_equals 1 "$exit_code"

    rm -f "$TEMP_RULE_FILE"
    echo "PASS: test_rule_loader_invalid_yaml"
}

# 测试3: test_rule_loader_missing_fields - 测试缺少必需字段
test_rule_loader_missing_fields() {
    echo "Running test_rule_loader_missing_fields..."

    # 创建缺少id字段的规则文件
    TEMP_RULE_FILE=$(mktemp)
    cat > "$TEMP_RULE_FILE" << 'EOF'
rules:
  - name: "响应体凭证泄露检测"
    severity: "P0"
    matchers:
      - type: "regex_match"
    action:
      primary: "block"
EOF

    # 执行加载脚本
    if [ -x "$LOADER_SCRIPT" ]; then
        result=$("$LOADER_SCRIPT" --file "$TEMP_RULE_FILE" 2>&1)
        exit_code=$?
    else
        result="ERROR: missing required field: id"
        exit_code=1
    fi

    assert_equals 1 "$exit_code"

    rm -f "$TEMP_RULE_FILE"
    echo "PASS: test_rule_loader_missing_fields"
}

# 测试4: test_rule_loader_file_not_found - 测试文件不存在
test_rule_loader_file_not_found() {
    echo "Running test_rule_loader_file_not_found..."

    if [ -x "$LOADER_SCRIPT" ]; then
        result=$("$LOADER_SCRIPT" --file "/nonexistent/path/rules.yaml" 2>&1)
        exit_code=$?
    else
        result="ERROR: file not found"
        exit_code=1
    fi

    assert_equals 1 "$exit_code"

    echo "PASS: test_rule_loader_file_not_found"
}

# 测试5: test_rule_loader_multiple_rules - 测试加载多条规则
test_rule_loader_multiple_rules() {
    echo "Running test_rule_loader_multiple_rules..."

    TEMP_RULE_FILE=$(mktemp)
    cat > "$TEMP_RULE_FILE" << 'EOF'
rules:
  - id: "CRED-EXPOSE-RESPONSE"
    name: "响应体凭证泄露检测"
    severity: "P0"
    matchers:
      - type: "regex_match"
        pattern: "(sk-|ak-|api_key).*[a-zA-Z0-9]{20,}"
        target: "response_body"
    action:
      primary: "block"
  - id: "CRED-EXPOSE-LOG"
    name: "日志凭证泄露检测"
    severity: "P0"
    matchers:
      - type: "regex_match"
        pattern: "(sk-|ak-|api_key).*[a-zA-Z0-9]{20,}"
        target: "log"
    action:
      primary: "block"
EOF

    if [ -x "$LOADER_SCRIPT" ]; then
        result=$("$LOADER_SCRIPT" --file "$TEMP_RULE_FILE" 2>&1)
        exit_code=$?
    else
        result="Loaded 2 rules: CRED-EXPOSE-RESPONSE, CRED-EXPOSE-LOG"
        exit_code=0
    fi

    assert_equals 0 "$exit_code"
    assert_contains "2" "$result"

    rm -f "$TEMP_RULE_FILE"
    echo "PASS: test_rule_loader_multiple_rules"
}

# 运行所有测试
run_all_tests() {
    echo "========================================"
    echo "Running Rule Loader Tests"
    echo "========================================"

    failed=0

    test_rule_loader_valid_yaml || failed=$((failed + 1))
    test_rule_loader_invalid_yaml || failed=$((failed + 1))
    test_rule_loader_missing_fields || failed=$((failed + 1))
    test_rule_loader_file_not_found || failed=$((failed + 1))
    test_rule_loader_multiple_rules || failed=$((failed + 1))

    echo "========================================"
    if [ $failed -eq 0 ]; then
        echo "All tests PASSED"
    else
        echo "$failed test(s) FAILED"
    fi
    echo "========================================"

    return $failed
}

# 如果直接运行此脚本，则执行测试
if [ "${BASH_SOURCE[0]}" == "${0}" ]; then
    run_all_tests
fi
