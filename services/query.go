package services

import (
	"context"
	"fmt"
	"NetworkAuth/models"
	"NetworkAuth/utils"
	"time"

	"gorm.io/gorm"
)

// ============================================================================
// 查询函数
// ============================================================================

// FindSettingByName 根据名称查找设置
// name: 设置名称
// db: 数据库连接
// 返回: 设置信息和错误
func FindSettingByName(name string, db *gorm.DB) (*models.Settings, error) {
	key := fmt.Sprintf("setting:%s", name)
	return utils.RedisGetOrSet(context.Background(), key, 5*time.Minute, func() (*models.Settings, error) {
		var setting models.Settings
		err := db.Where("name = ?", name).First(&setting).Error
		if err != nil {
			return nil, err
		}
		return &setting, nil
	})
}

// ============================================================================
// 更新函数
// ============================================================================

// UpdateEntityByID 根据ID更新实体
// model: 模型类型
// id: 实体ID
// updates: 更新字段
// db: 数据库连接
// 返回: 错误
func UpdateEntityByID(model interface{}, id uint, updates map[string]interface{}, db *gorm.DB) error {
	return db.Model(model).Where("id = ?", id).Updates(updates).Error
}

// BatchUpdateEntityStatus 批量更新实体状态
// model: 模型类型
// ids: 实体ID列表
// status: 新状态
// db: 数据库连接
// 返回: 错误
func BatchUpdateEntityStatus(model interface{}, ids []uint, status int, db *gorm.DB) error {
	if len(ids) == 0 {
		return nil
	}
	return db.Model(model).Where("id IN ?", ids).Update("status", status).Error
}

// ============================================================================
// 统计函数
// ============================================================================

// CountEntitiesByCondition 根据条件统计实体数量
// model: 模型类型
// condition: 查询条件
// db: 数据库连接
// args: 查询参数
// 返回: 数量和错误
func CountEntitiesByCondition(model interface{}, condition string, db *gorm.DB, args ...interface{}) (int64, error) {
	var count int64
	query := db.Model(model)
	if condition != "" {
		query = query.Where(condition, args...)
	}
	err := query.Count(&count).Error
	return count, err
}

// ============================================================================
// 通用查询函数
// ============================================================================

// FindEntitiesByCondition 根据条件查找实体
// model: 模型类型
// result: 结果容器
// condition: 查询条件
// db: 数据库连接
// args: 查询参数
// 返回: 错误
func FindEntitiesByCondition(model interface{}, result interface{}, condition string, db *gorm.DB, args ...interface{}) error {
	query := db.Model(model)
	if condition != "" {
		query = query.Where(condition, args...)
	}
	return query.Find(result).Error
}

// CheckEntityExists 检查实体是否存在
// model: 模型类型
// condition: 查询条件
// db: 数据库连接
// args: 查询参数
// 返回: 是否存在和错误
func CheckEntityExists(model interface{}, condition string, db *gorm.DB, args ...interface{}) (bool, error) {
	var count int64
	query := db.Model(model)
	if condition != "" {
		query = query.Where(condition, args...)
	}
	err := query.Count(&count).Error
	return count > 0, err
}

// ============================================================================
// 泛型查询函数
// ============================================================================

// Paginate 泛型分页查询
// query: 带条件的数据库查询对象
// page: 当前页码
// limit: 每页数量
// order: 排序规则 (例如 "created_at DESC")
// 返回: 数据列表, 总数, 错误
func Paginate[T any](query *gorm.DB, page, limit int, order string) ([]T, int64, error) {
	var list []T
	var total int64

	// 获取总数
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 排序
	if order != "" {
		query = query.Order(order)
	}

	// 分页查询
	offset := (page - 1) * limit
	if err := query.Offset(offset).Limit(limit).Find(&list).Error; err != nil {
		return nil, 0, err
	}

	return list, total, nil
}
