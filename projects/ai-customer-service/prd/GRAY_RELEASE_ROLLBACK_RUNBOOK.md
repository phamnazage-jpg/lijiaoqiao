# 灰度发布与回滚 Runbook

> 版本：v1.1  
> 状态：灰度门禁已定义，本地/容器化回滚演练已通过，待真实共享预生产/灰度环境演练  
> 关联：`docs/MONITORING_ALERTING.md`、`docs/GRAY_DASHBOARD_MINIMUM.md`、`docs/PREPROD_VERIFICATION_RECORD.md`、`docs/ROLLBACK_DRILL_RECORD.md`

---

## 1. 前提

开始任何灰度放量前，必须满足：

1. Gate B 已通过  
   当前状态：**本地/容器化预演已通过，真实共享预生产环境待复跑**
2. 最小鉴权已落地
3. 工单闭环语义已收口
4. 最小监控指标和阈值已定义

---

## 2. 灰度放量节奏

默认节奏如下：

| 档位 | 流量占比 | 最短观察时间 | 进入条件 | 回退条件 |
|------|----------|--------------|----------|----------|
| Stage 1 | 5% | 30 分钟 | Gate B 通过，部署稳定，核心指标全绿 | 任一 P0/P1 指标触发 |
| Stage 2 | 20% | 2 小时 | Stage 1 稳定，`5xx <= 0.5%`，`audit fail = 0` | 5xx > 1%、audit fail > 0、DB 异常 |
| Stage 3 | 50% | 半天 | Stage 2 稳定，handoff 比率无异常升高 | 指标明显劣化或人工链路承压 |
| Stage 4 | 100% | 次日 | Stage 3 稳定跨工作日，无新增 P0/P1 | 任一核心门禁不满足 |

说明：

- **最短观察时间不是建议，是门禁**
- 任意阶段都不允许跳级放量
- 任意阶段出现 P0/P1 指标时，不继续放量

---

## 3. 放量前检查单

- [ ] 共享预生产环境已复跑 Gate B 脚本
- [ ] 最近一次部署产物与验证记录关联清晰
- [ ] `live` / `ready` 探针正常
- [ ] PostgreSQL migration 版本与目标一致
- [ ] webhook signed request 联调已通过
- [ ] ticket / audit / dedup 验证通过
- [ ] 灰度 dashboard 可查看 8 个最小指标

---

## 4. 继续放量的判定条件

每个档位进入下一档前，必须满足：

1. `webhook 5xx <= 0.5%`
2. `webhook reject` 没有异常升高
3. `audit 写入失败数 = 0`
4. `postgres 连接异常 = 0`
5. `readiness down 次数 = 0` 或未影响流量池
6. `单实例重启次数 <= 2 / 10 分钟`
7. `handoff 比率 <= 25%` 或未高于基线 `2x`
8. ticket 创建量与人工处理能力匹配，没有积压失控

---

## 5. 立即回滚条件

满足以下任意条件，立即回滚当前灰度版本：

| 条件 | 原因 |
|------|------|
| Webhook 5xx `> 5%` 持续 5 分钟 | 服务主链不可接受 |
| PostgreSQL 异常导致 `ready` 持续失败 | 核心依赖异常 |
| Audit 写入失败数 `> 0` 且持续 5 分钟 | 合规/追溯链路断裂 |
| Ticket 创建链路断裂 | 人工服务主链损坏 |
| 全量 readiness down 或实例反复重启 | 当前版本不稳定 |

---

## 6. 建议回滚条件

出现以下情况时，停止继续放量并由 TechLead 决策是否回滚：

| 条件 | 处理 |
|------|------|
| Webhook 5xx `> 1%` 持续 5 分钟 | 冻结当前档位，评估回滚 |
| Handoff 比率高于基线 `2x` | 判断意图识别/降级是否异常 |
| Reject 数持续高于 20% | 检查上游签名或渠道配置 |
| 单实例重启次数过高 | 排查资源、崩溃或配置问题 |

---

## 7. 回滚动作

### 7.1 立即动作

1. 停止继续放量
2. 将灰度比例回退到上一个稳定档位
3. 若当前档位无稳定状态，直接回退到旧版本

### 7.2 回滚后必须检查

- [ ] `live` 正常
- [ ] `ready` 正常
- [ ] signed webhook 再次联调通过
- [ ] ticket 创建恢复
- [ ] audit 写入恢复
- [ ] PostgreSQL 无新错误

---

## 8. 演练要求

Gate C 前至少完成一次回滚演练，且留下证据：

1. 演练时间
2. 演练版本
3. 触发条件
4. 回滚动作
5. 回滚后验证结果

没有演练记录，不得宣称“可安全灰度放量”。

推荐入口：

- [scripts/verify_gate_c_rollback.sh](/home/long/project/立交桥/projects/ai-customer-service/scripts/verify_gate_c_rollback.sh)
- 最近一次本地/容器化记录：[ROLLBACK_DRILL_RECORD.md](/home/long/project/立交桥/projects/ai-customer-service/docs/ROLLBACK_DRILL_RECORD.md)

---

## 9. 当前状态结论

当前正确口径：

- **灰度门禁：已定义**
- **本地/容器化 Gate B：已通过**
- **本地/容器化 Gate C 回滚演练：已通过**
- **真实共享预生产环境 Gate B：待复跑**
- **Gate C 灰度监控与回滚演练：待完成**

因此：

> **现在可以说“灰度门禁框架已补齐”，但还不能说“灰度已经可执行上线”。**
