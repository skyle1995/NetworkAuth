package services

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

// ============================================================================
// 公开 API 请求签名（防重放 + 完整性 + 应用鉴权）
// ============================================================================
//
// 每个公开请求须携带 timestamp 与 sign：
//
//	sign = SHA256(app_uuid | api_type | data | timestamp | app_secret) 的大写十六进制
//
// 服务端用应用密钥重算校验：时间戳须在允许窗口内（防重放），签名须一致（防篡改，
// 且证明调用方持有应用密钥）。

// signWindowSeconds 允许的时间戳偏差窗口（秒）
const signWindowSeconds = 300

// SignOpenRequest 计算请求签名
func SignOpenRequest(appUUID string, apiType int, data string, timestamp int64, secret string) string {
	raw := strings.Join([]string{
		appUUID,
		strconv.Itoa(apiType),
		data,
		strconv.FormatInt(timestamp, 10),
		secret,
	}, "|")
	sum := sha256.Sum256([]byte(raw))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

// VerifyOpenSign 校验请求签名与时间戳窗口
func VerifyOpenSign(appUUID string, apiType int, data string, timestamp int64, sign, secret string) error {
	if strings.TrimSpace(sign) == "" {
		return errors.New("缺少签名")
	}
	now := time.Now().Unix()
	diff := now - timestamp
	if diff < 0 {
		diff = -diff
	}
	if diff > signWindowSeconds {
		return errors.New("请求已过期，请校准时间")
	}
	expected := SignOpenRequest(appUUID, apiType, data, timestamp, secret)
	if subtle.ConstantTimeCompare([]byte(expected), []byte(strings.ToUpper(strings.TrimSpace(sign)))) != 1 {
		return errors.New("签名校验失败")
	}
	return nil
}
