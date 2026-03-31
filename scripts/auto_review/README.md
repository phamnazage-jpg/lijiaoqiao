# 自动化Review系统使用指南

## 一、系统概述

本系统为立交桥项目提供自动化的周期性review功能，每3小时执行一次检查，每天生成一份完整报告。

## 二、系统架构

```
scripts/auto_review/
├── auto_review.sh          # 主脚本
├── auto_review_config.sh   # 配置文件
├── review.sh               # 快速入口
├── crontab_config         # Cron配置
└── task_queue.json         # 任务队列

review/
├── daily_reports/          # 每日报告目录
├── knowledge_base/         # 经验知识库
└── task_queue.json         # 任务队列
```

## 三、使用方法

### 3.1 手动执行

```bash
# 执行3小时review
./scripts/auto_review/review.sh hourly

# 执行每日全面review
./scripts/auto_review/review.sh daily

# 强制执行完整review
./scripts/auto_review/review.sh force
```

### 3.2 定时任务配置

添加到crontab：

```bash
# 编辑crontab
crontab -e

# 添加以下行：
# 每3小时执行一次
0 */3 * * * /home/long/project/立交桥/scripts/auto_review/review.sh hourly >> /home/long/project/立交桥/logs/auto_review/cron.log 2>&1

# 每天凌晨3点执行全面review
0 3 * * * /home/long/project/立交桥/scripts/auto_review/review.sh daily >> /home/long/project/立交桥/logs/auto_review/cron_daily.log 2>&1
```

## 四、生成的报告

### 4.1 每日报告
- 位置：`review/daily_reports/daily_review_YYYY-MM-DD.md`
- 内容：变更文件、待办任务、新发现问题、专家状态

### 4.2 Claude Code任务
- 位置：`review/claude_tasks_YYYY-MM-DD.md`
- 触发条件：发现问题或文档变更

### 4.3 经验知识库
- 位置：`review/knowledge_base/rules_and_experience_YYYY-MM-DD.md`
- 更新频率：每天凌晨3点

## 五、配置说明

编辑 `auto_review_config.sh` 可修改：

- 项目根目录
- Review频率
- 关键文档列表
- 专家角色列表

## 六、任务分发

当review发现问题或文档变更时，系统会：

1. 生成Claude任务文件
2. 更新任务队列
3. 记录到日志

用户可以查看任务文件并交给Claude Code执行。

## 七、日志

- 日志目录：`logs/auto_review/`
- 日志文件：`review_YYYYMMDD.log`

---

**维护者**：自动化系统
**更新时间**：2026-03-30
