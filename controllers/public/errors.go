package public

import "errors"

// 公开 API 通用错误
var (
	errUnsupported = errors.New("不支持的接口类型")
	errBadParams   = errors.New("参数格式错误")
)
