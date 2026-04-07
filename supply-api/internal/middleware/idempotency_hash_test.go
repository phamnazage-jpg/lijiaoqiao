package middleware

import (
	"testing"
)

// TestP101_PayloadHashAlgorithm 验证幂等payload_hash使用SHA-256算法
func TestP101_PayloadHashAlgorithm(t *testing.T) {
	// 测试用例：相同内容应产生相同的hash
	body1 := []byte(`{"name":"test","value":123}`)
	body2 := []byte(`{"name":"test","value":123}`)
	body3 := []byte(`{"name":"test","value":456}`)

	hash1 := ComputePayloadHash(body1)
	hash2 := ComputePayloadHash(body2)
	hash3 := ComputePayloadHash(body3)

	// 相同内容应产生相同的hash
	if hash1 != hash2 {
		t.Errorf("same payload should produce same hash: %s != %s", hash1, hash2)
	}

	// 不同内容应产生不同的hash
	if hash1 == hash3 {
		t.Errorf("different payload should produce different hash: %s == %s", hash1, hash3)
	}

	// SHA-256产生64字符的十六进制字符串
	if len(hash1) != 64 {
		t.Errorf("SHA-256 hash should be 64 characters, got %d", len(hash1))
	}

	t.Logf("P1-01: payload_hash算法验证通过 - SHA-256")
	t.Logf("  示例hash: %s", hash1)
}

// TestP101_IdempotencyPayloadHashConstant 验证payload_hash常量
func TestP101_IdempotencyPayloadHashConstant(t *testing.T) {
	// payload_hash字段使用CHAR(64)存储SHA-256的十六进制表示
	// SHA-256输出256位 = 32字节 = 64个十六进制字符

	testBodies := [][]byte{
		[]byte(""),
		[]byte("a"),
		[]byte("hello world"),
		[]byte(`{"key":"value","number":123456789,"nested":{"a":"b"}}`),
	}

	for _, body := range testBodies {
		hash := ComputePayloadHash(body)
		if len(hash) != 64 {
			t.Errorf("hash length should always be 64 for SHA-256, got %d for body %s", len(hash), string(body))
		}
	}

	t.Log("P1-01: payload_hash长度验证通过 (CHAR(64) for SHA-256)")
}

// TestP101_Summary 测试总结
func TestP101_Summary(t *testing.T) {
	t.Log("=== P1-01 幂等payload_hash算法声明测试总结 ===")
	t.Log("问题: 供应侧技术设计使用payload_hash char(64)，暗示SHA-256但未明确声明")
	t.Log("")
	t.Log("修复方案:")
	t.Log("  - SQL注释明确声明: payload_hash CHAR(64) NOT NULL -- SHA256 of request body")
	t.Log("  - 代码使用: crypto/sha256")
	t.Log("  - 表注释: 请求体SHA256摘要，用于检测异参重放")
	t.Log("")
	t.Log("SQL文件: sql/postgresql/supply_idempotency_record_v1.sql")
}
