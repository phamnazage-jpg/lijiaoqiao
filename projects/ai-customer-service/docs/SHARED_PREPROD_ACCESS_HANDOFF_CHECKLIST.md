# 共享预生产入口交接清单

> 状态：待共享预生产环境提供方回填  
> 最近更新：2026-05-06  
> 适用项目：`projects/ai-customer-service`  
> 目标：确保“真实共享预生产 Gate B 复跑”和“真实共享预生产/灰度环境 Gate C 回滚演练”具备可执行入口，而不是停留在口头说明

---

## 1. 这份清单解决什么问题

当前项目已经具备：

1. 代码级门禁通过
2. 本地/容器化 Gate B 通过
3. 本地/容器化 Gate C 回滚演练通过

当前仍然缺失的是：

1. **真实共享预生产环境 Gate B 复跑入口**
2. **真实共享预生产/灰度环境 Gate C 回滚演练入口**

这里的“入口”不是一个 URL，也不是一句“环境已经有了”，而是：

> **从当前执行机器出发，能真实操作共享预生产环境的运维通道。**

必须能够支持：

1. 启动/重启服务
2. 查看日志
3. 访问 health probe
4. 访问真实 PostgreSQL
5. 获取真实环境变量来源
6. 在该环境执行 Gate B 验证
7. 在该环境执行 Gate C 回滚演练
8. 留下可复核证据

---

## 2. 合格入口类型

满足以下任一类型即可：

### 2.1 SSH 主机入口

提供：

- 主机地址
- 用户名
- 登录方式
- 项目目录
- 启动/重启命令
- 日志路径
- 服务访问地址

适用场景：

- systemd 服务
- 直接运行二进制
- Docker / Podman 单机部署

### 2.2 Kubernetes 入口

提供：

- `kubectl` 可用
- `kubeconfig` 或 context
- namespace
- deployment / service 名称
- 查看日志权限
- rollout / undo 权限

适用场景：

- Kubernetes Deployment
- StatefulSet
- 多副本灰度切换

### 2.3 CI/CD 或发布平台入口

提供：

- 预生产部署流水线入口
- 环境变量/Secret 查看或确认方式
- 服务日志查看入口
- 重启/回滚入口
- 部署版本与提交号映射

适用场景：

- GitOps
- 平台托管部署
- 云上控制台发布

---

## 3. 不算合格入口的情况

以下情况都不够：

1. 只有共享预生产 URL
2. 只有数据库只读账号
3. 只有监控只读面板
4. 只有截图、文档或口头说明
5. 只能“看状态”，不能“重启/回滚/留痕”

原因很直接：

> Gate B / Gate C 都要求可操作性，不只是可观察性。

---

## 4. 入口必须满足的规范要求

### 4.1 部署对象明确

必须明确服务部署对象：

- systemd service 名称
- Docker / Podman 容器名称
- Kubernetes deployment / rollout 对象

不能只说“服务在那台机器上”，必须能回答：

1. 由谁启动
2. 怎么重启
3. 怎么回滚
4. 日志在哪

### 4.2 环境变量来源明确

必须明确共享预生产如何注入这些变量：

- `AI_CS_RUNTIME_ENV`
- `AI_CS_ADDR`
- `AI_CS_POSTGRES_ENABLED`
- `AI_CS_POSTGRES_DSN`
- `AI_CS_POSTGRES_MIGRATION_DIR`
- `AI_CS_WEBHOOK_SECRET`
- `AI_CS_WEBHOOK_TIMESTAMP_HEADER`
- `AI_CS_WEBHOOK_SIGNATURE_HEADER`
- `AI_CS_WEBHOOK_MAX_SKEW_SECONDS`

基线文档：

- [CONFIG_CONTRACT_BASELINE.md](/home/long/project/立交桥/projects/ai-customer-service/docs/CONFIG_CONTRACT_BASELINE.md)

必须至少能回答：

1. 变量值从哪里来
2. 谁负责维护
3. 如何在不泄露明文 secret 的前提下确认其已正确注入

### 4.3 数据库必须是共享预生产真实库

不能使用：

- 本地测试库
- 临时容器库
- 开发库

必须使用共享预生产 PostgreSQL，才能证明：

1. migration 基线真实可用
2. ticket 入库真实可用
3. audit 入库真实可用
4. dedup 入库真实可用

### 4.4 必须具备最小操作权限

入口必须允许执行以下动作：

1. 启动或重启当前版本
2. 查看最近日志
3. 访问 `/actuator/health/live`
4. 访问 `/actuator/health/ready`
5. 读取当前部署版本/镜像/tag/commit
6. 执行回滚动作
7. 验证回滚后主链恢复

### 4.5 必须可留痕

至少保留以下证据：

1. `summary.txt`
2. 服务日志路径
3. 部署版本 / 提交号
4. 健康检查结果
5. Gate B / Gate C 执行命令
6. 回滚前后版本信息
7. 必要时数据库验证摘要

---

## 5. Gate B 所需最小入口要求

如果当前只想完成“真实共享预生产 Gate B 复跑”，入口最少要具备：

1. 共享预生产服务启动权限
2. 共享预生产 PostgreSQL 可连
3. 真实 `AI_CS_*` 环境变量可确认
4. 服务地址可访问
5. 日志可读

执行入口：

- [scripts/verify_preprod_gate_b.sh](/home/long/project/立交桥/projects/ai-customer-service/scripts/verify_preprod_gate_b.sh)

对应证据模板：

- [PREPROD_VERIFICATION_RECORD.md](/home/long/project/立交桥/projects/ai-customer-service/docs/PREPROD_VERIFICATION_RECORD.md)

---

## 6. Gate C 所需额外入口要求

如果要完成“真实共享预生产/灰度环境 Gate C 回滚演练”，除 Gate B 外还必须额外明确：

1. **坏发布怎么制造**
   - 错误配置
   - 错误 DSN
   - 错误 Secret
   - 错误镜像/tag
2. **回滚对象是谁**
   - systemd service
   - container
   - deployment
3. **标准回滚动作是什么**
   - `systemctl restart ...`
   - `docker/podman restart ...`
   - `kubectl rollout undo ...`
4. **恢复完成如何判定**
   - `live` / `ready` 恢复
   - signed webhook 重新返回 `200`
   - ticket / audit / dedup 重新恢复写入

执行入口：

- [scripts/verify_gate_c_rollback.sh](/home/long/project/立交桥/projects/ai-customer-service/scripts/verify_gate_c_rollback.sh)

对应证据模板：

- [ROLLBACK_DRILL_RECORD.md](/home/long/project/立交桥/projects/ai-customer-service/docs/ROLLBACK_DRILL_RECORD.md)

---

## 7. 共享预生产入口交接模板

请环境提供方至少按下面模板回填：

```text
共享预生产入口类型：
- SSH / Kubernetes / CI-CD

如果是 SSH：
- 主机地址：
- 用户名：
- 登录方式：
- 项目目录：
- 服务启动命令：
- 服务重启命令：
- 服务停止命令：
- 日志路径：
- 服务访问地址：
- 环境变量来源文件或注入方式：

如果是 Kubernetes：
- kubeconfig/context：
- namespace：
- deployment 名称：
- service 名称：
- ingress / 访问地址：
- 查看日志命令：
- 重启命令：
- 回滚命令：
- Secret / ConfigMap 名称：

如果是 CI/CD：
- 平台名称：
- 流水线入口：
- 发布目标环境名称：
- 当前部署版本查看方式：
- 日志查看入口：
- 回滚入口：

数据库：
- 是否为共享预生产 PostgreSQL：
- DSN 获取方式：
- migration 目录所在位置：

Gate B 执行责任人：
- 负责人：
- 计划时间：

Gate C 回滚演练责任人：
- 负责人：
- 计划时间：

证据归档位置：
- summary.txt：
- service.log：
- 版本信息：
- 回滚记录：
```

---

## 8. 当前项目的真实阻断

截至 2026-05-06，当前执行机器上已确认：

1. **没有 `kubectl`**
2. **没有 `~/.kube/config`**
3. **没有共享预生产专用 `AI_CS_*` 环境**
4. **仓库内没有共享预生产部署清单**

因此当前阻断不是：

- Gate B/Gate C 脚本缺失
- 本地演练能力缺失
- 门禁文档缺失

而是：

> **真实共享预生产环境运维入口未交接。**

---

## 9. 当前结论

当前可以准确表达为：

1. **代码级门禁：通过**
2. **本地/容器化 Gate B：通过**
3. **本地/容器化 Gate C 回滚演练：通过**
4. **真实共享预生产 Gate B：待共享预生产入口交接后执行**
5. **真实共享预生产/灰度环境 Gate C：待共享预生产入口交接后执行**

> 没有入口，不应宣称“真实共享预生产已验证”；有入口后，才可以继续执行真实 Gate B / Gate C。
