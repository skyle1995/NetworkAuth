package models

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ============================================================================
// 常量定义
// ============================================================================

// 卡密状态常量
// 注意：卡密的状态语义与 App/User 的“0禁用/1启用”不同，单独定义避免混淆
const (
	CardStatusUnused = 0 // 未使用（库存中）
	CardStatusUsed   = 1 // 已使用（已激活或已充值核销）
	CardStatusFrozen = 2 // 已冻结/封停
)

// 永久时长标记：Duration 为该值时表示永久有效
const CardDurationPermanent = -1

// ============================================================================
// 结构体定义
// ============================================================================

// Card 卡密表模型
// 卡密是独立的“时长凭证”，制卡时不锁定用途：
//   - 卡密登录：首次使用时长出一个绑定该卡的账号（见 [Member].CardUUID）
//   - 用户充值：把面值时长加到目标账号的到期时间上
//
// 卡密不因被使用而删除，只是状态变为已使用并记录用途，便于后台追溯。
// CreatedAt/UpdatedAt 由 GORM 自动维护
type Card struct {
	// ID：主键，自增
	ID uint `gorm:"primaryKey;comment:卡密ID，自增主键" json:"id"`

	// UUID：卡密唯一标识符，自动生成
	UUID string `gorm:"uniqueIndex;size:36;not null;comment:卡密UUID，唯一标识符" json:"uuid"`

	// CardNo：卡号，与 AppUUID 联合唯一（不同应用允许相同卡号）
	CardNo string `gorm:"size:64;not null;uniqueIndex:idx_card_app_no;comment:卡号" json:"card_no"`

	// AppUUID：归属应用UUID
	AppUUID string `gorm:"size:36;not null;index;uniqueIndex:idx_card_app_no;comment:归属应用UUID" json:"app_uuid"`

	// BatchNo：制卡批次号，用于批量导出/删除/统计
	BatchNo string `gorm:"size:32;index;comment:制卡批次号" json:"batch_no"`

	// Duration：面值时长（单位：分钟，时长模式），-1 表示永久
	Duration int `gorm:"not null;default:0;comment:面值时长，单位分钟，-1为永久" json:"duration"`

	// Points：面值点数（点数模式）
	Points int `gorm:"not null;default:0;comment:面值点数，点数模式使用" json:"points"`

	// Status：卡密状态（0=未使用，1=已使用，2=已冻结）
	Status int `gorm:"default:0;not null;comment:卡密状态，0=未使用，1=已使用，2=已冻结" json:"status"`

	// UsedByMember：核销去向，记录被哪个账号使用（账号UUID）
	UsedByMember string `gorm:"size:36;index;comment:核销去向，使用该卡的账号UUID" json:"used_by_member"`

	// UsedAt：核销时间（未使用时为空）
	UsedAt *time.Time `gorm:"comment:核销时间" json:"used_at"`

	// Remark：备注信息
	Remark string `gorm:"size:255;comment:备注信息" json:"remark"`

	// 时间字段
	CreatedAt time.Time `gorm:"comment:创建时间" json:"created_at"`
	UpdatedAt time.Time `gorm:"comment:更新时间" json:"updated_at"`
}

// ============================================================================
// 结构体方法
// ============================================================================

// BeforeCreate 在创建记录前自动生成UUID
func (card *Card) BeforeCreate(tx *gorm.DB) error {
	if card.UUID == "" {
		card.UUID = strings.ToUpper(uuid.New().String())
	}
	return nil
}

// TableName 指定表名
func (Card) TableName() string {
	return "cards"
}

// ============================================================================
// 独立函数
// ============================================================================

// 卡密字符集：32 个字符，排除易混淆字符 0/O/1/I，便于人工手输
const cardCharset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// 单次批量生成上限，防止误传超大数量
const cardBatchLimit = 10000

// GenerateCardNo 生成单个卡号：可选前缀 + randomLen 位随机字符（默认16位）
// 使用 crypto/rand + 拒绝采样确保字符均匀分布。
// 卡号取自不含 0/O/1/I 的字符集，与常见注册用户名格式区分，降低命名空间冲突概率。
func GenerateCardNo(prefix string, randomLen int) string {
	if randomLen <= 0 {
		randomLen = 16
	}
	buf := make([]byte, randomLen)
	// 拒绝采样阈值：丢弃会造成取模偏置的高位字节
	maxValid := 256 - (256 % len(cardCharset))
	for i := range buf {
		for {
			b := make([]byte, 1)
			rand.Read(b)
			if int(b[0]) < maxValid {
				buf[i] = cardCharset[int(b[0])%len(cardCharset)]
				break
			}
		}
	}
	return strings.ToUpper(prefix) + string(buf)
}

// GenerateCardNos 批量生成卡号并做内存级去重，供制卡接口使用。
// 去重仅保证本批内不重复；落库时仍依赖 (app_uuid, card_no) 唯一索引兜底。
func GenerateCardNos(prefix string, randomLen, count int) ([]string, error) {
	if count <= 0 {
		return nil, fmt.Errorf("生成数量必须大于0")
	}
	if count > cardBatchLimit {
		return nil, fmt.Errorf("单次生成数量不能超过%d", cardBatchLimit)
	}

	set := make(map[string]struct{}, count)
	codes := make([]string, 0, count)
	// 最多重试 count*3 次，避免极端情况下死循环
	maxAttempts := count * 3
	for attempts := 0; len(codes) < count && attempts < maxAttempts; attempts++ {
		code := GenerateCardNo(prefix, randomLen)
		if _, exists := set[code]; exists {
			continue
		}
		set[code] = struct{}{}
		codes = append(codes, code)
	}
	if len(codes) < count {
		return nil, fmt.Errorf("生成卡号时去重失败，请重试")
	}
	return codes, nil
}
