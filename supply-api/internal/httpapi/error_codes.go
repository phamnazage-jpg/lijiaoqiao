package httpapi

// HTTP 层错误码（统一使用 SUP_HTTP_XXXX 格式）
const (
	// HTTP 4000-4099: 客户端错误
	CodeBadRequest       = "SUP_HTTP_4000" // 通用错误请求
	CodeMissingParam     = "SUP_HTTP_4003" // 缺少必需参数
	CodeInvalidPath      = "SUP_HTTP_4002" // 无效路径
	CodeNotFound         = "SUP_HTTP_4004" // 资源未找到
	CodeMethodNotAllowed = "SUP_HTTP_4005" // HTTP 方法不允许
	CodeConflict         = "SUP_HTTP_4009" // 资源冲突

	// HTTP 401-403: 认证授权错误
	CodeAuthContextMissing = "SUP_HTTP_4011" // 认证上下文缺失

	// HTTP 422: 业务验证错误
	CodeVerifyFailed       = "SUP_ACC_4001" // 验证失败
	CodeFKValidationFailed = "SUP_ACC_4002" // 外键验证失败

	// HTTP 500-503: 服务器错误
	CodeQueryFailed         = "SUP_HTTP_5001"  // 查询失败
	CodeCreateFailed        = "SUP_HTTP_5002"  // 创建失败
	CodeWithdrawFailed      = "SUP_SET_5001"   // 提现失败
	CodeGetFailed           = "SUP_AUDIT_5001" // 获取失败
	CodeBatchUpdateFailed   = "SUP_PKG_5001"   // 批量更新失败
	CodeFeatureDisabled     = "SUP_HTTP_5030"  // 功能已禁用
	CodeIdempotencyRequired = "SUP_HTTP_5031"  // 幂等中间件缺失
)
