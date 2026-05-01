# 🐉 小龙调度器 (XL Orchestrator)

多角色协同任务管理器，支持 PM → TechLead → Engineer → QA 的工作流。

## 快速开始

```bash
cd .xl-orchestrator

# 1. 创建工作流
python3 task_manager.py create "交立桥质量重构" --desc "从Demo到生产级的全面重构"

# 2. 添加任务
python3 task_manager.py add-task <wf_id> "出版PRD" \
  --role pm --stage requirements --est 30

python3 task_manager.py add-task <wf_id> "技术方案设计" \
  --role tech_lead --stage design --est 45 --deps <task_id>

# 3. 开始任务
python3 task_manager.py status <wf_id> <task_id> in_progress --assignee pm

# 4. 完成任务
python3 task_manager.py status <wf_id> <task_id> done

# 5. 查看进度
python3 task_manager.py report <wf_id>

# 6. 查看下一个任务
python3 task_manager.py next <wf_id> --role engineer
```

## 角色

| 角色 | 职责 |
|------|------|
| `xl_ceo` | 小龙CEO，战略分析与派发 |
| `pm` | 产品经理，输出PRD |
| `tech_lead` | 技术经理，架构与任务拆解 |
| `engineer` | 工程师，实现代码 |
| `qa` | 质量经理，审查把关 |

## 工作流阶段

1. **analysis** - 小龙分析与分解
2. **requirements** - PM出版PRD
3. **design** - TechLead技术设计
4. **implementation** - 工程师实现
5. **qa_review** - QA审查
6. **merged** - 完成合并

## 每日汇报

```bash
./daily-report.sh
```

## 数据存储

- 状态文件: `data/workflow_state.json`
- 报告文件: `data/reports/`
