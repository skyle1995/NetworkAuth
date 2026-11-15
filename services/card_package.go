package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"errors"
	"strings"

	"gorm.io/gorm"
)

// ============================================================================
// 卡密套餐：制卡的售卖单元
// ============================================================================
//
// 制卡时把套餐的面值（Duration/Points）与售价（Price）快照进 Card，
// 套餐后续改动不影响已售出的卡，三处核销逻辑读的仍是 Card 自身字段。

// ListCardPackages 列出卡密套餐（全站一份）。onlyEnabled 时只返回启用的。
func ListCardPackages(onlyEnabled bool) ([]models.CardPackage, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	var pkgs []models.CardPackage
	q := db.Model(&models.CardPackage{})
	if onlyEnabled {
		q = q.Where("status = 1")
	}
	if err := q.Order("sort ASC, id ASC").Find(&pkgs).Error; err != nil {
		return nil, err
	}
	return pkgs, nil
}

// SaveCardPackage 新增或更新卡密套餐。UUID 为空则新增。
// 面值按类型校验：时长套餐须有时长（含永久 -1），点数套餐须点数>0，避免生成 0 值废卡。
func SaveCardPackage(pkg *models.CardPackage) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	pkg.Name = strings.TrimSpace(pkg.Name)
	if pkg.Name == "" {
		return errors.New("套餐名称不能为空")
	}
	if err := validatePackageValue(pkg); err != nil {
		return err
	}
	if pkg.Price < 0 {
		return errors.New("售价不能为负")
	}

	if strings.TrimSpace(pkg.UUID) == "" {
		return db.Create(pkg).Error
	}
	var exists models.CardPackage
	if err := db.Where("uuid = ?", pkg.UUID).First(&exists).Error; err != nil {
		return errors.New("套餐不存在")
	}
	return db.Model(&exists).Updates(map[string]interface{}{
		"name":     pkg.Name,
		"type":     pkg.Type,
		"duration": pkg.Duration,
		"points":   pkg.Points,
		"price":    pkg.Price,
		"sort":     pkg.Sort,
		"status":   pkg.Status,
		"remark":   pkg.Remark,
	}).Error
}

// validatePackageValue 按套餐类型校验面值，并清零另一模式的无关字段
func validatePackageValue(pkg *models.CardPackage) error {
	switch pkg.Type {
	case models.PackageTypePoints:
		if pkg.Points <= 0 {
			return errors.New("点数套餐的面值点数必须大于0")
		}
		pkg.Duration = 0
	case models.PackageTypeTime:
		if pkg.Duration == 0 {
			return errors.New("时长套餐的面值时长不能为0")
		}
		if pkg.Duration < models.CardDurationPermanent {
			return errors.New("面值时长无效")
		}
		pkg.Points = 0
	default:
		return errors.New("套餐类型无效")
	}
	return nil
}

// DeleteCardPackage 删除套餐。已售出的卡密已快照面值，不受影响，仅清除来源引用。
func DeleteCardPackage(uuid string) error {
	uuid = strings.TrimSpace(uuid)
	if uuid == "" {
		return errors.New("套餐UUID不能为空")
	}
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Card{}).Where("package_uuid = ?", uuid).
			Update("package_uuid", "").Error; err != nil {
			return err
		}
		return tx.Where("uuid = ?", uuid).Delete(&models.CardPackage{}).Error
	})
}

// checkPackageValue 校验套餐面值有效（卡密全站通用，不再限定应用计费模式）。
// 一张卡可同时带时长与点数，核销时按面值非零项各自发放。
func checkPackageValue(pkg *models.CardPackage) error {
	if pkg.Duration == 0 && pkg.Points <= 0 {
		return errors.New("套餐面值无效：时长与点数至少配置一项")
	}
	return nil
}

// LoadEnabledPackage 取启用中的套餐（全站一份，无需应用维度）。
func LoadEnabledPackage(db *gorm.DB, packageUUID string) (*models.CardPackage, error) {
	packageUUID = strings.TrimSpace(packageUUID)
	if packageUUID == "" {
		return nil, errors.New("请选择卡密套餐")
	}
	var pkg models.CardPackage
	if err := db.Where("uuid = ?", packageUUID).First(&pkg).Error; err != nil {
		return nil, errors.New("套餐不存在")
	}
	if pkg.Status != 1 {
		return nil, errors.New("套餐已禁用")
	}
	return &pkg, nil
}
