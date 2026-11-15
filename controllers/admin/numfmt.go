package admin

import (
	"math"
	"strconv"
)

// fmtNum 把点数格式化成人类可读形式：
// 整数不带小数点（480），小数最多保留原始精度（1.5 / 0.25）。
//
// 点数现在支持小数，日志与提示里统一走这个函数，
// 避免出现「充值 1.5000000000000002 点」这种浮点尾巴。
func fmtNum(v float64) string {
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}
