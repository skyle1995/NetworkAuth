package models

// ============================================================================
// 标准错误码（机器可读）
// ============================================================================
//
// 公开 API 的失败响应统一形如：
//   { "code": 1, "msg": "点数不足，请充值", "err": "INSUFFICIENT_CREDITS" }
//
//   msg —— 中文，可直接展示给终端用户
//   err —— 机器码，客户端按它分支，**不要**解析中文
//
// 命名对齐 Keygen / OAuth 2.0 的既有语义，避免自造词：
//   https://keygen.sh/docs/api/  https://www.rfc-editor.org/rfc/rfc6749#section-5.2

const (
	// ---- 身份 / 令牌 ----

	// ErrAccountTokenInvalid 账号令牌无效或已过期（需重新登录）
	ErrAccountTokenInvalid = "ACCOUNT_TOKEN_INVALID"
	// ErrActivationRequired 需要先激活应用（用账号令牌调了应用级接口）
	ErrActivationRequired = "ACTIVATION_REQUIRED"
	// ErrActivationTokenInvalid 激活令牌无效/已释放（需重新激活）
	ErrActivationTokenInvalid = "ACTIVATION_TOKEN_INVALID"
	// ErrLeaseExpired 租约已过期（需重新激活，会重新计费）
	ErrLeaseExpired = "LEASE_EXPIRED"
	// ErrSignatureInvalid 请求签名校验失败
	ErrSignatureInvalid = "SIGNATURE_INVALID"

	// ---- 许可状态（License）----

	// ErrLicenseExpired 账号已到期（会员模式/会员有效期）
	ErrLicenseExpired = "LICENSE_EXPIRED"
	// ErrLicenseSuspended 账号被封停或拉黑
	ErrLicenseSuspended = "LICENSE_SUSPENDED"

	// ---- 权益（Entitlement）----

	// ErrEntitlementRequired 会员等级不足
	ErrEntitlementRequired = "ENTITLEMENT_REQUIRED"
	// ErrInsufficientCredits 点数不足
	ErrInsufficientCredits = "INSUFFICIENT_CREDITS"
	// ErrConcurrentAppLimit 同时激活的不同应用数超限（max_concurrent_apps）
	ErrConcurrentAppLimit = "CONCURRENT_APP_LIMIT"

	// ---- 设备与并发（Machine）----

	// ErrMachinesLimitExceeded 并发激活数已达上限
	ErrMachinesLimitExceeded = "MACHINES_LIMIT_EXCEEDED"
	// ErrFingerprintMismatch 设备指纹不符（绑定校验失败，需转绑）
	ErrFingerprintMismatch = "FINGERPRINT_MISMATCH"
	// ErrFingerprintRequired 需要设备指纹但请求未提供
	ErrFingerprintRequired = "FINGERPRINT_REQUIRED"

	// ---- 应用与请求 ----

	// ErrAppNotFound 应用不存在
	ErrAppNotFound = "APP_NOT_FOUND"
	// ErrAppDisabled 应用已停用
	ErrAppDisabled = "APP_DISABLED"
	// ErrHostAppNotActivatable 宿主应用不能作为插件被激活
	ErrHostAppNotActivatable = "HOST_APP_NOT_ACTIVATABLE"
	// ErrInvalidRequest 请求参数缺失或格式错误
	ErrInvalidRequest = "INVALID_REQUEST"
	// ErrAccountPolicyDenied 被全站账号策略拒绝（注册关闭/试用关闭等）
	ErrAccountPolicyDenied = "ACCOUNT_POLICY_DENIED"
)

// ErrorCodeMessages 标准错误码的默认中文文案。
// 业务层可以覆盖成更具体的描述（如带上所需等级名），但建议保留 err 不变。
var ErrorCodeMessages = map[string]string{
	ErrAccountTokenInvalid:    "登录状态已失效，请重新登录",
	ErrActivationRequired:     "请先启动应用",
	ErrActivationTokenInvalid: "应用授权已失效，请重新启动",
	ErrLeaseExpired:           "授权已到期，请重新启动应用",
	ErrSignatureInvalid:       "请求签名校验失败",
	ErrLicenseExpired:         "账号已到期",
	ErrLicenseSuspended:       "账号已被停用",
	ErrEntitlementRequired:    "会员等级不足",
	ErrInsufficientCredits:    "点数不足",
	ErrConcurrentAppLimit:     "同时运行的应用数已达上限",
	ErrMachinesLimitExceeded:  "已达最大同时在线数",
	ErrFingerprintMismatch:    "设备未在绑定范围，请先转绑",
	ErrFingerprintRequired:    "请提供设备码",
	ErrAppNotFound:            "应用不存在",
	ErrAppDisabled:            "应用已停用",
	ErrHostAppNotActivatable:  "该应用为宿主程序，不能作为插件启动",
	ErrInvalidRequest:         "请求参数有误",
	ErrAccountPolicyDenied:    "当前操作已被系统策略关闭",
}

// ErrMessage 返回错误码的中文兜底文案
func ErrMessage(code string) string {
	if m, ok := ErrorCodeMessages[code]; ok {
		return m
	}
	return "操作失败"
}
