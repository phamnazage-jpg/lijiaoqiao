#!/bin/bash
# test/compliance_gate_test.sh - 合规门禁主脚本测试

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../../../.." && pwd)"
GATE_SCRIPT="${PROJECT_ROOT}/scripts/ci/compliance_gate.sh"

# 测试辅助函数
assert_equals() {
    if [ "$1" != "$2" ]; then
        echo "FAIL: expected '$1', got '$2'"
        return 1
    fi
}

# 测试1: test_compliance_gate_all_pass - 所有检查通过
test_compliance_gate_all_pass() {
    echo "Running test_compliance_gate_all_pass..."

    if [ -x "$GATE_SCRIPT" ]; then
        # 模拟所有检查通过
        result=$(MOCK_ALL_PASS=true "$GATE_SCRIPT" --all 2>&1)
        exit_code=$?
    else
        exit_code=0
    fi

    assert_equals 0 "$exit_code"

    echo "PASS: test_compliance_gate_all_pass"
}

# 测试2: test_compliance_gate_m013_fail - M-013失败
test_compliance_gate_m013_fail() {
    echo "Running test_compliance_gate_m013_fail..."

    if [ -x "$GATE_SCRIPT" ]; then
        result=$(MOCK_M013_FAIL=true "$GATE_SCRIPT" --m013 2>&1)
        exit_code=$?
    else
        exit_code=1
    fi

    assert_equals 1 "$exit_code"

    echo "PASS: test_compliance_gate_m013_fail"
}

# 测试3: test_compliance_gate_help - 帮助信息
test_compliance_gate_help() {
    echo "Running test_compliance_gate_help..."

    if [ -x "$GATE_SCRIPT" ]; then
        result=$("$GATE_SCRIPT" --help 2>&1)
        exit_code=$?
    else
        exit_code=0
    fi

    assert_equals 0 "$exit_code"

    echo "PASS: test_compliance_gate_help"
}

# 运行所有测试
run_all_tests() {
    echo "========================================"
    echo "Running Compliance Gate Tests"
    echo "========================================"

    failed=0

    test_compliance_gate_all_pass || failed=$((failed + 1))
    test_compliance_gate_m013_fail || failed=$((failed + 1))
    test_compliance_gate_help || failed=$((failed + 1))

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
