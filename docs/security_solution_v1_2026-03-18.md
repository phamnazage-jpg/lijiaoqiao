# 安全解决方案（P0问题修复）

> 版本：v1.0
> 日期：2026-03-18
> 目的：系统性解决评审发现的安全P0问题

---

## 1. 计费数据防篡改机制

### 1.1 当前问题

- 只有 usage_records 表，缺乏完整性校验
- 无防篡改审计日志
- 无法追溯数据变更

### 1.2 解决方案

#### 1.2.1 双重记账设计

```python
# 双重记账：借方和贷方必须平衡
class DoubleEntryBilling:
    def record_billing(self, transaction: Transaction):
        # 1. 借方：用户账户余额
        self.debit(
            account_type='user_balance',
            account_id=transaction.user_id,
            amount=transaction.amount,
            currency=transaction.currency
        )

        # 2. 贷方：收入账户
        self.credit(
            account_type='revenue',
            account_id='platform_revenue',
            amount=transaction.amount,
            currency=transaction.currency
        )

        # 3. 验证平衡
        assert self.get_balance('user', transaction.user_id) + \
               self.get_balance('revenue', 'platform_revenue') == 0
```

#### 1.2.2 审计日志表

```sql
-- PostgreSQL 版本：计费审计日志表
CREATE TABLE IF NOT EXISTS billing_audit_log (
    id BIGSERIAL PRIMARY KEY,
    record_id BIGINT NOT NULL,
    table_name VARCHAR(50) NOT NULL,
    operation VARCHAR(20) NOT NULL,
    old_value JSONB,
    new_value JSONB,
    operator_id BIGINT NOT NULL,
    operator_ip INET,
    operator_role VARCHAR(50),
    request_id VARCHAR(64),
    record_hash CHAR(64) NOT NULL,
    previous_hash CHAR(64),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_billing_audit_log_record_id
    ON billing_audit_log (record_id);
CREATE INDEX IF NOT EXISTS idx_billing_audit_log_operator_id
    ON billing_audit_log (operator_id);
CREATE INDEX IF NOT EXISTS idx_billing_audit_log_created_at
    ON billing_audit_log (created_at DESC);
CREATE INDEX IF NOT EXISTS idx_billing_audit_log_request_id
    ON billing_audit_log (request_id);

-- PostgreSQL 触发器：自动记录变更（示例）
CREATE OR REPLACE FUNCTION fn_audit_supply_usage_update()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    v_prev_hash CHAR(64);
BEGIN
    SELECT record_hash
      INTO v_prev_hash
      FROM billing_audit_log
     WHERE record_id = OLD.id
     ORDER BY id DESC
     LIMIT 1;

    INSERT INTO billing_audit_log (
        record_id,
        table_name,
        operation,
        old_value,
        new_value,
        operator_id,
        operator_ip,
        operator_role,
        request_id,
        record_hash,
        previous_hash
    )
    VALUES (
        OLD.id,
        'supply_usage_records',
        'UPDATE',
        to_jsonb(OLD),
        to_jsonb(NEW),
        COALESCE(NULLIF(current_setting('app.operator_id', true), ''), '0')::BIGINT,
        NULLIF(current_setting('app.operator_ip', true), '')::INET,
        NULLIF(current_setting('app.operator_role', true), ''),
        NULLIF(current_setting('app.request_id', true), ''),
        encode(digest(to_jsonb(NEW)::text, 'sha256'), 'hex'),
        v_prev_hash
    );

    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_usage_before_update ON supply_usage_records;
CREATE TRIGGER trg_usage_before_update
BEFORE UPDATE ON supply_usage_records
FOR EACH ROW
EXECUTE FUNCTION fn_audit_supply_usage_update();
```

#### 1.2.3 实时对账机制

```python
class BillingReconciliation:
    def hourly_reconciliation(self):
        """小时级对账"""
        # 1. 获取计费记录
        billing_records = self.get_billing_records(
            start_time=self.hour_ago,
            end_time=datetime.now()
        )

        # 2. 获取用户消费记录
        usage_records = self.get_usage_records(
            start_time=self.hour_ago,
            end_time=datetime.now()
        )

        # 3. 比对
        discrepancies = []
        for billing, usage in zip(billing_records, usage_records):
            if not self.is_match(billing, usage):
                discrepancies.append({
                    'billing_id': billing.id,
                    'usage_id': usage.id,
                    'difference': billing.amount - usage.amount
                })

        # 4. 告警
        if discrepancies:
            self.send_alert('billing_discrepancy', discrepancies)

    def real_time_verification(self):
        """实时验证（请求级别）"""
        # 每个请求完成后立即验证
        request = self.get_current_request()
        expected_cost = self.calculate_cost(request.usage)
        actual_cost = self.get_billing_record(request.id).amount

        # 允许0.1%误差
        if abs(expected_cost - actual_cost) > expected_cost * 0.001:
            raise BillingAccuracyError(f"计费差异: {expected_cost} vs {actual_cost}")
```

---

## 2. 跨租户隔离强化

### 2.1 当前问题

- team_id 和 organization_id 字段存在
- 但缺乏强制验证和行级安全

### 2.2 解决方案

#### 2.2.1 强制租户上下文验证

```python
class TenantContextMiddleware:
    def process_request(self, request):
        # 1. 从Token提取租户ID
        tenant_id = self.extract_tenant_id(request.token)

        # 2. 从URL/Header强制验证
        if request.tenant_id and request.tenant_id != tenant_id:
            raise TenantMismatchError()

        # 3. 强制设置租户上下文
        request.tenant_id = tenant_id

        # 4. 存储到请求上下文
        self.set_context('tenant_id', tenant_id)
```

#### 2.2.2 数据库行级安全（RLS）

```sql
-- 启用行级安全
ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;

-- 创建策略：用户只能访问自己的Key
CREATE POLICY api_keys_tenant_isolation
ON api_keys
FOR ALL
USING (tenant_id = current_setting('app.tenant_id')::BIGINT);

-- 对所有敏感表启用RLS
ALTER TABLE billing_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE usage_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE team_members ENABLE ROW LEVEL SECURITY;
```

#### 2.2.3 敏感操作二次验证

```python
class SensitiveOperationGuard:
    # 需要二次验证的操作
    SENSITIVE_ACTIONS = [
        'billing.write',      # 写账单
        'admin.tenant_write', # 租户管理
        'provider.withdraw',  # 供应方提现
    ]

    def verify(self, user_id, action, context):
        if action not in self.SENSITIVE_ACTIONS:
            return True

        # 1. 检查用户权限级别
        user = self.get_user(user_id)
        if user.role == 'admin':
            return True

        # 2. 检查是否需要二次验证
        if self.requires_mfa(action, context):
            # 发送验证码
            self.send_verification_code(user)
            return False

        # 3. 记录审计日志
        self.audit_log(user_id, action, context)

        return True
```

---

## 3. 密钥轮换机制

### 3.1 当前问题

- API Key 无失效机制
- 无法强制轮换
- 无生命周期管理

### 3.2 解决方案

#### 3.2.1 密钥生命周期管理

```python
class APIKeyLifecycle:
    # 配置
    KEY_EXPIRY_DAYS = 90           # 有效期90天
    WARNING_DAYS = 14               # 提前14天提醒
    GRACE_PERIOD_DAYS = 7          # 宽限期7天
    MAX_KEYS_PER_USER = 10         # 每个用户最多10个Key

    def generate_key(self, user_id, description) -> APIKey:
        # 1. 检查Key数量限制
        current_keys = self.count_user_keys(user_id)
        if current_keys >= self.MAX_KEYS_PER_USER:
            raise MaxKeysExceededError()

        # 2. 生成Key
        key = self._generate_key_string()

        # 3. 存储Key信息
        api_key = APIKey(
            key_hash=self.hash(key),
            key_prefix=key[:12],  # 显示前缀
            user_id=user_id,
            description=description,
            expires_at=datetime.now() + timedelta(days=self.KEY_EXPIRY_DAYS),
            created_at=datetime.now(),
            status='active',
            version=1
        )

        # 4. 保存到数据库
        self.save(api_key)

        return api_key

    def is_key_valid(self, key: APIKey) -> ValidationResult:
        # 1. 检查状态
        if key.status == 'disabled':
            return ValidationResult(False, 'Key is disabled')

        if key.status == 'expired':
            return ValidationResult(False, 'Key is expired')

        # 2. 检查是否过期
        if key.expires_at and key.expires_at < datetime.now():
            # 检查是否在宽限期
            if key.expires_at > datetime.now() - timedelta(days=self.GRACE_PERIOD_DAYS):
                # 在宽限期，提醒但不拒绝
                return ValidationResult(True, 'Key expiring soon', warning=True)
            return ValidationResult(False, 'Key expired')

        # 3. 检查是否需要轮换提醒
        days_until_expiry = (key.expires_at - datetime.now()).days
        if days_until_expiry <= self.WARNING_DAYS:
            # 异步通知用户
            self.notify_key_expiring(key, days_until_expiry)

        return ValidationResult(True, 'Valid')
```

#### 3.2.2 密钥泄露应急处理

```python
class KeyCompromiseHandler:
    def report_compromised(self, key_id, reporter_id):
        """报告Key泄露"""
        # 1. 立即禁用Key
        key = self.get_key(key_id)
        key.status = 'compromised'
        key.disabled_at = datetime.now()
        key.disabled_by = reporter_id
        self.save(key)

        # 2. 通知用户
        user = self.get_user(key.user_id)
        self.send_notification(user, 'key_compromised', {
            'key_id': key_id,
            'reported_at': datetime.now()
        })

        # 3. 记录审计日志
        self.audit_log('key_compromised', {
            'key_id': key_id,
            'reported_by': reporter_id,
            'action': 'disabled'
        })

        # 4. 自动创建新Key（可选）
        new_key = self.generate_key(key.user_id, 'Auto-generated replacement')
        return new_key

    def rotate_key(self, key_id):
        """主动轮换Key"""
        old_key = self.get_key(key_id)

        # 1. 创建新Key
        new_key = self.generate_key(
            old_key.user_id,
            f"Rotation of {old_key.description}"
        )

        # 2. 标记旧Key为轮换
        old_key.status = 'rotated'
        old_key.rotated_at = datetime.now()
        old_key.replaced_by = new_key.id
        self.save(old_key)

        return new_key
```

---

## 4. 激活码安全强化

### 4.1 当前问题

- 6位随机数entropy不足
- MD5校验和可碰撞

### 4.2 解决方案

```python
import secrets
import hashlib
import hmac

class SecureActivationCode:
    def generate(self, user_id: int, expiry_days: int) -> str:
        # 1. 使用 crypto.random 替代 random
        # 16字节 = 128位 entropy
        random_bytes = secrets.token_bytes(16)
        random_hex = random_bytes.hex()

        # 2. 使用 HMAC-SHA256 替代 MD5
        expiry = datetime.now() + timedelta(days=expiry_days)
        expiry_str = expiry.strftime("%Y%m%d")

        # 3. 构建原始字符串
        raw = f"lgw-act-{user_id}-{expiry_str}-{random_hex}"

        # 4. HMAC 签名（使用应用密钥）
        signature = hmac.new(
            self.secret_key.encode(),
            raw.encode(),
            hashlib.sha256
        ).hexdigest()[:16]

        return f"{raw}-{signature}"

    def verify(self, code: str) -> VerificationResult:
        parts = code.split('-')
        if len(parts) != 6:
            return VerificationResult(False, 'Invalid format')

        # 1. 解析各部分
        _, _, user_id, expiry_str, random_hex, signature = parts

        # 2. 验证签名
        raw = f"lgw-act-{user_id}-{expiry_str}-{random_hex}"
        expected_signature = hmac.new(
            self.secret_key.encode(),
            raw.encode(),
            hashlib.sha256
        ).hexdigest()[:16]

        if not hmac.compare_digest(signature, expected_signature):
            return VerificationResult(False, 'Invalid signature')

        # 3. 验证过期
        expiry = datetime.strptime(expiry_str, "%Y%m%d")
        if expiry < datetime.now():
            return VerificationResult(False, 'Expired')

        return VerificationResult(True, 'Valid', user_id=int(user_id))
```

---

## 4. DDoS防护机制

### 4.1 防护层级

```python
class DDoSProtection:
    """DDoS防护 - 修复S-D-01"""

    # 三层防护
    TIERS = [
        {'name': 'L4', 'layer': 'tcp', 'method': 'syn_cookie'},
        {'name': 'L7', 'layer': 'http', 'method': 'rate_limit'},
        {'name': 'APP', 'layer': 'application', 'method': 'challenge'}
    ]

    # 限流配置
    RATE_LIMITS = {
        'global': {'requests': 100000, 'window': 60},
        'per_ip': {'requests': 1000, 'window': 60},
        'per_token': {'requests': 100, 'window': 60},
        'burst': {'requests': 50, 'window': 1}
    }

    # IP黑名单
    def check_ip_blacklist(self, ip: str) -> bool:
        """检查IP是否在黑名单"""
        return self.redis.sismember('ddos:blacklist', ip)

    def add_to_blacklist(self, ip: str, reason: str, duration: int = 3600):
        """加入黑名单"""
        self.redis.sadd('ddos:blacklist', ip)
        self.redis.expire('ddos:blacklist', duration)
        # 记录原因
        self.redis.hset('ddos:blacklist:reasons', ip, json.dumps({
            'reason': reason,
            'added_at': datetime.now().isoformat()
        }))
```

### 4.2 攻击检测

```python
class AttackDetector:
    """攻击检测"""

    # 检测规则
    RULES = {
        'syn_flood': {'threshold': 1000, 'window': 10, 'action': 'block'},
        'http_flood': {'threshold': 500, 'window': 60, 'action': 'rate_limit'},
        'slowloris': {'threshold': 50, 'window': 60, 'action': 'block'},
        'credential_stuffing': {'threshold': 100, 'window': 60, 'action': 'challenge'}
    }

    async def detect(self, metrics: AttackMetrics) -> DetectionResult:
        """检测攻击"""
        for rule_name, rule in self.RULES.items():
            if metrics.exceeds_threshold(rule):
                return DetectionResult(
                    attack=True,
                    rule=rule_name,
                    action=rule['action'],
                    severity='HIGH' if rule['action'] == 'block' else 'MEDIUM'
                )
        return DetectionResult(attack=False)
```

---

## 5. 日志脱敏规则

### 5.1 脱敏字段定义

```python
class LogDesensitization:
    """日志脱敏 - 修复S-D-02"""

    # 脱敏规则
    RULES = {
        'api_key': {
            'pattern': r'(sk-[a-zA-Z0-9]{20,})',
            'replacement': r'sk-***',
            'level': 'SENSITIVE'
        },
        'password': {
            'pattern': r'(password["\']?\s*[:=]\s*["\']?)([^"\']+)',
            'replacement': r'\1***',
            'level': 'SENSITIVE'
        },
        'email': {
            'pattern': r'([a-zA-Z0-9._%+-]+)@([a-zA-Z0-9.-]+\.[a-zA-Z]{2,})',
            'replacement': r'\1***@\2',
            'level': 'PII'
        },
        'phone': {
            'pattern': r'(1[3-9]\d)(\d{4})(\d{4})',
            'replacement': r'\1****\3',
            'level': 'PII'
        },
        'ip_address': {
            'pattern': r'(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})',
            'replacement': r'\1 (masked)',
            'level': 'NETWORK'
        },
        'credit_card': {
            'pattern': r'(\d{4}[\s-]?\d{4}[\s-]?\d{4}[\s-]?\d{4})',
            'replacement': r'****-****-****-\4',
            'level': 'SENSITIVE'
        }
    }

    def desensitize(self, log: dict) -> dict:
        """脱敏处理"""
        import re
        result = {}
        for key, value in log.items():
            if isinstance(value, str):
                result[key] = self._desensitize_value(value)
            else:
                result[key] = value
        return result
```

### 5.2 日志级别

```python
class LogLevel:
    """日志级别"""

    LEVELS = {
        'DEBUG': {'mask': False, 'retention_days': 7},
        'INFO': {'mask': False, 'retention_days': 30},
        'WARNING': {'mask': False, 'retention_days': 90},
        'ERROR': {'mask': False, 'retention_days': 365},
        'SENSITIVE': {'mask': True, 'retention_days': 365}  # 敏感日志必须脱敏
    }

    def should_mask(self, level: str) -> bool:
        """是否需要脱敏"""
        return self.LEVELS.get(level, {}).get('mask', False)
```

---

## 6. 密钥定期轮换

### 6.1 定期轮换策略

```python
class KeyRotationScheduler:
    """密钥定期轮换 - 修复S-D-03"""

    # 轮换配置
    ROTATION_CONFIG = {
        'api_key': {'days': 90, 'warning_days': 14},
        'internal_key': {'days': 30, 'warning_days': 7},
        'provider_key': {'days': 60, 'warning_days': 10}
    }

    async def schedule_rotation(self):
        """调度轮换"""
        while True:
            # 1. 查找需要轮换的Key
            keys_due = await self.find_keys_due_for_rotation()

            # 2. 发送提醒
            for key in keys_due:
                await self.send_rotation_warning(key)

            # 3. 自动轮换（超过宽限期）
            keys_expired = await self.find_expired_keys()
            for key in keys_expired:
                await self.auto_rotate(key)

            await asyncio.sleep(3600)  # 每小时检查

    async def auto_rotate(self, key: APIKey):
        """自动轮换"""
        # 1. 创建新Key
        new_key = await self.generate_key(key.user_id, key.description)

        # 2. 标记旧Key
        key.status = 'rotating'
        key.rotated_at = datetime.now()
        key.replaced_by = new_key.id

        # 3. 通知用户
        await self.notify_user(key.user_id, {
            'type': 'key_rotated',
            'old_key_id': key.id,
            'new_key': new_key.key_prefix + '***'
        })
```

---

## 7. 实施计划

### 7.1 优先级

| 任务 | 负责人 | 截止 | 依赖 |
|------|--------|------|------|
| 计费防篡改机制 | 后端 | S1前 | - |
| 跨租户隔离强化 | 架构 | S1前 | - |
| 密钥轮换机制 | 后端 | S0-M1 | - |
| 激活码安全强化 | 后端 | S0-M1 | - |
| DDoS防护机制 | 安全 | S0-M2 | - |
| 日志脱敏规则 | 后端 | S0-M1 | - |
| 密钥定期轮换 | 后端 | S0-M2 | - |

### 7.2 验证标准

- 所有计费操作都有审计日志
- 跨租户访问被强制拦截
- Key可以正常轮换和失效
- 激活码无法伪造
- DDoS攻击可被检测和阻断
- 敏感日志自动脱敏

---

**文档状态**：安全解决方案（修复版）
**关联文档**：
- `security_api_key_vulnerability_analysis_v1_2026-03-18.md`
- `supply_detailed_design_v1_2026-03-18.md`
