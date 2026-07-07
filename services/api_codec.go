package services

import (
	"NetworkAuth/models"
	"NetworkAuth/utils/encrypt"
	"encoding/hex"
	"errors"
)

// ============================================================================
// 公开接口加解密管道
// ============================================================================
//
// 每个接口（models.API）独立配置提交算法与返回算法及其密钥。方向约定：
//   - 提交（客户端 → 服务端）：用「提交算法 + 提交密钥」解密客户端上报的密文。
//   - 返回（服务端 → 客户端）：用「返回算法 + 返回密钥」加密服务端返回的明文。
//
// 各算法密钥存储约定（与后台制钥保持一致）：
//   - 不加密：无密钥
//   - RC4：private_key 存 16 进制密钥
//   - RSA / RSA动态：public_key / private_key 存 PEM
//   - 易加密：private_key 存逗号分隔整数

// APICodec 绑定单个接口配置的编解码器
type APICodec struct {
	api *models.API
}

// NewAPICodec 基于接口配置构造编解码器
func NewAPICodec(api *models.API) *APICodec {
	return &APICodec{api: api}
}

// DecryptRequest 用提交算法解密客户端上报的密文
func (co *APICodec) DecryptRequest(cipher string) (string, error) {
	return decodeByAlgorithm(co.api.SubmitAlgorithm, co.api.SubmitPublicKey, co.api.SubmitPrivateKey, cipher)
}

// EncryptResponse 用返回算法加密服务端返回的明文
func (co *APICodec) EncryptResponse(plain string) (string, error) {
	return encodeByAlgorithm(co.api.ReturnAlgorithm, co.api.ReturnPublicKey, co.api.ReturnPrivateKey, plain)
}

// encodeByAlgorithm 按算法对明文加密
func encodeByAlgorithm(algorithm int, publicKey, privateKey, plain string) (string, error) {
	switch algorithm {
	case models.AlgorithmNone:
		return plain, nil
	case models.AlgorithmRC4:
		enc, err := rc4Codec(privateKey)
		if err != nil {
			return "", err
		}
		return enc.Encrypt(plain)
	case models.AlgorithmEasy:
		enc, err := easyCodec(privateKey)
		if err != nil {
			return "", err
		}
		return enc.Encrypt(plain), nil
	case models.AlgorithmRSA:
		// 返回方向用公钥加密，客户端持私钥解密
		pub, err := encrypt.PublicKeyFromPEM(publicKey)
		if err != nil {
			return "", errors.New("返回公钥无效")
		}
		return encrypt.NewRSAEncrypt(pub, nil).EncryptLargeData(plain)
	case models.AlgorithmRSADynamic:
		enc, err := encrypt.NewRSADynamicEncrypt(publicKey, privateKey)
		if err != nil {
			return "", errors.New("返回动态密钥无效")
		}
		return enc.Encrypt(plain)
	default:
		return "", errors.New("不支持的返回算法")
	}
}

// decodeByAlgorithm 按算法对密文解密
func decodeByAlgorithm(algorithm int, publicKey, privateKey, cipher string) (string, error) {
	switch algorithm {
	case models.AlgorithmNone:
		return cipher, nil
	case models.AlgorithmRC4:
		enc, err := rc4Codec(privateKey)
		if err != nil {
			return "", err
		}
		return enc.Decrypt(cipher)
	case models.AlgorithmEasy:
		enc, err := easyCodec(privateKey)
		if err != nil {
			return "", err
		}
		return enc.Decrypt(cipher), nil
	case models.AlgorithmRSA:
		// 提交方向用私钥解密，客户端持公钥加密
		priv, err := encrypt.PrivateKeyFromPEM(privateKey)
		if err != nil {
			return "", errors.New("提交私钥无效")
		}
		return encrypt.NewRSAEncrypt(nil, priv).DecryptLargeData(cipher)
	case models.AlgorithmRSADynamic:
		enc, err := encrypt.NewRSADynamicEncrypt(publicKey, privateKey)
		if err != nil {
			return "", errors.New("提交动态密钥无效")
		}
		return enc.Decrypt(cipher)
	default:
		return "", errors.New("不支持的提交算法")
	}
}

// rc4Codec 由 16 进制密钥串构造 RC4 编解码器
func rc4Codec(hexKey string) (*encrypt.RC4Encrypt, error) {
	if hexKey == "" {
		return nil, errors.New("RC4密钥为空")
	}
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, errors.New("RC4密钥格式无效")
	}
	return encrypt.NewRC4Encrypt(key), nil
}

// easyCodec 由逗号分隔整数密钥串构造易加密编解码器（加解密同密钥）
func easyCodec(keyStr string) (*encrypt.EasyEncrypt, error) {
	key := encrypt.ParseKeyFromString(keyStr)
	if len(key) == 0 {
		return nil, errors.New("易加密密钥为空")
	}
	return encrypt.NewEasyEncrypt(key, key), nil
}
