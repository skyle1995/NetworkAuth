package utils

import (
	"strconv"
	"sync"

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
	slideAnswerPrefix = "slide:ans:" // 缺口正确 X 的存储前缀
	slideTokenPrefix  = "slide:tok:" // 一次性通过令牌的存储前缀
	slideTolerance    = 5            // 落点与缺口 X 的容差(像素)
	slideImgWidth     = 300          // 背景大图宽(像素，前端按此原尺寸渲染，保证坐标 1:1)
	slideImgHeight    = 180          // 背景大图高
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

// IssueSlideToken 滑块校验通过后签发一次性令牌（登录时消费）
func IssueSlideToken() string {
	token := uuid.NewString()
	_ = CaptchaStore.Set(slideTokenPrefix+token, "1")
	return token
}

// ConsumeSlideToken 消费一次性令牌：存在即通过并立即作废；不存在/已用过则失败
func ConsumeSlideToken(token string) bool {
	if token == "" {
		return false
	}
	return CaptchaStore.Get(slideTokenPrefix+token, true) == "1"
}
