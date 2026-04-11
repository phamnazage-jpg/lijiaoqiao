# 性能回归测试

本目录包含 Supply API 的性能回归测试，确保关键路径的性能不会随着代码变更而退化。

## 测试文件

| 文件 | 说明 |
|------|------|
| `domain_bench_test.go` | 领域模型性能测试（账号、套餐、结算服务） |
| `middleware_bench_test.go` | 中间件性能测试（认证、限流、追踪、幂等） |

## 运行方式

```bash
# 运行所有性能测试（跳过快速测试）
go test -tags=slow ./internal/benchmark/...

# 运行特定基准测试
go test -tags=slow -run=XXX ./internal/benchmark/... -bench=BenchmarkAccountService_Create

# 查看内存分配
go test -tags=slow ./internal/benchmark/... -bench=BenchmarkAccountService_Create -benchmem

# 输出详细信息
go test -tags=slow ./internal/benchmark/... -bench=BenchmarkAccountService_Create -v
```

## 性能目标

| 模块 | 操作 | 目标 | 阈值 |
|------|------|------|------|
| AccountService | Create | < 1ms | 5ms |
| AccountService | Verify | < 5ms | 10ms |
| PackageService | CreateDraft | < 2ms | 10ms |
| PackageService | BatchUpdatePrice(50) | < 20ms | 50ms |
| SettlementService | Withdraw | < 5ms | 20ms |
| Middleware | Auth | < 0.5ms | 1ms |
| Middleware | RateLimit | < 0.2ms | 0.5ms |
| Middleware | Tracing | < 0.1ms | 0.5ms |

## CI/CD 集成

性能测试在 CI/CD 中作为门禁检查：

```yaml
# .github/workflows/performance.yml
- name: Run Performance Tests
  run: |
    go test -tags=slow -bench=. -benchmem ./internal/benchmark/... \
      | tee performance_report.txt
```

## 添加新的性能测试

```go
//go:build slow
// +build slow

package benchmark

func BenchmarkYourOperation(b *testing.B) {
    if testing.Short() {
        b.Skip("Skipping benchmark in short mode")
    }

    // 准备测试数据
    setup()

    b.ResetTimer()
    b.ReportAllocs()

    for i := 0; i < b.N; i++ {
        // 执行被测操作
    }
}
```
