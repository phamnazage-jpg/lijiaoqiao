# TOK-006 统一 Gate 单页发布判定模板

- 日期：{{DATE}}
- 执行批次：{{BATCH_ID}}
- 环境：{{ENV_NAME}}
- 执行人：{{OPERATOR}}

## 1. Gate 矩阵

| Gate | 状态（PASS/FAIL/BLOCKED） | 环境（mock/staging/prod-like） | 证据路径 |
|---|---|---|---|
| TOK-005 dry-run | {{TOK005_STATUS}} | {{TOK005_ENV}} | {{TOK005_EVIDENCE}} |
| SUP-004 账号挂载 | {{SUP004_STATUS}} | {{SUP004_ENV}} | {{SUP004_EVIDENCE}} |
| SUP-005 套餐发布 | {{SUP005_STATUS}} | {{SUP005_ENV}} | {{SUP005_EVIDENCE}} |
| SUP-006 结算提现 | {{SUP006_STATUS}} | {{SUP006_ENV}} | {{SUP006_EVIDENCE}} |
| SUP-007 边界专项 | {{SUP007_STATUS}} | {{SUP007_ENV}} | {{SUP007_EVIDENCE}} |

## 2. 关键约束

| 项目 | 值 | 结论 |
|---|---|---|
| TOK-005 staging readiness | {{TOK005_STAGING_READY}} | {{TOK005_STAGING_NOTE}} |
| M-013（敏感值泄露事件） | {{M013_VALUE}} | {{M013_RESULT}} |
| M-014（平台凭证入站覆盖） | {{M014_VALUE}} | {{M014_RESULT}} |
| M-015（绕平台直连事件） | {{M015_VALUE}} | {{M015_RESULT}} |
| M-016（query key 外拒率） | {{M016_VALUE}} | {{M016_RESULT}} |

## 3. 发布判定

- [ ] GO
- [ ] CONDITIONAL_GO
- [ ] NO_GO

判定依据：{{DECISION_REASON}}

## 4. 阻塞与动作

| 级别 | 问题 | 动作 | 负责人 | 截止日期 |
|---|---|---|---|---|
| {{P_LEVEL}} | {{ISSUE}} | {{ACTION}} | {{OWNER}} | {{DUE_DATE}} |

## 5. 签署

1. 架构负责人：{{ARCH_SIGN}}
2. 安全负责人：{{SEC_SIGN}}
3. QA 负责人：{{QA_SIGN}}
4. 平台负责人：{{PLAT_SIGN}}
