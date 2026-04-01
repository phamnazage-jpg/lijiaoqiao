package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"lijiaoqiao/supply-api/internal/repository"
)

// IdempotencyConfig 幂等中间件配置
type IdempotencyConfig struct {
	TTL           time.Duration // 幂等有效期，默认24h
	ProcessingTTL time.Duration // 处理中状态有效期，默认30s
	Enabled       bool          // 是否启用幂等
}

// IdempotencyMiddleware 幂等中间件
type IdempotencyMiddleware struct {
	idempotencyRepo *repository.IdempotencyRepository
	config          IdempotencyConfig
}

// NewIdempotencyMiddleware 创建幂等中间件
func NewIdempotencyMiddleware(repo *repository.IdempotencyRepository, config IdempotencyConfig) *IdempotencyMiddleware {
	if config.TTL == 0 {
		config.TTL = 24 * time.Hour
	}
	if config.ProcessingTTL == 0 {
		config.ProcessingTTL = 30 * time.Second
	}
	return &IdempotencyMiddleware{
		idempotencyRepo: repo,
		config:          config,
	}
}

// IdempotencyKey 幂等键信息
type IdempotencyKey struct {
	TenantID   int64
	OperatorID int64
	APIPath    string
	Key        string
}

// ExtractIdempotencyKey 从请求中提取幂等信息
func ExtractIdempotencyKey(r *http.Request, tenantID, operatorID int64) (*IdempotencyKey, error) {
	requestID := r.Header.Get("X-Request-Id")
	if requestID == "" {
		return nil, fmt.Errorf("missing X-Request-Id header")
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		return nil, fmt.Errorf("missing Idempotency-Key header")
	}

	if len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
		return nil, fmt.Errorf("Idempotency-Key length must be 16-128")
	}

	// 从路径提取API路径（去除前缀）
	apiPath := r.URL.Path
	if strings.HasPrefix(apiPath, "/api/v1") {
		apiPath = strings.TrimPrefix(apiPath, "/api/v1")
	}

	return &IdempotencyKey{
		TenantID:   tenantID,
		OperatorID: operatorID,
		APIPath:    apiPath,
		Key:        idempotencyKey,
	}, nil
}

// ComputePayloadHash 计算请求体的SHA256哈希
func ComputePayloadHash(body []byte) string {
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}

// IdempotentHandler 幂等处理器函数
type IdempotentHandler func(ctx context.Context, w http.ResponseWriter, r *http.Request, record *repository.IdempotencyRecord) error

// Wrap 包装HTTP处理器以实现幂等
func (m *IdempotencyMiddleware) Wrap(handler IdempotentHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !m.config.Enabled {
			handler(r.Context(), w, r, nil)
			return
		}

		ctx := r.Context()

		// 从context获取租户和操作者ID（由鉴权中间件设置）
		tenantID := getTenantID(ctx)
		operatorID := getOperatorID(ctx)

		// 提取幂等信息
		idempKey, err := ExtractIdempotencyKey(r, tenantID, operatorID)
		if err != nil {
			writeIdempotencyError(w, http.StatusBadRequest, "IDEMPOTENCY_KEY_INVALID", err.Error())
			return
		}

		// 读取请求体
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeIdempotencyError(w, http.StatusBadRequest, "BODY_READ_ERROR", err.Error())
			return
		}
		// 重新填充body以供后续处理
		r.Body = io.NopCloser(bytes.NewBuffer(body))

		// 计算payload hash
		payloadHash := ComputePayloadHash(body)

		// 查询已存在的幂等记录
		existingRecord, err := m.idempotencyRepo.GetByKey(ctx, idempKey.TenantID, idempKey.OperatorID, idempKey.APIPath, idempKey.Key)
		if err != nil {
			writeIdempotencyError(w, http.StatusInternalServerError, "IDEMPOTENCY_CHECK_FAILED", err.Error())
			return
		}

		if existingRecord != nil {
			// 存在记录，处理不同情况
			switch existingRecord.Status {
			case repository.IdempotencyStatusSucceeded:
				// 同参重放：返回原结果
				if existingRecord.PayloadHash == payloadHash {
					writeIdempotentReplay(w, existingRecord.ResponseCode, existingRecord.ResponseBody)
					return
				}
				// 异参重放：返回409冲突
				writeIdempotencyError(w, http.StatusConflict, "IDEMPOTENCY_PAYLOAD_MISMATCH",
					fmt.Sprintf("same idempotency key but different payload, original request_id: %s", existingRecord.RequestID))
				return

			case repository.IdempotencyStatusProcessing:
				// 处理中：检查是否超时
				if time.Since(existingRecord.UpdatedAt) < m.config.ProcessingTTL {
					retryAfter := m.config.ProcessingTTL - time.Since(existingRecord.UpdatedAt)
					writeIdempotencyProcessing(w, int(retryAfter.Milliseconds()), existingRecord.RequestID)
					return
				}
				// 超时：允许重试（记录会自然过期）

			case repository.IdempotencyStatusFailed:
				// 失败状态也允许重试
			}
		}

		// 使用AcquireLock获取锁
		requestID := r.Header.Get("X-Request-Id")
		lockedRecord, err := m.idempotencyRepo.AcquireLock(ctx, idempKey.TenantID, idempKey.OperatorID, idempKey.APIPath, idempKey.Key, m.config.TTL)
		if err != nil {
			writeIdempotencyError(w, http.StatusInternalServerError, "IDEMPOTENCY_LOCK_FAILED", err.Error())
			return
		}

		// 更新记录中的request_id和payload_hash
		if lockedRecord.ID != 0 && (lockedRecord.RequestID == "" || lockedRecord.PayloadHash == "") {
			lockedRecord.RequestID = requestID
			lockedRecord.PayloadHash = payloadHash
		}

		// 执行实际业务处理
		err = handler(ctx, w, r, lockedRecord)

		// 根据处理结果更新幂等记录
		if err != nil {
			// 业务处理失败
			errMsg, _ := json.Marshal(map[string]string{"error": err.Error()})
			_ = m.idempotencyRepo.UpdateFailed(ctx, lockedRecord.ID, http.StatusInternalServerError, errMsg)
			return
		}

		// 业务处理成功，更新为成功状态
		// 注意：这里需要从w中获取实际的响应码和body
		// 简化处理：使用200
		successBody, _ := json.Marshal(map[string]interface{}{"status": "ok"})
		_ = m.idempotencyRepo.UpdateSuccess(ctx, lockedRecord.ID, http.StatusOK, successBody)
	}
}

// writeIdempotencyError 写入幂等错误
func writeIdempotencyError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := map[string]interface{}{
		"request_id": "",
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	}
	json.NewEncoder(w).Encode(resp)
}

// writeIdempotencyProcessing 写入处理中状态
func writeIdempotencyProcessing(w http.ResponseWriter, retryAfterMs int, requestID string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After-Ms", fmt.Sprintf("%d", retryAfterMs))
	w.Header().Set("X-Request-Id", requestID)
	w.WriteHeader(http.StatusAccepted)
	resp := map[string]interface{}{
		"request_id": requestID,
		"error": map[string]string{
			"code":    "IDEMPOTENCY_IN_PROGRESS",
			"message": "request is being processed, please retry later",
		},
	}
	json.NewEncoder(w).Encode(resp)
}

// writeIdempotentReplay 写入幂等重放响应
func writeIdempotentReplay(w http.ResponseWriter, status int, body json.RawMessage) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Idempotent-Replay", "true")
	w.WriteHeader(status)
	if body != nil {
		w.Write(body)
	}
}

// context keys
type contextKey string

const (
	tenantIDKey   contextKey = "tenant_id"
	operatorIDKey contextKey = "operator_id"
)

// WithTenantID 在context中设置租户ID
func WithTenantID(ctx context.Context, tenantID int64) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// WithOperatorID 在context中设置操作者ID
func WithOperatorID(ctx context.Context, operatorID int64) context.Context {
	return context.WithValue(ctx, operatorIDKey, operatorID)
}

func getTenantID(ctx context.Context) int64 {
	if v := ctx.Value(tenantIDKey); v != nil {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}

func getOperatorID(ctx context.Context) int64 {
	if v := ctx.Value(operatorIDKey); v != nil {
		if id, ok := v.(int64); ok {
			return id
		}
	}
	return 0
}
