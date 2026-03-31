#===============================================================================
# 自动化Review配置
#===============================================================================

# 项目根目录
export PROJECT_ROOT="/home/long/project/立交桥"

# Review目录
export REVIEW_DIR="$PROJECT_ROOT/review"
export REPORT_DIR="$REVIEW_DIR/daily_reports"
export KNOWLEDGE_DIR="$REVIEW_DIR/knowledge_base"

# 任务队列
export TASK_QUEUE="$REVIEW_DIR/task_queue.json"

# 日志目录
export LOG_DIR="$PROJECT_ROOT/logs/auto_review"

# Review频率（小时）
export REVIEW_INTERVAL=3

# 每日更新时间（小时，24小时制）
export DAILY_UPDATE_HOUR=3

# 需要检查的关键文档列表
export CRITICAL_DOCS=(
    "docs/architecture_solution_v1_2026-03-18.md"
    "docs/api_solution_v1_2026-03-18.md"
    "docs/security_solution_v1_2026-03-18.md"
    "docs/business_solution_v1_2026-03-18.md"
    "docs/llm_gateway_prd_v1_2026-03-25.md"
    "docs/supply_technical_design_enhanced_v1_2026-03-25.md"
)

# 专家评审角色
readonly EXPERT_ROLES="E01:架构负责人,E02:平台工程负责人,E03:SRE负责人,E04:安全负责人,E05:计费/数据负责人,E06:合规/法务接口人,E07:产品负责人,E13:用户代表,E14:测试负责人,E15:网关专家"

# Claude Code命令（用于分发任务）
export CLAUDE_CLI_CMD="claude"

# 是否启用Claude Code任务分发
export ENABLE_TASK_DISPATCH=true
