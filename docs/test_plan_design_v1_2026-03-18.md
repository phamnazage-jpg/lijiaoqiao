# 测试方案设计

> 版本：v1.0
> 日期：2026-03-18
> 依据：testing skill 最佳实践

---

## 1. 测试策略概述

### 1.1 测试金字塔

```
              ╱╲
             ╱  ╲        E2E Tests (10%)
            ╱    ╲
           ╱──────╲
          ╱        ╲      Integration Tests (20%)
         ╱──────────╲
        ╱            ╲
       ╱────────────╲     Unit Tests (70%)
      ╱              ╲
     ╱────────────────╲
```

### 1.2 测试目标

| 指标 | 目标 | 说明 |
|------|------|------|
| 代码覆盖率 | >= 80% | 核心业务 |
| 单元测试通过率 | 100% | 必须通过 |
| 集成测试通过率 | 100% | 必须通过 |
| E2E测试通过率 | 95% | 允许5% flaky |
| 构建门禁 | 100% | CI必须通过 |

---

## 2. 单元测试

### 2.1 测试框架

```python
# pytest.ini
[pytest]
testpaths = tests/unit
python_files = test_*.py
python_classes = Test*
python_functions = test_*
addopts =
    -v
    --strict-markers
    --tb=short
    --cov=llm_gateway
    --cov-report=term-missing
    --cov-report=html
markers =
    unit: Unit tests
    integration: Integration tests
    e2e: End-to-end tests
    slow: Slow running tests
```

### 2.2 单元测试示例

```python
# tests/unit/service/test_billing.py
import pytest
from decimal import Decimal
from unittest.mock import Mock, patch
from llm_gateway.service.billing import BillingService
from llm_gateway.service.repository import BillingRepository
from llm_gateway.service.balance import BalanceManager

class TestBillingService:
    """计费服务单元测试"""

    @pytest.fixture
    def billing_service(self):
        """Fixture: 计费服务实例"""
        repo = Mock(spec=BillingRepository)
        balance_mgr = Mock(spec=BalanceManager)
        return BillingService(repo, balance_mgr)

    def test_estimate_cost_gpt4(self, billing_service):
        """测试GPT-4成本估算"""
        # Arrange
        request = Mock()
        request.Model = "gpt-4"
        request.Messages.Tokens.return_value = 1000
        request.Options.MaxTokens = 1000

        # Act
        cost = billing_service.EstimateCost(request)

        # Assert
        assert cost.Amount > 0
        assert cost.Currency == "USD"

    def test_estimate_cost_gpt35(self, billing_service):
        """测试GPT-3.5成本估算"""
        # Arrange
        request = Mock()
        request.Model = "gpt-3.5-turbo"
        request.Messages.Tokens.return_value = 1000
        request.Options.MaxTokens = 1000

        # Act
        cost = billing_service.EstimateCost(request)

        # Assert
        # GPT-3.5应该比GPT-4便宜
        gpt4_cost = billing_service.EstimateCost(self._create_request("gpt-4"))
        assert cost.Amount < gpt4_cost.Amount

    @pytest.mark.parametrize("model,expected_tokens", [
        ("gpt-4", 100),
        ("gpt-3.5-turbo", 50),
        ("claude-3-opus", 150),
    ])
    def test_estimate_cost_models(self, billing_service, model, expected_tokens):
        """参数化测试：不同模型成本估算"""
        request = self._create_request(model)
        cost = billing_service.EstimateCost(request)
        assert cost.Amount > 0

    def _create_request(self, model):
        """创建测试请求"""
        request = Mock()
        request.Model = model
        request.Messages.Tokens.return_value = 1000
        request.Options.MaxTokens = 1000
        return request

    def test_insufficient_balance(self, billing_service):
        """测试余额不足场景"""
        # Arrange
        billing_service.balance_mgr.Reserve.return_value = None
        billing_service.balance_mgr.ErrInsufficientBalance = Exception()

        request = self._create_request("gpt-4")

        # Act & Assert
        with pytest.raises(Exception) as exc_info:
            billing_service.ProcessRequest(request)

        assert "insufficient" in str(exc_info.value).lower()

    def test_process_request_success(self, billing_service):
        """测试成功处理请求"""
        # Arrange
        billing_service.balanceMgr.Reserve.return_value = Mock(Amount=Decimal("0.10"))
        billing_service.balanceMgr.Charge.return_value = None
        billing_service.repo.Create.return_value = None

        request = self._create_request("gpt-4")
        request.Response = Mock()
        request.Response.Usage.PromptTokens = 500
        request.Response.Usage.CompletionTokens = 500
        request.ID = "req-123"

        # Act
        record = billing_service.ProcessRequest(request)

        # Assert
        assert record is not None
        assert record.UserID == request.UserID
        billing_service.balanceMgr.Reserve.assert_called_once()
        billing_service.repo.Create.assert_called_once()
```

### 2.3 Router服务测试

```python
# tests/unit/service/test_router.py
import pytest
from unittest.mock import Mock, AsyncMock
from llm_gateway.service.router import RouterService
from llm_gateway.internal.adapter import Registry, Provider

class TestRouterService:
    """路由服务单元测试"""

    @pytest.fixture
    def mock_provider(self):
        """Mock供应商"""
        provider = Mock(spec=Provider)
        provider.Name.return_value = "openai"
        provider.HealthCheck.return_value = None
        provider.Call.return_value = Mock(
            id="resp-123",
            choices=[Mock(delta=Mock(content="Hello"))],
            usage=Mock(prompt_tokens=10, completion_tokens=5)
        )
        return provider

    @pytest.fixture
    def router_service(self, mock_provider):
        """Fixture: 路由服务实例"""
        registry = Mock(spec=Registry)
        registry.GetAvailableProviders.return_value = [mock_provider]
        registry.Get.return_value = mock_provider
        return RouterService(registry)

    @pytest.mark.asyncio
    async def test_route_success(self, router_service, mock_provider):
        """测试成功路由"""
        # Arrange
        request = Mock()
        request.Model = "gpt-4"
        request.UserID = 1
        request.TenantID = 1
        request.Messages = []

        # Act
        response = await router_service.Route(request)

        # Assert
        assert response is not None
        mock_provider.Call.assert_called_once()

    @pytest.mark.asyncio
    async def test_route_no_provider(self, router_service):
        """测试无可用供应商"""
        # Arrange
        router_service.adapterRegistry.GetAvailableProviders.return_value = []

        request = Mock()
        request.Model = "gpt-4"

        # Act & Assert
        with pytest.raises(Exception) as exc_info:
            await router_service.Route(request)

        assert "no provider" in str(exc_info.value).lower()

    @pytest.mark.asyncio
    async def test_route_fallback_on_error(self, router_service, mock_provider):
        """测试失败时降级"""
        # Arrange
        mock_provider.Call.side_effect = [Exception("API Error"), Mock()]

        request = Mock()
        request.Model = "gpt-4"

        # Act
        response = await router_service.Route(request)

        # Assert
        assert response is not None
        assert mock_provider.Call.call_count == 2  # 重试一次
```

---

## 3. 集成测试

### 3.1 测试夹具

```python
# tests/conftest.py
import pytest
import asyncio
from sqlalchemy import create_engine
from sqlalchemy.orm import sessionmaker
from httpx import AsyncClient
from llm_gateway.main import app
from llm_gateway.database import Base

@pytest.fixture(scope="session")
def event_loop():
    """创建事件循环"""
    loop = asyncio.get_event_loop_policy().new_event_loop()
    yield loop
    loop.close()

@pytest.fixture(scope="function")
def db_engine():
    """测试数据库引擎"""
    engine = create_engine("sqlite:///:memory:")
    Base.metadata.create_all(engine)
    yield engine
    Base.metadata.drop_all(engine)

@pytest.fixture(scope="function")
def db_session(db_engine):
    """测试数据库会话"""
    Session = sessionmaker(bind=db_engine)
    session = Session()
    yield session
    session.close()

@pytest.fixture(scope="function")
async def client():
    """测试客户端"""
    async with AsyncClient(app=app, base_url="http://test") as ac:
        yield ac

@pytest.fixture
def test_user(db_session):
    """创建测试用户"""
    user = User(
        email="test@example.com",
        password_hash="hashed_password",
        name="Test User"
    )
    db_session.add(user)
    db_session.commit()
    return user
```

### 3.2 API集成测试

```python
# tests/integration/api/test_chat.py
import pytest
from httpx import AsyncClient

class TestChatAPI:
    """聊天API集成测试"""

    @pytest.mark.asyncio
    async def test_chat_completions_success(self, client: AsyncClient, test_user):
        """测试成功创建聊天完成"""
        # Arrange
        token = await self._get_token(client, test_user)

        # Act
        response = await client.post(
            "/v1/chat/completions",
            json={
                "model": "gpt-3.5-turbo",
                "messages": [
                    {"role": "user", "content": "Hello"}
                ]
            },
            headers={"Authorization": f"Bearer {token}"}
        )

        # Assert
        assert response.status_code == 200
        data = response.json()
        assert "choices" in data
        assert len(data["choices"]) > 0

    @pytest.mark.asyncio
    async def test_chat_completions_unauthorized(self, client: AsyncClient):
        """测试未授权访问"""
        # Act
        response = await client.post(
            "/v1/chat/completions",
            json={
                "model": "gpt-3.5-turbo",
                "messages": [
                    {"role": "user", "content": "Hello"}
                ]
            }
        )

        # Assert
        assert response.status_code == 401

    @pytest.mark.asyncio
    async def test_chat_completions_invalid_model(self, client: AsyncClient, test_user):
        """测试无效模型"""
        # Arrange
        token = await self._get_token(client, test_user)

        # Act
        response = await client.post(
            "/v1/chat/completions",
            json={
                "model": "invalid-model",
                "messages": [
                    {"role": "user", "content": "Hello"}
                ]
            },
            headers={"Authorization": f"Bearer {token}"}
        )

        # Assert
        assert response.status_code == 400
        assert "model" in response.json()["error"]["code"].lower()

    async def _get_token(self, client, user):
        """获取测试令牌"""
        response = await client.post(
            "/v1/auth/token",
            json={
                "email": user.email,
                "password": "test_password"
            }
        )
        return response.json()["access_token"]
```

---

## 4. 契约测试

### 4.1 Provider契约测试

```python
# tests/contract/test_provider_adapter.py
import pytest
from llm_gateway.internal.adapter import ProviderAdapter
from llm_gateway.service.adapter import OpenAIAdapter

class TestProviderContract:
    """供应商适配器契约测试"""

    @pytest.fixture
    def adapter(self):
        """适配器实例"""
        return OpenAIAdapter(api_key="test-key")

    @pytest.mark.asyncio
    async def test_response_structure(self, adapter):
        """测试响应结构符合契约"""
        # Act
        response = await adapter.chat_completion(
            model="gpt-3.5-turbo",
            messages=[{"role": "user", "content": "Hello"}]
        )

        # Assert - 验证必需字段
        assert hasattr(response, 'id')
        assert hasattr(response, 'model')
        assert hasattr(response, 'choices')
        assert hasattr(response, 'usage')
        assert response.usage.prompt_tokens >= 0
        assert response.usage.completion_tokens >= 0
        assert response.usage.total_tokens >= 0

    @pytest.mark.asyncio
    async def test_error_mapping(self, adapter):
        """测试错误码映射"""
        # 测试各种错误情况
        test_cases = [
            (Exception("invalid_api_key"), "INVALID_KEY"),
            (Exception("rate_limit_exceeded"), "RATE_LIMIT"),
            (Exception("insufficient_quota"), "INSUFFICIENT_QUOTA"),
        ]

        for original_error, expected_code in test_cases:
            result = adapter.map_error(original_error)
            assert result.code == expected_code

    @pytest.mark.asyncio
    async def test_streaming(self, adapter):
        """测试流式响应"""
        # Act
        response = await adapter.chat_completion(
            model="gpt-3.5-turbo",
            messages=[{"role": "user", "content": "Count to 5"}],
            stream=True
        )

        # Assert
        chunks = []
        async for chunk in response.stream():
            chunks.append(chunk)
            if len(chunks) >= 5:
                break

        assert len(chunks) > 0
        assert all(hasattr(c, 'delta') for c in chunks)
```

### 4.2 契约漂移检测

```yaml
# .github/workflows/contract-test.yml
name: Contract Tests

on:
  pull_request:
    branches: [main]
  push:
    branches: [main]

jobs:
  contract-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3

      - name: Set up Python
        uses: actions/setup-python@v4
        with:
          python-version: '3.11'

      - name: Install dependencies
        run: |
          pip install pytest pytest-asyncio pact

      - name: Run contract tests
        run: |
          pytest tests/contract/ -v --contract=true

      - name: Publish contract
        if: github.ref == 'refs/heads/main'
        run: |
          pact-broker publish \
            pactDir=./pacts \
            brokerUrl=${{ secrets.PACT_BROKER_URL }} \
            brokerToken=${{ secrets.PACT_BROKER_TOKEN }}
```

---

## 5. E2E测试

### 5.1 Playwright E2E测试

```python
# tests/e2e/test_user_journey.py
import pytest
from playwright.async_api import async_playwright

class TestUserJourney:
    """用户旅程E2E测试"""

    @pytest.fixture
    async def browser_context(self):
        """浏览器上下文"""
        async with async_playwright() as p:
            browser = await p.chromium.launch()
            context = await browser.new_context()
            yield context
            await context.close()
            await browser.close()

    @pytest.mark.asyncio
    async def test_complete_user_flow(self, browser_context):
        """测试完整用户流程"""
        page = await browser_context.new_page()

        # 1. 注册
        await page.goto("https://app.lgateway.com/register")
        await page.fill("[name=email]", "user@example.com")
        await page.fill("[name=password]", "SecurePassword123!")
        await page.click("button[type=submit]")
        await page.wait_for_selector(".dashboard")

        # 2. 创建API Key
        await page.click("text=API Keys")
        await page.click("text=Create Key")
        await page.fill("[name=description]", "Test Key")
        await page.click("button:has-text('Create')")
        api_key = await page.text_content(".api-key")

        # 3. 测试API调用
        response = await self._call_api(api_key, {
            "model": "gpt-3.5-turbo",
            "messages": [{"role": "user", "content": "Hello"}]
        })
        assert response.status == 200

        # 4. 查看使用量
        await page.click("text=Usage")
        await page.wait_for_selector(".usage-chart")

        # 5. 检查账单
        await page.click("text=Billing")
        await page.wait_for_selector(".balance")

    async def _call_api(self, api_key, payload):
        """调用API"""
        import httpx
        async with httpx.AsyncClient() as client:
            return await client.post(
                "https://api.lgateway.com/v1/chat/completions",
                json=payload,
                headers={"Authorization": f"Bearer {api_key}"}
            )
```

---

## 6. 性能测试

### 6.1 负载测试

```python
# tests/performance/test_load.py
import pytest
import asyncio
import time
from locust import HttpUser, task, between

class LLMGatewayUser(HttpUser):
    """Locust负载测试用户"""
    wait_time = between(0.5, 2)

    def on_start(self):
        """初始化"""
        response = self.client.post("/v1/auth/token", json={
            "email": "test@example.com",
            "password": "password"
        })
        self.token = response.json()["access_token"]

    @task(10)
    def chat_completion(self):
        """聊天完成请求"""
        self.client.post(
            "/v1/chat/completions",
            json={
                "model": "gpt-3.5-turbo",
                "messages": [{"role": "user", "content": "Hello"}]
            },
            headers={"Authorization": f"Bearer {self.token}"}
        )

    @task(1)
    def list_models(self):
        """列出模型"""
        self.client.get(
            "/v1/models",
            headers={"Authorization": f"Bearer {self.token}"}
        )
```

### 6.2 性能基准

```yaml
# k6/performance.js
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '2m', target: 100 },  // 2分钟内增加到100用户
    { duration: '5m', target: 100 },  // 保持100用户5分钟
    { duration: '2m', target: 200 },  // 增加到200用户
    { duration: '5m', target: 200 },  // 保持200用户5分钟
    { duration: '2m', target: 0 },   // 降到0
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],  // P95 < 500ms
    http_req_failed: ['rate<0.01'],   // 失败率 < 1%
  },
};

export default function () {
  const payload = JSON.stringify({
    model: 'gpt-3.5-turbo',
    messages: [{ role: 'user', content: 'Hello' }]
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${__ENV.API_KEY}`,
    },
  };

  const res = http.post('https://api.lgateway.com/v1/chat/completions', payload, params);
  check(res, { 'status was 200': (r) => r.status === 200 });
  sleep(1);
}
```

---

## 7. 测试覆盖率目标

### 7.1 覆盖率矩阵

| 模块 | 目标覆盖率 | 关键测试 |
|------|-----------|----------|
| Router Service | 90% | 路由选择、fallback |
| Billing Service | 85% | 计费、扣款、退款 |
| Auth Service | 80% | 认证、授权 |
| Adapter | 85% | 供应商调用、错误处理 |
| Middleware | 75% | 限流、日志 |
| API Handlers | 70% | 请求验证、响应格式化 |

---

## 8. CI/CD集成

### 8.1 GitHub Actions

```yaml
# .github/workflows/test.yml
name: Test Pipeline

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main]

jobs:
  unit-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-python@v4
        with:
          python-version: '3.11'
      - run: pip install -r requirements-test.txt
      - run: pytest tests/unit/ -v --cov

  integration-test:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: postgres:15
        env:
          POSTGRES_PASSWORD: test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-python@v4
        with:
          python-version: '3.11'
      - run: pip install -r requirements-test.txt
      - run: pytest tests/integration/ -v

  contract-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - run: pytest tests/contract/ -v --contract

  e2e-test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-node@v3
        with:
          node-version: '18'
      - run: npm install
      - run: npx playwright install
      - run: npx playwright test

  security-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: snyk/actions/python@master
        env:
          SNYK_TOKEN: ${{ secrets.SNYK_TOKEN }}
```

---

## 9. 混沌工程测试

### 9.1 故障注入策略

```python
# tests/chaos/test_fault_injection.py
import pytest
from chaos.engine import ChaosEngine

class TestChaosEngineering:
    """混沌工程测试 - 验证系统韧性"""

    @pytest.fixture
    def chaos(self):
        """混沌引擎"""
        return ChaosEngine()

    @pytest.mark.asyncio
    async def test_provider_timeout_handling(self, chaos):
        """测试供应商超时处理"""
        # 注入：供应商响应超时
        await chaos.inject_latency(
            target="provider:openai",
            delay=30  # 30秒延迟
        )

        # 验证：系统触发降级
        response = await router.route(request)
        assert response.fallback_triggered
        assert response.fallback_provider == "anthropic"

    @pytest.mark.asyncio
    async def test_circuit_breaker_open(self, chaos):
        """测试断路器打开"""
        # 注入：连续失败
        await chaos.inject_errors(
            target="provider:azure",
            count=10,
            error_type="connection"
        )

        # 验证：断路器打开
        cb_state = await chaos.get_circuit_state("azure")
        assert cb_state == "OPEN"

    @pytest.mark.asyncio
    async def test_network_partition(self, chaos):
        """测试网络分区"""
        # 注入：网络分区
        await chaos.network_partition(
            source="gateway",
            target="billing",
            drop_packets=0.5
        )

        # 验证：异步计费
        billing = await router.route(request)
        assert billing.async_processed

    @pytest.mark.asyncio
    async def test_database_failure(self, chaos):
        """测试数据库故障"""
        # 注入：主库故障
        await chaos.failover_database(
            from_primary=True
        )

        # 验证：自动切换到从库
        db_state = await get_database_state()
        assert db_state.active == "replica"
        assert db_state.data_consistent
```

### 9.2 韧性验证场景

| 场景 | 注入故障 | 预期行为 |
|------|----------|----------|
| 单Provider宕机 | kill provider进程 | 自动切换到备选Provider |
| Redis不可用 | 网络隔离 | 降级到本地限流 |
| 数据库故障 | 主库不可用 | 自动切换从库，写入延迟处理 |
| 流量突增 | 10倍QPS | 限流生效，无雪崩 |
| 依赖服务超时 | 注入超时 | 快速失败，不阻塞 |

---

## 10. 安全测试

### 10.1 OWASP Top 10 防护测试

```python
# tests/security/test_owasp.py
import pytest
from security.scanner import VulnerabilityScanner

class TestSecurityVulnerabilities:
    """安全漏洞测试"""

    def test_sql_injection_prevention(self):
        """测试SQL注入防护"""
        # 恶意输入
        malicious_inputs = [
            "' OR '1'='1",
            "'; DROP TABLE users;--",
            "1' UNION SELECT * FROM passwords--"
        ]

        for payload in malicious_inputs:
            response = api.get(f"/users?name={payload}")
            assert response.status_code == 400
            assert "injection" not in response.text.lower()

    def test_api_key_exposure(self):
        """测试API Key泄露检测"""
        # 模拟响应包含敏感信息
        response = api.get("/v1/models")
        assert api_key not in response.text
        assert not any(k in response.headers for k in ['X-API-Key', 'Authorization'])

    def test_rate_limiting_bypass(self):
        """测试限流绕过防护"""
        # 尝试绕过限流
        for i in range(150):
            response = api.post("/v1/chat/completions", data)
            if i >= 100:
                assert response.status_code == 429

    def test_privilege_escalation(self):
        """测试权限提升防护"""
        # 普通用户尝试访问管理员API
        response = api_admin.delete("/admin/users/1")
        assert response.status_code == 403

    def test_cors_misconfiguration(self):
        """测试CORS配置"""
        response = api.options("/api/v1/")
        assert "Access-Control-Allow-Origin" in response.headers
        # 验证不允许任意Origin
        assert response.headers.get("Access-Control-Allow-Origin") != "*"
```

### 10.2 密钥轮换测试

```python
# tests/security/test_key_rotation.py
class TestKeyRotation:
    """密钥轮换测试"""

    def test_automatic_key_rotation(self):
        """测试自动密钥轮换"""
        # 1. 触发轮换
        rotation_service.trigger_rotation()

        # 2. 验证新密钥生效
        new_key = key_manager.get_active_key()
        assert new_key.version > old_key.version
        assert new_key.is_active

        # 3. 验证旧密钥过期
        assert not old_key.is_active
        # 验证有过渡期
        assert old_key.expires_at > now

    def test_key_rotation_graceful(self):
        """测试轮换期间服务不中断"""
        # 模拟轮换期间的请求
        requests = [api_request() for _ in range(100)]
        results = parallel_execute(requests)

        # 验证所有请求成功（使用旧密钥或新密钥）
        assert all(r.success for r in results)
```

### 10.3 日志脱敏测试

```python
# tests/security/test_log_redaction.py
class TestLogRedaction:
    """日志脱敏测试"""

    def test_sensitive_data_redaction(self):
        """测试敏感数据脱敏"""
        # 记录包含敏感信息的日志
        logger.info(f"User {user_id} payment: {credit_card}")

        # 验证日志已脱敏
        log_entry = get_latest_log()
        assert credit_card not in log_entry.message
        assert "****" in log_entry.message  # 脱敏后格式
        assert "4" in log_entry.message  # 保留后4位

    def test_pii_detection(self):
        """测试PII检测"""
        pii_data = [
            "13812345678",  # 手机号
            "user@example.com",  # 邮箱
            "610102199001011234",  # 身份证
        ]

        for pii in pii_data:
            logger.info(f"User data: {pii}")
            log = get_latest_log()
            assert pii not in log.message
```

---

## 11. 可观测性测试

### 11.1 指标验证测试

```python
# tests/observability/test_metrics.py
class TestMetricsEmission:
    """指标发射测试"""

    def test_request_latency_histogram(self):
        """测试请求延迟直方图"""
        # 发送请求
        response = api.post("/v1/chat/completions", request_data)

        # 验证指标
        metrics = prometheus.get_metrics("http_request_duration_seconds")
        assert metrics.labels["method"] == "POST"
        assert metrics.labels["status"] == "200"
        assert metrics.value > 0

    def test_billing_amount_gauge(self):
        """测试计费金额仪表"""
        # 执行计费
        billing.charge(user_id, amount)

        # 验证指标
        metrics = prometheus.get_metrics("billing_charged_amount")
        assert metrics.labels["currency"] == "USD"
        assert metrics.value == amount

    def test_provider_failure_counter(self):
        """测试供应商失败计数"""
        # 触发失败
        for _ in range(5):
            try:
                provider.call(request)
            except Exception:
                pass

        # 验证计数器
        counter = prometheus.get_metrics("provider_calls_total")
        assert counter.labels["status"] == "error"
        assert counter.value >= 5
```

### 11.2 链路追踪验证

```python
# tests/observability/test_tracing.py
class TestDistributedTracing:
    """分布式追踪测试"""

    def test_trace_context_propagation(self):
        """测试Trace上下文传播"""
        # 发起请求
        response = api.post("/v1/chat/completions", request)

        # 验证TraceID
        trace_id = response.headers["X-Trace-ID"]
        spans = jaeger.get_spans(trace_id)

        # 验证链路完整
        assert len(spans) >= 4  # gateway -> router -> adapter -> provider
        assert all(s.parent_id in [s.id for s in spans] for s in spans)

    def test_span_attributes(self):
        """测试Span属性完整"""
        spans = jaeger.get_spans(trace_id)

        for span in spans:
            assert span.name
            assert span.service_name
            assert span.start_time
            assert span.duration > 0
            # 验证关键属性
            if span.name == "provider.call":
                assert span.attributes["provider"]
                assert span.attributes["model"]
```

### 11.3 告警触发验证

```python
# tests/observability/test_alerts.py
class TestAlerting:
    """告警测试"""

    def test_high_latency_alert(self):
        """测试高延迟告警"""
        # 注入高延迟
        for _ in range(10):
            await provider.call(delay=5)

        # 验证告警
        alert = alert_manager.get_latest_alert()
        assert alert.name == "HighLatencyP99"
        assert alert.severity == "P1"

    def test_low_balance_alert(self):
        """测试低余额告警"""
        # 设置低余额
        balance.set_balance(user_id, 10)

        # 触发检查
        await balance.check_threshold()

        # 验证告警
        alert = alert_manager.get_latest_alert()
        assert alert.name == "LowBalance"
        assert user_id in alert.targets

---

## 12. 测试数据管理

### 12.1 测试数据工厂

```python
# tests/fixtures/factories.py
import factory
from datetime import datetime

class UserFactory(factory.Factory):
    """用户测试数据工厂"""

    class Meta:
        model = dict

    user_id = factory.Sequence(lambda n: 10000 + n)
    email = factory.LazyAttribute(lambda o: f"user{o.user_id}@test.com")
    name = factory.Faker("name")
    tier = "growth"
    balance = factory.Faker("pydecimal", left_digits=5, right_digits=2)
    created_at = factory.LazyFunction(datetime.now)


class APIKeyFactory(factory.Factory):
    """API Key测试数据工厂"""

    class Meta:
        model = dict

    key_id = factory.Sequence(lambda n: f"sk-test-{n:08d}")
    user_id = factory.SubFactory(UserFactory)
    name = "Test Key"
    quota = 10000
    rate_limit = 1000
    is_active = True
    created_at = factory.LazyFunction(datetime.now)


class ProviderFactory(factory.Factory):
    """Provider测试数据工厂"""

    class Meta:
        model = dict

    provider_id = factory.Sequence(lambda n: n)
    name = factory.Iterator(["openai", "anthropic", "azure", "google"])
    api_base = "https://api.example.com"
    latency_p99 = factory.Faker("pyint", min_value=50, max_value=500)
    availability = factory.Faker("pyfloat", min_value=0.95, max_value=1.0)
    cost_per_1k = factory.Faker("pyfloat", min_value=0.5, max_value=10.0)
```

### 12.2 测试数据隔离

```python
# tests/conftest.py
import pytest
from tests.fixtures.database import TestDatabase

@pytest.fixture(scope="session")
def test_db():
    """测试数据库会话级fixture"""
    db = TestDatabase()
    db.init(schema="tests/fixtures/schema.sql")
    yield db
    db.cleanup()

@pytest.fixture
def clean_user(test_db):
    """每个测试前清理用户数据"""
    test_db.execute("DELETE FROM users WHERE email LIKE '%@test.com'")
    yield
    test_db.execute("DELETE FROM users WHERE email LIKE '%@test.com'")

@pytest.fixture
def isolated_balance(test_db):
    """隔离的余额测试"""
    # 每个测试使用独立账户
    account_id = test_db.create_test_account()
    test_db.set_balance(account_id, 10000)
    yield account_id
    test_db.cleanup_account(account_id)
```

### 12.3 测试数据版本管理

```yaml
# tests/data/version.yaml
# 测试数据版本管理
version: "1.0"

datasets:
  user_tier_free:
    count: 100
    balance_range: [0, 100]
    tier: free

  user_tier_growth:
    count: 50
    balance_range: [100, 10000]
    tier: growth

  user_tier_enterprise:
    count: 10
    balance_range: [10000, 100000]
    tier: enterprise

  provider_active:
    - name: openai
      models: [gpt-4, gpt-3.5-turbo]
      status: active
    - name: anthropic
      models: [claude-3-opus, claude-3-sonnet]
      status: active
```

---

## 13. 部署验证测试

### 13.1 环境一致性验证

```python
# tests/deployment/test_environment.py
class TestEnvironmentConsistency:
    """环境一致性验证"""

    def test_environment_variables(self):
        """验证环境变量配置"""
        required_vars = [
            "DATABASE_URL",
            "REDIS_URL",
            "KAFKA_BROKERS",
            "LOG_LEVEL",
        ]

        for var in required_vars:
            assert os.environ.get(var), f"Missing env var: {var}"

    def test_database_schema_version(self):
        """验证数据库schema版本"""
        # 获取当前版本
        current_version = db.get_schema_version()

        # 获取期望版本
        expected_version = get_code_schema_version()

        assert current_version == expected_version, \
            f"Schema mismatch: db={current_version}, code={expected_version}"

    def test_dependencies_installed(self):
        """验证依赖包版本"""
        import pkg_resources

        requirements = open("requirements.txt").read()
        for req in pkg_resources.parse_requirements(requirements):
            try:
                installed = pkg_resources.get_distribution(req.project_name)
                assert str(installed.version) in str(req.specifier)
            except Exception as e:
                pytest.fail(f"Dependency issue: {req}, error: {e}")
```

### 13.2 健康检查验证

```python
# tests/deployment/test_health.py
class TestHealthChecks:
    """健康检查验证"""

    def test_gateway_health(self):
        """测试网关健康"""
        response = requests.get("http://localhost:8080/health")
        assert response.status_code == 200

        data = response.json()
        assert data["status"] == "healthy"
        assert "version" in data

    def test_service_dependencies(self):
        """测试服务依赖"""
        response = requests.get("http://localhost:8080/health/ready")
        data = response.json()

        # 验证所有依赖健康
        assert data["dependencies"]["database"]["status"] == "up"
        assert data["dependencies"]["redis"]["status"] == "up"
        assert data["dependencies"]["kafka"]["status"] == "up"

    def test_startup_probe(self):
        """测试启动探针"""
        # 模拟服务启动
        start_time = time.time()

        while time.time() - start_time < 30:
            try:
                response = requests.get("http://localhost:8080/health")
                if response.status_code == 200:
                    break
            except Exception:
                pass
            time.sleep(1)

        # 验证30秒内启动完成
        assert time.time() - start_time < 30
```

### 13.3 配置验证

```python
# tests/deployment/test_config.py
class TestConfigurationValidation:
    """配置验证测试"""

    def test_secret_rotation_config(self):
        """验证密钥轮换配置"""
        config = get_config()

        assert config.rotation_enabled is True
        assert config.rotation_interval_days == 90
        assert config.grace_period_hours == 24

    def test_rate_limit_config(self):
        """验证限流配置"""
        config = get_config()

        assert config.rate_limit.global_limit == 100000
        assert config.rate_limit.tenant_limit == 10000
        assert config.rate_limit.apikey_limit == 1000

    def test_circuit_breaker_config(self):
        """验证断路器配置"""
        config = get_config()

        assert config.circuit_breaker.failure_threshold == 5
        assert config.circuit_breaker.timeout_seconds == 60
        assert config.circuit_breaker.half_open_max_calls == 3
```

### 13.4 金丝雀部署验证

```python
# tests/deployment/test_canary.py
class TestCanaryDeployment:
    """金丝雀部署验证"""

    def test_canary_routing(self):
        """测试金丝雀路由"""
        # 发送流量到新版本
        for i in range(100):
            response = api.post("/v1/chat/completions", request)

        # 验证10%流量到新版本
        metrics = get_canary_metrics()
        assert 0.05 < metrics.canary_percentage < 0.15

    def test_canary_error_rate(self):
        """测试金丝雀错误率"""
        errors = get_canary_errors()
        assert errors.new_version_error_rate < 0.01
        assert errors.new_version_error_rate < errors.old_version_error_rate * 2

    def test_rollback_on_failure(self):
        """测试失败自动回滚"""
        # 注入失败
        inject_failure("canary", error_rate=0.5)

        # 等待检测和回滚
        time.sleep(60)

        # 验证已回滚
        version = get_current_version()
        assert version == "stable"


### 9.1 与技术架构一致性

| 测试项 | 对应模块 | 验证点 |
|--------|----------|--------|
| Provider Adapter测试 | `technical_architecture.md` | 契约符合 |
| 路由策略测试 | `technical_architecture.md` | 选择算法 |
| 计费精度测试 | `business_solution_v1.md` | Decimal精度 |
| 限流测试 | `p1_optimization_solution_v1.md` | 多维度 |
| 风控测试 | `security_solution_v1.md` | 规则执行 |

---

**文档状态**：测试方案设计
**关联文档**：
- `technical_architecture_design_v1_2026-03-18.md`
- `architecture_solution_v1_2026-03-18.md`
- `security_solution_v1_2026-03-18.md`
