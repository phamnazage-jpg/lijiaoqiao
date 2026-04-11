package sms

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// TencentSMSService implements SMSService for Tencent Cloud SMS.
type TencentSMSService struct {
	config     *Config
	httpClient *http.Client
}

// NewTencentSMSService creates a new Tencent Cloud SMS service.
func NewTencentSMSService(config *Config) *TencentSMSService {
	if config == nil {
		config = DefaultConfig()
	}
	return &TencentSMSService{
		config: config,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// IsEnabled returns whether SMS service is enabled.
func (t *TencentSMSService) IsEnabled() bool {
	return t.config.Enabled
}

// SendVerificationCode sends an SMS verification code via Tencent Cloud.
func (t *TencentSMSService) SendVerificationCode(ctx context.Context, phoneNumber string) (string, error) {
	if !t.config.Enabled {
		return "", ErrSMSServiceDisabled
	}

	code, err := GenerateCode(t.config.CodeLength)
	if err != nil {
		return "", err
	}

	codeID := fmt.Sprintf("tencent-%d", time.Now().UnixNano())

	// Tencent Cloud SMS API request
	// Sign and send request
	err = t.sendSMS(ctx, phoneNumber, code)
	if err != nil {
		return "", fmt.Errorf("failed to send SMS via Tencent Cloud: %w", err)
	}

	fmt.Printf("[TencentSMS] Code '%s' sent to %s\n", code, phoneNumber)
	return codeID, nil
}

// sendSMS sends SMS via Tencent Cloud API
func (t *TencentSMSService) sendSMS(ctx context.Context, phoneNumber, code string) error {
	// Tencent Cloud SMS API endpoint
	endpoint := fmt.Sprintf("https://sms.tencentcloudapi.com/?Action=SendSms&Region=%s", t.config.Region)

	// Build request body
	params := url.Values{}
	params.Set("PhoneNumberSet.0", phoneNumber)
	params.Set("SmsSdkAppId", t.config.AppID)
	params.Set("TemplateId", t.config.TemplateCode)
	params.Set("TemplateParamSet.0", code)
	params.Set("SignName", t.config.SignName)

	// Sign request (simplified - in production use proper Tencent Auth)
	body := strings.NewReader(params.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-TC-Action", "SendSms")
	req.Header.Set("X-TC-Version", "2021-01-11")
	req.Header.Set("X-TC-Timestamp", fmt.Sprintf("%d", time.Now().Unix()))
	req.Header.Set("X-TC-Region", t.config.Region)

	// In production, add proper authentication headers
	// Use Tencent Cloud SDK for proper signing:
	// https://github.com/TencentCloud/tencentcloud-sdk-go-intl

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Tencent SMS API error: status=%d, body=%s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// VerifyCode is a no-op for Tencent - verification is handled by the code store
// In production, you would verify against your own code store or use Tencent's verification API
func (t *TencentSMSService) VerifyCode(ctx context.Context, codeID string, phoneNumber string, code string) (bool, error) {
	// This would typically verify against your own code storage
	// For Tencent, you'd store the code after sending and verify here
	return false, fmt.Errorf("TencentSMSService.VerifyCode not implemented - use InMemoryCodeStore")
}

// TencentSMSResponse represents the Tencent Cloud SMS API response.
type TencentSMSResponse struct {
	Response struct {
		RequestID string `json:"RequestId"`
		SendStatusSet []struct {
			SerialNo     string `json:"SerialNo"`
			PhoneNumber  string `json:"PhoneNumber"`
			CountryCode  string `json:"CountryCode"`
			InvokeID     string `json:"InvokeId"`
			Fee          int    `json:"Fee"`
			StatusCode   string `json:"StatusCode"`
			Code         string `json:"Code"`         // 腾讯云实际字段名
			StatusMessage string `json:"StatusMessage"`
			Message      string `json:"Message"`     // 腾讯云实际字段名
		} `json:"SendStatusSet"`
	} `json:"Response"`
}

// parseResponse parses Tencent Cloud SMS API response.
func parseTencentResponse(body []byte) (*TencentSMSResponse, error) {
	var resp TencentSMSResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
