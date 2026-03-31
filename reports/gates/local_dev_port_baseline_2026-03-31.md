# 本机开发端口基线（2026-03-31）

- 目的：在继续 STG 本地演练前，清理蚊子残留与冲突进程，固化端口基线。
- 执行时间：2026-03-31（Asia/Shanghai）

## 1. 清理动作

已停止以下冲突/残留进程（TERM 后必要时 KILL）：

1. `158085`（蚊子后端 Java，监听 8080）
2. `170093`（蚊子前端 H5 Node，监听 5176）
3. `180724`（蚊子前端 Admin Node，监听 5177）
4. `216458`（`supply-api`，监听 18082，干扰 M-021 smoke）
5. `135336` / `135522`（`platform-token-runtime` 历史常驻进程，监听 18081）

## 2. 端口状态（清理后）

| 端口 | 状态 | 说明 |
|---|---|---|
| 5176 | FREE | 蚊子 H5 已清理 |
| 5177 | FREE | 蚊子 Admin 已清理 |
| 8080 | FREE | 蚊子后端已清理 |
| 18080 | FREE | STG mock 运行时可按需启动 |
| 18081 | FREE | token runtime 常驻进程已清理 |
| 18082 | FREE | M-021 smoke 历史冲突端口已释放 |
| 3000 | OCCUPIED | 非 STG 关键端口，当前保留，不阻断本次 STG 演练 |

## 3. 复测结果

1. 清理后复跑 `staging_release_pipeline.sh`（`local/mock`）：
   - 报告：`reports/gates/staging_release_pipeline_2026-03-31_100942.md`
   - 结果：`PASS`
2. 关联总控流水：
   - 报告：`reports/gates/superpowers_release_pipeline_2026-03-31_100943.md`
   - 结果：`PASS`

## 4. 固化检查命令

```bash
cd "/home/long/project/立交桥"
ss -ltnp | grep -E ":3000|:5176|:5177|:8080|:18080|:18081|:18082" || true
```

判定规则：
1. `5176/5177/8080/18080/18081/18082` 应为空闲或由本次演练临时进程占用。
2. 若 `18082` 被占用，M-021 仍可通过自动端口避让执行，但建议先查明占用来源。

