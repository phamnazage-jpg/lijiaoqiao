package platformadapter

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/bridge/ai-customer-service/internal/domain/message"
	"github.com/bridge/ai-customer-service/internal/service/dialog"
)

type IngressContext struct {
	Platform    string
	PathChannel string
	ReceivedAt  time.Time
}

type PlatformInboundMeta struct {
	EventID         string
	Platform        string
	Channel         string
	SourceMessageID string
	SourceUserID    string
	CallbackTarget  string
}

type PlatformAdapter interface {
	Platform() string
	ParseInbound(r *http.Request, body []byte, ctx IngressContext) (*message.UnifiedMessage, *PlatformInboundMeta, error)
	BuildIngressAck(result *dialog.Result, meta *PlatformInboundMeta) any
}

type RequestError struct {
	Status  int
	Code    string
	Message string
}

func (e *RequestError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s %s", strings.TrimSpace(e.Code), strings.TrimSpace(e.Message))
}

func NewRequestError(status int, code, message string) *RequestError {
	return &RequestError{Status: status, Code: code, Message: message}
}
