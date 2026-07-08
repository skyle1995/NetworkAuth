package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"errors"
	"strings"
	"time"

	"github.com/dop251/goja"
	"github.com/sirupsen/logrus"
)

// ============================================================================
// 远程函数服务端执行（type 44）
// ============================================================================
//
// 函数代码存于服务端，在 goja(纯 Go 的 JS 引擎) 沙箱内执行：客户端只传参数、收结果，
// 看不到代码逻辑（防破解）。goja 默认无文件/网络/require 能力；再加执行超时中断，
// 避免死循环拖垮服务。
//
// 约定：存储的函数代码是 function(params){ ... } 的函数体，通过 return 返回结果。

// functionExecTimeout 单次函数执行超时
const functionExecTimeout = 3 * time.Second

// ExecuteFunction 校验登录后，在沙箱内执行指定别名的远程函数并返回结果。
func ExecuteFunction(appUUID, token, alias string, params any) (any, error) {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return nil, errors.New("函数别名不能为空")
	}

	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	if _, _, err := authActiveMember(db, appUUID, token); err != nil {
		return nil, err
	}

	// 别名全局唯一，限定属于本应用或全局("0")
	var function models.Function
	if err := db.Where("alias = ? AND app_uuid IN ?", alias, []string{strings.TrimSpace(appUUID), "0"}).
		First(&function).Error; err != nil {
		return nil, errors.New("函数不存在")
	}

	return runJSFunction(function.Code, params)
}

// runJSFunction 在带超时的 goja 沙箱内执行函数体，传入 params，返回其 return 值。
func runJSFunction(code string, params any) (result any, err error) {
	vm := goja.New()

	// 超时中断：定时器从另一 goroutine 调用 Interrupt 打断执行
	timer := time.AfterFunc(functionExecTimeout, func() {
		vm.Interrupt("函数执行超时")
	})
	defer timer.Stop()

	// 兜底 recover，避免异常脚本导致 panic 冒泡
	defer func() {
		if r := recover(); r != nil {
			logrus.WithField("panic", r).Warn("remote function panicked")
			result = nil
			err = errors.New("函数执行异常")
		}
	}()

	// 将代码包装为 function(params){...}
	wrapped := "(function(params){\n" + code + "\n})"
	val, e := vm.RunString(wrapped)
	if e != nil {
		logrus.WithError(e).Warn("remote function compile failed")
		return nil, errors.New("函数编译失败")
	}
	fn, ok := goja.AssertFunction(val)
	if !ok {
		return nil, errors.New("函数格式无效")
	}

	ret, e := fn(goja.Undefined(), vm.ToValue(params))
	if e != nil {
		logrus.WithError(e).Warn("remote function execution failed")
		return nil, errors.New("函数执行失败")
	}
	return ret.Export(), nil
}
