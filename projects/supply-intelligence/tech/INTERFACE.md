# Supply-Intelligence 核心接口设计

> 版本：v1.0 | 状态：初稿

---

## 1. 内部模块间接口

### 1.1 ProbeService

```go
type ProbeService interface {
    // 执行单次探针
    Probe(ctx context.Context, accountID string) (*ProbeResult, error)
    // 批量探针（按供应商或全量）
    ProbeBatch(ctx context.Context, filter ProbeFilter) (*BatchProbeResult, error)
    // 获取探针结果历史
    GetProbeHistory(ctx context.Context, accountID string, limit int) ([]ProbeResult, error)
    // 手动触发掠针（运营干预）
    TriggerManualProbe(ctx context.Context, accountID string, actorID string) (*ProbeResult, error)
}

type ProbeResult struct {
    AccountID     string
    Status        string // active suspended disabled
    RiskScore     int    // 0-100
    RiskReason    string
    LatencyMs     int
    ResponseCode  int
    CheckedAt     time.Time
    NextCheckAt   time.Time
}

type ProbeFilter struct {
    Platform      *string
    Status        *string
    RiskScoreMin  *int
    RiskScoreMax  *int
}
```

### 1.2 DiscoveryService

```go
type DiscoveryService interface {
    // 执行单次全网扫描
    Scan(ctx context.Context) (*ScanResult, error)
    // 获取最近扫描结果
    GetLastScan(ctx context.Context) (*ScanResult, error)
    // 获取候选模型列表
    ListCandidates(ctx context.Context, filter CandidateFilter) ([]ModelCandidate, error)
    // 手动触发扫描
    TriggerManualScan(ctx context.Context, actorID string) (*ScanResult, error)
    // 忽略候选模型
    IgnoreCandidate(ctx context.Context, candidateID string, reason string, actorID string) error
}

type ScanResult struct {
    ScannedAt     time.Time
    Platforms     []string
    NewModels     int
    RemovedModels int
    Errors        []ScanError
}

type ModelCandidate struct {
    ID            string
    Platform      string
    ModelID       string
    Status        string // discovered queued testing test_passed test_failed ignored
    DiscoveredAt  time.Time
    TestedAt      *time.Time
    TestResult    *TestResult
}
```

### 1.3 AdmissionService

```go
type AdmissionService interface {
    // 执行准入测试
    RunTest(ctx context.Context, candidateID string) (*TestResult, error)
    // 获取测试结果
    GetTestResult(ctx context.Context, candidateID string) (*TestResult, error)
    // 手动确认上架（运营干预）
    Publish(ctx context.Context, candidateID string, actorID string) error
    // 强制上架（测试失败但运营确认）
    ForcePublish(ctx context.Context, candidateID string, reason string, actorID string) error
}

type TestResult struct {
    CandidateID   string
    Status        string // passed failed
    Dimensions    []TestDimension
    FailedReason  *string
    ExecutedAt    time.Time
    DurationMs    int
}

type TestDimension struct {
    Name      string
    Passed    bool
    Detail    string
}
```

### 1.4 AccountService

```go
type AccountService interface {
    // 创建账号（手动或自动）
    CreateAccount(ctx context.Context, req CreateAccountRequest) (*SupplyAccount, error)
    // 获取账号信息
    GetAccount(ctx context.Context, accountID string) (*SupplyAccount, error)
    // 更新账号状态
    UpdateStatus(ctx context.Context, accountID string, status string, reason string) error
    // 轮换密钥
    RotateKey(ctx context.Context, accountID string, actorID string) error
    // 列表账号
    ListAccounts(ctx context.Context, filter AccountFilter) ([]SupplyAccount, error)
}

type SupplyAccount struct {
    ID          string
    Platform    string
    ProxyID     string
    Status      string
    RiskScore   int
    APIKeyHint  string // 密钥前 4 后 4
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### 1.5 HealthBoardService

```go
type HealthBoardService interface {
    // 获取供应商健康大盘
    GetBoard(ctx context.Context, scope BoardScope) (*HealthBoard, error)
    // 获取模型比价报表
    GetPricingComparison(ctx context.Context, modelID string) ([]PricingComparison, error)
    // 获取供应链覆盖率
    GetCoverage(ctx context.Context) (*CoverageReport, error)
    // 获取预测分析
    GetPredictions(ctx context.Context, minConfidence float64) ([]Prediction, error)
}

type HealthBoard struct {
    Accounts      []AccountHealth
    Candidates    []CandidateSummary
    Coverage      float64
    FreshnessIndex float64
}
```

---

## 2. 外部系统集成接口

### 2.1 与 Bridge Gateway 集成

| 方法 | 路径 | 请求 | 响应 | 说明 |
|------|------|------|------|------|
| 查询账号状态 | `GET /internal/supply-intelligence/accounts/{id}/health` | - | `ProbeResult` | Gateway 路由决策时查询 |
| 查询模型定价 | `GET /internal/supply-intelligence/pricing/{model_id}` | - | `PricingInfo` | 动态定价参考 |
| 获取推荐供应商 | `GET /internal/supply-intelligence/recommendations` | `?model={model_id}&strategy=cost` | `[]Recommendation` | 智能路由推荐 |

### 2.2 与 supply-api 集成

| 方法 | 路径 | 请求 | 响应 | 说明 |
|------|------|------|------|------|
| 读取账号列表 | `GET /internal/supply/accounts` | - | `[]SupplyAccount` | 探针器获取待检测账号 |
| 更新账号状态 | `POST /internal/supply/accounts/{id}/status` | `{"status":"suspended","reason":""}` | `{"success":true}` | 探针结果写回 |
| 读取模型列表 | `GET /internal/supply/packages` | - | `[]SupplyPackage` | 扫描比对基准 |
| 创建模型 | `POST /internal/supply/packages` | `SupplyPackage` | `{"id":""}` | 准入测试通过后上架 |
| 获取审计日志格式 | `GET /internal/supply/audit/schema` | - | `{"schema":{}}` | 审计事件格式一致 |

---

## 3. API 接口规范

### 3.1 REST API 基础

- **基础路径**: `/api/v1/supply-intelligence/`
- **内部路径** (集成模式): `/internal/supply-intelligence/`
- **内容类型**: `application/json`
- **错误响应格式**:

```json
{
  "error": {
    "code": "SI_PRB_4001",
    "message": "供应商账号不存在",
    "details": {}
  }
}
```

### 3.2 核心端点

#### 探针管理

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/v1/supply-intelligence/probes` | 列表探针结果 |
| POST | `/api/v1/supply-intelligence/probes/{account_id}` | 手动触发探针 |
| GET | `/api/v1/supply-intelligence/probes/{account_id}/history` | 探针历史 |

#### 扫描与发现

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/v1/supply-intelligence/discovery/scan` | 手动触发全网扫描 |
| GET | `/api/v1/supply-intelligence/discovery/candidates` | 列表候选模型 |
| GET | `/api/v1/supply-intelligence/discovery/candidates/{id}` | 获取候选模型详情 |
| POST | `/api/v1/supply-intelligence/discovery/candidates/{id}/ignore` | 忽略候选模型 |

#### 准入测试

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/v1/supply-intelligence/admission/{candidate_id}/test` | 手动执行准入测试 |
| GET | `/api/v1/supply-intelligence/admission/{candidate_id}/result` | 获取测试结果 |
| POST | `/api/v1/supply-intelligence/admission/{candidate_id}/publish` | 确认上架 |
| POST | `/api/v1/supply-intelligence/admission/{candidate_id}/force-publish` | 强制上架 |

#### 账号管理

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/v1/supply-intelligence/accounts` | 列表账号 |
| POST | `/api/v1/supply-intelligence/accounts` | 创建账号 |
| GET | `/api/v1/supply-intelligence/accounts/{id}` | 获取账号 |
| POST | `/api/v1/supply-intelligence/accounts/{id}/rotate-key` | 轮换密钥 |
| POST | `/api/v1/supply-intelligence/accounts/{id}/status` | 更新状态 |

#### 健康大盘

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/v1/supply-intelligence/health-board` | 获取健康大盘 |
| GET | `/api/v1/supply-intelligence/pricing/{model_id}/comparison` | 模型比价 |
| GET | `/api/v1/supply-intelligence/coverage` | 供应链覆盖率 |
| GET | `/api/v1/supply-intelligence/predictions` | 预测分析 |

### 3.3 错误码定义

| 错误码 | HTTP 状态 | 说明 |
|---------|-----------|------|
| `SI_PRB_4001` | 404 | 供应商账号不存在 |
| `SI_PRB_4002` | 429 | 探针频率过高，请等待 |
| `SI_DIS_4001` | 404 | 候选模型不存在 |
| `SI_DIS_4002` | 409 | 候选模型状态不允许忽略 |
| `SI_ADM_4001` | 404 | 准入测试任务不存在 |
| `SI_ADM_4002` | 409 | 准入测试正在执行中 |
| `SI_ADM_4003` | 400 | 测试未通过，无法上架 |
| `SI_ACC_4001` | 404 | 账号不存在 |
| `SI_ACC_4002` | 409 | 账号状态不允许此操作 |
| `SI_ACC_4003` | 403 | 无权执行此操作 |
| `SI_BRD_4001` | 400 | 查询参数无效 |

### 3.4 WebSocket 接口

**路径**: `/ws/v1/supply-intelligence/board`

- 运营工作台订阅后，实时推送探针结果、候选模型变更、状态变更待办。
- 心跳间隔 30 秒。
