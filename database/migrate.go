package database

import (
	"NetworkAuth/models"

	"github.com/sirupsen/logrus"
)

// AutoMigrate 自动迁移数据库模型
// - 会确保必要的数据表结构存在
// - 不会破坏已有数据
func AutoMigrate() error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	if err := db.AutoMigrate(AllModels()...); err != nil {
		logrus.WithError(err).Error("AutoMigrate 执行失败")
		return err
	}
	logrus.Info("AutoMigrate 执行完成")
	return nil
}


// MigrateVariablePerApp 把变量表从「alias 全局唯一」改成「(app_uuid, alias) 唯一」，
// 让每个应用可以各有一把 zex_key。
// 幂等：索引已是新形态时直接跳过。
func MigrateVariablePerApp() error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&models.Variable{}) {
		return nil
	}
	// 旧索引名由 GORM 按字段生成：idx_variables_alias（唯一）
	if db.Migrator().HasIndex(&models.Variable{}, "idx_variables_alias") {
		if err := db.Exec("DROP INDEX IF EXISTS idx_variables_alias").Error; err != nil {
			logrus.WithError(err).Warn("删除 variables.alias 唯一索引失败（可忽略）")
		}
	}
	if !db.Migrator().HasIndex(&models.Variable{}, "idx_var_app_alias") {
		if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_var_app_alias ON variables(app_uuid, alias)").Error; err != nil {
			return err
		}
	}
	return nil
}

// MigrateBillingFlags 把旧的单一 billing_mode 枚举拆成三个正交开关：
//
//	bill_time_enabled   计时（要账号有效期）
//	bill_points_enabled 点数（扣点）
//	level_gate_enabled  会员限制（要求等级 ≥ required_level）
//
// 旧 → 新映射：
//
//	mode 0 计时 → time=1, points=0, level=(required_level>1)
//	mode 1 点数 → time=0, points=1, level=(required_level>1)
//	mode 2 高级 → time=0, points=1, level=1
//
// 幂等：完成后写 settings 标记 billing_flags_migrated，之后不再执行，
// 避免把用户后来手动关掉的开关又打开。
func MigrateBillingFlags() error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&models.App{}) || !db.Migrator().HasTable(&models.Settings{}) {
		return nil
	}

	const marker = "billing_flags_migrated"
	var done int64
	if err := db.Model(&models.Settings{}).Where("name = ?", marker).Count(&done).Error; err != nil {
		return err
	}
	if done > 0 {
		return nil
	}

	stmts := []string{
		`UPDATE apps SET bill_time_enabled = 1, bill_points_enabled = 0,
		        level_gate_enabled = CASE WHEN required_level > 1 THEN 1 ELSE 0 END
		   WHERE billing_mode = 0`,
		`UPDATE apps SET bill_time_enabled = 0, bill_points_enabled = 1,
		        level_gate_enabled = CASE WHEN required_level > 1 THEN 1 ELSE 0 END
		   WHERE billing_mode = 1`,
		`UPDATE apps SET bill_time_enabled = 0, bill_points_enabled = 1, level_gate_enabled = 1
		   WHERE billing_mode = 2`,
	}
	for _, sql := range stmts {
		if err := db.Exec(sql).Error; err != nil {
			logrus.WithError(err).Error("拆分计费开关失败")
			return err
		}
	}

	if err := db.Create(&models.Settings{
		Name:        marker,
		Value:       "1",
		Description: "计费模式已拆分为计时/点数/会员限制三个开关，标记防止重复迁移",
	}).Error; err != nil {
		return err
	}
	logrus.Info("计费模式已拆分为三个开关（计时/点数/会员限制）")
	return nil
}

// BackfillDefaultMemberLevel 给「未分配等级」的账号补上默认档。
//
// 背景：门槛为 0 的等级是全站免费档，任何账号都该属于它。
// 早期建号路径只写 total_recharge 不写 level_uuid，导致账号等级为空；
// 显示层会兜底成该等级名，但数据本身是空的，按「等级=XX」筛选时会对不上。
//
// 幂等：只处理 level_uuid 为空的账号；库里没有 threshold<=0 的启用等级时直接跳过。
func BackfillDefaultMemberLevel() error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&models.MemberLevel{}) || !db.Migrator().HasTable(&models.Member{}) {
		return nil
	}

	var lv models.MemberLevel
	if err := db.Where("status = 1 AND threshold <= 0").Order("threshold DESC").First(&lv).Error; err != nil {
		return nil // 没有默认档，无需回填
	}

	res := db.Model(&models.Member{}).
		Where("level_uuid IS NULL OR level_uuid = ''").
		Update("level_uuid", lv.UUID)
	if res.Error != nil {
		logrus.WithError(res.Error).Error("回填默认会员等级失败")
		return res.Error
	}
	if res.RowsAffected > 0 {
		logrus.Infof("已为 %d 个账号回填默认等级「%s」", res.RowsAffected, lv.Name)
	}
	return nil
}
// NormalizeUpdateStrategy 一次性把旧版「强制更新(force_update) + 更新方式(download_type: 自动/手动)」
// 规整为合并后的新三态 download_type（0不启用/1强制/2自由），随后删除已废弃的 force_update 列：
//   - 启用更新(download_type<>0)且原为强制 → download_type=1（强制）
//   - 启用更新且原非强制 → download_type=2（自由）
//   - download_type=0 保持不启用
// 幂等：force_update 列不存在（新库或已迁移过）时直接跳过。
func NormalizeUpdateStrategy() error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	m := db.Migrator()
	// 列已不存在（全新库或已迁移过）→ 无需处理
	if !m.HasColumn(&models.App{}, "force_update") {
		return nil
	}
	// 按旧 force_update 把已启用更新的应用映射到新方式（WHERE/SET 直接用列名，不依赖模型字段）
	if err := db.Model(&models.App{}).
		Where("download_type <> ? AND force_update = 1", models.DownloadTypeDisabled).
		Update("download_type", models.DownloadTypeForce).Error; err != nil {
		return err
	}
	if err := db.Model(&models.App{}).
		Where("download_type <> ? AND force_update = 0", models.DownloadTypeDisabled).
		Update("download_type", models.DownloadTypeFree).Error; err != nil {
		return err
	}
	// 删除废弃列（modernc/sqlite 支持 ALTER TABLE DROP COLUMN）
	return db.Exec("ALTER TABLE apps DROP COLUMN force_update").Error
}

// AllModels 返回参与迁移与结构整理的全部模型清单。
// AutoMigrate 与 db tidy 维护命令共用此清单，避免两处各维护一份导致遗漏。
func AllModels() []any {
	return []any{
		&models.Settings{},
		&models.PortalNavigation{},
		&models.OperationLog{},
		&models.LoginLog{},
		&models.User{},
		&models.App{},
		&models.AppVersion{},
		&models.API{},
		&models.Variable{},
		&models.Function{},
		&models.RefreshToken{},
		&models.ApiKey{},
		&models.CardPackage{},
		&models.MemberLevel{},
		// 一号通：混合模式的「应用 × 会员等级 → 点数消耗」配置表。
		// 必须在迁移清单里，否则全新安装后该表不存在，混合模式一用就报 no such table。
		&models.AppLevelPoint{},
		&models.Card{},
		&models.Member{},
		&models.Binding{},
		
		// 二级令牌：账号令牌（身份）+ 应用激活（使用权/租约）。
		// 与 member_sessions 并存期：member_sessions 退场前两者都要建表。
		&models.MemberAccountToken{},
		&models.AppActivation{}, &models.AppAccountBilling{},
		&models.MemberLog{},
		&models.Blacklist{},
	}
}
