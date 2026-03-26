# 架构解决方案（P0问题修复）

> 版本：v1.0
> 日期：2026-03-18
> 目的：系统性解决评审发现的架构P0问题

---

## 1. Router Core 自研风险控制

### 1.1 当前问题

- S2目标60%接管率激进
- 首次自研缺乏经验
- 时间只有13周

### 1.2 解决方案

#### 1.2.1 终验目标不变，增加过程缓冲

| 指标 | 原目标 | v4.1收敛目标 | 理由 |
|------|--------|-------------|------|
| 全供应商接管率（终验） | >=60% | **>=60%（不降档）** | 保持与主基线一致 |
| 全供应商接管率（过程） | - | **40%中间检查点** | 降低推进风险 |
| 国内供应商接管率 | 100% | **100%** | 保持不变 |
| 验收时间 | S2结束 | **S2结束（13周）** | 与S2主基线一致 |

#### 1.2.2 分阶段验证

```python
class RouterTakeoverPlan:
    STAGES = [
        {
            'name': 'S2-A',
            'target_rate': 0.10,
            'duration_weeks': 4,
            'goal': '验证稳定性',
            'success_criteria': {
                'availability': 0.999,
                'latency_p99': 200,
                'error_rate': 0.001
            }
        },
        {
            'name': 'S2-B',
            'target_rate': 0.30,
            'duration_weeks': 4,
            'goal': '优化性能',
            'success_criteria': {
                'availability': 0.9995,
                'latency_p99': 150,
                'error_rate': 0.0005
            }
        },
        {
            'name': 'S2-C1',
            'target_rate': 0.40,
            'duration_weeks': 2,
            'goal': '中间检查点',
            'success_criteria': {
                'availability': 0.9999,
                'latency_p99': 120,
                'error_rate': 0.0001
            }
        },
        {
            'name': 'S2-C2',
            'target_rate': 0.60,  # 终验目标保持60%
            'duration_weeks': 4,
            'goal': '达成终验目标',
            'success_criteria': {
                'availability': 0.9999,
                'latency_p99': 100,
                'error_rate': 0.0001
            }
        }
    ]
```

#### 1.2.3 原型提前启动

```
时间线：

W1-W4:  Router Core 原型开发（提前开始）
  │
W5-W8:  S0 阶段
  │
W9-S2:  继续开发 + 集成测试
  │
S2-A:   10% 流量验证
S2-B:   30% 流量验证
S2-C:   40% 中间检查点（终验目标仍为60%）
```

---

## 2. Subapi 耦合解耦

### 2.1 当前问题

- 直接依赖 subapi
- 升级可能破坏兼容
- 定制困难

### 2.2 解决方案

#### 2.2.1 Provider Adapter 抽象层

```python
from abc import ABC, abstractmethod

class ProviderAdapter(ABC):
    """供应商适配器抽象基类"""

    @abstractmethod
    async def chat_completion(
        self,
        model: str,
        messages: List[Message],
        options: CompletionOptions
    ) -> CompletionResponse:
        """发送聊天完成请求"""
        pass

    @abstractmethod
    async def get_usage(self, response: Response) -> Usage:
        """获取使用量"""
        pass

    @abstractmethod
    def map_error(self, error: Exception) -> ProviderError:
        """错误码映射"""
        pass

    @abstractmethod
    async def health_check(self) -> bool:
        """健康检查"""
        pass

    @property
    @abstractmethod
    def provider_name(self) -> str:
        """供应商名称"""
        pass

# ==================== Subapi 适配器 ====================
class SubapiAdapter(ProviderAdapter):
    """Subapi 适配器"""

    def __init__(self, config: SubapiConfig):
        self.client = SubapiClient(config)
        self.retry_config = RetryConfig(
            max_attempts=3,
            backoff_factor=2,
            retry_on_status=[429, 500, 502, 503, 504]
        )

    async def chat_completion(self, model, messages, options):
        # 1. 构建请求
        request = self.build_request(model, messages, options)

        # 2. 发送请求（带重试）
        response = await self.retry_with_backoff(
            lambda: self.client.post('/v1/chat/completions', request)
        )

        # 3. 转换响应
        return self.transform_response(response)

    def map_error(self, error: Exception) -> ProviderError:
        # Subapi 错误码 -> 统一错误码
        error_mapping = {
            'invalid_api_key': ProviderError.INVALID_KEY,
            'rate_limit_exceeded': ProviderError.RATE_LIMIT,
            'insufficient_quota': ProviderError.INSUFFICIENT_QUOTA,
            'model_not_found': ProviderError.MODEL_NOT_FOUND,
        }
        return error_mapping.get(error.code, ProviderError.UNKNOWN)

# ==================== 自研 Router Core 适配器 ====================
class RouterCoreAdapter(ProviderAdapter):
    """自研 Router Core 适配器"""

    def __init__(self, config: RouterCoreConfig):
        self.client = RouterCoreClient(config)

    async def chat_completion(self, model, messages, options):
        # 直接调用内部服务
        response = await self.client.chat_complete(
            model=model,
            messages=messages,
            **options.to_dict()
        )
        return self.transform_response(response)

    def map_error(self, error: Exception) -> ProviderError:
        # Router Core 错误码 -> 统一错误码
        return RouterCoreErrorMapper.map(error)
```

#### 2.2.2 适配器注册中心

```python
class AdapterRegistry:
    """适配器注册中心"""

    def __init__(self):
        self._adapters: Dict[str, ProviderAdapter] = {}
        self._fallbacks: Dict[str, str] = {}

    def register(self, provider: str, adapter: ProviderAdapter,
                 fallback: str = None):
        """注册适配器"""
        self._adapters[provider] = adapter
        if fallback:
            self._fallbacks[provider] = fallback

    def get(self, provider: str) -> ProviderAdapter:
        """获取适配器"""
        if provider not in self._adapters:
            raise AdapterNotFoundError(f"No adapter for {provider}")
        return self._adapters[provider]

    def get_with_fallback(self, provider: str) -> ProviderAdapter:
        """获取适配器（带降级）- 修复版"""
        adapter = self.get(provider)

        # 修复A-D-01: 使用异步心跳，避免同步阻塞
        # 从缓存获取健康状态，不实时调用
        health_status = self._health_cache.get(provider)

        if not health_status or not health_status.is_healthy:
            # 异步更新健康状态，不阻塞请求
            asyncio.create_task(self._update_health_async(provider))

            # 降级到备用
            if provider in self._fallbacks:
                fallback_adapter = self.get(self._fallbacks[provider])
                fallback_health = self._health_cache.get(self._fallbacks[provider])
                if fallback_health and fallback_health.is_healthy:
                    return fallback_adapter

        return adapter

    async def _update_health_async(self, provider: str):
        """异步更新健康状态"""
        try:
            adapter = self.get(provider)
            is_healthy = await adapter.health_check()
            self._health_cache[provider] = HealthStatus(
                is_healthy=is_healthy,
                checked_at=datetime.now()
            )
        except Exception as e:
            logger.error(f"健康检查失败: {provider}", e)
            self._health_cache[provider] = HealthStatus(is_healthy=False)

# 使用示例
registry = AdapterRegistry()
registry.register('openai', SubapiAdapter(subapi_config), fallback='azure')
registry.register('anthropic', SubapiAdapter(subapi_config))
registry.register('domestic', RouterCoreAdapter(router_config))
```

#### 2.2.3 契约测试

```python
import pytest

class TestProviderContract:
    """供应商适配器契约测试"""

    @pytest.mark.asyncio
    async def test_chat_completion_response_structure(self, adapter: ProviderAdapter):
        """测试响应结构"""
        response = await adapter.chat_completion(
            model='gpt-4',
            messages=[{'role': 'user', 'content': 'Hello'}]
        )

        # 验证必需字段
        assert response.id is not None
        assert response.model is not None
        assert response.choices is not None
        assert response.usage is not None
        assert response.usage.prompt_tokens >= 0
        assert response.usage.completion_tokens >= 0
        assert response.usage.total_tokens >= 0

    @pytest.mark.asyncio
    async def test_error_mapping(self, adapter: ProviderAdapter):
        """测试错误码映射"""
        # 测试各种错误情况
        error_cases = [
            (InvalidKeyError(), ProviderError.INVALID_KEY),
            (RateLimitError(), ProviderError.RATE_LIMIT),
            (QuotaExceededError(), ProviderError.INSUFFICIENT_QUOTA),
        ]

        for original, expected in error_cases:
            result = adapter.map_error(original)
            assert result == expected

    @pytest.mark.asyncio
    async def test_streaming_response(self, adapter: ProviderAdapter):
        """测试流式响应"""
        response = await adapter.chat_completion(
            model='gpt-4',
            messages=[{'role': 'user', 'content': 'Count to 5'}],
            stream=True
        )

        # 验证流式响应
        chunks = []
        async for chunk in response.stream():
            chunks.append(chunk)
            if len(chunks) >= 5:
                break

        assert len(chunks) > 0
        assert all(hasattr(c, 'delta') for c in chunks)
```

---

## 3. 数据一致性保证

### 3.1 当前问题

- 异步写入可能失败
- 进程崩溃可能导致数据丢失
- 分布式事务未处理

### 3.2 解决方案

#### 3.2.1 同步预扣 + 异步确认

```python
class BillingService:
    async def handle_request(self, request: LLMRequest) -> Response:
        # 1. 同步预扣额度（乐观锁）
        estimated_cost = self.estimate_cost(request)
        success = await self.reserve_balance(
            user_id=request.user_id,
            amount=estimated_cost,
            request_id=request.request_id
        )

        if not success:
            raise InsufficientBalanceError()

        # 2. 处理请求
        try:
            response = await self.router.route(request)

            # 3. 同步计算实际费用
            actual_cost = self.calculate_actual_cost(response)

            # 4. 同步扣减余额（补偿事务）
            await self.charge(
                user_id=request.user_id,
                amount=actual_cost,
                request_id=request.request_id,
                # 记录预扣信息用于对账
                reserved_amount=estimated_cost,
                final_amount=actual_cost
            )

            # 5. 记录使用量（异步，可重试）
            asyncio.create_task(
                self.record_usage_async(
                    user_id=request.user_id,
                    usage=response.usage,
                    request_id=request.request_id
                )
            )

            return response

        except Exception as e:
            # 6. 请求失败，释放预扣额度
            await self.release_reservation(
                user_id=request.user_id,
                amount=estimated_cost,
                request_id=request.request_id
            )
            raise e
```

#### 3.2.2 补偿事务队列

```python
class CompensationQueue:
    """补偿事务队列 - 修复版"""

    # 修复A-D-02: 增加最大重试时间和指数退避
    MAX_RETRY_COUNT = 5  # 增加到5次
    MAX_RETRY_SECONDS = 3600  # 最大重试时间1小时
    BASE_DELAY = 1  # 基础延迟1秒

    def __init__(self, redis: Redis, db: Database):
        self.redis = redis
        self.db = db

    async def enqueue_compensation(self, transaction: CompensationTransaction):
        """加入补偿队列"""
        await self.redis.lpush(
            'compensation_queue',
            json.dumps({
                'type': transaction.type,
                'data': transaction.data,
                'retry_count': 0,
                'created_at': datetime.now().isoformat(),
                'first_retry_at': None
            })
        )

    async def process_compensations(self):
        """处理补偿队列（后台任务）- 修复版"""
        while True:
            # 1. 获取待处理项
            item = await self.redis.lpop('compensation_queue')
            if not item:
                await asyncio.sleep(1)
                continue

            transaction = json.loads(item)

            try:
                # 2. 执行补偿
                await self.execute_compensation(transaction)

            except Exception as e:
                # 3. 指数退避重试逻辑
                retry_count = transaction['retry_count']
                created_at = datetime.fromisoformat(transaction['created_at'])
                elapsed = (datetime.now() - created_at).total_seconds()

                # 检查是否超过最大重试时间
                if elapsed > self.MAX_RETRY_SECONDS:
                    # 超过最大时间，告警人工处理
                    await self.alert_manual_intervention(transaction, e)
                    continue

                # 指数退避：1s, 2s, 4s, 8s, 16s
                if retry_count < self.MAX_RETRY_COUNT:
                    transaction['retry_count'] += 1
                    transaction['first_retry_at'] = datetime.now().isoformat()

                    delay = min(self.BASE_DELAY * (2 ** retry_count), 60)  # 最大延迟60秒
                    await asyncio.sleep(delay)
                    await self.redis.lpush('compensation_queue', json.dumps(transaction))
                else:
                    # 4. 超过重试次数，告警人工处理
                    await self.alert_manual_intervention(transaction, e)
```

#### 3.2.3 实时对账

```python
class RealTimeReconciliation:
    """实时对账"""

    async def verify_billing(self, request_id: str):
        """验证单笔计费"""
        # 1. 获取预扣记录
        reservation = await self.get_reservation(request_id)

        # 2. 获取实际扣费记录
        charge = await self.get_charge(request_id)

        # 3. 获取使用量记录
        usage = await self.get_usage(request_id)

        # 4. 验证一致性
        if reservation and charge:
            # 预扣 vs 实扣
            diff = abs(reservation.amount - charge.amount)
            # 修复A-D-03: 对账精度提高到0.001元
            if diff > 0.001:  # 允许0.1分误差
                await self.alert('billing_discrepancy', {
                    'request_id': request_id,
                    'reserved': reservation.amount,
                    'charged': charge.amount
                })

        if charge and usage:
            # 扣费 vs 使用量
            expected = self.calculate_cost(usage)
            if abs(charge.amount - expected) > expected * 0.001:
                await self.alert('usage_charge_mismatch', {
                    'request_id': request_id,
                    'usage': usage,
                    'charged': charge.amount,
                    'expected': expected
                })
```

---

## 4. 实施计划

### 4.1 任务分解

| 任务 | 负责人 | 截止 | 依赖 |
|------|--------|------|------|
| Router Core 目标调整 | 产品 | 立即 | - |
| Provider Adapter 抽象层 | 架构 | S0-M1 | - |
| 适配器注册中心 | 后端 | S0-M1 | - |
| 契约测试框架 | 测试 | S0-M2 | - |
| 同步预扣机制 | 后端 | S1前 | - |
| 补偿队列 | 后端 | S1前 | - |
| 实时对账 | 后端 | S1前 | - |

### 4.2 验证标准

- Router Core 60%接管率稳定运行（40%仅为中间检查点）
- 任意时刻可切换 subapi / 自研
- 计费数据0误差

---

**文档状态**：架构解决方案
**关联文档**：
- `llm_gateway_product_technical_blueprint_v1_2026-03-16.md`
- `s2_takeover_buffer_strategy_v1_2026-03-18.md`
