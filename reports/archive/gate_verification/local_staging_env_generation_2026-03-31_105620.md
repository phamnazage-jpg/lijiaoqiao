# Local Staging Env Generation

- 时间戳：2026-03-31_105620
- 输出文件：`/home/long/project/立交桥/scripts/supply-gate/.env.staging-real`
- API_BASE_URL：`http://127.0.0.1:18080`
- token nominal expiry(UTC)：`2026-03-31T04:56:20Z`
- token runtime：`http://127.0.0.1:18091`
- runtime auto-start：`1`

## Token 摘要（不含明文）

| role | length | sha256_12 |
|---|---:|---|
| owner | 36 | 5e005018d060 |
| viewer | 36 | b627263085b6 |
| admin | 36 | 7f58157fb7ff |

## 下一步

1. 使用该 env 执行：`ALLOW_LOCAL_MOCK_STAGING=1 bash scripts/ci/staging_release_pipeline.sh /home/long/project/立交桥/scripts/supply-gate/.env.staging-real`
2. 若切换真实 staging，更新 `API_BASE_URL` 后复跑。
