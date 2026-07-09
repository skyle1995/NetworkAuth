package services

import (
	"errors"
	"strings"

	"NetworkAuth/database"
	"NetworkAuth/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ============================================================================
// 黑名单服务：设备(机器码) / IP / 地区(省市) 维度封禁
// ============================================================================
//
// 与账号级黑名单（Member.Status=2）互补：账号黑名单封的是「人」，本表封的是
// 「设备 / 网络 / 地域」。登录收尾 CheckBlacklist 命中任一即拒绝。
// 地区以「省+市」为粒度（地级）。

// RegionValue 组装地区黑名单的 Value（"省/市"），用于唯一约束与展示。
func RegionValue(province, city string) string {
	province = strings.TrimSpace(province)
	city = strings.TrimSpace(city)
	return province + "/" + city
}

// blacklistReason 命中时的拒绝文案。
func blacklistReason(typ int) string {
	switch typ {
	case models.BlacklistTypeMachine:
		return "当前设备已被列入黑名单"
	case models.BlacklistTypeIP:
		return "当前IP已被列入黑名单"
	case models.BlacklistTypeRegion:
		return "当前所在地区已被列入黑名单"
	default:
		return "已被列入黑名单"
	}
}

// CheckBlacklist 登录收尾校验：机器码/IP/地区任一命中黑名单即返回 blocked=true。
// province/city 为登录 IP 解析出的归属地（可为空，空则地区维度不参与匹配）。
func CheckBlacklist(db *gorm.DB, appUUID, machineCode, ip, province, city string) (bool, string) {
	appUUID = strings.TrimSpace(appUUID)
	machineCode = strings.TrimSpace(machineCode)
	ip = strings.TrimSpace(ip)
	province = strings.TrimSpace(province)
	city = strings.TrimSpace(city)

	// 逐维度精确查询，命中即返回对应文案（比一次性 OR 更易给出准确原因）
	if machineCode != "" {
		if existsBlacklist(db, appUUID, models.BlacklistTypeMachine, "value = ?", machineCode) {
			return true, blacklistReason(models.BlacklistTypeMachine)
		}
	}
	if ip != "" {
		if existsBlacklist(db, appUUID, models.BlacklistTypeIP, "value = ?", ip) {
			return true, blacklistReason(models.BlacklistTypeIP)
		}
	}
	// 地区需省市都能解析出来才匹配（避免空串误伤）
	if province != "" && city != "" {
		if existsBlacklist(db, appUUID, models.BlacklistTypeRegion, "province = ? AND city = ?", province, city) {
			return true, blacklistReason(models.BlacklistTypeRegion)
		}
	}
	return false, ""
}

// existsBlacklist 判断某应用下指定类型 + 条件的黑名单是否存在。
func existsBlacklist(db *gorm.DB, appUUID string, typ int, cond string, args ...any) bool {
	q := db.Model(&models.Blacklist{}).
		Where("app_uuid = ? AND type = ?", appUUID, typ).
		Where(cond, args...)
	var count int64
	if err := q.Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

// AddBlacklistEntry 新增一条黑名单（存在则忽略，不报错）。
func AddBlacklistEntry(db *gorm.DB, entry *models.Blacklist) error {
	entry.AppUUID = strings.TrimSpace(entry.AppUUID)
	entry.Value = strings.TrimSpace(entry.Value)
	if entry.AppUUID == "" || entry.Value == "" {
		return errors.New("应用与命中值不能为空")
	}
	// app_uuid + type + value 冲突时跳过（幂等）
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "app_uuid"}, {Name: "type"}, {Name: "value"}},
		DoNothing: true,
	}).Create(entry).Error
}

// BlacklistOptions 拉黑账号时附带的维度选择。
type BlacklistOptions struct {
	Device bool // 拉黑该账号的设备(机器码)
	IP     bool // 拉黑该账号的IP
	Region bool // 拉黑该账号IP所属地区(省市)
}

// BlacklistMemberFull 拉黑账号：账号置黑 + 清会话，并按所选维度把其绑定的
// 设备/IP/地区写入黑名单表。返回各维度新增条数。
func BlacklistMemberFull(memberID uint, opts BlacklistOptions) (map[string]any, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	var member models.Member
	if err := db.First(&member, memberID).Error; err != nil {
		return nil, errors.New("账号不存在")
	}

	// 该账号的绑定（用于按维度拉黑）
	var bindings []models.Binding
	if err := db.Where("member_uuid = ?", member.UUID).Find(&bindings).Error; err != nil {
		return nil, err
	}

	added := map[string]int{"device": 0, "ip": 0, "region": 0}
	regionSeen := map[string]bool{}

	err = db.Transaction(func(tx *gorm.DB) error {
		// 账号置黑并清空其全部会话（立即掉线）
		if err := tx.Model(&member).Update("status", models.MemberStatusBlack).Error; err != nil {
			return err
		}
		if err := tx.Where("member_uuid = ?", member.UUID).Delete(&models.MemberSession{}).Error; err != nil {
			return err
		}

		for _, b := range bindings {
			switch b.Type {
			case models.BindingTypeMachine:
				if opts.Device {
					if err := AddBlacklistEntry(tx, &models.Blacklist{
						AppUUID: member.AppUUID, Type: models.BlacklistTypeMachine, Value: b.Value,
						MemberUUID: member.UUID, Username: member.Username, Remark: "拉黑账号时附带",
					}); err != nil {
						return err
					}
					added["device"]++
				}
			case models.BindingTypeIP:
				if opts.IP {
					if err := AddBlacklistEntry(tx, &models.Blacklist{
						AppUUID: member.AppUUID, Type: models.BlacklistTypeIP, Value: b.Value,
						Province: b.Province, City: b.City,
						MemberUUID: member.UUID, Username: member.Username, Remark: "拉黑账号时附带",
					}); err != nil {
						return err
					}
					added["ip"]++
				}
				if opts.Region {
					province, city := b.Province, b.City
					// 绑定未存省市则按当前库实时解析补全
					if province == "" && city == "" {
						province, city = ResolveIPRegion(b.Value)
					}
					if province != "" && city != "" {
						key := RegionValue(province, city)
						if !regionSeen[key] {
							regionSeen[key] = true
							if err := AddBlacklistEntry(tx, &models.Blacklist{
								AppUUID: member.AppUUID, Type: models.BlacklistTypeRegion, Value: key,
								Province: province, City: city,
								MemberUUID: member.UUID, Username: member.Username, Remark: "拉黑账号时附带",
							}); err != nil {
								return err
							}
							added["region"]++
						}
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	AddMemberLog(member.AppUUID, member.UUID, member.Username, "拉黑", "", "")
	return map[string]any{
		"username": member.Username,
		"device":   added["device"],
		"ip":       added["ip"],
		"region":   added["region"],
	}, nil
}

// SessionBlacklistOptions 从在线会话拉黑时的维度选择。
type SessionBlacklistOptions struct {
	Device  bool // 拉黑该会话的设备(机器码)
	IP      bool // 拉黑该会话的IP
	Region  bool // 拉黑该会话IP所属地区(省/市)
	Account bool // 同时拉黑账号(置黑+清该账号全部会话)
}

// BlacklistFromSession 从一条在线会话直接拉黑其 设备/IP/地区（可选连带拉黑账号），
// 并踢掉本应用内命中该设备/IP 的所有在线会话，使封禁立即生效。
func BlacklistFromSession(appUUID, memberUUID, username, machineCode, ip, province, city string, opts SessionBlacklistOptions) (map[string]any, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	appUUID = strings.TrimSpace(appUUID)
	machineCode = strings.TrimSpace(machineCode)
	ip = strings.TrimSpace(ip)
	province = strings.TrimSpace(province)
	city = strings.TrimSpace(city)
	if appUUID == "" {
		return nil, errors.New("应用不能为空")
	}

	// 地区若未带省市则按当前 IP 实时解析
	if opts.Region && province == "" && city == "" {
		province, city = ResolveIPRegion(ip)
	}

	added := map[string]int{"device": 0, "ip": 0, "region": 0}
	kicked := int64(0)

	err = db.Transaction(func(tx *gorm.DB) error {
		if opts.Device && machineCode != "" {
			if err := AddBlacklistEntry(tx, &models.Blacklist{
				AppUUID: appUUID, Type: models.BlacklistTypeMachine, Value: machineCode,
				MemberUUID: memberUUID, Username: username, Remark: "在线会话拉黑",
			}); err != nil {
				return err
			}
			added["device"] = 1
			// 踢掉本应用内该设备的全部在线会话
			res := tx.Where("app_uuid = ? AND machine_code = ?", appUUID, machineCode).Delete(&models.MemberSession{})
			if res.Error != nil {
				return res.Error
			}
			kicked += res.RowsAffected
		}
		if opts.IP && ip != "" {
			if err := AddBlacklistEntry(tx, &models.Blacklist{
				AppUUID: appUUID, Type: models.BlacklistTypeIP, Value: ip,
				Province: province, City: city,
				MemberUUID: memberUUID, Username: username, Remark: "在线会话拉黑",
			}); err != nil {
				return err
			}
			added["ip"] = 1
			res := tx.Where("app_uuid = ? AND ip = ?", appUUID, ip).Delete(&models.MemberSession{})
			if res.Error != nil {
				return res.Error
			}
			kicked += res.RowsAffected
		}
		if opts.Region && province != "" && city != "" {
			if err := AddBlacklistEntry(tx, &models.Blacklist{
				AppUUID: appUUID, Type: models.BlacklistTypeRegion, Value: RegionValue(province, city),
				Province: province, City: city,
				MemberUUID: memberUUID, Username: username, Remark: "在线会话拉黑",
			}); err != nil {
				return err
			}
			added["region"] = 1
		}
		// 连带拉黑账号：置黑 + 清该账号全部会话
		if opts.Account && strings.TrimSpace(memberUUID) != "" {
			if err := tx.Model(&models.Member{}).Where("uuid = ?", memberUUID).
				Update("status", models.MemberStatusBlack).Error; err != nil {
				return err
			}
			res := tx.Where("member_uuid = ?", memberUUID).Delete(&models.MemberSession{})
			if res.Error != nil {
				return res.Error
			}
			kicked += res.RowsAffected
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	AddMemberLog(appUUID, memberUUID, username, "拉黑", "在线会话拉黑", ip)
	return map[string]any{
		"device": added["device"], "ip": added["ip"], "region": added["region"],
		"account": opts.Account, "kicked": kicked,
	}, nil
}

// ListBlacklist 分页查询黑名单（app_uuid / type 筛选，value/username 搜索）。
func ListBlacklist(appUUID string, typ *int, search string, page, limit int) ([]models.Blacklist, int64, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, 0, err
	}
	query := db.Model(&models.Blacklist{})
	if appUUID = strings.TrimSpace(appUUID); appUUID != "" {
		query = query.Where("app_uuid = ?", appUUID)
	}
	if typ != nil {
		query = query.Where("type = ?", *typ)
	}
	if search = strings.TrimSpace(search); search != "" {
		like := "%" + search + "%"
		query = query.Where("value LIKE ? OR username LIKE ?", like, like)
	}
	return Paginate[models.Blacklist](query, page, limit, "created_at DESC")
}

// AddBlacklistManual 后台手动新增一条黑名单。
func AddBlacklistManual(appUUID string, typ int, value, province, city, remark string) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	if typ != models.BlacklistTypeMachine && typ != models.BlacklistTypeIP && typ != models.BlacklistTypeRegion {
		return errors.New("无效的黑名单类型")
	}
	entry := &models.Blacklist{
		AppUUID: appUUID, Type: typ, Province: strings.TrimSpace(province),
		City: strings.TrimSpace(city), Remark: strings.TrimSpace(remark),
	}
	if typ == models.BlacklistTypeRegion {
		entry.Value = RegionValue(province, city)
		if entry.Province == "" || entry.City == "" {
			return errors.New("地区黑名单需填写省份与城市")
		}
	} else {
		entry.Value = strings.TrimSpace(value)
	}
	return AddBlacklistEntry(db, entry)
}

// RemoveBlacklist 按ID批量移除黑名单。
func RemoveBlacklist(ids []uint) error {
	if len(ids) == 0 {
		return nil
	}
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	return db.Where("id IN ?", ids).Delete(&models.Blacklist{}).Error
}
