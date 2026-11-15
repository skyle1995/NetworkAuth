package services

import (
	"crypto/ed25519"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 租约签名：服务端用离线 Ed25519 私钥对「应用 + 激活令牌 + 设备指纹 + 到期时间」签名，
// 客户端 loader 用内嵌公钥验签。
//
// 为什么需要：租约文件在设备上是明文，之前只做结构校验（token 非空、expire_ts 未过期），
// 任何人都能手写一条 `token=deadbeef / expire_ts=9999999999` 让插件跑起来。
// 签名后 expire_ts 与设备指纹都无法被伪造或跨设备搬运。
const leaseMsgPrefix = "KTRWLEASE1"

func leaseKeyPath() string {
	if p := strings.TrimSpace(os.Getenv("KTRW_LEASE_SIGN_KEY")); p != "" {
		return p
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ktrw", "license_signing.key")
}

// leasePrivateKey 读取 32 字节 Ed25519 种子（hex 或原始字节）。
func leasePrivateKey() (ed25519.PrivateKey, error) {
	path := leaseKeyPath()
	if path == "" {
		return nil, errors.New("租约签名密钥未配置")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取租约签名密钥失败: %w", err)
	}
	txt := strings.TrimSpace(string(raw))
	if len(txt) == 64 {
		if seed, derr := hex.DecodeString(txt); derr == nil && len(seed) == ed25519.SeedSize {
			return ed25519.NewKeyFromSeed(seed), nil
		}
	}
	if len(raw) == ed25519.SeedSize {
		return ed25519.NewKeyFromSeed(raw), nil
	}
	return nil, errors.New("租约签名密钥格式不对（需要 32 字节种子或 64 位 hex）")
}

// LeaseMessage 构造被签名的规范化字符串。
func LeaseMessage(appUUID, token, fingerprint string, expireUnix int64) []byte {
	return []byte(fmt.Sprintf("%s|%s|%s|%s|%d",
		leaseMsgPrefix, strings.ToUpper(strings.TrimSpace(appUUID)),
		strings.TrimSpace(token), strings.ToLower(strings.TrimSpace(fingerprint)), expireUnix))
}

// SignLease 返回十六进制签名；密钥缺失时返回错误（调用方应把错误暴露给客户端，不能静默放行）。
func SignLease(appUUID, token, fingerprint string, expireUnix int64) (string, error) {
	priv, err := leasePrivateKey()
	if err != nil {
		return "", err
	}
	sig := ed25519.Sign(priv, LeaseMessage(appUUID, token, fingerprint, expireUnix))
	return hex.EncodeToString(sig), nil
}
