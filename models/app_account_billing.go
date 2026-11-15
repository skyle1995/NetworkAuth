package models

import "time"

// AppAccountBilling 账号在某应用的「合并计费」周期账本。
//
// 默认（BillingScope=0 单独计费）时，计费周期记在每个 activation 上，
// 同一账号在 3 台设备上就是 3 份周期费。
//
// 打开「合并计费」（BillingScope=1）后，周期提升到**账号 + 应用**维度：
// 不论同时在线几台设备，只维护一个 period_end，只按一份钱收。
// 这张表就是那份周期的落点。
//
// 单独计费的应用不会写这张表。
type AppAccountBilling struct {
	// ID：主键
	ID uint `gorm:"primaryKey;comment:主键" json:"id"`

	// MemberUUID + AppUUID：唯一确定「某账号在某应用」的计费单元
	MemberUUID string `gorm:"size:36;not null;uniqueIndex:idx_app_account_billing;comment:账号UUID" json:"member_uuid"`
	AppUUID    string `gorm:"size:36;not null;uniqueIndex:idx_app_account_billing;comment:应用UUID" json:"app_uuid"`

	// PeriodEnd：当前计费周期的结束时间，所有设备共享
	PeriodEnd *time.Time `gorm:"index;comment:合并计费周期结束时间" json:"period_end"`

	// LastChargeAt：最近一次扣费时间（排查问题用）
	LastChargeAt *time.Time `gorm:"comment:最近扣费时间" json:"last_charge_at"`

	CreatedAt time.Time `gorm:"comment:创建时间" json:"created_at"`
	UpdatedAt time.Time `gorm:"comment:更新时间" json:"updated_at"`
}

// TableName 指定表名
func (AppAccountBilling) TableName() string {
	return "app_account_billings"
}
