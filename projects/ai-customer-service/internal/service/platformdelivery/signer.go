package platformdelivery

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultTimestampHeader = "X-CS-Timestamp"
	DefaultSignatureHeader = "X-CS-Signature"
)

type Signer struct {
	Secret          string
	TimestampHeader string
	SignatureHeader string
}

func (s Signer) Headers(body []byte, now time.Time) (http.Header, error) {
	if strings.TrimSpace(s.Secret) == "" {
		return nil, fmt.Errorf("secret is required")
	}
	if now.IsZero() {
		now = time.Now()
	}
	timestamp := strconv.FormatInt(now.Unix(), 10)
	headers := make(http.Header)
	headers.Set(s.timestampHeader(), timestamp)
	headers.Set(s.signatureHeader(), computeSignature(s.Secret, timestamp, body))
	return headers, nil
}

func (s Signer) timestampHeader() string {
	if strings.TrimSpace(s.TimestampHeader) == "" {
		return DefaultTimestampHeader
	}
	return s.TimestampHeader
}

func (s Signer) signatureHeader() string {
	if strings.TrimSpace(s.SignatureHeader) == "" {
		return DefaultSignatureHeader
	}
	return s.SignatureHeader
}

func computeSignature(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
