# API设计解决方案（P0问题修复）

> 版本：v1.0
> 日期：2026-03-18
> 目的：系统性解决评审发现的API设计P0问题

---

## 1. API版本管理策略

### 1.1 当前问题

- 无版本管理策略
- breaking change 无法处理
- 旧版本无法废弃

### 1.2 解决方案

#### 1.2.1 版本策略：URL Path

```python
# API 版本配置
API_VERSION_CONFIG = {
    'v1': {
        'status': 'deprecated',
        'sunset_date': '2027-06-01',  # 废弃日期
        'migration_guide': '/docs/v1-migration',
        'features': ['basic_chat', 'embeddings']
    },
    'v2': {
        'status': 'active',
        'features': ['basic_chat', 'embeddings', 'streaming', 'tools']
    },
    'v3': {
        'status': 'beta',
        'features': ['basic_chat', 'embeddings', 'streaming', 'tools', 'batch']
    }
}

# 版本检查中间件
class APIVersionMiddleware:
    def process_request(self, request, handler):
        # 1. 提取版本
        path_parts = request.path.split('/')
        version = path_parts[1] if len(path_parts) > 1 else 'v1'

        # 2. 验证版本存在
        if version not in API_VERSION_CONFIG:
            return ErrorResponse(
                status=404,
                error={
                    'code': 'API_VERSION_NOT_FOUND',
                    'message': f'API version {version} not found',
                    'available_versions': list(API_VERSION_CONFIG.keys())
                }
            )

        # 3. 检查废弃状态
        config = API_VERSION_CONFIG[version]
        if config['status'] == 'deprecated':
            # 添加废弃警告头
            request.headers['Deprecation'] = f'="{config["sunset_date"]}"'
            request.headers['Link'] = f'<{config["migration_guide"]}>; rel="migration"'

        # 4. 存储版本信息
        request.api_version = version

        return handler(request)
```

#### 1.2.2 废弃流程

```python
class APIDeprecationManager:
    def __init__(self):
        self.timeline = {
            'v1': {
                'announced': '2026-03-01',
                'deprecated': '2026-06-01',
                'sunset': '2027-06-01',
                'migration_guide': '/docs/v1-migration'
            }
        }

    def handle_request(self, request):
        """处理废弃版本请求"""
        version = request.api_version
        config = API_VERSION_CONFIG[version]

        if config['status'] == 'deprecated':
            # 1. 添加警告响应头
            response.headers['Deprecation'] = 'true'
            response.headers['Sunset'] = config['sunset_date']

            # 2. 记录废弃版本使用
            metrics.increment('api.deprecated_version.used', tags={
                'version': version
            })

        return response

    def get_migration_guide(self, from_version, to_version):
        """获取迁移指南"""
        return {
            'from': from_version,
            'to': to_version,
            'breaking_changes': [
                {
                    'endpoint': '/v1/chat/completions',
                    'change': 'Response format changed',
                    'migration': 'Use response_format v2 compatibility mode'
                }
            ],
            'tools': [
                {
                    'name': 'Migration SDK',
                    'description': 'Auto-convert requests to new format',
                    'install': 'pip install lgw-migration'
                }
            ]
        }
```

---

## 2. 完整错误码体系

### 2.1 当前问题

- 只有HTTP状态码
- 无业务错误码
- 错误信息不完整

### 2.2 解决方案

#### 2.2.1 错误码定义

```python
from enum import Enum

class ErrorCode(Enum):
    # 认证授权 (AUTH_*)
    AUTH_INVALID_TOKEN = ('AUTH_001', 'Invalid or expired token', 401, False)
    AUTH_INSUFFICIENT_PERMISSION = ('AUTH_002', 'Insufficient permissions', 403, False)
    AUTH_MFA_REQUIRED = ('AUTH_003', 'MFA verification required', 403, False)

    # 计费 (BILLING_*)
    BILLING_INSUFFICIENT_BALANCE = ('BILLING_001', 'Insufficient balance', 402, False)
    BILLING_CHARGE_FAILED = ('BILLING_002', 'Charge failed', 500, True)
    BILLING_REFUND_FAILED = ('BILLING_003', 'Refund failed', 500, True)
    BILLING_DISCREPANCY = ('BILLING_004', 'Billing discrepancy detected', 500, True)

    # 路由 (ROUTER_*)
    ROUTER_NO_PROVIDER_AVAILABLE = ('ROUTER_001', 'No provider available', 503, True)
    ROUTER_ALL_PROVIDERS_FAILED = ('ROUTER_002', 'All providers failed', 503, True)
    ROUTER_TIMEOUT = ('ROUTER_003', 'Request timeout', 504, True)

    # 供应商 (PROVIDER_*)
    PROVIDER_INVALID_KEY = ('PROVIDER_001', 'Invalid API key', 401, False)
    PROVIDER_RATE_LIMIT = ('PROVIDER_002', 'Rate limit exceeded', 429, False)
    PROVIDER_QUOTA_EXCEEDED = ('PROVIDER_003', 'Quota exceeded', 402, False)
    PROVIDER_MODEL_NOT_FOUND = ('PROVIDER_004', 'Model not found', 404, False)
    PROVIDER_ERROR = ('PROVIDER_005', 'Provider error', 502, True)

    # 限流 (RATE_LIMIT_*)
    RATE_LIMIT_EXCEEDED = ('RATE_LIMIT_001', 'Rate limit exceeded', 429, False)
    RATE_LIMIT_TOKEN_EXCEEDED = ('RATE_LIMIT_002', 'Token limit exceeded', 429, False)
    RATE_LIMIT_BURST_EXCEEDED = ('RATE_LIMIT_003', 'Burst limit exceeded', 429, False)

    # 通用 (COMMON_*)
    COMMON_INVALID_REQUEST = ('COMMON_001', 'Invalid request', 400, False)
    COMMON_RESOURCE_NOT_FOUND = ('COMMON_002', 'Resource not found', 404, False)
    COMMON_INTERNAL_ERROR = ('COMMON_003', 'Internal error', 500, True)
    COMMON_SERVICE_UNAVAILABLE = ('COMMON_004', 'Service unavailable', 503, True)

    def __init__(self, code, message, status_code, retryable):
        self.code = code
        self.message = message
        self.status_code = status_code
        self.retryable = retryable
```

#### 2.2.2 错误响应格式

```python
class ErrorResponse:
    def __init__(
        self,
        error_code: ErrorCode,
        message: str = None,
        details: dict = None,
        request_id: str = None,
        doc_url: str = None
    ):
        self.error = {
            'code': error_code.code,
            'message': message or error_code.message,
            'details': details or {},
            'request_id': request_id,
            'doc_url': doc_url or f'/docs/errors/{error_code.code.lower()}',
            'retryable': error_code.retryable
        }

    def to_dict(self):
        return self.error

    def to_json(self):
        return json.dumps(self.error)

# 使用示例
raise ErrorResponse(
    error_code=ErrorCode.BILLING_INSUFFICIENT_BALANCE,
    details={
        'required': 100.00,
        'available': 50.00,
        'currency': 'USD',
        'top_up_url': '/api/v1/billing/top-up'
    },
    request_id=get_request_id()
)
```

#### 2.2.3 错误码文档生成

```yaml
# openapi.yaml 部分
components:
  ErrorCode:
    type: object
    properties:
      code:
        type: string
        example: BILLING_001
      message:
        type: string
        example: Insufficient balance
      details:
        type: object
      request_id:
        type: string
      doc_url:
        type: string
      retryable:
        type: boolean

  errors:
    BILLING_INSUFFICIENT_BALANCE:
      status: 402
      message: "余额不足"
      details:
        required:
          type: number
          description: "所需金额"
        available:
          type: number
          description: "可用余额"
        top_up_url:
          type: string
          description: "充值链接"
      retryable: false
```

---

## 3. SDK 规划

### 3.1 当前问题

- 无官方SDK
- 开发者体验差

### 3.2 解决方案

#### 3.2.1 SDK 路线图

```
Phase 1 (S1): 兼容层
├── Python SDK (OpenAI兼容)
├── Node.js SDK (OpenAI兼容)
└── 透明迁移工具

Phase 2 (S2): 自有SDK
├── Python SDK (自有API)
├── Node.js SDK (自有API)
└── Go SDK

Phase 3 (S3): 高级功能
├── 重试中间件
├── 缓存中间件
├── 指标中间件
└── 框架集成 (LangChain, LlamaIndex)
```

#### 3.2.2 Python SDK 设计

```python
# lgw-sdk-python
class LLMGateway:
    """LLM Gateway Python SDK"""

    def __init__(
        self,
        api_key: str,
        base_url: str = "https://api.lgateway.com",
        timeout: float = 60.0,
        max_retries: int = 3
    ):
        self.api_key = api_key
        self.base_url = base_url
        self.timeout = timeout
        self.max_retries = max_retries
        self._session = requests.Session()

        # 默认配置
        self.default_headers = {
            'Authorization': f'Bearer {api_key}',
            'Content-Type': 'application/json'
        }

    def chat.completions(
        self,
        model: str,
        messages: List[Dict],
        **kwargs
    ) -> ChatCompletion:
        """聊天完成"""
        response = self._request(
            method='POST',
            path='/v1/chat/completions',
            json={
                'model': model,
                'messages': messages,
                **kwargs
            }
        )
        return ChatCompletion(**response)

    def _request(self, method, path, **kwargs):
        """发送请求（带重试）"""
        url = f"{self.base_url}{path}"
        headers = {**self.default_headers, **kwargs.pop('headers', {})}

        for attempt in range(self.max_retries):
            try:
                response = self._session.request(
                    method=method,
                    url=url,
                    headers=headers,
                    timeout=self.timeout,
                    **kwargs
                )
                response.raise_for_status()
                return response.json()

            except requests.exceptions.RequestException as e:
                if attempt == self.max_retries - 1:
                    raise
                # 指数退避
                time.sleep(2 ** attempt)

# 使用示例
client = LLMGateway(api_key="lgw-xxx")
response = client.chat.completions(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello"}]
)
print(response.choices[0].message.content)
```

#### 3.2.3 Node.js SDK 设计

```typescript
// lgw-sdk-node
export class LLMGateway {
  private apiKey: string;
  private baseURL: string;
  private maxRetries: number;

  constructor(config: LLMGatewayConfig) {
    this.apiKey = config.apiKey;
    this.baseURL = config.baseURL || 'https://api.lgateway.com';
    this.maxRetries = config.maxRetries || 3;
  }

  async chat.completions(
    params: ChatCompletionParams
  ): Promise<ChatCompletion> {
    const response = await this.request(
      'POST',
      '/v1/chat/completions',
      params
    );
    return response as ChatCompletion;
  }

  private async request<T>(
    method: string,
    path: string,
    body?: any,
    retries: number = 0
  ): Promise<T> {
    try {
      const response = await fetch(`${this.baseURL}${path}`, {
        method,
        headers: {
          'Authorization': `Bearer ${this.apiKey}`,
          'Content-Type': 'application/json',
        },
        body: body ? JSON.stringify(body) : undefined,
      });

      if (!response.ok) {
        throw new LLMGatewayError(await response.json());
      }

      return response.json();
    } catch (error) {
      if (retries < this.maxRetries) {
        await this.sleep(Math.pow(2, retries));
        return this.request(method, path, body, retries + 1);
      }
      throw error;
    }
  }
}
```

---

## 4. 实施计划

### 4.1 任务分解

| 任务 | 负责人 | 截止 | 依赖 |
|------|--------|------|------|
| API版本管理中间件 | 架构 | S0-M1 | - |
| 错误码体系定义 | 后端 | S0-M1 | - |
| 错误响应格式统一 | 后端 | S0-M1 | - |
| Python SDK开发 | 前端 | S1 | - |
| Node.js SDK开发 | 前端 | S1 | - |

### 4.2 验证标准

- API版本可管理、可废弃
- 所有错误都有完整错误码
- SDK可通过pip/npm安装

---

**文档状态**：API设计解决方案
**关联文档**：
- `llm_gateway_prd_v0_2026-03-16.md`
