package httpapi

import (
	"errors"
	"net/http"
	"net/url"

	"lijiaoqiao/supply-api/internal/audit/handler"
	"lijiaoqiao/supply-api/internal/audit/service"
)

// AlertAPI 告警API处理器
type AlertAPI struct {
	alertHandler *handler.AlertHandler
}

// NewAlertAPI 创建告警API处理器
func NewAlertAPI(alertSvc *service.AlertService) (*AlertAPI, error) {
	if alertSvc == nil {
		return nil, errors.New("alert service is required")
	}

	alertHandler := handler.NewAlertHandler(alertSvc)

	return &AlertAPI{
		alertHandler: alertHandler,
	}, nil
}

// Register 注册告警路由
func (a *AlertAPI) Register(mux *http.ServeMux) {
	// Alert CRUD
	mux.HandleFunc("/api/v1/audit/alerts", a.handleAlert)
	mux.HandleFunc("/api/v1/audit/alerts/", a.handleAlertByID)
}

// handleAlert 处理 /api/v1/audit/alerts 的路由分发
func (a *AlertAPI) handleAlert(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		a.alertHandler.CreateAlert(w, r)
	case http.MethodGet:
		a.alertHandler.ListAlerts(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, CodeMethodNotAllowed, "method not allowed")
	}
}

// handleAlertByID 处理 /api/v1/audit/alerts/{alert_id} 的路由分发
func (a *AlertAPI) handleAlertByID(w http.ResponseWriter, r *http.Request) {
	// 提取路径最后部分判断操作
	path := r.URL.Path
	if len(path) > 0 && path[len(path)-1] == '/' {
		path = path[:len(path)-1]
	}

	parts := splitPath(path)
	if len(parts) < 5 {
		writeError(w, http.StatusBadRequest, CodeInvalidPath, "invalid path")
		return
	}

	alertID := parts[4]

	// 检查是否是特殊操作
	if len(parts) > 5 && parts[5] == "resolve" {
		if r.Method == http.MethodPost {
			a.alertHandler.ResolveAlert(w, r)
		} else {
			writeError(w, http.StatusMethodNotAllowed, CodeMethodNotAllowed, "method not allowed")
		}
		return
	}

	// 常规CRUD操作
	switch r.Method {
	case http.MethodGet:
		// 安全设置alert_id到查询参数
		query := make(url.Values)
		for k, v := range r.URL.Query() {
			query[k] = v
		}
		query.Set("alert_id", alertID)
		r.URL.RawQuery = query.Encode()
		a.alertHandler.GetAlert(w, r)
	case http.MethodPut:
		query := make(url.Values)
		for k, v := range r.URL.Query() {
			query[k] = v
		}
		query.Set("alert_id", alertID)
		r.URL.RawQuery = query.Encode()
		a.alertHandler.UpdateAlert(w, r)
	case http.MethodDelete:
		query := make(url.Values)
		for k, v := range r.URL.Query() {
			query[k] = v
		}
		query.Set("alert_id", alertID)
		r.URL.RawQuery = query.Encode()
		a.alertHandler.DeleteAlert(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, CodeMethodNotAllowed, "method not allowed")
	}
}

// splitPath 分割路径
func splitPath(path string) []string {
	var parts []string
	var current []byte
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			if len(current) > 0 {
				parts = append(parts, string(current))
				current = nil
			}
		} else {
			current = append(current, path[i])
		}
	}
	if len(current) > 0 {
		parts = append(parts, string(current))
	}
	return parts
}
