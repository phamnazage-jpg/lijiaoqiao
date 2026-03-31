#!/bin/bash
#===============================================================================
# 自动化周期性Review脚本
# 功能：每3小时执行一次项目全面review，生成报告并分发任务
# 使用：./auto_review.sh [hourly|daily|force]
#===============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
REVIEW_DIR="$PROJECT_ROOT/review"
REPORT_DIR="$REVIEW_DIR/daily_reports"
KNOWLEDGE_DIR="$REVIEW_DIR/knowledge_base"
TASK_QUEUE="$REVIEW_DIR/task_queue.json"
LOG_DIR="$PROJECT_ROOT/logs/auto_review"

# 加载配置
source "$SCRIPT_DIR/auto_review_config.sh"

#-------------------------------------------------------------------------------
# 日志函数
#-------------------------------------------------------------------------------
log() {
    local level=$1
    shift
    local msg="[$(date '+%Y-%m-%d %H:%M:%S')] [$level] $*"
    echo "$msg"
    echo "$msg" >> "$LOG_DIR/review_$(date '+%Y%m%d').log"
}

#-------------------------------------------------------------------------------
# 初始化
#-------------------------------------------------------------------------------
init() {
    mkdir -p "$LOG_DIR" "$REPORT_DIR" "$KNOWLEDGE_DIR"
    
    # 初始化任务队列
    if [ ! -f "$TASK_QUEUE" ]; then
        echo '{"tasks":[], "last_updated":"", "last_review_date":""}' > "$TASK_QUEUE"
    fi
    
    log "INFO" "Auto review system initialized"
}

#-------------------------------------------------------------------------------
# 获取文档变更状态
#-------------------------------------------------------------------------------
get_doc_changes() {
    local since="${1:-24h}"
    git -C "$PROJECT_ROOT" diff --name-only --since="$since" -- "docs/" "review/" 2>/dev/null || echo ""
}

#-------------------------------------------------------------------------------
# 读取上一份报告
#-------------------------------------------------------------------------------
read_last_report() {
    local last_report=$(ls -t "$REPORT_DIR"/daily_review_*.md 2>/dev/null | head -1)
    if [ -n "$last_report" ]; then
        echo "$last_report"
    else
        echo ""
    fi
}

#-------------------------------------------------------------------------------
# 提取未完成任务
#-------------------------------------------------------------------------------
extract_pending_tasks() {
    local report_file="$1"
    if [ -z "$report_file" ] || [ ! -f "$report_file" ]; then
        echo "[]"
        return
    fi
    
    # 提取P0/P1任务
    grep -E "^\s*\|.*P[01].*\|" "$report_file" 2>/dev/null | \
        grep -v "完成\|已关闭\|CLOSED" | \
        sed -E 's/\|/,/g' | \
        awk -F',' '{print "{\"id\":\""$1"\",\"desc\":\""$3"\",\"owner\":\""$4"\"}"}' | \
        jq -s '.' 2>/dev/null || echo "[]"
}

#-------------------------------------------------------------------------------
# 执行Review检查
#-------------------------------------------------------------------------------
perform_review() {
    local review_type="$1"
    log "INFO" "Starting $review_type review..."
    
    # 收集变更文件
    local changes=""
    local change_count=0
    changes=$(get_doc_changes "3 hours ago" 2>/dev/null || echo "")
    change_count=$(echo "$changes" | grep -c "." 2>/dev/null || echo "0")
    
    # 检查是否有新的设计文档
    local new_docs=0
    new_docs=$(git -C "$PROJECT_ROOT" status --porcelain "docs/" 2>/dev/null | grep "^??" | wc -l 2>/dev/null || echo "0")
    new_docs=${new_docs:-0}
    
    # 检查未完成任务
    local last_report=$(read_last_report)
    local pending_tasks="[]"
    local pending_count=0
    if [ -n "$last_report" ] && [ -f "$last_report" ]; then
        pending_tasks=$(extract_pending_tasks "$last_report" 2>/dev/null || echo "[]")
        pending_count=$(echo "$pending_tasks" | jq 'length' 2>/dev/null || echo "0")
    fi
    pending_count=${pending_count:-0}
    
    # 执行快速检查（模拟专家review）
    local issues_found=0
    
    # 检查关键文档是否存在
    local critical_doc
    for critical_doc in "$REVIEW_DIR/comprehensive_expert_review_report_v2_2026-03-18.md" "$PROJECT_ROOT/docs/architecture_solution_v1_2026-03-18.md"; do
        if [ ! -f "$critical_doc" ]; then
            log "WARN" "Critical document missing: $critical_doc"
            issues_found=$((issues_found + 1))
        fi
    done
    
    # 构建JSON（确保所有值都是有效的）
    local change_files_json="[]"
    if [ -n "$changes" ]; then
        change_files_json=$(echo "$changes" | jq -R -s 'split("\n") | map(select(length > 0))' 2>/dev/null || echo "[]")
    fi
    
    # 确定是否需要处理
    local action_required="false"
    if [ "$issues_found" -gt 0 ] || [ "$change_count" -gt 0 ]; then
        action_required="true"
    fi
    
    # 返回Review结果
    cat << EOF
{
    "review_type": "$review_type",
    "timestamp": "$(date -Iseconds)",
    "changes_count": $change_count,
    "new_docs_count": $new_docs,
    "pending_tasks_count": $pending_count,
    "issues_found": $issues_found,
    "change_files": $change_files_json,
    "action_required": $action_required
}
EOF
}

#-------------------------------------------------------------------------------
# 生成每日报告
#-------------------------------------------------------------------------------
generate_daily_report() {
    local review_data="$1"
    local date_str=$(date '+%Y-%m-%d')
    
    log "INFO" "Generating daily review report for $date_str..."
    
    local report_file="$REPORT_DIR/daily_review_${date_str}.md"
    local last_report=$(read_last_report)
    
    # 提取review数据
    local changes_count new_docs_count pending_tasks_count issues_found
    changes_count=$(echo "$review_data" | jq -r '.changes_count // 0' 2>/dev/null || echo "0")
    new_docs_count=$(echo "$review_data" | jq -r '.new_docs_count // 0' 2>/dev/null || echo "0")
    pending_tasks_count=$(echo "$review_data" | jq -r '.pending_tasks_count // 0' 2>/dev/null || echo "0")
    issues_found=$(echo "$review_data" | jq -r '.issues_found // 0' 2>/dev/null || echo "0")
    
    # 提取变更文件列表
    local change_list
    change_list=$(echo "$review_data" | jq -r '.change_files[] // empty' 2>/dev/null | sed 's/^/- /' || echo "无变更")
    
    # 生成报告头部
    cat > "$report_file" << EOF
# 立交桥项目每日Review报告

> 生成时间：$(date '+%Y-%m-%d %H:%M:%S')
> 报告日期：$date_str
> Review类型：每日全面检查

---

## 一、Review执行摘要

| 指标 | 数值 | 较昨日 |
|------|------|--------|
| 文档变更数 | $changes_count | - |
| 新增文档数 | $new_docs_count | - |
| 待完成任务 | $pending_tasks_count | - |
| 发现问题 | $issues_found | - |

---

## 二、变更文件清单

$change_list

---

## 三、待完成任务追踪

### 3.1 P0问题（阻断上线）

EOF

    # 添加P0任务列表
    if [ -n "$last_report" ]; then
        grep -A 50 "### 3.1 P0问题" "$last_report" 2>/dev/null | head -30 >> "$report_file" || echo "| - | - | - | - |" >> "$report_file"
    else
        echo "| 编号 | 问题描述 | Owner | 状态 |" >> "$report_file"
        echo "|-----|----------|-------|------|" >> "$report_file"
        echo "| - | 暂无 | - | - |" >> "$report_file"
    fi

    cat >> "$report_file" << EOF

### 3.2 P1问题（高优先级）

EOF

    if [ -n "$last_report" ]; then
        grep -A 30 "### 3.2 P1问题" "$last_report" 2>/dev/null | head -20 >> "$report_file" || echo "| - | - | - |" >> "$report_file"
    else
        echo "| 编号 | 问题描述 | Owner |" >> "$report_file"
        echo "|-----|----------|-------|" >> "$report_file"
    fi

    # 确定行动项文本
    local action_text="无"
    local new_issue_text="| - | - | 无新问题 | - |"
    if [ "$issues_found" -gt 0 ]; then
        action_text="存在 $issues_found 个问题需处理"
        new_issue_text="| NEW-001 | P1 | 新发现的问题（待详细记录） | $(date '+%Y-%m-%d %H:%M') |"
    fi
    
    cat >> "$report_file" << EOF

---

## 四、新发现问题

| 编号 | 等级 | 问题描述 | 发现时间 |
|------|------|----------|----------|
$new_issue_text

---

## 五、建议行动项

1. **立即处理**：$action_text
2. **持续跟进**：$pending_tasks_count 个待办任务
3. **文档更新**：$new_docs_count 个新文档待审核

---

## 六、专家评审状态

| 轮次 | 主题 | 结论 | 日期 |
|------|------|------|------|
| Round-1 | 架构与替换路径 | CONDITIONAL GO | 2026-03-19 |
| Round-2 | 兼容与计费一致性 | CONDITIONAL GO | 2026-03-22 |
| Round-3 | 安全与合规攻防 | CONDITIONAL GO | 2026-03-25 |
| Round-4 | 可靠性与回滚演练 | CONDITIONAL GO | 2026-03-29 |

---

**报告状态**：自动生成
**下次更新**：$(date -d '+3 hours' '+%Y-%m-%d %H:%M')

EOF

    log "INFO" "Daily report generated: $report_file"
    echo "$report_file"
}

#-------------------------------------------------------------------------------
# 更新任务队列
#-------------------------------------------------------------------------------
update_task_queue() {
    local review_data="$1"
    local date_str=$(date '+%Y-%m-%d')
    
    # 提取数据
    local changes_count issues_found action_required
    changes_count=$(echo "$review_data" | jq -r '.changes_count // 0' 2>/dev/null || echo "0")
    issues_found=$(echo "$review_data" | jq -r '.issues_found // 0' 2>/dev/null || echo "0")
    action_required=$(echo "$review_data" | jq -r '.action_required // "false"' 2>/dev/null || echo "false")
    
    # 读取当前队列
    local current_queue=$(cat "$TASK_QUEUE" 2>/dev/null || echo '{"tasks":[]}')
    
    # 更新JSON
    local updated
    updated=$(echo "$current_queue" | jq --arg timestamp "$(date -Iseconds)" \
        --arg date "$date_str" \
        --argjson changes "$changes_count" \
        --argjson issues "$issues_found" \
        '.last_updated = $timestamp | .last_review_date = $date | .review_stats.total_reviews += 1 | .review_stats.issues_found += $issues')
    
    echo "$updated" > "$TASK_QUEUE"
    
    # 如果有问题需要处理，生成任务文件
    if [ "$issues_found" -gt 0 ]; then
        local task_file="$REVIEW_DIR/pending_tasks_$(date '+%Y%m%d_%H%M%S').json"
        echo "$review_data" > "$task_file"
        log "WARN" "Issues found! Task file created: $task_file"
    fi
}

#-------------------------------------------------------------------------------
# 生成Claude Code任务
#-------------------------------------------------------------------------------
generate_claude_tasks() {
    local review_data="$1"
    local date_str=$(date '+%Y-%m-%d')
    
    # 提取数据
    local needs_action issues_found changes_count pending_tasks_count
    needs_action=$(echo "$review_data" | jq -r '.action_required // "false"' 2>/dev/null || echo "false")
    issues_found=$(echo "$review_data" | jq -r '.issues_found // 0' 2>/dev/null || echo "0")
    changes_count=$(echo "$review_data" | jq -r '.changes_count // 0' 2>/dev/null || echo "0")
    pending_tasks_count=$(echo "$review_data" | jq -r '.pending_tasks_count // 0' 2>/dev/null || echo "0")
    
    # 提取变更文件列表
    local change_list
    change_list=$(echo "$review_data" | jq -r '.change_files[] // empty' 2>/dev/null | sed 's/^/1. 审核文档：/' || echo "1. 检查并处理review发现的问题")
    
    if [ "$needs_action" = "true" ]; then
        local task_file="$REVIEW_DIR/claude_tasks_${date_str}.md"
        
        cat > "$task_file" << EOF
# Claude Code 执行任务

> 生成时间：$(date '+%Y-%m-%d %H:%M:%S')
> 触发条件：Review发现需要处理的问题

## 执行要求

请Claude Code CLI按照以下规范执行：

1. **遵循superpowers插件规范**
2. **严格按照项目规划设计执行**
3. **优先处理P0问题**

## 待处理问题清单

- 问题数量：$issues_found
- 文档变更：$changes_count 个文件
- 待办任务：$pending_tasks_count 个

## 具体任务

$change_list

---

**状态**：等待执行
**优先级**：高
EOF
        
        log "INFO" "Claude tasks generated: $task_file"
        echo "$task_file"
    else
        echo ""
    fi
}

#-------------------------------------------------------------------------------
# 更新经验知识库（每日3点执行）
#-------------------------------------------------------------------------------
update_knowledge_base() {
    local is_daily=$(date '+%H')
    
    # 只在每天3点执行
    if [ "$is_daily" != "03" ]; then
        log "INFO" "Skipping knowledge base update (not 3am, current: ${is_daily}00)"
        return 0
    fi
    
    log "INFO" "Updating knowledge base..."
    
    local date_str=$(date '+%Y-%m-%d')
    local kb_file="$KNOWLEDGE_BASE/rules_and_experience_${date_str}.md"
    
    # 收集当天经验
    local issues=""
    local last_report=$(read_last_report)
    if [ -n "$last_report" ]; then
        issues=$(grep -E "^\|.*P[01]" "$last_report" 2>/dev/null | wc -l)
    fi
    
    cat > "$kb_file" << EOF
# 立交桥项目经验与规则

> 更新时间：$(date '+%Y-%m-%d %H:%M:%S')
> 版本：$(date '+%Y%m%d')

## 一、项目关键规范

### 1.1 架构原则
- Provider Adapter抽象层设计
- 三层降级策略（同平台换号/同区域换平台/全局降级）
- 分阶段验证（S2-A/B/C1/C2）

### 1.2 安全红线
- 内网隔离 + mTLS双向认证
- 契约漂移CI阻断
- 密钥90天轮换

### 1.3 质量门禁
- 接管率 >= 99.9% 覆盖率
- 自动回滚 <= 10分钟
- 服务恢复 <= 30分钟
- 用户通知 <= 15分钟

## 二、待解决P0问题

- 数量：$issues 个（来自最新报告）

## 三、专家评审结论

| 维度 | 结论 | 评分 |
|------|------|------|
| 架构 | CONDITIONAL GO | 3.5/5 |
| API设计 | CONDITIONAL GO | 4.0/5 |
| 安全防护 | CONDITIONAL GO | 3.0/5 |
| 业务合规 | CONDITIONAL GO | 3.5/5 |
| 计费精度 | CONDITIONAL GO | 4.0/5 |
| 可靠性 | CONDITIONAL GO | 3.0/5 |

## 四、行动优先级

1. **P0**：安全验证、契约测试、降级演练
2. **P1**：用户体验、SLA文档、计费准确性
3. **P2**：SDK开发、法务确认、DDoS防护

---

**状态**：每日自动更新
**下次更新**：$(date -d '+1 day' -d '3:00' '+%Y-%m-%d %H:%M')
EOF
    
    log "INFO" "Knowledge base updated: $kb_file"
}

#-------------------------------------------------------------------------------
# 主函数
#-------------------------------------------------------------------------------
main() {
    local mode="${1:-hourly}"
    
    init
    
    case "$mode" in
        hourly)
            log "INFO" "Running hourly review..."
            local review_result=$(perform_review "hourly")
            update_task_queue "$review_result"
            generate_claude_tasks "$review_result"
            ;;
        daily)
            log "INFO" "Running daily full review..."
            local review_result=$(perform_review "daily")
            local report=$(generate_daily_report "$review_result")
            update_task_queue "$review_result"
            generate_claude_tasks "$review_result"
            update_knowledge_base
            log "INFO" "Daily review completed. Report: $report"
            ;;
        force)
            log "WARN" "Running forced full review..."
            local review_result=$(perform_review "force")
            local report=$(generate_daily_report "$review_result")
            update_task_queue "$review_result"
            generate_claude_tasks "$review_result"
            log "INFO" "Forced review completed. Report: $report"
            ;;
        *)
            echo "Usage: $0 [hourly|daily|force]"
            exit 1
            ;;
    esac
}

main "$@"
