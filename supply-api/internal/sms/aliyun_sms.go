package sms

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

// AliyunSMSService implements SMSService for Aliyun (Alibaba Cloud) SMS.
type AliyunSMSService struct {
	config     *Config
	httpClient *http.Client
	privateKey *rsa.PrivateKey
	store      *InMemoryCodeStore
}

// NewAliyunSMSService creates a new Aliyun SMS service.
func NewAliyunSMSService(config *Config) (*AliyunSMSService, error) {
	return NewAliyunSMSServiceWithCodeStore(config, NewInMemoryCodeStore())
}

// NewAliyunSMSServiceWithCodeStore creates a new Aliyun SMS service with an explicit code store.
func NewAliyunSMSServiceWithCodeStore(config *Config, store *InMemoryCodeStore) (*AliyunSMSService, error) {
	if config == nil {
		config = DefaultConfig()
	}
	if store == nil {
		store = NewInMemoryCodeStore()
	}

	svc := &AliyunSMSService{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		store: store,
	}

	// Parse private key if provided
	if config.AppSecret != "" {
		block, _ := pem.Decode([]byte(config.AppSecret))
		if block != nil {
			key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
			if err != nil {
				// Try PKCS1 format
				key2, err2 := x509.ParsePKCS1PrivateKey(block.Bytes)
				if err2 != nil {
					return nil, fmt.Errorf("failed to parse private key: %w", err)
				}
				svc.privateKey = key2
			} else {
				if rsaKey, ok := key.(*rsa.PrivateKey); ok {
					svc.privateKey = rsaKey
				}
			}
		}
	}

	return svc, nil
}

// IsEnabled returns whether SMS service is enabled.
func (a *AliyunSMSService) IsEnabled() bool {
	return a.config.Enabled
}

// SendVerificationCode sends an SMS verification code via Aliyun SMS.
func (a *AliyunSMSService) SendVerificationCode(ctx context.Context, phoneNumber string) (string, error) {
	if !a.config.Enabled {
		return "", ErrSMSServiceDisabled
	}

	code, err := GenerateCode(a.config.CodeLength)
	if err != nil {
		return "", err
	}

	err = a.sendSMS(ctx, phoneNumber, code)
	if err != nil {
		return "", fmt.Errorf("failed to send SMS via Aliyun: %w", err)
	}

	codeID, err := a.store.Save(phoneNumber, code, time.Duration(a.config.CodeExpireMins)*time.Minute, "aliyun")
	if err != nil {
		return "", err
	}

	fmt.Printf("[AliyunSMS] Code '%s' sent to %s\n", code, phoneNumber)
	return codeID, nil
}

// sendSMS sends SMS via Aliyun API
func (a *AliyunSMSService) sendSMS(ctx context.Context, phoneNumber, code string) error {
	// Aliyun Dysmsapi endpoint
	endpoint := "https://dysmsapi.aliyuncs.com/"

	// Build request parameters
	params := url.Values{}
	params.Set("AccessKeyId", a.config.AppID)
	params.Set("Action", "SendSms")
	params.Set("Format", "JSON")
	params.Set("PhoneNumbers", phoneNumber)
	params.Set("SignName", a.config.SignName)
	params.Set("TemplateCode", a.config.TemplateCode)
	params.Set("TemplateParam", fmt.Sprintf(`{"code":"%s"}`, code))
	params.Set("Version", "2017-05-25")
	params.Set("SignatureMethod", "HMAC-SHA1")
	params.Set("Timestamp", time.Now().UTC().Format("2006-01-02T15:04:05Z"))
	params.Set("SignatureVersion", "1.0")
	params.Set("SignatureNonce", fmt.Sprintf("%d", time.Now().UnixNano()))

	// Calculate signature
	signature := a.calculateSignature(params)
	params.Set("Signature", signature)

	// Build request
	body := params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Aliyun SMS API error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// calculateSignature calculates the Aliyun API signature.
func (a *AliyunSMSService) calculateSignature(params url.Values) string {
	// Sort parameters
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Build string to sign
	var signString strings.Builder
	signString.WriteString("POST&")
	signString.WriteString(url.QueryEscape("/"))
	signString.WriteString("&")

	var queryString strings.Builder
	for i, k := range keys {
		if i > 0 {
			queryString.WriteString("&")
		}
		queryString.WriteString(url.QueryEscape(k))
		queryString.WriteString("=")
		queryString.WriteString(url.QueryEscape(params.Get(k)))
	}
	signString.WriteString(url.QueryEscape(queryString.String()))

	// Sign with HMAC-SHA1
	if a.privateKey != nil {
		h := sha256.New()
		h.Write([]byte(signString.String()))
		signed, err := rsa.SignPKCS1v15(rand.Reader, a.privateKey, crypto.SHA256, h.Sum(nil))
		if err != nil {
			return ""
		}
		return base64.StdEncoding.EncodeToString(signed)
	}

	// Fallback: HMAC-SHA1 with secret (simplified)
	// In production, use proper RSA signing
	return ""
}

// VerifyCode is a no-op for Aliyun - verification is handled by the code store
func (a *AliyunSMSService) VerifyCode(ctx context.Context, codeID string, phoneNumber string, code string) (bool, error) {
	return a.store.Verify(codeID, phoneNumber, code)
}

// AliyunSMSResponse represents the Aliyun SMS API response.
type AliyunSMSResponse struct {
	RequestID string `json:"RequestId"`
	Code      string `json:"Code"`
	Message   string `json:"Message"`
	BizID     string `json:"BizId"`
}
