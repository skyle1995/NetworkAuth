package services

import (
	"NetworkAuth/models"
	"strconv"

	"gorm.io/gorm"
)

// setTestQuota 在单个用例内收紧/放开账号级配额（设备数 / 并发数 / 设备免费转绑次数）。
func setTestQuota(db *gorm.DB, devices, concurrent, rebindFree int) {
	items := map[string]string{
		models.SettingAccountMaxDevices:            itoa(devices),
		models.SettingAccountMaxConcurrent:         itoa(concurrent),
		models.SettingAccountMachineRebindFreeCount: itoa(rebindFree),
	}
	for k, v := range items {
		db.Where("name = ?", k).Delete(&models.Settings{})
		db.Create(&models.Settings{Name: k, Value: v})
	}
	ResetSettingsService()
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

// seedTestQuota 给测试库写入宽松的账号级配额。
func seedTestQuota(db *gorm.DB) {
	quota := map[string]string{
		models.SettingAccountMaxDevices:             "20",
		models.SettingAccountMaxConcurrent:          "20",
		models.SettingAccountMachineRebindFreeCount: "20",
		models.SettingAccountMachineRebindLimitTime: "1",
		models.SettingAccountIPRebindFreeCount:      "20",
		models.SettingAccountIPRebindLimitTime:      "1",
	}
	for k, v := range quota {
		db.Where("name = ?", k).Delete(&models.Settings{})
		db.Create(&models.Settings{Name: k, Value: v})
	}
	ResetSettingsService()
}

// setRebindPolicy 设置两套转绑策略：设备(开关/免费/窗口/上限/扣除) 与 IP(开关/免费/窗口/上限/扣除)。
func setRebindPolicy(db *gorm.DB, mEnabled, ipEnabled, mFree, mWindow, mMax, mDeduct, ipFree, ipWindow, ipMax, ipDeduct int) {
	items := map[string]string{
		models.SettingAccountMachineRebindEnabled:   itoa(mEnabled),
		models.SettingAccountMachineRebindFreeCount: itoa(mFree),
		models.SettingAccountMachineRebindLimitTime: itoa(mWindow),
		models.SettingAccountMachineRebindMaxCount:  itoa(mMax),
		models.SettingAccountMachineRebindDeductMinutes: itoa(mDeduct),
		models.SettingAccountMachineRebindDeductPoints:  itoa(mDeduct),
		models.SettingAccountIPRebindEnabled:         itoa(ipEnabled),
		models.SettingAccountIPRebindFreeCount:       itoa(ipFree),
		models.SettingAccountIPRebindLimitTime:       itoa(ipWindow),
		models.SettingAccountIPRebindMaxCount:        itoa(ipMax),
		models.SettingAccountIPRebindDeductMinutes: itoa(ipDeduct),
		models.SettingAccountIPRebindDeductPoints:  itoa(ipDeduct),
	}
	for k, v := range items {
		db.Where("name = ?", k).Delete(&models.Settings{})
		db.Create(&models.Settings{Name: k, Value: v})
	}
	ResetSettingsService()
}

// setMachineVerify 设置账号级设备校验开关（0 不校验 / 1 校验）。
func setMachineVerify(db *gorm.DB, on int) {
	db.Where("name = ?", models.SettingAccountMachineVerify).Delete(&models.Settings{})
	db.Create(&models.Settings{Name: models.SettingAccountMachineVerify, Value: itoa(on)})
	ResetSettingsService()
}

// setIPVerify 设置账号级 IP 校验强度。
func setIPVerify(db *gorm.DB, level int) {
	db.Where("name = ?", models.SettingAccountIPVerify).Delete(&models.Settings{})
	db.Create(&models.Settings{Name: models.SettingAccountIPVerify, Value: itoa(level)})
	ResetSettingsService()
}

// setRebindWindow 兼容旧调用：同时设置设备与 IP 的计数窗口。
func setRebindWindow(db *gorm.DB, window int) {
	db.Where("name = ?", models.SettingAccountMachineRebindLimitTime).Delete(&models.Settings{})
	db.Create(&models.Settings{Name: models.SettingAccountMachineRebindLimitTime, Value: itoa(window)})
	db.Where("name = ?", models.SettingAccountIPRebindLimitTime).Delete(&models.Settings{})
	db.Create(&models.Settings{Name: models.SettingAccountIPRebindLimitTime, Value: itoa(window)})
	ResetSettingsService()
}