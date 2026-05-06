package handlers

import (
	"net/http"
	"strings"
	"time"
)

type PlatformWebhookSecurity struct {
	TimestampHeader string
	SignatureHeader string
	MaxSkew         time.Duration
	Audit           AuditRecorder
	Sub2APISecret   string
	NewAPISecret    string
}

func (s PlatformWebhookSecurity) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		platform, _, ok := parsePlatformWebhookPath(r.URL.Path)
		if !ok {
			next.ServeHTTP(w, r)
			return
		}
		security, enabled := s.securityForPlatform(platform)
		if !enabled {
			next.ServeHTTP(w, r)
			return
		}
		security.Wrap(next).ServeHTTP(w, r)
	})
}

func (s PlatformWebhookSecurity) securityForPlatform(platform string) (WebhookSecurity, bool) {
	secret := strings.TrimSpace(s.secretForPlatform(platform))
	if secret == "" {
		return WebhookSecurity{}, false
	}
	return WebhookSecurity{
		Secret:          secret,
		TimestampHeader: s.TimestampHeader,
		SignatureHeader: s.SignatureHeader,
		MaxSkew:         s.MaxSkew,
		Audit:           s.Audit,
	}, true
}

func (s PlatformWebhookSecurity) secretForPlatform(platform string) string {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case "sub2api":
		return s.Sub2APISecret
	case "newapi":
		return s.NewAPISecret
	default:
		return ""
	}
}
