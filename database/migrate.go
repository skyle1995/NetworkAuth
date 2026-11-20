package database

import (
	"strconv"

	"NetworkAuth/models"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
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
// ReorderAPIIDs 把 apis 表的自增 id 按「接口分类顺序」重排成 1..N。
//
// 背景：接口是安装时按分类顺序批量创建的，id 天然就是分类顺序；
// 但旧库如果经历过删除/重建某些条目，id 会出现跳号，
// 后台按 id 排序时就会看到 1、2、5、7 这种不连贯的编号。
//
// 幂等：已经是 1..N 且顺序与分类一致时直接跳过，不会每次启动都改。
func ReorderAPIIDs() error {
	db, err := GetDB()
	if err != nil {
		return err
	}
	if !db.Migrator().HasTable(&models.API{}) {
		return nil
	}

	type apiRow struct {
		ID      uint
		APIType int
	}
	var rows []apiRow
	if err := db.Table("apis").Select("id, api_type").Order("id ASC").Scan(&rows).Error; err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	// 目标顺序：先按接口分类顺序，再是分类表里没有的历史条目（保持原 id 相对顺序）
	rank := make(map[int]int, len(rows))
	for i, t := range models.GetDefaultAPITypes() {
		rank[t] = i
	}
	ordered := make([]apiRow, 0, len(rows))
	for _, t := range models.GetDefaultAPITypes() {
		for _, r := range rows {
			if r.APIType == t {
				ordered = append(ordered, r)
				break
			}
		}
	}
	for _, r := range rows {
		if _, known := rank[r.APIType]; !known {
			ordered = append(ordered, r)
		}
	}

	// 已就位（id 依次为 1..N 且顺序一致）→ 跳过
	aligned := true
	for i, r := range ordered {
		if r.ID != uint(i+1) {
			aligned = false
			break
		}
	}
	if aligned {
		return nil
	}

	// 两段式重排：先把 id 整体上移到一个足够大的偏移（避开主键冲突且不产生负数，
	// MySQL 的 bigint unsigned 不接受负值），再逐行写回目标 id。
	const idOffset = 100000
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Exec("UPDATE apis SET id = id + ?", idOffset).Error; err != nil {
			return err
		}
		for i, r := range ordered {
			if err := tx.Exec("UPDATE apis SET id = ? WHERE id = ?", i+1, r.ID+idOffset).Error; err != nil {
				return err
			}
		}
		// 自增起点跟到 N+1，下一步新增不会跳号
		switch tx.Dialector.Name() {
		case "sqlite":
			if err := tx.Exec("UPDATE sqlite_sequence SET seq = ? WHERE name = 'apis'", len(ordered)).Error; err != nil {
				logrus.WithError(err).Debug("重置 sqlite_sequence 失败（可忽略）")
			}
		default:
			if err := tx.Exec("ALTER TABLE apis AUTO_INCREMENT = " + strconv.Itoa(len(ordered)+1)).Error; err != nil {
				logrus.WithError(err).Debug("重置 AUTO_INCREMENT 失败（可忽略）")
			}
		}
		logrus.WithField("count", len(ordered)).Info("已按分类顺序重排接口 id")
		return nil
	})
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
