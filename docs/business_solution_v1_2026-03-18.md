# 业务解决方案（P0问题修复）

> 版本：v1.0
> 日期：2026-03-18
> 目的：系统性解决评审发现的业务P0问题

---

## 1. 资金合规解决方案

### 1.1 当前问题

- 资金池可能需要支付牌照
- 资金沉淀处理不明确
- 税务合规未确认

### 1.2 解决方案

#### 1.2.1 资金托管模式

```
传统模式（有问题）：
用户 ──▶ 平台账户 ──▶ 供应方结算
              ↑
         资金沉淀

托管模式（推荐）：
用户 ──▶ 第三方支付 ──▶ 平台运营账户 ──▶ 供应方结算
           (Stripe)      (无沉淀)      (T+N结算)
```

```python
class PaymentService:
    """支付服务 - 修复B-D-01：支持多支付渠道"""

    # 支持的支付渠道
    CHANNELS = {
        'stripe': {'name': 'Stripe', 'regions': ['US', 'EU', 'APAC']},
        'alipay': {'name': '支付宝', 'regions': ['CN']},
        'wechat': {'name': '微信支付', 'regions': ['CN']},
        'bank': {'name': '银行转账', 'regions': ['CN', 'US']}
    }

    def __init__(self):
        # 初始化各渠道
        self.providers = {
            'stripe': StripePaymentProvider(),
            'alipay': AlipayProvider(),
            'wechat': WechatPayProvider(),
            'bank': BankTransferProvider()
        }

    async def create_payment(
        self,
        user_id: int,
        amount: float,
        currency: str,
        channel: str = None
    ) -> PaymentResult:
        """创建支付 - 自动选择渠道"""
        # 1. 自动选择最优渠道
        if not channel:
            channel = self.select_optimal_channel(user_id, currency)

        # 2. 获取渠道提供商
        provider = self.providers.get(channel)
        if not provider:
            raise PaymentChannelError(f"不支持的支付渠道: {channel}")

        # 3. 创建支付
        payment = await provider.create_checkout(
            amount=int(amount * 100),
            currency=currency,
            metadata={'user_id': user_id, 'type': 'top_up'}
        )

        return PaymentResult(
            channel=channel,
            payment_url=payment.url,
            payment_id=payment.id,
            expires_at=payment.expires_at
        )

    def select_optimal_channel(self, user_id: int, currency: str) -> str:
        """自动选择最优渠道"""
        # 根据用户位置和币种选择
        user = self.get_user(user_id)
        region = user.region

        # 优先使用用户地区支持的渠道
        for channel, config in self.CHANNELS.items():
            if region in config['regions']:
                return channel

        # 默认使用Stripe
        return 'stripe'
```

#### 1.2.2 结算T+N模式

```python
class SettlementService:
    """结算服务：T+N 结算"""

    # 根据供应方等级确定结算周期
    SETTLEMENT_CONFIG = {
        'new': {'days': 30, 'min_amount': 100},      # 新供应方
        'stable': {'days': 14, 'min_amount': 50},    # 稳定供应方
        'excellent': {'days': 7, 'min_amount': 0},  # 优质供应方
    }

    async def process_settlement(self, provider_id, period):
        """处理结算"""
        # 1. 获取结算周期配置
        provider = await self.get_provider(provider_id)
        config = self.SETTLEMENT_CONFIG[provider.tier]

        # 2. 计算结算金额
        pending_amount = await self.get_pending_amount(provider_id, period)

        # 3. 检查最低金额
        if pending_amount < config['min_amount']:
            return {'status': 'pending', 'reason': 'Below minimum'}

        # 4. 对账验证
        if not await self.verify_settlement(provider_id, period):
            raise SettlementError('Verification failed')

        # 5. 风控检查
        if await self.is_flagged(provider_id):
            await self.flag_for_manual_review(provider_id, pending_amount)
            return {'status': 'pending', 'reason': 'Manual review'}

        # 6. 执行结算
        settlement = await self.execute_settlement(
            provider_id=provider_id,
            amount=pending_amount,
            settlement_days=config['days']
        )

        return {'status': 'completed', 'settlement_id': settlement.id}
```

#### 1.2.3 税务合规

```python
class TaxService:
    """税务服务"""

    # 代扣代缴个人所得税率（示例）
    PERSONAL_INCOME_TAX_RATES = {
        0: 0.00,       # 0-3000
        3000: 0.03,   # 3001-12000
        12000: 0.10,  # 12001-25000
        25000: 0.20,  # 25001-35000
        35000: 0.25,  # 35001-55000
        55000: 0.30,  # 55001-80000
        80000: 0.35,  # 80001+
    }

    def calculate_tax(self, income):
        """计算个人所得税"""
        tax = 0
        remaining = income

        for threshold, rate in self.PERSONAL_INCOME_TAX_RATES.items():
            if remaining <= 0:
                break

            taxable = min(remaining, threshold + 3000 if threshold > 0 else threshold)
            tax += taxable * rate
            remaining -= taxable

        return tax

    def process_settlement_with_tax(self, provider_id, gross_amount):
        """结算并代扣税"""
        # 1. 计算税额
        tax = self.calculate_tax(gross_amount)

        # 2. 净收入
        net_amount = gross_amount - tax

        # 3. 生成税务凭证
        tax_record = {
            'provider_id': provider_id,
            'gross_amount': gross_amount,
            'tax': tax,
            'net_amount': net_amount,
            'tax_period': self.get_current_period(),
            'generated_at': datetime.now()
        }

        # 4. 存储并生成报表
        await self.save_tax_record(tax_record)

        return {
            'gross_amount': gross_amount,
            'tax': tax,
            'net_amount': net_amount,
            'tax_certificate_url': f'/api/v1/tax/{tax_record.id}'
        }
```

---

## 2. 计费精度解决方案

### 2.1 当前问题

- 浮点数精度问题
- 并发计费问题
- 退款处理问题

### 2.2 解决方案

#### 2.2.1 Decimal 精确计算

```python
from decimal import Decimal, ROUND_HALF_UP
from dataclasses import dataclass

@dataclass
class Money:
    amount: Decimal
    currency: str = 'USD'

    @classmethod
    def from_float(cls, amount: float, currency: str = 'USD') -> 'Money':
        # 从浮点数创建，避免精度问题
        return cls(amount=Decimal(str(amount)), currency=currency)

    def __add__(self, other: 'Money') -> 'Money':
        if self.currency != other.currency:
            raise CurrencyMismatchError()
        return Money(amount=self.amount + other.amount, currency=self.currency)

    def __mul__(self, multiplier: Decimal) -> 'Money':
        return Money(amount=self.amount * multiplier, currency=self.currency)

    def round(self, places: int = 2) -> 'Money':
        quantizer = Decimal(10) ** -places
        return Money(
            amount=self.amount.quantize(quantizer, rounding=ROUND_HALF_UP),
            currency=self.currency
        )
```

#### 2.2.2 高精度计费引擎

```python
class PrecisionBillingEngine:
    """高精度计费引擎"""

    def calculate_cost(
        self,
        model: str,
        usage: Usage,
        price: Price
    ) -> Money:
        # 1. 使用 Decimal 计算
        input_cost = Decimal(str(usage.prompt_tokens)) * Decimal(str(price.input_per_1k)) / 1000
        output_cost = Decimal(str(usage.completion_tokens)) * Decimal(str(price.output_per_1k)) / 1000

        # 2. 计算总价
        total = (input_cost + output_cost).quantize(
            Decimal('0.01'),
            rounding=ROUND_HALF_UP
        )

        # 3. 返回 Money 对象
        return Money(amount=total, currency=price.currency)

    def charge(
        self,
        user_id: int,
        cost: Money,
        transaction_id: str
    ) -> BillingResult:
        # 1. 分布式锁防止并发
        with self.distributed_lock(f'billing:{user_id}', timeout=5):
            # 2. 获取当前余额
            balance = self.get_balance(user_id)

            # 3. 扣款
            if balance < cost.amount:
                raise InsufficientBalanceError()

            new_balance = balance - cost.amount
            self.set_balance(user_id, new_balance)

            # 4. 记录流水
            self.record_transaction(
                user_id=user_id,
                amount=-cost.amount,
                transaction_id=transaction_id,
                balance_after=new_balance
            )

            return BillingResult(
                success=True,
                transaction_id=transaction_id,
                amount=cost.amount,
                balance_after=new_balance
            )
```

---

## 3. 结算风控解决方案

### 3.1 当前问题

- 虚假挂载风险
- 额度作弊风险
- 恶意退款风险

### 3.2 解决方案

#### 3.2.1 多维度风控

```python
class ProviderSettlementRiskController:
    """供应方结算风控"""

    RISK_INDICATORS = {
        'abnormal_usage_pattern': {
            'weight': 30,
            'check': lambda p: self.check_usage_pattern(p)
        },
        'low_verification_rate': {
            'weight': 20,
            'check': lambda p: self.check_verification_rate(p)
        },
        'high_refund_rate': {
            'weight': 25,
            'check': lambda p: self.check_refund_rate(p)
        },
        'new_account': {
            'weight': 15,
            'check': lambda p: self.check_account_age(p)
        },
        'inconsistent_income': {
            'weight': 10,
            'check': lambda p: self.check_income_consistency(p)
        }
    }

    async def evaluate_settlement_risk(self, provider_id, amount) -> RiskResult:
        """评估结算风险"""
        provider = await self.get_provider(provider_id)

        # 1. 计算风险分数
        risk_score = 0
        risk_details = []

        for indicator_name, config in self.RISK_INDICATORS.items():
            is_risky, detail = config['check'](provider)
            if is_risky:
                risk_score += config['weight']
                risk_details.append({
                    'indicator': indicator_name,
                    'detail': detail
                })

        # 2. 风险分级
        if risk_score >= 70:
            # 高风险：拒绝结算
            return RiskResult(
                level='HIGH',
                action='BLOCK',
                score=risk_score,
                details=risk_details,
                message='Settlement blocked due to high risk'
            )
        elif risk_score >= 40:
            # 中风险：人工审核
            await self.queue_for_review(provider_id, amount)
            return RiskResult(
                level='MEDIUM',
                action='REVIEW',
                score=risk_score,
                details=risk_details,
                message='Settlement queued for manual review'
            )
        else:
            # 低风险：正常结算
            return RiskResult(
                level='LOW',
                action='APPROVE',
                score=risk_score,
                details=risk_details,
                message='Settlement approved'
            )
```

#### 3.2.2 阶梯结算策略

```python
class TieredSettlement:
    """阶梯结算策略"""

    TIERS = {
        'new': {
            'settlement_days': 30,
            'max_daily_settlement': 1000,
            'require_verification': True,
            '保证金': 500
        },
        'stable': {
            'settlement_days': 14,
            'max_daily_settlement': 5000,
            'require_verification': False,
            '保证金': 0
        },
        'excellent': {
            'settlement_days': 7,
            'max_daily_settlement': None,  # 无限制
            'require_verification': False,
            '保证金': 0
        }
    }

    def calculate_settlement_limit(self, provider) -> dict:
        """计算结算限额"""
        tier = self.get_provider_tier(provider)

        # 基于历史结算金额动态调整
        history_avg = self.get_avg_daily_settlement(provider.id, 30)
        max_limit = self.TIERS[tier]['max_daily_settlement']

        # 限制为历史平均的2倍，防止突然大额
        if max_limit:
            effective_limit = min(max_limit, history_avg * 2)
        else:
            effective_limit = history_avg * 2

        return {
            'tier': tier,
            'settlement_days': self.TIERS[tier]['settlement_days'],
            'daily_limit': effective_limit,
            'require_verification': self.TIERS[tier]['require_verification']
        }
```

---

## 4. 实施计划

### 4.1 任务分解

| 任务 | 负责人 | 截止 | 依赖 |
|------|--------|------|------|
| 法务合规确认 | 产品 | 立即 | - |
| 支付SDK集成 | 后端 | S0-M2 | 法务确认 |
| 税务计算模块 | 后端 | S0-M2 | - |
| Decimal计费引擎 | 后端 | S1前 | - |
| 结算风控模块 | 风控 | S0-M1 | - |
| 阶梯结算策略 | 后端 | S0-M1 | - |

### 4.2 验证标准

- 资金由第三方托管
- 计费精度 100%
- 结算风控拦截率 >95%

---

**文档状态**：业务解决方案
**关联文档**：
- `business_model_profitability_design_v1_2026-03-18.md`
- `supply_side_product_design_v1_2026-03-18.md`
