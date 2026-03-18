package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

// ============================================================================
// 结构体定义
// ============================================================================

// CryptoManager 加密管理器，提供高性能的加密解密服务
type CryptoManager struct {
	key    []byte
	gcm    cipher.AEAD
	mutex  sync.RWMutex
	inited bool
}

// ============================================================================
// 全局变量
// ============================================================================

// 全局加密管理器实例
var cryptoManager = &CryptoManager{}

// ============================================================================
// 私有函数
// ============================================================================

// InitEncryption 初始化加密管理器
// 必须在应用启动时调用，传入加密密钥
func InitEncryption(secret string) error {
	cryptoManager.mutex.Lock()
	defer cryptoManager.mutex.Unlock()

	if cryptoManager.inited {
		return nil
	}

	if secret == "" {
		return errors.New("加密密钥不能为空")
	}

	// 生成AES密钥
	sum := sha256.Sum256([]byte(secret))
	cryptoManager.key = sum[:]

	// 创建AES cipher
	block, err := aes.NewCipher(cryptoManager.key)
	if err != nil {
		return err
	}

	// 创建GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	cryptoManager.gcm = gcm
	cryptoManager.inited = true
	return nil
}

// initCrypto 检查加密管理器是否已初始化
func (cm *CryptoManager) initCrypto() error {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()

	if !cm.inited {
		return errors.New("加密管理器未初始化，请先调用InitEncryption")
	}
	return nil
}

// ============================================================================
// 加密解密函数
// ============================================================================

// EncryptString 字符串加密（AES-256-GCM）
// 使用缓存的密钥和GCM实例，提高性能
func EncryptString(plain string) (string, error) {
	if err := cryptoManager.initCrypto(); err != nil {
		return "", err
	}

	cryptoManager.mutex.RLock()
	gcm := cryptoManager.gcm
	cryptoManager.mutex.RUnlock()

	// 生成随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// 加密
	ciphertext := gcm.Seal(nil, nonce, []byte(plain), nil)
	buf := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(buf), nil
}

// DecryptString 字符串解密（AES-256-GCM）
// 使用缓存的密钥和GCM实例，提高性能
func DecryptString(enc string) (string, error) {
	if err := cryptoManager.initCrypto(); err != nil {
		return "", err
	}

	cryptoManager.mutex.RLock()
	gcm := cryptoManager.gcm
	cryptoManager.mutex.RUnlock()

	// 解码base64
	data, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}

	// 检查数据长度
	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}

	// 分离nonce和密文
	nonce := data[:gcm.NonceSize()]
	ciphertext := data[gcm.NonceSize():]

	// 解密
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plain), nil
}

// EncryptStringBatch 批量加密字符串
// 减少锁竞争，提高批量处理性能
func EncryptStringBatch(plains []string) ([]string, error) {
	if err := cryptoManager.initCrypto(); err != nil {
		return nil, err
	}

	cryptoManager.mutex.RLock()
	gcm := cryptoManager.gcm
	cryptoManager.mutex.RUnlock()

	results := make([]string, len(plains))
	for i, plain := range plains {
		// 生成随机nonce
		nonce := make([]byte, gcm.NonceSize())
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			return nil, err
		}

		// 加密
		ciphertext := gcm.Seal(nil, nonce, []byte(plain), nil)
		buf := append(nonce, ciphertext...)
		results[i] = base64.StdEncoding.EncodeToString(buf)
	}
	return results, nil
}

// DecryptStringBatch 批量解密字符串
// 减少锁竞争，提高批量处理性能
func DecryptStringBatch(encs []string) ([]string, error) {
	if err := cryptoManager.initCrypto(); err != nil {
		return nil, err
	}

	cryptoManager.mutex.RLock()
	gcm := cryptoManager.gcm
	cryptoManager.mutex.RUnlock()

	results := make([]string, len(encs))
	for i, enc := range encs {
		// 解码base64
		data, err := base64.StdEncoding.DecodeString(enc)
		if err != nil {
			return nil, err
		}

		// 检查数据长度
		if len(data) < gcm.NonceSize() {
			return nil, errors.New("ciphertext too short")
		}

		// 分离nonce和密文
		nonce := data[:gcm.NonceSize()]
		ciphertext := data[gcm.NonceSize():]

		// 解密
		plain, err := gcm.Open(nil, nonce, ciphertext, nil)
		if err != nil {
			return nil, err
		}

		results[i] = string(plain)
	}
	return results, nil
}

// GenerateRandomSalt 生成随机密码盐值
// 生成32字节（64个十六进制字符）的随机盐值，用于加密
// 返回: 十六进制格式的盐值字符串和错误信息
func GenerateRandomSalt() (string, error) {
	length := 32 // 固定32字节

	// 生成随机字节
	bytes := make([]byte, length)
	if _, err := io.ReadFull(rand.Reader, bytes); err != nil {
		return "", err
	}

	// 转换为十六进制字符串
	return fmt.Sprintf("%x", bytes), nil
}

// EncryptStringWithSalt 使用盐值进行字符串加密（AES-256-GCM）
// 将明文和盐值组合后进行加密，增强安全性
// plain: 待加密的明文字符串
// salt: 加密盐值
// 返回: base64编码的密文字符串和错误信息
func EncryptStringWithSalt(plain, salt string) (string, error) {
	if err := cryptoManager.initCrypto(); err != nil {
		return "", err
	}

	cryptoManager.mutex.RLock()
	gcm := cryptoManager.gcm
	cryptoManager.mutex.RUnlock()

	// 将明文和盐值组合
	combined := plain + salt

	// 生成随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// 加密
	ciphertext := gcm.Seal(nil, nonce, []byte(combined), nil)
	buf := append(nonce, ciphertext...)
	return base64.StdEncoding.EncodeToString(buf), nil
}

// DecryptStringWithSalt 使用盐值进行字符串解密（AES-256-GCM）
// 解密密文并移除盐值，返回原始明文
// enc: base64编码的密文字符串
// salt: 解密盐值
// 返回: 解密后的明文字符串和错误信息
func DecryptStringWithSalt(enc, salt string) (string, error) {
	if err := cryptoManager.initCrypto(); err != nil {
		return "", err
	}

	cryptoManager.mutex.RLock()
	gcm := cryptoManager.gcm
	cryptoManager.mutex.RUnlock()

	// 解码base64
	data, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return "", err
	}

	// 检查数据长度
	if len(data) < gcm.NonceSize() {
		return "", errors.New("ciphertext too short")
	}

	// 分离nonce和密文
	nonce := data[:gcm.NonceSize()]
	ciphertext := data[gcm.NonceSize():]

	// 解密
	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	// 移除盐值，返回原始明文
	combined := string(plain)
	if len(combined) < len(salt) {
		return "", errors.New("decrypted data too short")
	}

	// 验证盐值是否匹配
	if combined[len(combined)-len(salt):] != salt {
		return "", errors.New("salt mismatch")
	}

	return combined[:len(combined)-len(salt)], nil
}

// HashPasswordWithSalt 使用盐值对密码进行哈希处理
// 将密码和盐值组合后先用SHA256处理，再使用bcrypt进行哈希
// 这样可以避免bcrypt的72字节限制问题
// password: 原始密码
// salt: 密码盐值
// 返回: bcrypt哈希值和错误信息
func HashPasswordWithSalt(password, salt string) (string, error) {
	// 将密码和盐值组合
	combined := password + salt

	// 先使用SHA256处理组合后的字符串，确保长度固定且不超过bcrypt限制
	hash := sha256.Sum256([]byte(combined))
	sha256Hash := fmt.Sprintf("%x", hash) // 64字节的十六进制字符串

	// 使用bcrypt进行哈希（成本因子10，平衡安全性和性能）
	hashed, err := bcrypt.GenerateFromPassword([]byte(sha256Hash), 10)
	if err != nil {
		return "", err
	}

	return string(hashed), nil
}

// VerifyPasswordWithSalt 验证密码和盐值的组合是否匹配哈希值
// password: 原始密码
// salt: 密码盐值
// hashedPassword: 存储的哈希密码
// 返回: 验证结果（true表示匹配）
func VerifyPasswordWithSalt(password, salt, hashedPassword string) bool {
	// 将密码和盐值组合
	combined := password + salt

	// 先使用SHA256处理组合后的字符串，与哈希生成逻辑保持一致
	hash := sha256.Sum256([]byte(combined))
	sha256Hash := fmt.Sprintf("%x", hash) // 64字节的十六进制字符串

	// 使用bcrypt验证
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(sha256Hash))
	return err == nil
}

// GenerateSHA256Hash 生成字符串的SHA256哈希值
// 用于生成密码哈希摘要，用于JWT验证
func GenerateSHA256Hash(input string) string {
	hash := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", hash)
}
