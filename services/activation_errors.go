package services

import (
	"errors"

	"NetworkAuth/models"
)

// ============================================================================
// 激活相关错误类型
// ============================================================================
//
// 这些错误除了中文文案，还携带**机器可读的标准错误码**（对齐 Keygen / OAuth），
// 供公开 API 在响应里同时给出 msg（展示）与 err（分支）。
//
// 用法：
//   var e *EntitlementError
//   if errors.As(err, &e) { code = models.ErrEntitlementRequired; msg = e.Error() }

// CodedError 带标准错误码的错误
type CodedError interface {
	error
	ErrCode() string
}

// ErrFingerprintRequired 应用要求设备指纹但未提供
var ErrFingerprintRequired = NewCodedError(models.ErrFingerprintRequired, "")

// ErrActivationRequiredMarker 需要先激活（用账号令牌调了应用级接口）
var ErrActivationRequiredMarker = errors.New(models.ErrMessage(models.ErrActivationRequired))

// activationErr 通用带码错误
type activationErr struct {
	code string
	msg  string
}

func (e *activationErr) Error() string { return e.msg }
func (e *activationErr) ErrCode() string { return e.code }

// NewCodedError 构造带错误码的错误
func NewCodedError(code, msg string) error {
	if msg == "" {
		msg = models.ErrMessage(code)
	}
	return &activationErr{code: code, msg: msg}
}

// ErrorCodeOf 提取错误的机器码；不是 CodedError 时返回空串
func ErrorCodeOf(err error) string {
	var ce CodedError
	if errors.As(err, &ce) {
		return ce.ErrCode()
	}
	return ""
}

// ---- 具体错误类型 ----

// EntitlementError 会员等级不足
type EntitlementError struct {
	NeedLevel int
	NeedName  string
}

func (e *EntitlementError) Error() string {
	if e.NeedName != "" {
		return "会员等级不足，需要 " + e.NeedName + " 及以上"
	}
	return models.ErrMessage(models.ErrEntitlementRequired)
}
func (e *EntitlementError) ErrCode() string { return models.ErrEntitlementRequired }

// LicenseExpiredError 账号已到期
type LicenseExpiredError struct{}

func (e *LicenseExpiredError) Error() string   { return models.ErrMessage(models.ErrLicenseExpired) }
func (e *LicenseExpiredError) ErrCode() string { return models.ErrLicenseExpired }

// InsufficientCreditsError 点数不足
type InsufficientCreditsError struct {
	Need float64
	Have float64
}

func (e *InsufficientCreditsError) Error() string {
	return models.ErrMessage(models.ErrInsufficientCredits)
}
func (e *InsufficientCreditsError) ErrCode() string { return models.ErrInsufficientCredits }

// FingerprintMismatchError 设备指纹不符（需转绑）
type FingerprintMismatchError struct {
	Cause error
}

func (e *FingerprintMismatchError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return models.ErrMessage(models.ErrFingerprintMismatch)
}
func (e *FingerprintMismatchError) ErrCode() string { return models.ErrFingerprintMismatch }
func (e *FingerprintMismatchError) Unwrap() error   { return e.Cause }

// MachinesLimitError 并发激活数超限
type MachinesLimitError struct {
	Limit int
}

func (e *MachinesLimitError) Error() string   { return models.ErrMessage(models.ErrMachinesLimitExceeded) }
func (e *MachinesLimitError) ErrCode() string { return models.ErrMachinesLimitExceeded }

// isFatalActivationError 判断是否需要把激活置死（客户端必须重新激活）
func isFatalActivationError(err error) bool {
	if err == nil {
		return false
	}
	var (
		entitlement *EntitlementError
		expired     *LicenseExpiredError
		credits     *InsufficientCreditsError
	)
	if errors.As(err, &entitlement) || errors.As(err, &expired) || errors.As(err, &credits) {
		return true
	}
	return false
}
