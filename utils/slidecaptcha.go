package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	assetsImages "github.com/wenlng/go-captcha-assets/resources/images"
	assetsTiles "github.com/wenlng/go-captcha-assets/resources/tiles"
	"github.com/wenlng/go-captcha/v2/base/option"
	"github.com/wenlng/go-captcha/v2/slide"
)

// ============================================================================
// 滑动拼图验证码（基于 wenlng/go-captcha v2）
//
// 两步流程：
//  1. 前端 GET 取拼图（背景大图 + 拼图块），后端把正确缺口 X 存入 CaptchaStore；
//  2. 前端把拼图块拖到缺口，POST 提交落点 x，后端在容差内校验通过后签发一次性令牌；
//  3. 登录时提交该一次性令牌，后端消费（一次性作废）即视为通过验证码。
//
// 存储复用已有的 CaptchaStore（内存、带 TTL、一次性），key 加前缀隔离命名空间。
// ============================================================================

const (
	slideAnswerPrefix = "slide:ans:"    // 缺口正确 X 的存储前缀
	slideTokenPrefix  = "slide:tok:"    // 一次性通过令牌的存储前缀
	slideTolerance    = 3               // 落点与缺口 X 的容差(像素)；容差越大越容易被脚本随机撞中
	slideImgWidth     = 300             // 背景大图宽(像素，前端按此原尺寸渲染，保证坐标 1:1)
	slideImgHeight    = 180             // 背景大图高
	slideTokenTTL     = 5 * time.Minute // 一次性令牌有效期：通过验证后需在此时长内提交登录
)

var (
	slideBuilderOnce sync.Once
	slideBuilder     slide.Builder
	slideBuilderErr  error
)

// getSlideBuilder 懒加载滑块构建器（资源初始化一次即可复用）
func getSlideBuilder() (slide.Builder, error) {
	slideBuilderOnce.Do(func() {
		b := slide.NewBuilder(slide.WithImageSize(option.Size{Width: slideImgWidth, Height: slideImgHeight}))
		imgs, err := assetsImages.GetImages()
		if err != nil {
			slideBuilderErr = err
			return
		}
		tiles, err := assetsTiles.GetTiles()
		if err != nil {
			slideBuilderErr = err
			return
		}
		graphs := make([]*slide.GraphImage, 0, len(tiles))
		for _, t := range tiles {
			graphs = append(graphs, &slide.GraphImage{
				OverlayImage: t.OverlayImage,
				ShadowImage:  t.ShadowImage,
				MaskImage:    t.MaskImage,
			})
		}
		b.SetResources(slide.WithBackgrounds(imgs), slide.WithGraphImages(graphs))
		slideBuilder = b
	})
	return slideBuilder, slideBuilderErr
}

// SlideCaptchaData 下发给前端的拼图数据（不含正确答案 X）
type SlideCaptchaData struct {
	ID           string `json:"id"`            // 验证码 ID（校验时回传）
	MasterB64    string `json:"master_image"`  // 背景大图(带缺口) base64（data URI）
	TileB64      string `json:"tile_image"`    // 拼图块 base64（data URI）
	MasterWidth  int    `json:"master_width"`  // 背景大图宽（前端按此原尺寸渲染）
	MasterHeight int    `json:"master_height"` // 背景大图高
	TileX        int    `json:"tile_x"`        // 拼图块初始显示 x（图片像素坐标）
	TileY        int    `json:"tile_y"`        // 拼图块显示 y
	TileWidth    int    `json:"tile_width"`
	TileHeight   int    `json:"tile_height"`
}

// GenerateSlideCaptcha 生成一道滑动拼图，正确缺口 X 存入 CaptchaStore（一次性、带 TTL）
func GenerateSlideCaptcha() (*SlideCaptchaData, error) {
	b, err := getSlideBuilder()
	if err != nil {
		return nil, err
	}
	capt := b.Make()
	data, err := capt.Generate()
	if err != nil {
		return nil, err
	}
	block := data.GetData()
	master, err := data.GetMasterImage().ToBase64()
	if err != nil {
		return nil, err
	}
	tile, err := data.GetTileImage().ToBase64()
	if err != nil {
		return nil, err
	}

	id := uuid.NewString()
	// 存正确缺口 X（登录/校验时不下发给前端）
	_ = CaptchaStore.Set(slideAnswerPrefix+id, strconv.Itoa(block.X))

	return &SlideCaptchaData{
		ID:           id,
		MasterB64:    master,
		TileB64:      tile,
		MasterWidth:  slideImgWidth,
		MasterHeight: slideImgHeight,
		TileX:        block.DX,
		TileY:        block.DY,
		TileWidth:    block.Width,
		TileHeight:   block.Height,
	}, nil
}

// VerifySlideCaptcha 校验拼图落点 x 是否命中缺口（容差内）；无论成败都作废该题（一次性）
func VerifySlideCaptcha(id string, x int) bool {
	if id == "" {
		return false
	}
	val := CaptchaStore.Get(slideAnswerPrefix+id, true) // clear=true：一次性
	if val == "" {
		return false
	}
	want, err := strconv.Atoi(val)
	if err != nil {
		return false
	}
	d := x - want
	if d < 0 {
		d = -d
	}
	return d <= slideTolerance
}

// slideTokenPayload 一次性通过令牌的内容
//
// 安全要点：
//   - 绑定签发时的客户端 IP 与 User-Agent：令牌被脚本搬运/批量囤积到其他来源后无法使用；
//   - 绑定签发它的验证码类型（slide / click）：登录配置为 click 时，攻击者不能拿
//     slide 接口签发的令牌混进登录（历史漏洞：两种验证码共用无类型的一次性令牌，
//     管理员把类型切换成更强的 click 后，攻击者仍可调用 slide 接口用更弱且可脚本化的滑块刷令牌）。
type slideTokenPayload struct {
	IP   string `json:"ip"`   // 签发时的客户端 IP
	UA   string `json:"ua"`   // 签发时 User-Agent 的摘要
	Type string `json:"type"` // 签发该令牌的验证码类型：slide / click
	TS   int64  `json:"ts"`   // 签发时间（Unix 秒），用于校验有效期
}

// IssueSlideToken 滑块/点击验证通过后签发一次性令牌（登录时消费）
// ip、ua 为当前请求的客户端信息，captchaType 为签发该令牌的验证码类型，
// 三者都会在消费时做一致性校验。
func IssueSlideToken(ip, ua, captchaType string) string {
	if captchaType == "" {
		return ""
	}
	token := uuid.NewString()
	payload := slideTokenPayload{
		IP:   ip,
		UA:   fingerprintUA(ua),
		Type: captchaType,
		TS:   time.Now().Unix(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	_ = CaptchaStore.Set(slideTokenPrefix+token, string(raw))
	return token
}

// ConsumeSlideToken 消费一次性令牌：
//   - 令牌必须存在且未被使用过（取出即作废，防重放）；
//   - 必须在有效期内；
//   - IP 与 User-Agent 必须与签发时一致（防止跨客户端搬运令牌）；
//   - 验证码类型必须与当前登录配置一致（防止用未启用的弱验证码换取登录令牌）。
func ConsumeSlideToken(token, ip, ua, expectedType string) bool {
	if token == "" || expectedType == "" {
		return false
	}

	raw := CaptchaStore.Get(slideTokenPrefix+token, true)
	if raw == "" {
		return false
	}

	var payload slideTokenPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return false
	}

	if payload.Type != expectedType {
		return false
	}

	if time.Since(time.Unix(payload.TS, 0)) > slideTokenTTL {
		return false
	}

	return payload.IP == ip && payload.UA == fingerprintUA(ua)
}

// fingerprintUA 生成 User-Agent 的短摘要，用于令牌绑定而不必保存完整 UA
func fingerprintUA(ua string) string {
	sum := sha256.Sum256([]byte(ua))
	return hex.EncodeToString(sum[:8])
}
