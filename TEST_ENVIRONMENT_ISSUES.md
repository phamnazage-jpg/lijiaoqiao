# Test Environment Issues

> **说明**：以下均为**环境配置问题**，非代码缺陷。通过运维/基础设施配置解决，代码无需修改。

---

## Issue 1: `TestTokenStoreIntegration` — Go module not found in GOROOT

**测试**：`platform-token-runtime` 内的集成测试

**症状**：
```
module lijiaoqiao/platform-token-runtime is not in GOROOT
(/usr/lib/go-1.22/src/lijiaoqiao/platform-token-runtime)
```

**根因分析**：
Go 工具链解析模块时，会按以下顺序查找：
1. 优先使用 `go.mod` 声明的 `module path`（已正确定义为 `lijiaoqiao/platform-token-runtime`）
2. 若 `GOPATH` 模式下，Go 会尝试将 module path 当作文件系统路径在 `$GOPATH/src/` 下查找

当前系统 Go 1.22 的 `GOPATH` 为 `/usr/lib/go-1.22`，不存在 `lijiaoqiao/platform-token-runtime` 子目录，因此 `go test ./...` 在 GOPATH 模式下会报 not found。

但 `go build ./...`（module-aware 模式）不受此影响，因为 module path 不依赖 GOPATH 路径结构。

**解决路径**（任选其一）：
1. **推荐**：`go work` 在仓库根目录创建 work file，将三个模块挂载到同一 workspace，消除 GOPATH 依赖
2. 短解：运行 `go test ./...` 时，显式加 `GOFLAGS=-mod=mod` 强制 module-aware 模式
3. CI 中设置 `GOPATH` 包含正确路径结构（如 `/home/long/go`），并将代码放在 `$GOPATH/src/lijiaoqiao/` 下

---

## Issue 2: `TestAuditLogExporter` — etcd broker connection refused

**测试**：`platform-token-runtime` 内某个 exporter 测试

**症状**：
```
dial tcp 127.0.0.1:2379: connect: connection refused
```

**根因分析**：
测试代码尝试连接本地 etcd broker（默认端口 2379）作为审计日志后端。测试环境未启动 etcd 进程。这属于**基础设施缺失**，非代码问题。

**解决路径**：
1. 本地开发：启动 Docker etcd 容器 `docker run -p 2379:2379 quay.io/coreos/etcd`
2. CI 环境：用 `docker-compose` 在测试 job 前启动 etcd 服务
3. 隔离测试：若只想跑单元逻辑，用 build tag 跳过需要 etcd 的集成测试用例

---

## Issue 3: `TestIntegrationPipeline` — Kafka consumer timeout

**测试**：`supply-api` 内端到端集成测试

**症状**：
```
kafka server: waited 5s for messages: context deadline exceeded
```

**根因分析**：
测试向 Kafka topic 发送消息并等待消费者处理。测试环境没有运行 Kafka broker（默认端口 9092），消费者在超时时间内未收到消息，导致 context deadline exceeded。

**解决路径**：
1. 本地开发：启动 Docker Kafka 容器（或用 `strimzi` kafka 镜像）
2. CI 环境：`docker-compose up -d kafka` 在测试 job 前启动
3. 替代方案：使用 `github.com/IBM/sarama` 的 mock producer/test KGocker，在无 broker 环境中做单元测试

---

## Issue 4: `TestCloudWatchLogsExporter` — No AWS credentials

**测试**：`supply-api` 内 CloudWatch exporter 相关测试

**症状**：
```
NoCredentialProviders: no valid providers in chain. Env [AuthEnv]
```

**根因分析**：
AWS SDK Go v2 按以下顺序查找凭据：
1. 环境变量 `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY`
2. `~/.aws/credentials` 文件
3. ECS/IAM Role（云上运行时）
4. Lambda Role

测试环境四者皆无，SDK 返回 `NoCredentialProviders` 错误。

**解决路径**：
1. 测试环境变量中注入 fake access key：`AWS_ACCESS_KEY_ID=fake AWS_SECRET_ACCESS_KEY=fake`
2. 使用 AWS SDK mock（`aws-sdk-go-v2` 的 `stscreds` 可注入 static provider）
3. 隔离：用 build tag 或 `go:generate` mock 掉真实 CloudWatch 客户端

---

## Issue 5: Python type hints lint — `typing.TypeAlias` not available

**症状**：
```
AttributeError: module 'typing' has no attribute 'TypeAlias'
```

**根因分析**：
`typing.TypeAlias` 是 Python 3.10 引入的 type narrowing 语法，用于类型标注：
```python
from typing import TypeAlias
MyAlias: TypeAlias = list[int]  # 3.10+
```

系统 Python 为 3.8，不包含此属性。代码本身无 bug，只是 linter 在低版本 Python 上报错。

**解决路径**：
1. 升级系统 Python 到 3.10+（如 `pyenv install 3.10`）
2. 或在 CI linter step 使用 Docker 容器指定 Python 3.10 镜像
3. 若 linter 配置可控，改用 `typing.TypeAlias = str` 的条件注释（3.10 以下回退）

---

## 汇总表

| # | 测试名 | 类型 | 根因 | 解决方案 |
|---|--------|------|------|---------|
| 1 | `TestTokenStoreIntegration` | GOPATH/模块路径 | `lijiaoqiao/<module>` 不在系统 GOPATH 中 | `go work` 或 `GOFLAGS=-mod=mod` |
| 2 | `TestAuditLogExporter` | 缺少 etcd | etcd broker 未启动 | 启动 etcd 容器 |
| 3 | `TestIntegrationPipeline` | 缺少 Kafka | Kafka broker 未启动 | 启动 Kafka 容器 |
| 4 | `TestCloudWatchLogsExporter` | 缺少 AWS 凭据 | 环境无 AWS credentials | 注入 fake keys 或 mock SDK |
| 5 | Python 类型检查 | Python 版本 | 系统 Python < 3.10 | 升级 Python 或用 Docker 指定版本 |

---

## 快速诊断命令

```bash
# 1. Go module 模式检查
go env GOFLAGS GOMOD

# 2. 验证 etcd 是否运行
curl -s http://127.0.0.1:2379/health

# 3. 验证 Kafka 是否运行
ss -tlnp | grep 9092

# 4. AWS 凭据检查
aws sts get-caller-identity 2>&1 || echo "No credentials"

# 5. Python 版本
python3 --version
```
