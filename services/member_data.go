package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/utils"
	"encoding/base64"
	"errors"
	"strings"

	"gorm.io/gorm"
)

// ============================================================================
// 客户端数据类接口与用户自助操作
// ============================================================================
//
// 这些接口均要求“已登录且账号可用”（authActiveMember 校验令牌+状态+到期）。

// GetAppData 获取程序数据（type 42）：返回应用的 AppData（base64 存储，解码后返回）。
func GetAppData(appUUID, token string) (any, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	member, _, err := authActiveMember(db, appUUID, token)
	if err != nil {
		return nil, err
	}
	var app models.App
	if err := db.Where("uuid = ?", strings.TrimSpace(appUUID)).First(&app).Error; err != nil {
		return nil, errors.New("应用不存在")
	}
	data := app.AppData
	if decoded, derr := base64.StdEncoding.DecodeString(app.AppData); derr == nil {
		data = string(decoded)
	}
	// 同时返回该账号的独有用户数据
	return map[string]any{"data": data, "user_data": member.Data}, nil
}

// maxUserDataBytes 账号数据最大字节数（16MB），防止客户端写入过大内容。
// 列类型为 longtext（MySQL 上限 4GB / SQLite 无限），实际写入上限由本常量控制。
const maxUserDataBytes = 16 * 1024 * 1024

// GetUserData 获取账号数据（type 45）：返回当前登录用户的独有数据。
func GetUserData(appUUID, token string) (any, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	member, _, err := authActiveMember(db, appUUID, token)
	if err != nil {
		return nil, err
	}
	return map[string]any{"user_data": member.Data}, nil
}

// SetUserData 设置账号数据（type 54）：写入当前登录用户的独有数据（覆盖式）。
func SetUserData(appUUID, token, data string) (any, error) {
	if len(data) > maxUserDataBytes {
		return nil, errors.New("账号数据过大，最大 16MB")
	}
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	member, _, err := authActiveMember(db, appUUID, token)
	if err != nil {
		return nil, err
	}
	if err := db.Model(member).Update("data", data).Error; err != nil {
		return nil, err
	}
	return map[string]any{"message": "保存成功"}, nil
}

// GetVariable 获取变量数据（type 43）：按别名返回本应用或全局变量的数据。
func GetVariable(appUUID, token, alias string) (any, error) {
	alias = strings.TrimSpace(alias)
	if alias == "" {
		return nil, errors.New("变量别名不能为空")
	}
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	if _, _, err := authActiveMember(db, appUUID, token); err != nil {
		return nil, err
	}
	// 别名全局唯一，限定属于本应用或全局("0")
	var variable models.Variable
	if err := db.Where("alias = ? AND app_uuid IN ?", alias, []string{strings.TrimSpace(appUUID), "0"}).
		First(&variable).Error; err != nil {
		return nil, errors.New("变量不存在")
	}
	return map[string]any{"alias": variable.Alias, "data": variable.Data}, nil
}

// GetFunction 获取远程函数代码（type 44）：按别名返回本应用或全局函数的代码。
func GetFunction(appUUID, token, alias string) (any, error) {
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
	var function models.Function
	if err := db.Where("alias = ? AND app_uuid IN ?", alias, []string{strings.TrimSpace(appUUID), "0"}).
		First(&function).Error; err != nil {
		return nil, errors.New("函数不存在")
	}
	return map[string]any{"alias": function.Alias, "code": function.Code}, nil
}

// ChangeMemberPassword 修改账号密码（type 50）：校验旧密码后设置新密码。
// 仅注册账号支持；卡密账号无密码，不支持修改。
func ChangeMemberPassword(appUUID, token, oldPassword, newPassword string) (any, error) {
	if strings.TrimSpace(newPassword) == "" {
		return nil, errors.New("新密码不能为空")
	}
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	member, _, err := authActiveMember(db, appUUID, token)
	if err != nil {
		return nil, err
	}
	if member.Type != models.MemberTypeRegister || member.PasswordSalt == "" {
		return nil, errors.New("该账号不支持修改密码")
	}
	if !utils.VerifyPasswordWithSalt(oldPassword, member.PasswordSalt, member.Password) {
		return nil, errors.New("原密码错误")
	}

	salt, err := utils.GenerateRandomSalt()
	if err != nil {
		return nil, err
	}
	hashed, err := utils.HashPasswordWithSalt(newPassword, salt)
	if err != nil {
		return nil, err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(member).Updates(map[string]interface{}{
			"password":      hashed,
			"password_salt": salt,
		}).Error; err != nil {
			return err
		}
		// 改密后清除全部会话，强制重新登录
		return tx.Where("member_uuid = ?", member.UUID).Delete(&models.MemberSession{}).Error
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"message": "密码修改成功，请重新登录"}, nil
}

// ResetPasswordByCode 找回密码（type 26）：邮箱验证码校验通过后重设密码，无需登录/旧密码。
// 仅注册账号支持；重置后清空该账号全部会话。
func ResetPasswordByCode(appUUID, email, code, newPassword string) (any, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if strings.TrimSpace(newPassword) == "" {
		return nil, errors.New("新密码不能为空")
	}
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	if _, err := loadEnabledApp(db, appUUID); err != nil {
		return nil, err
	}
	// 先校验验证码
	if err := VerifyResetCode(appUUID, email, code); err != nil {
		return nil, err
	}

	var member models.Member
	if err := db.Where("app_uuid = ? AND username = ?", strings.TrimSpace(appUUID), email).First(&member).Error; err != nil {
		return nil, errors.New("该邮箱未注册")
	}
	if member.Type != models.MemberTypeRegister || member.PasswordSalt == "" {
		return nil, errors.New("该账号不支持找回密码")
	}

	salt, err := utils.GenerateRandomSalt()
	if err != nil {
		return nil, err
	}
	hashed, err := utils.HashPasswordWithSalt(newPassword, salt)
	if err != nil {
		return nil, err
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&member).Updates(map[string]interface{}{
			"password":      hashed,
			"password_salt": salt,
		}).Error; err != nil {
			return err
		}
		// 重置后清除全部会话
		return tx.Where("member_uuid = ?", member.UUID).Delete(&models.MemberSession{}).Error
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"message": "密码重置成功，请用新密码登录"}, nil
}
