package alert

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"lijiaoqiao/gateway/internal/config"
)

// AlertType 告警类型
type AlertType string

const (
	AlertBudgetExceeded     AlertType = "budget_exceeded"
	AlertRateLimitExceeded AlertType = "rate_limit_exceeded"
	AlertProviderFailure   AlertType = "provider_failure"
	AlertHighErrorRate     AlertType = "high_error_rate"
	AlertLatencySpike      AlertType = "latency_spike"
	AlertManualIntervention AlertType = "manual_intervention"
)

// Alert 告警
type Alert struct {
	Type      AlertType
	Title     string
	Message   string
	Severity  string // "info", "warning", "error", "critical"
	TenantID  int64
	RequestID string
	Metadata  map[string]interface{}
	Timestamp time.Time
}

// Sender 告警发送器接口
type Sender interface {
	Send(ctx context.Context, alert *Alert) error
}

// Manager 告警管理器
type Manager struct {
	senders []Sender
}

// NewManager 创建告警管理器
func NewManager(cfg *config.AlertConfig) (*Manager, error) {
	m := &Manager{
		senders: make([]Sender, 0),
	}

	// 添加邮件发送器
	if cfg.Email.Enabled {
		m.senders = append(m.senders, NewEmailSender(&cfg.Email))
	}

	// 添加钉钉发送器
	if cfg.DingTalk.Enabled {
		sender, err := NewDingTalkSender(cfg.DingTalk.WebHook, cfg.DingTalk.Secret)
		if err != nil {
			return nil, fmt.Errorf("failed to create DingTalk sender: %w", err)
		}
		m.senders = append(m.senders, sender)
	}

	// 添加飞书发送器
	if cfg.Feishu.Enabled {
		sender, err := NewFeishuSender(cfg.Feishu.WebHook, cfg.Feishu.Secret)
		if err != nil {
			return nil, fmt.Errorf("failed to create Feishu sender: %w", err)
		}
		m.senders = append(m.senders, sender)
	}

	return m, nil
}

// Send 发送告警
func (m *Manager) Send(ctx context.Context, alert *Alert) error {
	if len(m.senders) == 0 {
		return fmt.Errorf("no alert sender configured")
	}

	var lastErr error
	for _, sender := range m.senders {
		if err := sender.Send(ctx, alert); err != nil {
			lastErr = err
			// 继续尝试其他发送器
			continue
		}
	}

	return lastErr
}

// SendBudgetAlert 发送预算告警
func (m *Manager) SendBudgetAlert(ctx context.Context, tenantID int64, current, limit float64) error {
	return m.Send(ctx, &Alert{
		Type:     AlertBudgetExceeded,
		Title:    "Budget Alert",
		Message:  fmt.Sprintf("Tenant %d exceeded budget: current=%.2f, limit=%.2f", tenantID, current, limit),
		Severity: "warning",
		TenantID: tenantID,
		Metadata: map[string]interface{}{
			"current_usage": current,
			"limit":        limit,
		},
		Timestamp: time.Now(),
	})
}

// SendProviderFailureAlert 发送Provider故障告警
func (m *Manager) SendProviderFailureAlert(ctx context.Context, provider string, err error) error {
	return m.Send(ctx, &Alert{
		Type:     AlertProviderFailure,
		Title:    "Provider Failure",
		Message:  fmt.Sprintf("Provider %s failed: %v", provider, err),
		Severity: "error",
		Metadata: map[string]interface{}{
			"provider": provider,
			"error":    err.Error(),
		},
		Timestamp: time.Now(),
	})
}

// EmailSender 邮件发送器
type EmailSender struct {
	cfg *config.EmailConfig
}

// NewEmailSender 创建邮件发送器
func NewEmailSender(cfg *config.EmailConfig) *EmailSender {
	return &EmailSender{cfg: cfg}
}

func (s *EmailSender) Send(ctx context.Context, alert *Alert) error {
	// 构建邮件内容
	subject := fmt.Sprintf("[%s] %s - %s", strings.ToUpper(alert.Severity), alert.Type, alert.Title)

	body := fmt.Sprintf(`
告警类型: %s
严重程度: %s
时间: %s
消息: %s
`, alert.Type, alert.Severity, alert.Timestamp.Format(time.RFC3339), alert.Message)

	if alert.TenantID > 0 {
		body += fmt.Sprintf("\n租户ID: %d", alert.TenantID)
	}

	// 构建邮件
	msg := fmt.Sprintf("From: %s\r\n"+
		"To: %s\r\n"+
		"Subject: %s\r\n"+
		"Content-Type: text/plain; charset=UTF-8\r\n"+
		"\r\n"+
		"%s",
		s.cfg.From,
		strings.Join(s.cfg.To, ","),
		subject,
		body)

	// 发送邮件
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	auth := smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	err := smtp.SendMail(addr, auth, s.cfg.From, s.cfg.To, []byte(msg))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

// DingTalkSender 钉钉发送器
type DingTalkSender struct {
	webHook string
	secret  string
	client  *http.Client
}

// NewDingTalkSender 创建钉钉发送器
func NewDingTalkSender(webHook, secret string) (*DingTalkSender, error) {
	return &DingTalkSender{
		webHook: webHook,
		secret:  secret,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (s *DingTalkSender) Send(ctx context.Context, alert *Alert) error {
	// 获取签名
	timestamp, sign := s.generateSign()

	// 构建请求URL
	url := fmt.Sprintf("%s&timestamp=%d&sign=%s", s.webHook, timestamp, sign)

	// 构建消息
	msg := map[string]interface{}{
		"msgtype": "markdown",
		"markdown": map[string]string{
			"title": fmt.Sprintf("[%s] %s", strings.ToUpper(alert.Severity), alert.Title),
			"text": fmt.Sprintf(`### [%s] %s

**类型**: %s
**严重程度**: %s
**时间**: %s
**消息**: %s`,
				strings.ToUpper(alert.Severity),
				alert.Title,
				alert.Type,
				alert.Severity,
				alert.Timestamp.Format(time.RFC3339),
				alert.Message,
			),
		},
	}

	jsonData, _ := json.Marshal(msg)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("DingTalk API returned status: %d", resp.StatusCode)
	}

	return nil
}

func (s *DingTalkSender) generateSign() (int64, string) {
	timestamp := time.Now().UnixMilli()
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, s.secret)

	h := hmac.New(sha256.New, []byte(s.secret))
	h.Write([]byte(stringToSign))
	signature := base64.StdEncoding.EncodeToString(h.Sum(nil))

	return timestamp, urlEncode(signature)
}

// FeishuSender 飞书发送器
type FeishuSender struct {
	webHook string
	secret  string
	client  *http.Client
}

// NewFeishuSender 创建飞书发送器
func NewFeishuSender(webHook, secret string) (*FeishuSender, error) {
	return &FeishuSender{
		webHook: webHook,
		secret:  secret,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}, nil
}

func (s *FeishuSender) Send(ctx context.Context, alert *Alert) error {
	// 获取tenant_access_token (简化实现)
	token, err := s.getTenantAccessToken()
	if err != nil {
		return err
	}

	// 构建消息
	msg := map[string]interface{}{
		"msg_type": "interactive",
		"card": map[string]interface{}{
			"header": map[string]interface{}{
				"title": map[string]string{
					"tag": "plain_text",
					"content": fmt.Sprintf("[%s] %s", strings.ToUpper(alert.Severity), alert.Title),
				},
				"template": s.getTemplateColor(alert.Severity),
			},
			"elements": []map[string]interface{}{
				{
					"tag": "div",
					"text": map[string]string{
						"tag": "lark_md",
						"content": fmt.Sprintf("**类型**: %s\n**严重程度**: %s\n**时间**: %s\n**消息**: %s",
							alert.Type,
							alert.Severity,
							alert.Timestamp.Format(time.RFC3339),
							alert.Message,
						),
					},
				},
			},
		},
	}

	jsonData, _ := json.Marshal(msg)

	url := fmt.Sprintf("%s?tenant_access_token=%s", s.webHook, token)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Feishu API returned status: %d", resp.StatusCode)
	}

	return nil
}

func (s *FeishuSender) getTenantAccessToken() (string, error) {
	// 简化实现，实际应该调用飞书API获取tenant_access_token
	// https://open.feishu.cn/document/ukTMukTMukTM/ukDNz4SO0MjL5QDO/auth-v3/auth/tenant_access_token/internal
	return "dummy_token", nil
}

func (s *FeishuSender) getTemplateColor(severity string) string {
	switch severity {
	case "critical":
		return "red"
	case "error":
		return "orange"
	case "warning":
		return "yellow"
	default:
		return "blue"
	}
}

// urlEncode URL编码
func urlEncode(str string) string {
	result := ""
	for _, c := range str {
		if c == '+' || c == ' ' || c == '/' || c == '=' {
			result += fmt.Sprintf("%%%02X", c)
		} else {
			result += string(c)
		}
	}
	return result
}
