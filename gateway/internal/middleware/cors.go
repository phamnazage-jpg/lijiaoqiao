package middleware

import (
	"net/http"
	"strings"
)

// CORSConfig CORS配置
type CORSConfig struct {
	AllowOrigins     []string // 允许的来源域名
	AllowMethods     []string // 允许的HTTP方法
	AllowHeaders     []string // 允许的请求头
	ExposeHeaders    []string // 允许暴露给客户端的响应头
	AllowCredentials bool     // 是否允许携带凭证
	MaxAge           int      // 预检请求缓存时间（秒）
}

// DefaultCORSConfig 返回默认CORS配置
func DefaultCORSConfig() CORSConfig {
	return CORSConfig{
		AllowOrigins: []string{"*"}, // 生产环境应限制具体域名
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{"Authorization", "Content-Type", "X-Request-ID", "X-Request-Key"},
		ExposeHeaders: []string{"X-Request-ID"},
		AllowCredentials: false,
		MaxAge: 86400, // 24小时
	}
}

// CORSMiddleware 创建CORS中间件
func CORSMiddleware(config CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 处理CORS预检请求
			if r.Method == http.MethodOptions {
				handleCORSPreflight(w, r, config)
				return
			}

			// 处理实际请求的CORS头
			setCORSHeaders(w, r, config)
			next.ServeHTTP(w, r)
		})
	}
}

// handleCORS Preflight 处理预检请求
func handleCORSPreflight(w http.ResponseWriter, r *http.Request, config CORSConfig) {
func handleCORS Preflight(w http.ResponseWriter, r *http.Request, config CORSConfig) {
	origin := r.Header.Get("Origin")

	// 检查origin是否被允许
	if !isOriginAllowed(origin, config.AllowOrigins) {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	// 设置预检响应头
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(config.AllowMethods, ", "))
	w.Header().Set("Access-Control-Allow-Headers", strings.Join(config.AllowHeaders, ", "))
	w.Header().Set("Access-Control-Max-Age", string(rune(config.MaxAge)))

	if config.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}

	w.WriteHeader(http.StatusNoContent)
}

// setCORSHeaders 设置实际请求的CORS响应头
func setCORSHeaders(w http.ResponseWriter, r *http.Request, config CORSConfig) {
	origin := r.Header.Get("Origin")

	// 检查origin是否被允许
	if !isOriginAllowed(origin, config.AllowOrigins) {
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", origin)

	if len(config.ExposeHeaders) > 0 {
		w.Header().Set("Access-Control-Expose-Headers", strings.Join(config.ExposeHeaders, ", "))
	}

	if config.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	}
}

// isOriginAllowed 检查origin是否在允许列表中
func isOriginAllowed(origin string, allowedOrigins []string) bool {
	if origin == "" {
		return false
	}

	for _, allowed := range allowedOrigins {
		if allowed == "*" {
			return true
		}
		if strings.EqualFold(allowed, origin) {
			return true
		}
		// 支持通配符子域名 *.example.com
		if strings.HasPrefix(allowed, "*.") {
			domain := allowed[2:]
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}
	return false
}