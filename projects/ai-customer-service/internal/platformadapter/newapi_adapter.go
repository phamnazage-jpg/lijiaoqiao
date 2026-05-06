package platformadapter

import (
	"net/http"

	"github.com/bridge/ai-customer-service/internal/domain/message"
	"github.com/bridge/ai-customer-service/internal/service/dialog"
)

type NewAPIAdapter struct{}

func NewNewAPIAdapter() *NewAPIAdapter {
	return &NewAPIAdapter{}
}

func (a *NewAPIAdapter) Platform() string {
	return "newapi"
}

func (a *NewAPIAdapter) ParseInbound(_ *http.Request, _ []byte, _ IngressContext) (*message.UnifiedMessage, *PlatformInboundMeta, error) {
	return nil, nil, NewRequestError(http.StatusNotImplemented, "CS_PLATFORM_5010", "newapi profile is not implemented")
}

func (a *NewAPIAdapter) BuildIngressAck(_ *dialog.Result, meta *PlatformInboundMeta) any {
	resp := map[string]any{
		"accepted": false,
		"platform": a.Platform(),
	}
	if meta != nil {
		resp["event_id"] = meta.EventID
	}
	return resp
}
