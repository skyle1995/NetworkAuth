package utils

import (
	"encoding/json"
	"sync"

	"github.com/google/uuid"
	"github.com/golang/freetype/truetype"
	assetsChars "github.com/wenlng/go-captcha-assets/bindata/chars"
	assetsImages "github.com/wenlng/go-captcha-assets/resources/images"
	assetsFont "github.com/wenlng/go-captcha-assets/resources/fonts/fzshengsksjw"
	"github.com/wenlng/go-captcha/v2/base/option"
	"github.com/wenlng/go-captcha/v2/click"
)

// ============================================================================
// 点击文字验证码（基于 wenlng/go-captcha v2 click）
//
// 两步流程（与滑块一致）：
//  1. 前端 GET 取图：master(打乱的汉字大图) + thumb(提示按此顺序点击的缩略图)，
//     后端把「需点击的有序汉字框坐标」存入 CaptchaStore；
//  2. 前端在 master 上依次点击，POST 提交各点击点，后端逐点在框内(带 padding)校验，
//     全部命中且顺序正确则签发一次性令牌；
//  3. 登录时消费该令牌（与滑块共用令牌机制）。
// ============================================================================

const (
	clickAnswerPrefix = "click:ans:" // 有序点击框坐标(JSON)的存储前缀
	clickImgWidth     = 300          // master 大图宽(前端按此原尺寸渲染，坐标 1:1)
	clickImgHeight    = 200          // master 大图高
	clickPadding      = 4            // 命中判定的容差(像素)
)

var (
	clickBuilderOnce sync.Once
	clickBuilder     click.Builder
	clickBuilderErr  error
)

func getClickBuilder() (click.Builder, error) {
	clickBuilderOnce.Do(func() {
		b := click.NewBuilder(click.WithImageSize(option.Size{Width: clickImgWidth, Height: clickImgHeight}))
		fs, err := assetsFont.GetFont()
		if err != nil {
			clickBuilderErr = err
			return
		}
		bgs, err := assetsImages.GetImages()
		if err != nil {
			clickBuilderErr = err
			return
		}
		b.SetResources(
			click.WithChars(assetsChars.GetChineseChars()),
			click.WithFonts([]*truetype.Font{fs}),
			click.WithBackgrounds(bgs),
		)
		clickBuilder = b
	})
	return clickBuilder, clickBuilderErr
}

// clickDot 需点击的汉字框（左上角 + 宽高）
type clickDot struct {
	X, Y, W, H int
}

// ClickCaptchaData 下发给前端的点击文字数据（不含答案坐标）
type ClickCaptchaData struct {
	ID           string `json:"id"`
	MasterB64    string `json:"master_image"` // 打乱汉字的大图
	ThumbB64     string `json:"thumb_image"`  // 提示按序点击的缩略图
	MasterWidth  int    `json:"master_width"`
	MasterHeight int    `json:"master_height"`
	DotCount     int    `json:"dot_count"` // 需点击的字数
}

// ClickPoint 前端提交的一次点击（master 图片像素坐标）
type ClickPoint struct {
	X int `json:"x"`
	Y int `json:"y"`
}

// GenerateClickCaptcha 生成一道点击文字验证码，有序答案框存入 CaptchaStore
func GenerateClickCaptcha() (*ClickCaptchaData, error) {
	b, err := getClickBuilder()
	if err != nil {
		return nil, err
	}
	capt := b.Make()
	data, err := capt.Generate()
	if err != nil {
		return nil, err
	}
	master, err := data.GetMasterImage().ToBase64()
	if err != nil {
		return nil, err
	}
	thumb, err := data.GetThumbImage().ToBase64()
	if err != nil {
		return nil, err
	}

	// 按 index 顺序整理答案框
	dotMap := data.GetData()
	dots := make([]clickDot, len(dotMap))
	for i := 0; i < len(dotMap); i++ {
		if d, ok := dotMap[i]; ok {
			dots[i] = clickDot{X: d.X, Y: d.Y, W: d.Width, H: d.Height}
		}
	}
	raw, err := json.Marshal(dots)
	if err != nil {
		return nil, err
	}

	id := uuid.NewString()
	_ = CaptchaStore.Set(clickAnswerPrefix+id, string(raw))

	return &ClickCaptchaData{
		ID:           id,
		MasterB64:    master,
		ThumbB64:     thumb,
		MasterWidth:  clickImgWidth,
		MasterHeight: clickImgHeight,
		DotCount:     len(dots),
	}, nil
}

// VerifyClickCaptcha 校验有序点击点是否全部命中；无论成败都作废该题(一次性)
func VerifyClickCaptcha(id string, points []ClickPoint) bool {
	if id == "" {
		return false
	}
	val := CaptchaStore.Get(clickAnswerPrefix+id, true) // 一次性
	if val == "" {
		return false
	}
	var dots []clickDot
	if err := json.Unmarshal([]byte(val), &dots); err != nil {
		return false
	}
	if len(points) != len(dots) || len(dots) == 0 {
		return false
	}
	for i, d := range dots {
		if !click.Validate(points[i].X, points[i].Y, d.X, d.Y, d.W, d.H, clickPadding) {
			return false
		}
	}
	return true
}
