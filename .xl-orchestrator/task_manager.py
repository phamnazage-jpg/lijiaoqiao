#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
小龙调度器 (XL Orchestrator)
多角色协同任务管理器，支持PM→TechLead→Engineer→QA的工作流
"""

import json
import os
import sys
import hashlib
import subprocess
from datetime import datetime, timedelta
from pathlib import Path
from typing import Dict, List, Optional, Any, Literal
from dataclasses import dataclass, field, asdict
from enum import Enum

# 数据文件路径
DATA_DIR = Path(__file__).parent / "data"
STATE_FILE = DATA_DIR / "workflow_state.json"
REPORTS_DIR = DATA_DIR / "reports"


class TaskStatus(str, Enum):
    PENDING = "pending"
    IN_PROGRESS = "in_progress"
    BLOCKED = "blocked"
    REVIEW = "review"
    APPROVED = "approved"
    DONE = "done"
    FAILED = "failed"


class Role(str, Enum):
    XL_CEO = "xl_ceo"
    PM = "pm"
    TECH_LEAD = "tech_lead"
    ENGINEER = "engineer"
    QA = "qa"


class Stage(str, Enum):
    ANALYSIS = "analysis"          # 小龙分析
    REQUIREMENTS = "requirements"  # PM出PRD
    DESIGN = "design"              # TechLead出技术方案
    IMPLEMENTATION = "implementation"  # 工程师实现
    QA_REVIEW = "qa_review"        # QA审查
    MERGED = "merged"              # 完成合并


@dataclass
class Task:
    id: str
    title: str
    description: str
    role: Role
    stage: Stage
    status: TaskStatus = TaskStatus.PENDING
    parent_id: Optional[str] = None
    dependencies: List[str] = field(default_factory=list)
    assignee: Optional[str] = None
    created_at: str = field(default_factory=lambda: datetime.now().isoformat())
    started_at: Optional[str] = None
    completed_at: Optional[str] = None
    deliverables: List[str] = field(default_factory=list)
    review_feedback: Optional[str] = None
    review_status: Optional[Literal["approved", "changes_requested", "comment"]] = None
    priority: int = 1  # 1=最高
    estimated_minutes: int = 5
    actual_minutes: Optional[int] = None
    tags: List[str] = field(default_factory=list)
    metadata: Dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict:
        return asdict(self)

    @classmethod
    def from_dict(cls, data: dict) -> "Task":
        return cls(
            id=data["id"],
            title=data["title"],
            description=data["description"],
            role=Role(data["role"]),
            stage=Stage(data["stage"]),
            status=TaskStatus(data["status"]),
            parent_id=data.get("parent_id"),
            dependencies=data.get("dependencies", []),
            assignee=data.get("assignee"),
            created_at=data.get("created_at", datetime.now().isoformat()),
            started_at=data.get("started_at"),
            completed_at=data.get("completed_at"),
            deliverables=data.get("deliverables", []),
            review_feedback=data.get("review_feedback"),
            review_status=data.get("review_status"),
            priority=data.get("priority", 1),
            estimated_minutes=data.get("estimated_minutes", 5),
            actual_minutes=data.get("actual_minutes"),
            tags=data.get("tags", []),
            metadata=data.get("metadata", {}),
        )


@dataclass
class Workflow:
    id: str
    title: str
    description: str
    created_at: str = field(default_factory=lambda: datetime.now().isoformat())
    updated_at: str = field(default_factory=lambda: datetime.now().isoformat())
    current_stage: Stage = Stage.ANALYSIS
    tasks: List[Task] = field(default_factory=list)
    status: Literal["active", "paused", "completed", "failed"] = "active"
    metadata: Dict[str, Any] = field(default_factory=dict)

    def to_dict(self) -> dict:
        return {
            "id": self.id,
            "title": self.title,
            "description": self.description,
            "created_at": self.created_at,
            "updated_at": self.updated_at,
            "current_stage": self.current_stage.value,
            "status": self.status,
            "metadata": self.metadata,
            "tasks": [t.to_dict() for t in self.tasks],
        }

    @classmethod
    def from_dict(cls, data: dict) -> "Workflow":
        wf = cls(
            id=data["id"],
            title=data["title"],
            description=data["description"],
            created_at=data.get("created_at", datetime.now().isoformat()),
            updated_at=data.get("updated_at", datetime.now().isoformat()),
            current_stage=Stage(data.get("current_stage", "analysis")),
            status=data.get("status", "active"),
            metadata=data.get("metadata", {}),
        )
        wf.tasks = [Task.from_dict(t) for t in data.get("tasks", [])]
        return wf


class TaskManager:
    """任务管理器: 保存/加载状态、派发任务、生成报告"""

    def __init__(self):
        DATA_DIR.mkdir(parents=True, exist_ok=True)
        REPORTS_DIR.mkdir(parents=True, exist_ok=True)
        self.workflows: Dict[str, Workflow] = {}
        self._load_state()

    def _load_state(self):
        if STATE_FILE.exists():
            try:
                with open(STATE_FILE, "r", encoding="utf-8") as f:
                    data = json.load(f)
                    for wf_id, wf_data in data.get("workflows", {}).items():
                        self.workflows[wf_id] = Workflow.from_dict(wf_data)
            except Exception as e:
                print(f"[警告] 加载状态失败: {e}")

    def _save_state(self):
        data = {
            "updated_at": datetime.now().isoformat(),
            "workflows": {wf_id: wf.to_dict() for wf_id, wf in self.workflows.items()},
        }
        with open(STATE_FILE, "w", encoding="utf-8") as f:
            json.dump(data, f, ensure_ascii=False, indent=2)

    def create_workflow(self, title: str, description: str) -> Workflow:
        wf_id = hashlib.md5(f"{title}{datetime.now().isoformat()}".encode()).hexdigest()[:8]
        wf = Workflow(id=wf_id, title=title, description=description)
        self.workflows[wf_id] = wf
        self._save_state()
        return wf

    def get_workflow(self, wf_id: str) -> Optional[Workflow]:
        return self.workflows.get(wf_id)

    def add_task(
        self,
        wf_id: str,
        title: str,
        description: str,
        role: Role,
        stage: Stage,
        parent_id: Optional[str] = None,
        dependencies: Optional[List[str]] = None,
        estimated_minutes: int = 5,
        priority: int = 1,
        tags: Optional[List[str]] = None,
    ) -> Task:
        wf = self.workflows.get(wf_id)
        if not wf:
            raise ValueError(f"Workflow {wf_id} 不存在")

        task_id = f"{wf_id}-{len(wf.tasks)+1:03d}"
        task = Task(
            id=task_id,
            title=title,
            description=description,
            role=role,
            stage=stage,
            parent_id=parent_id,
            dependencies=dependencies or [],
            estimated_minutes=estimated_minutes,
            priority=priority,
            tags=tags or [],
        )
        wf.tasks.append(task)
        wf.updated_at = datetime.now().isoformat()
        self._save_state()
        return task

    def update_task_status(
        self,
        wf_id: str,
        task_id: str,
        status: TaskStatus,
        assignee: Optional[str] = None,
        deliverables: Optional[List[str]] = None,
        review_feedback: Optional[str] = None,
        review_status: Optional[Literal["approved", "changes_requested", "comment"]] = None,
    ) -> Task:
        wf = self.workflows.get(wf_id)
        if not wf:
            raise ValueError(f"Workflow {wf_id} 不存在")

        task = next((t for t in wf.tasks if t.id == task_id), None)
        if not task:
            raise ValueError(f"Task {task_id} 不存在")

        # 检查依赖是否完成
        if status == TaskStatus.IN_PROGRESS:
            for dep_id in task.dependencies:
                dep = next((t for t in wf.tasks if t.id == dep_id), None)
                if dep and dep.status not in [TaskStatus.DONE, TaskStatus.APPROVED]:
                    raise ValueError(f"依赖任务 {dep_id} (状态: {dep.status}) 未完成")
            task.started_at = datetime.now().isoformat()

        if status in [TaskStatus.DONE, TaskStatus.APPROVED]:
            task.completed_at = datetime.now().isoformat()
            if task.started_at:
                start = datetime.fromisoformat(task.started_at)
                end = datetime.fromisoformat(task.completed_at)
                task.actual_minutes = int((end - start).total_seconds() / 60)

        task.status = status
        if assignee:
            task.assignee = assignee
        if deliverables:
            task.deliverables.extend(deliverables)
        if review_feedback:
            task.review_feedback = review_feedback
        if review_status:
            task.review_status = review_status

        wf.updated_at = datetime.now().isoformat()
        self._update_workflow_stage(wf)
        self._save_state()
        return task

    def _update_workflow_stage(self, wf: Workflow):
        """根据任务状态自动更新工作流阶段"""
        stages_order = [
            Stage.ANALYSIS,
            Stage.REQUIREMENTS,
            Stage.DESIGN,
            Stage.IMPLEMENTATION,
            Stage.QA_REVIEW,
            Stage.MERGED,
        ]

        current_idx = 0
        for stage in stages_order:
            stage_tasks = [t for t in wf.tasks if t.stage == stage]
            if not stage_tasks:
                continue
            all_done = all(t.status in [TaskStatus.DONE, TaskStatus.APPROVED] for t in stage_tasks)
            if all_done:
                current_idx = stages_order.index(stage) + 1
            else:
                current_idx = stages_order.index(stage)
                break

        if current_idx < len(stages_order):
            wf.current_stage = stages_order[current_idx]
        else:
            wf.current_stage = Stage.MERGED
            wf.status = "completed"

    def get_next_tasks(self, wf_id: str, role: Optional[Role] = None) -> List[Task]:
        """获取下一个可执行的任务"""
        wf = self.workflows.get(wf_id)
        if not wf:
            return []

        pending = [t for t in wf.tasks if t.status == TaskStatus.PENDING]
        ready = []
        for task in pending:
            deps_done = all(
                next((t for t in wf.tasks if t.id == dep_id), None) in [TaskStatus.DONE, TaskStatus.APPROVED]
                for dep_id in task.dependencies
            ) if task.dependencies else True
            if deps_done:
                ready.append(task)

        if role:
            ready = [t for t in ready if t.role == role]

        return sorted(ready, key=lambda t: (t.priority, t.created_at))

    def generate_progress_report(self, wf_id: str) -> str:
        """生成进度报告"""
        wf = self.workflows.get(wf_id)
        if not wf:
            return f"Workflow {wf_id} 不存在"

        total = len(wf.tasks)
        done = len([t for t in wf.tasks if t.status in [TaskStatus.DONE, TaskStatus.APPROVED]])
        in_progress = len([t for t in wf.tasks if t.status == TaskStatus.IN_PROGRESS])
        blocked = len([t for t in wf.tasks if t.status == TaskStatus.BLOCKED])
        review = len([t for t in wf.tasks if t.status == TaskStatus.REVIEW])

        progress_pct = (done / total * 100) if total > 0 else 0

        # 各角色统计
        role_stats = {}
        for role in Role:
            role_tasks = [t for t in wf.tasks if t.role == role]
            role_done = len([t for t in role_tasks if t.status in [TaskStatus.DONE, TaskStatus.APPROVED]])
            role_stats[role.value] = {
                "total": len(role_tasks),
                "done": role_done,
                "progress": f"{role_done / len(role_tasks) * 100:.0f}%" if role_tasks else "N/A",
            }

        # 阶段统计
        stage_stats = {}
        for stage in Stage:
            stage_tasks = [t for t in wf.tasks if t.stage == stage]
            stage_done = len([t for t in stage_tasks if t.status in [TaskStatus.DONE, TaskStatus.APPROVED]])
            stage_stats[stage.value] = {
                "total": len(stage_tasks),
                "done": stage_done,
                "status": "✅ 完成" if stage_tasks and stage_done == len(stage_tasks) else ("🔄 进行中" if stage_tasks else "N/A"),
            }

        report = f"""
# 📊 进度报告: {wf.title}

## 概览
- **工作流ID**: `{wf.id}`
- **当前阶段**: {wf.current_stage.value}
- **总体状态**: {wf.status}
- **总体进度**: {done}/{total} ({progress_pct:.1f}%)

## 任务状态
| 状态 | 数量 |
|------|------|
| 完成 | {done} |
| 进行中 | {in_progress} |
| 待审查 | {review} |
| 阻塞 | {blocked} |
| 待处理 | {total - done - in_progress - blocked - review} |

## 各角色进度
| 角色 | 完成 | 总数 | 进度 |
|------|------|------|------|
"""
        for role_name, stats in role_stats.items():
            report += f"| {role_name} | {stats['done']} | {stats['total']} | {stats['progress']} |\n"

        report += "\n## 各阶段状态\n| 阶段 | 状态 | 完成 | 总数 |\n|------|------|------|------|\n"
        for stage_name, stats in stage_stats.items():
            report += f"| {stage_name} | {stats['status']} | {stats['done']} | {stats['total']} |\n"

        # 进行中的任务
        active = [t for t in wf.tasks if t.status == TaskStatus.IN_PROGRESS]
        if active:
            report += "\n## 🔄 进行中的任务\n"
            for t in active:
                report += f"- **{t.id}** [{t.role.value}] {t.title} (预计{t.estimated_minutes}min)\n"

        # 阻塞的任务
        if blocked:
            report += "\n## ⚠️ 阻塞的任务\n"
            for t in blocked:
                report += f"- **{t.id}** [{t.role.value}] {t.title}\n"
                if t.review_feedback:
                    report += f"  > 反馈: {t.review_feedback}\n"

        # 审查中的任务
        if review:
            report += "\n## 👀 审查中的任务\n"
            for t in review:
                report += f"- **{t.id}** [{t.role.value}] {t.title}\n"
                if t.review_status:
                    report += f"  > 状态: {t.review_status}\n"

        return report

    def generate_daily_report(self, date: Optional[str] = None) -> str:
        """生成每日汇报"""
        if date is None:
            date = datetime.now().strftime("%Y-%m-%d")

        completed_today = []
        started_today = []
        in_progress = []

        for wf in self.workflows.values():
            for t in wf.tasks:
                if t.completed_at and t.completed_at.startswith(date):
                    completed_today.append((wf, t))
                if t.started_at and t.started_at.startswith(date):
                    started_today.append((wf, t))
                if t.status == TaskStatus.IN_PROGRESS:
                    in_progress.append((wf, t))

        report = f"""
# 📋 每日工作汇报 ({date})

## 今日完成 ({len(completed_today)} 项)
"""
        if completed_today:
            for wf, t in completed_today:
                actual = f"，实际耗时 {t.actual_minutes}min" if t.actual_minutes else ""
                report += f"- [{wf.title}] {t.title} ({t.role.value}){actual}\n"
        else:
            report += "暂无\n"

        report += f"\n## 今日开始 ({len(started_today)} 项)\n"
        if started_today:
            for wf, t in started_today:
                report += f"- [{wf.title}] {t.title} ({t.role.value})\n"
        else:
            report += "暂无\n"

        report += f"\n## 进行中 ({len(in_progress)} 项)\n"
        if in_progress:
            for wf, t in in_progress:
                report += f"- [{wf.title}] {t.title} ({t.role.value})\n"
        else:
            report += "暂无\n"

        # 整体统计
        total_tasks = sum(len(wf.tasks) for wf in self.workflows.values())
        total_done = sum(
            len([t for t in wf.tasks if t.status in [TaskStatus.DONE, TaskStatus.APPROVED]])
            for wf in self.workflows.values()
        )
        overall = (total_done / total_tasks * 100) if total_tasks > 0 else 0
        report += f"""
## 总体统计
- 活跃工作流: {len([w for w in self.workflows.values() if w.status == 'active'])}
- 总任务数: {total_tasks}
- 总完成: {total_done}
- 整体进度: {overall:.1f}%
"""
        return report

    def save_report(self, wf_id: str, report_type: str = "progress") -> Path:
        """保存报告到文件"""
        timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")
        if report_type == "progress":
            report = self.generate_progress_report(wf_id)
            filename = f"progress_{wf_id}_{timestamp}.md"
        else:
            report = self.generate_daily_report()
            filename = f"daily_{timestamp}.md"

        filepath = REPORTS_DIR / filename
        with open(filepath, "w", encoding="utf-8") as f:
            f.write(report)
        return filepath

    def list_workflows(self) -> List[Workflow]:
        return list(self.workflows.values())

    def get_blocked_tasks(self, wf_id: str) -> List[Task]:
        wf = self.workflows.get(wf_id)
        if not wf:
            return []
        return [t for t in wf.tasks if t.status == TaskStatus.BLOCKED]


# CLI 接口

def main():
    import argparse
    parser = argparse.ArgumentParser(description="小龙调度器 - 多角色任务管理")
    subparsers = parser.add_subparsers(dest="command", help="命令")

    # create
    create_parser = subparsers.add_parser("create", help="创建新工作流")
    create_parser.add_argument("title", help="工作流标题")
    create_parser.add_argument("--desc", default="", help="工作流描述")

    # add-task
    add_parser = subparsers.add_parser("add-task", help="添加任务")
    add_parser.add_argument("wf_id", help="工作流ID")
    add_parser.add_argument("title", help="任务标题")
    add_parser.add_argument("--desc", default="", help="任务描述")
    add_parser.add_argument("--role", choices=[r.value for r in Role], required=True, help="角色")
    add_parser.add_argument("--stage", choices=[s.value for s in Stage], required=True, help="阶段")
    add_parser.add_argument("--deps", default="", help="依赖任务ID，用逗号分隔")
    add_parser.add_argument("--est", type=int, default=5, help="预估时间(分钟)")
    add_parser.add_argument("--priority", type=int, default=1, help="优先级(1=最高)")

    # status
    status_parser = subparsers.add_parser("status", help="更新任务状态")
    status_parser.add_argument("wf_id", help="工作流ID")
    status_parser.add_argument("task_id", help="任务ID")
    status_parser.add_argument("new_status", choices=[s.value for s in TaskStatus], help="新状态")
    status_parser.add_argument("--assignee", default=None, help="执行人")
    status_parser.add_argument("--feedback", default=None, help="审查反馈")

    # next
    next_parser = subparsers.add_parser("next", help="查看下一个任务")
    next_parser.add_argument("wf_id", help="工作流ID")
    next_parser.add_argument("--role", choices=[r.value for r in Role], default=None, help="按角色过滤")

    # report
    report_parser = subparsers.add_parser("report", help="生成报告")
    report_parser.add_argument("wf_id", help="工作流ID")
    report_parser.add_argument("--type", choices=["progress", "daily"], default="progress", help="报告类型")

    # list
    subparsers.add_parser("list", help="列出所有工作流")

    # daily
    subparsers.add_parser("daily", help="生成每日汇报")

    args = parser.parse_args()
    tm = TaskManager()

    if args.command == "create":
        wf = tm.create_workflow(args.title, args.desc)
        print(f"创建工作流成功: {wf.id}")

    elif args.command == "add-task":
        deps = args.deps.split(",") if args.deps else []
        task = tm.add_task(
            wf_id=args.wf_id,
            title=args.title,
            description=args.desc,
            role=Role(args.role),
            stage=Stage(args.stage),
            dependencies=deps,
            estimated_minutes=args.est,
            priority=args.priority,
        )
        print(f"添加任务成功: {task.id}")

    elif args.command == "status":
        task = tm.update_task_status(
            wf_id=args.wf_id,
            task_id=args.task_id,
            status=TaskStatus(args.new_status),
            assignee=args.assignee,
            review_feedback=args.feedback,
        )
        print(f"更新状态成功: {task.id} -> {task.status.value}")

    elif args.command == "next":
        role = Role(args.role) if args.role else None
        tasks = tm.get_next_tasks(args.wf_id, role)
        if tasks:
            print("下一个任务:")
            for t in tasks[:5]:
                print(f"  {t.id} [{t.role.value}] {t.title}")
        else:
            print("暂无可执行任务")

    elif args.command == "report":
        path = tm.save_report(args.wf_id, args.type)
        print(f"报告已保存: {path}")

    elif args.command == "list":
        wfs = tm.list_workflows()
        print(f"活跃工作流 ({len(wfs)}):")
        for wf in wfs:
            total = len(wf.tasks)
            done = len([t for t in wf.tasks if t.status in [TaskStatus.DONE, TaskStatus.APPROVED]])
            print(f"  {wf.id}: {wf.title} [{wf.status}] 进度 {done}/{total}")

    elif args.command == "daily":
        report = tm.generate_daily_report()
        print(report)

    else:
        parser.print_help()


if __name__ == "__main__":
    main()
