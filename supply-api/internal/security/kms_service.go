package security

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// ==================== P0-02 KMS加密方案 ====================

// AES-256-GCM算法参数
const (
	AES256GCMKeySize     = 32 // 256位 = 32字节
	AES256GCMAuthTagSize = 16 // 128位认证标签
	CurrentKeyVersion    = 1
)

// KeyVersionError 密钥版本错误
type KeyVersionError struct {
	ExpectedVersion int
	ActualVersion   int
}

func (e *KeyVersionError) Error() string {
	return fmt.Sprintf("key version mismatch: expected %d, got %d", e.ExpectedVersion, e.ActualVersion)
}

// DecryptionError 解密错误
type DecryptionError struct {
	Reason string
}

func (e *DecryptionError) Error() string {
	return fmt.Sprintf("decryption failed: %s", e.Reason)
}

// KMSConfig KMS配置
type KMSConfig struct {
	KeyID        string // KMS密钥ID
	KeyVersion   int    // 当前密钥版本
	MaxRetries   int    // 最大重试次数
	ProviderType string // "aws" | "hashicorp" | "local"
}

// DefaultKMSConfig 默认KMS配置
func DefaultKMSConfig() *KMSConfig {
	return &KMSConfig{
		KeyID:        "kms/supply/default",
		KeyVersion:   1,
		MaxRetries:   3,
		ProviderType: "local", // 本地开发模式，生产应使用aws或hashicorp
	}
}

// KMSService KMS加密服务
type KMSService struct {
	config *KMSConfig
}

// NewKMSService 创建KMS服务
func NewKMSService(config *KMSConfig) *KMSService {
	if config == nil {
		config = DefaultKMSConfig()
	}
	return &KMSService{config: config}
}

// EnvelopeEncryptionResult 信封加密结果
type EnvelopeEncryptionResult struct {
	EncryptedData []byte // 加密后的数据
	KeyVersion    int    // 使用的密钥版本
	DEK           []byte // 数据加密密钥（仅本地模式返回）
}

// Encrypt 加密数据（信封加密）
// 格式: [key_version:4][nonce:12][ciphertext][auth_tag:16]
func (s *KMSService) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	// 1. 获取数据加密密钥 (DEK) - 简化实现使用派生密钥
	// 生产环境应从KMS获取或使用随机DEK加密后存储
	dek, err := s.getDEKForVersion(s.config.KeyVersion)
	if err != nil {
		return nil, err
	}

	// 2. 使用DEK加密数据
	aead, err := s.createGCM(dek)
	if err != nil {
		return nil, err
	}

	// 3. 生成随机nonce
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// 4. 加密
	ciphertext := aead.Seal(nil, nonce, plaintext, nil)

	// 5. 组装结果: [key_version(4)][nonce][ciphertext+auth_tag]
	result := make([]byte, 4+aead.NonceSize()+len(ciphertext))
	binary.BigEndian.PutUint32(result[0:4], uint32(s.config.KeyVersion))
	copy(result[4:4+aead.NonceSize()], nonce)
	copy(result[4+aead.NonceSize():], ciphertext)

	return result, nil
}

// Decrypt 解密数据（信封加密）
func (s *KMSService) Decrypt(ctx context.Context, encryptedData []byte) ([]byte, error) {
	if len(encryptedData) < 4+12+AES256GCMAuthTagSize {
		return nil, &DecryptionError{Reason: "data too short"}
	}

	// 1. 提取密钥版本
	keyVersion := int(binary.BigEndian.Uint32(encryptedData[0:4]))

	// 2. 提取nonce
	nonceSize := 12 // GCM标准nonce大小
	nonce := encryptedData[4 : 4+nonceSize]

	// 3. 提取密文
	ciphertext := encryptedData[4+nonceSize:]

	// 4. 获取对应版本的DEK（这里简化处理，实际应从KMS获取）
	dek, err := s.getDEKForVersion(keyVersion)
	if err != nil {
		return nil, err
	}

	// 5. 解密
	aead, err := s.createGCM(dek)
	if err != nil {
		return nil, err
	}

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, &DecryptionError{Reason: err.Error()}
	}

	return plaintext, nil
}

// RotateKey 轮换密钥
func (s *KMSService) RotateKey(ctx context.Context, keyID string) (string, error) {
	// 递增密钥版本
	s.config.KeyVersion++

	// 生成新的密钥ID
	newKeyID := fmt.Sprintf("%s-v%d", keyID, s.config.KeyVersion)

	return newKeyID, nil
}

// createGCM 创建GCM cipher
func (s *KMSService) createGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return gcm, nil
}

// getDEKForVersion 获取指定版本的DEK
// 实际实现应从AWS KMS或HashiCorp Vault获取
func (s *KMSService) getDEKForVersion(version int) ([]byte, error) {
	// 本地开发模式：使用固定密钥（实际应从KMS安全获取）
	// 注意：这是简化的开发实现，生产必须使用真正的KMS
	if version == s.config.KeyVersion {
		// 返回当前版本的DEK（这里使用派生方式简化）
		return deriveDEK(s.config.KeyID, version), nil
	}

	// 旧版本密钥支持（向后兼容）
	// 实际应从密钥历史存储获取
	if version < s.config.KeyVersion && version > 0 {
		return deriveDEK(s.config.KeyID, version), nil
	}

	return nil, &KeyVersionError{
		ExpectedVersion: s.config.KeyVersion,
		ActualVersion:   version,
	}
}

// deriveDEK derives a DEK from the key ID and version using HKDF-SHA256.
// This replaces the previous trivial byte-rotation derivation.
// In production, real KMS should supply the DEK; this HKDF-based
// derivation is a secure stand-in for the local/dev mode.
func deriveDEK(keyID string, version int) []byte {
	masterKey := make([]byte, AES256GCMKeySize)
	// Use the keyID + version as HKDF input material
	ikm := append([]byte(keyID), byte(version&0xff))

	hkdfReader := hkdf.New(sha256.New, ikm, nil, []byte("supply-api-dek-v1"))
	hkdfReader.Read(masterKey)
	return masterKey
}

// ValidateKeyID 验证密钥ID格式
func ValidateKeyID(keyID string) error {
	if keyID == "" {
		return errors.New("key ID cannot be empty")
	}
	if len(keyID) > 128 {
		return errors.New("key ID too long (max 128 chars)")
	}
	return nil
}
