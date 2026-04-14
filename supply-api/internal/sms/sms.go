// Package sms provides SMS verification code services for Tencent Cloud and Aliyun.
package sms

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"sync"
	"time"
)

// ProviderType represents the SMS service provider type.
type ProviderType string

const (
	ProviderTencent ProviderType = "tencent" // 腾讯云
	ProviderAliyun  ProviderType = "aliyun"  // 阿里云
)

// Config holds SMS service configuration.
type Config struct {
	Provider       ProviderType `json:"provider" yaml:"provider"`
	Enabled        bool         `json:"enabled" yaml:"enabled"`
	AppID          string       `json:"app_id" yaml:"app_id"`                     // 腾讯云: SDK AppID; 阿里云: AccessKey ID
	AppSecret      string       `json:"app_secret" yaml:"app_secret"`             // 腾讯云: AppKey; 阿里云: AccessKey Secret
	SignName       string       `json:"sign_name" yaml:"sign_name"`               // 短信签名
	TemplateCode   string       `json:"template_code" yaml:"template_code"`       // 短信模板CODE
	Region         string       `json:"region" yaml:"region"`                     // 区域 (腾讯云用)
	Endpoint       string       `json:"endpoint" yaml:"endpoint"`                 // 自定义 endpoint (可选)
	CodeLength     int          `json:"code_length" yaml:"code_length"`           // 验证码长度，默认6位
	CodeExpireMins int          `json:"code_expire_mins" yaml:"code_expire_mins"` // 验证码有效期(分钟)，默认5分钟
}

// DefaultConfig returns a default SMS configuration.
func DefaultConfig() *Config {
	return &Config{
		Enabled:        false, // 默认关闭，使用硬编码验证码
		Provider:       ProviderTencent,
		CodeLength:     6,
		CodeExpireMins: 5,
		Region:         "ap-guangzhou",
	}
}

// SMSService defines the interface for SMS verification code services.
type SMSService interface {
	// SendVerificationCode sends an SMS verification code to the given phone number.
	// Returns the verification code ID (used for later verification) and error.
	SendVerificationCode(ctx context.Context, phoneNumber string) (string, error)

	// VerifyCode verifies if the provided code matches the one sent to the phone.
	// Returns true if valid, false otherwise.
	VerifyCode(ctx context.Context, codeID string, phoneNumber string, code string) (bool, error)

	// IsEnabled returns whether SMS service is enabled.
	IsEnabled() bool
}

// InMemoryCodeStore stores verification codes in memory (for development/testing).
type InMemoryCodeStore struct {
	mu    sync.Mutex
	codes map[string]*codeEntry
}

type codeEntry struct {
	Code      string
	Phone     string
	ExpiresAt time.Time
}

// NewInMemoryCodeStore creates a new in-memory code store.
func NewInMemoryCodeStore() *InMemoryCodeStore {
	return &InMemoryCodeStore{
		codes: make(map[string]*codeEntry),
	}
}

// GenerateCode generates a random numeric code of specified length.
func GenerateCode(length int) (string, error) {
	if length <= 0 {
		length = 6
	}
	code := ""
	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", fmt.Errorf("failed to generate random number: %w", err)
		}
		code += fmt.Sprintf("%d", n.Int64())
	}
	return code, nil
}

// MockSMSService is a mock SMS service for development/testing.
type MockSMSService struct {
	store  *InMemoryCodeStore
	config *Config
}

// NewMockSMSService creates a new mock SMS service.
func NewMockSMSService(config *Config) *MockSMSService {
	if config == nil {
		config = DefaultConfig()
	}
	return &MockSMSService{
		store:  NewInMemoryCodeStore(),
		config: config,
	}
}

// IsEnabled returns true if the SMS service is enabled.
func (m *MockSMSService) IsEnabled() bool {
	return m.config.Enabled
}

// ErrSMSServiceDisabled indicates SMS service is disabled
var ErrSMSServiceDisabled = errors.New("SMS service is disabled")
var ErrCodeStoreNotConfigured = errors.New("SMS code store is not configured")

// Save stores a verification code and returns the generated code ID.
func (s *InMemoryCodeStore) Save(phoneNumber, code string, ttl time.Duration, prefix string) (string, error) {
	if s == nil {
		return "", ErrCodeStoreNotConfigured
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if prefix == "" {
		prefix = "code"
	}

	codeID := fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())

	s.mu.Lock()
	defer s.mu.Unlock()
	s.codes[codeID] = &codeEntry{
		Code:      code,
		Phone:     phoneNumber,
		ExpiresAt: time.Now().Add(ttl),
	}
	return codeID, nil
}

// Verify checks a code by ID and enforces one-time usage.
func (s *InMemoryCodeStore) Verify(codeID, phoneNumber, code string) (bool, error) {
	if s == nil {
		return false, ErrCodeStoreNotConfigured
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.codes[codeID]
	if !ok {
		return false, nil
	}
	if time.Now().After(entry.ExpiresAt) {
		delete(s.codes, codeID)
		return false, nil
	}
	if entry.Phone != phoneNumber || entry.Code != code {
		return false, nil
	}

	delete(s.codes, codeID)
	return true, nil
}

// SendVerificationCode generates and "sends" a verification code.
func (m *MockSMSService) SendVerificationCode(ctx context.Context, phoneNumber string) (string, error) {
	if !m.config.Enabled {
		// 安全修复: 禁用时返回错误，而非返回mock code
		return "", ErrSMSServiceDisabled
	}

	code, err := GenerateCode(m.config.CodeLength)
	if err != nil {
		return "", err
	}

	codeID, err := m.store.Save(phoneNumber, code, time.Duration(m.config.CodeExpireMins)*time.Minute, "mock")
	if err != nil {
		return "", err
	}

	// In a real implementation, this would send the SMS via HTTP API
	fmt.Printf("[SMS Mock] Sending code '%s' to %s (codeID: %s)\n", code, phoneNumber, codeID)
	return codeID, nil
}

// VerifyCode verifies the provided code.
func (m *MockSMSService) VerifyCode(ctx context.Context, codeID string, phoneNumber string, code string) (bool, error) {
	if !m.config.Enabled {
		// 安全修复: 禁用时拒绝验证，返回错误
		return false, ErrSMSServiceDisabled
	}

	return m.store.Verify(codeID, phoneNumber, code)
}
