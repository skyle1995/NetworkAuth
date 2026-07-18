package services

import (
	"NetworkAuth/database"
	"NetworkAuth/models"
	"NetworkAuth/utils"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
)

// ============================================================================
// 账号认证服务（公开 API 业务核心）
// ============================================================================
//
// 面向客户端的登录/心跳/登出逻辑。会话采用单令牌模型：
// 登录颁发随机令牌写入 member.LoginToken，新登录覆盖旧令牌（即顶号），
// 登出清空令牌。多开（MultiOpenCount>1）的多会话留待后续会话表实现。

// LoginResult 登录成功返回的信息
type LoginResult struct {
	Token             string    `json:"token"`
	Username          string    `json:"username"`
	Type              int       `json:"type"`
	Mode              int       `json:"mode"` // 运营模式：0时长/1点数
	Permanent         bool      `json:"permanent"`
	ExpiredAt         time.Time `json:"expired_at"`         // 时长模式有效
	Points            int       `json:"points"`             // 点数模式有效
	HeartbeatInterval int       `json:"heartbeat_interval"` // 心跳间隔（分钟），客户端据此周期心跳
	// 会员信息：累计充值（分）+ 当前会员等级名与返利比例（空等级即默认「免费账号」）
	TotalRecharge int    `json:"total_recharge"` // 累计充值金额（单位：分）
	LevelName     string `json:"level_name"`     // 会员等级名，空=免费账号
	RebateRate    int    `json:"rebate_rate"`    // 当前等级充值返利比例（%）
	// Update：更新判断结果。仅当应用更新方式非「不启用」时返回；据登录提交的版本号判断是否需要更新。
	Update *LoginUpdate `json:"update,omitempty"`
}

// LoginUpdate 登录时的更新判断结果（仅 download_type != 0 时随登录返回）。
// 客户端据 download_type 决定强制/自由更新；need_update 为本次登录版本是否落后。
type LoginUpdate struct {
	DownloadType  int    `json:"download_type"`  // 更新方式：1 强制 / 2 自由
	NeedUpdate    bool   `json:"need_update"`    // 客户端版本低于应用版本
	LatestVersion string `json:"latest_version"` // 应用当前版本
	DownloadURL   string `json:"download_url"`   // 下载/更新地址
}

// buildLoginUpdate 依据应用更新方式与客户端提交的版本号，构造登录返回中的更新结果。
// 更新方式为「不启用」(download_type=0) 时返回 nil —— 登录不带更新信息。
func buildLoginUpdate(app *models.App, clientVersion string) *LoginUpdate {
	if app.DownloadType == models.DownloadTypeDisabled {
		return nil
	}
	return &LoginUpdate{
		DownloadType:  app.DownloadType,
		NeedUpdate:    compareVersion(clientVersion, app.Version) < 0,
		LatestVersion: app.Version,
		DownloadURL:   app.DownloadURL,
	}
}

// ForceUpdateError 强制更新拦截：应用为「强制更新」且客户端版本过旧时拒绝登录，
// 附带更新信息供客户端引导用户升级后再登录。
type ForceUpdateError struct {
	Update *LoginUpdate
}

func (e *ForceUpdateError) Error() string {
	if e.Update != nil {
		return "客户端版本过低，请更新至 " + e.Update.LatestVersion + " 后再登录"
	}
	return "客户端版本过低，请更新后再登录"
}

// checkForceUpdate 强制更新登录门禁：仅「强制更新」(download_type=1) 且客户端版本落后时拒绝。
// 必须在任何登录副作用（核销卡密、建号、开会话）之前调用，避免拒绝时已产生消费。
func checkForceUpdate(app *models.App, version string) error {
	upd := buildLoginUpdate(app, version)
	if upd != nil && upd.NeedUpdate && upd.DownloadType == models.DownloadTypeForce {
		return &ForceUpdateError{Update: upd}
	}
	return nil
}

// StatusResult 账号状态查询返回的信息
type StatusResult struct {
	Username          string    `json:"username"`
	Type              int       `json:"type"` // 来源类型：0注册/1卡密
	Status            int       `json:"status"`
	Mode              int       `json:"mode"`
	Permanent         bool      `json:"permanent"`
	ExpiredAt         time.Time `json:"expired_at"`
	Points            int       `json:"points"`
	HeartbeatInterval int       `json:"heartbeat_interval"` // 心跳间隔（分钟），客户端可据此动态调整
}

// generateSessionToken 生成 32 字节随机会话令牌（64 位十六进制）
func generateSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// isPermanent 判断到期时间是否为永久
func isPermanent(expiredAt time.Time) bool {
	return expiredAt.Equal(models.PermanentTime)
}

// checkMemberUsable 按运营模式校验账号是否可用（时长：未到期；点数：余额>0）。
func checkMemberUsable(app *models.App, m *models.Member) error {
	// 免费模式：不计费，账号即便已到期/无点数也放行（仅令牌、账号状态等仍由调用方校验）。
	if app.OperationMode == models.OperationModeFree {
		return nil
	}
	if app.OperationMode == models.OperationModePoints {
		if app.PointsChargeMode == models.PointsChargePerTime {
			// 按时：仍在已预扣周期内，或余额够买下一个周期
			if time.Now().Before(m.ExpiredAt) {
				return nil
			}
			if m.Points >= pointsPerPeriod(app) {
				return nil
			}
			return errors.New("点数不足")
		}
		// 按次：登录时已扣费，会话内不再以点数拦截
		return nil
	}
	if !isPermanent(m.ExpiredAt) && m.ExpiredAt.Before(time.Now()) {
		return errors.New("账号已到期")
	}
	return nil
}

// pointsPerPeriod 按时模式每周期扣点（至少1）
func pointsPerPeriod(app *models.App) int {
	if app.PointsPerPeriod <= 0 {
		return 1
	}
	return app.PointsPerPeriod
}

// pointsPeriodMinutes 按时模式周期分钟数（至少1）
func pointsPeriodMinutes(app *models.App) int {
	if app.PointsPeriodMinutes <= 0 {
		return 60
	}
	return app.PointsPeriodMinutes
}

// applyLoginCharge 登录时的点数扣费（在事务内调用）。
//   - 时长模式：无扣费
//   - 按次：登录扣 PointsPerLogin，不足则拒绝
//   - 按时：若已过预扣周期，扣一个周期并顺延到期时间，不足则拒绝
func applyLoginCharge(tx *gorm.DB, app *models.App, m *models.Member) error {
	if app.OperationMode != models.OperationModePoints {
		return nil
	}
	if app.PointsChargeMode == models.PointsChargePerTime {
		// 心跳触发扣费模式：登录不预扣，交由心跳按需结算
		if app.PointsHeartbeatCharge == 1 {
			return nil
		}
		return settlePointsTime(tx, app, m)
	}
	// 按次
	cost := app.PointsPerLogin
	if cost <= 0 {
		return nil // 登录免费，点数仅由显式扣点消耗
	}
	if m.Points < cost {
		return errors.New("点数不足")
	}
	newPoints := m.Points - cost
	if err := tx.Model(m).Update("points", newPoints).Error; err != nil {
		return err
	}
	m.Points = newPoints
	return nil
}

// settlePointsTime 按时预扣费结算：过了预扣周期则扣一个周期并顺延（离线不补扣）。
func settlePointsTime(tx *gorm.DB, app *models.App, m *models.Member) error {
	if app.OperationMode != models.OperationModePoints || app.PointsChargeMode != models.PointsChargePerTime {
		return nil
	}
	now := time.Now()
	if now.Before(m.ExpiredAt) {
		return nil // 仍在已付周期内
	}
	cost := pointsPerPeriod(app)
	if m.Points < cost {
		return errors.New("点数不足")
	}
	newPoints := m.Points - cost
	newExpiry := now.Add(time.Duration(pointsPeriodMinutes(app)) * time.Minute)
	if err := tx.Model(m).Updates(map[string]interface{}{
		"points":     newPoints,
		"expired_at": newExpiry,
	}).Error; err != nil {
		return err
	}
	m.Points = newPoints
	m.ExpiredAt = newExpiry
	return nil
}

// buildStatusResult 依据运营模式构造状态返回
func buildStatusResult(app *models.App, m *models.Member) *StatusResult {
	return &StatusResult{
		Username:          m.Username,
		Type:              m.Type,
		Status:            m.Status,
		Mode:              app.OperationMode,
		Permanent:         isPermanent(m.ExpiredAt),
		ExpiredAt:         m.ExpiredAt,
		Points:            m.Points,
		HeartbeatInterval: heartbeatMinutes(app),
	}
}

// CardLogin 卡密登录：卡号即身份。
// 未使用的卡首次登录激活并自动创建绑定该卡的账号；已使用的卡走登录校验。
func CardLogin(appUUID, cardNo, machineCode, ip, version, deviceName string) (*LoginResult, error) {
	appUUID = strings.TrimSpace(appUUID)
	cardNo = strings.TrimSpace(cardNo)
	if appUUID == "" || cardNo == "" {
		return nil, errors.New("应用与卡号不能为空")
	}
	if strings.TrimSpace(version) == "" {
		return nil, errors.New("请提供客户端版本号")
	}

	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}

	// 校验应用存在且启用，并读取多开/机器验证配置
	var app models.App
	if err := db.Where("uuid = ?", appUUID).First(&app).Error; err != nil {
		return nil, errors.New("应用不存在")
	}
	if app.Status != 1 {
		return nil, errors.New("应用已停用")
	}
	if app.CardLoginEnabled != 1 {
		return nil, errors.New("该应用未开启卡密登录")
	}
	// 强制更新门禁：须在核销卡密/建号前拦截，避免拒绝登录却已消费卡
	if err := checkForceUpdate(&app, version); err != nil {
		return nil, err
	}

	var member models.Member
	err = db.Transaction(func(tx *gorm.DB) error {
		var card models.Card
		if err := tx.Where("app_uuid = ? AND card_no = ?", appUUID, cardNo).First(&card).Error; err != nil {
			return errors.New("卡号不存在")
		}
		if card.Status == models.CardStatusFrozen {
			return errors.New("卡密已被冻结")
		}

		if card.Status == models.CardStatusUnused {
			// 首次使用：激活并创建绑定该卡的账号
			member = models.Member{
				AppUUID:  appUUID,
				Username: cardNo,
				Type:     models.MemberTypeCard,
				CardUUID: card.UUID,
				Status:   models.MemberStatusNormal,
			}
			// 新号无等级、返利为 0，故按原面值发放
			if app.OperationMode == models.OperationModePoints {
				// 点数模式：卡面值为点数；ExpiredAt 留零值——按次不参与、按时首登即购一个周期
				member.Points = card.Points
			} else {
				member.ExpiredAt = expiryFromDuration(card.Duration)
			}
			if err := tx.Create(&member).Error; err != nil {
				return errors.New("激活卡密失败")
			}
			if err := settleMemberLevel(tx, appUUID, &member, card.Price); err != nil {
				return err
			}
			if err := MarkCardUsed(tx, card.ID, member.UUID); err != nil {
				return errors.New("核销卡密失败")
			}
			return nil
		}

		// 已使用：定位其绑定的账号
		if err := tx.Where("app_uuid = ? AND username = ?", appUUID, cardNo).First(&member).Error; err != nil {
			return errors.New("卡密账号不存在")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return finishMemberLogin(db, &app, &member, machineCode, ip, version, deviceName)
}

// finishMemberLogin 完成登录的公共收尾：状态/到期校验、机器码绑定、多开会话管理、颁发令牌。
// version 为客户端提交的版本号：更新方式开启时用于判断是否需要更新，结果随登录返回。
func finishMemberLogin(db *gorm.DB, app *models.App, member *models.Member, machineCode, ip, version, deviceName string) (*LoginResult, error) {
	deviceName = strings.TrimSpace(deviceName)
	// 有效多开 = 应用多开数 + 会员等级额外多开（下限 1）；三处（机器绑定/IP绑定/会话）统一使用
	effMultiOpen := effectiveMultiOpen(db, app, member)
	if member.Status == models.MemberStatusBlack {
		return nil, errors.New("账号已被拉黑")
	}
	if member.Status == models.MemberStatusDisabled {
		return nil, errors.New("账号已被封停")
	}
	if err := checkMemberUsable(app, member); err != nil {
		return nil, err
	}

	// 设备/IP/地区黑名单校验：命中任一即拒绝（先于绑定，避免为黑名单目标建立绑定/扣费）
	blProvince, blCity := ResolveIPRegion(ip)
	if blocked, reason := CheckBlacklist(db, app.UUID, machineCode, ip, blProvince, blCity); blocked {
		return nil, errors.New(reason)
	}

	// 机器码绑定（开启机器验证时）：已绑定则放行，未绑定且未超多开则新增，超出则拒绝
	if app.MachineVerify == 1 && strings.TrimSpace(machineCode) != "" {
		if err := ensureMachineBinding(db, member.UUID, machineCode, deviceName, effMultiOpen); err != nil {
			return nil, err
		}
	}
	if err := ensureIPBinding(db, member.UUID, ip, app.IPVerify, effMultiOpen); err != nil {
		return nil, err
	}

	token, err := generateSessionToken()
	if err != nil {
		return nil, err
	}

	err = db.Transaction(func(tx *gorm.DB) error {
		// 点数模式登录扣费（按次扣点 / 按时预扣一个周期）
		if err := applyLoginCharge(tx, app, member); err != nil {
			return err
		}
		// 清理该用户的失效会话（超过校验间隔未活跃）
		if err := cleanStaleSessions(tx, member.UUID, offlineTimeoutMinutes(app)); err != nil {
			return err
		}
		// 多开数量控制：按「多开范围」以设备/IP/会话为单位计数（含会员额外多开）
		maxOpen := effMultiOpen
		var sessions []models.MemberSession
		if err := tx.Where("member_uuid = ?", member.UUID).
			Order("last_active_at ASC").Find(&sessions).Error; err != nil {
			return err
		}

		newKey := sessionOpenKey(app.MultiOpenScope, machineCode, ip, token)
		// 同「开」重登（同一机器/IP）先清掉旧会话，保证一个开只占一个名额
		var sameKey []uint
		for _, s := range sessions {
			if sessionOpenKey(app.MultiOpenScope, s.MachineCode, s.IP, s.Token) == newKey {
				sameKey = append(sameKey, s.ID)
			}
		}
		if len(sameKey) > 0 {
			if err := tx.Delete(&models.MemberSession{}, sameKey).Error; err != nil {
				return err
			}
		}

		// 统计剩余不同「开」的数量
		distinct := map[string][]uint{}
		var order []string
		for _, s := range sessions {
			k := sessionOpenKey(app.MultiOpenScope, s.MachineCode, s.IP, s.Token)
			if k == newKey {
				continue // 已在上面清除
			}
			if _, ok := distinct[k]; !ok {
				order = append(order, k)
			}
			distinct[k] = append(distinct[k], s.ID)
		}

		if len(distinct) >= maxOpen {
			if app.LoginType == 1 {
				return errors.New("已达最大同时在线数")
			}
			// 顶号：踢掉最早的「开」(该机器/IP 的全部会话)直到腾出空位
			for _, k := range order {
				if len(distinct) < maxOpen {
					break
				}
				if err := tx.Delete(&models.MemberSession{}, distinct[k]).Error; err != nil {
					return err
				}
				delete(distinct, k)
			}
		}

		now := time.Now()
		session := models.MemberSession{
			Token:        token,
			MemberUUID:   member.UUID,
			AppUUID:      member.AppUUID,
			MachineCode:  machineCode,
			DeviceName:   deviceName,
			IP:           ip,
			Version:      strings.TrimSpace(version),
			LastActiveAt: now,
		}
		if err := tx.Create(&session).Error; err != nil {
			return err
		}
		updates := map[string]interface{}{
			"last_login_at": &now,
			"last_login_ip": ip,
		}
		// 账号尚无注册设备时，用本次登录设备回填为注册设备。
		// 后台建号 / 卡密登录 / 未开设备限制时注册的账号 register_machine 为空，
		// 回填后设备维度注册限制与风控才有据可依。
		if strings.TrimSpace(member.RegisterMachine) == "" && strings.TrimSpace(machineCode) != "" {
			mc := strings.TrimSpace(machineCode)
			updates["register_machine"] = mc
			member.RegisterMachine = mc
		}
		return tx.Model(member).Updates(updates).Error
	})
	if err != nil {
		return nil, err
	}

	loginAction := "账号登录"
	if member.Type == models.MemberTypeCard {
		loginAction = "卡密登录"
	}
	AddMemberLog(member.AppUUID, member.UUID, member.Username, loginAction, machineCode, ip)

	levelName, rebateRate := memberLevelInfo(db, member)
	return &LoginResult{
		Token:             token,
		Username:          member.Username,
		Type:              member.Type,
		Mode:              app.OperationMode,
		Permanent:         isPermanent(member.ExpiredAt),
		ExpiredAt:         member.ExpiredAt,
		Points:            member.Points,
		HeartbeatInterval: heartbeatMinutes(app),
		TotalRecharge:     member.TotalRecharge,
		LevelName:         levelName,
		RebateRate:        rebateRate,
		Update:            buildLoginUpdate(app, version),
	}, nil
}

// heartbeatMinutes 返回应用的心跳间隔（分钟），未配置时回退默认 10。
func heartbeatMinutes(app *models.App) int {
	if app.CheckInterval <= 0 {
		return 10
	}
	return app.CheckInterval
}

// offlineTimeoutMinutes 返回应用的自动离线时长（分钟），未配置时回退默认 30。
func offlineTimeoutMinutes(app *models.App) int {
	if app.OfflineTimeout <= 0 {
		return 30
	}
	return app.OfflineTimeout
}

// sessionOpenKey 依据多开范围计算一个会话属于哪个「开」：
// 单电脑按机器码、单IP按IP；无法分组(空值)或全部电脑范围时按会话令牌(各自独立)。
func sessionOpenKey(scope int, machineCode, ip, token string) string {
	switch scope {
	case models.MultiOpenScopeMachine:
		if strings.TrimSpace(machineCode) != "" {
			return "m:" + machineCode
		}
	case models.MultiOpenScopeIP:
		if strings.TrimSpace(ip) != "" {
			return "i:" + ip
		}
	}
	return "t:" + token
}

// cleanStaleSessions 删除某用户超过 checkIntervalMin 分钟未活跃的会话。
func cleanStaleSessions(tx *gorm.DB, memberUUID string, checkIntervalMin int) error {
	if checkIntervalMin <= 0 {
		checkIntervalMin = 10
	}
	deadline := time.Now().Add(-time.Duration(checkIntervalMin) * time.Minute)
	return tx.Where("member_uuid = ? AND last_active_at < ?", memberUUID, deadline).
		Delete(&models.MemberSession{}).Error
}

// ensureMachineBinding 确保机器码已绑定；未绑定时在多开数量内新增，超出则拒绝。
func ensureMachineBinding(db *gorm.DB, memberUUID, machineCode, deviceName string, multiOpenCount int) error {
	var existing models.Binding
	err := db.Where("member_uuid = ? AND type = ? AND value = ?",
		memberUUID, models.BindingTypeMachine, machineCode).First(&existing).Error
	if err == nil {
		// 已绑定：刷新设备名（客户端可能改了系统版本），放行
		if deviceName != "" && existing.DeviceName != deviceName {
			db.Model(&existing).Update("device_name", deviceName)
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	var count int64
	if err := db.Model(&models.Binding{}).
		Where("member_uuid = ? AND type = ?", memberUUID, models.BindingTypeMachine).
		Count(&count).Error; err != nil {
		return err
	}
	if multiOpenCount <= 0 {
		multiOpenCount = 1
	}
	if int(count) >= multiOpenCount {
		return errors.New("机器码未绑定，请先进行机器码转绑")
	}
	return db.Create(&models.Binding{
		MemberUUID: memberUUID,
		Type:       models.BindingTypeMachine,
		Value:      machineCode,
		DeviceName: deviceName,
	}).Error
}

// effectiveMultiOpen 有效多开数 = 应用多开数 + 会员等级额外多开，下限 1。
func effectiveMultiOpen(db *gorm.DB, app *models.App, m *models.Member) int {
	extra, _ := memberLevelExtras(db, m)
	n := app.MultiOpenCount + extra
	if n <= 0 {
		n = 1
	}
	return n
}

// ensureIPBinding 确保登录 IP 满足应用 IP 验证配置；首次登录会自动绑定当前 IP。
func ensureIPBinding(db *gorm.DB, memberUUID, ip string, ipVerify, multiOpenCount int) error {
	if ipVerify == 0 {
		return nil
	}
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return errors.New("登录IP不能为空")
	}
	province, city := ResolveIPRegion(ip)

	var bindings []models.Binding
	if err := db.Where("member_uuid = ? AND type = ?", memberUUID, models.BindingTypeIP).
		Find(&bindings).Error; err != nil {
		return err
	}

	// 按验证级别判定是否已满足：3=同省，2=同市，其余=精确IP；
	// 地区无法解析时统一退回精确IP匹配。
	for _, b := range bindings {
		if ipVerify == 3 && province != "" && b.Province == province {
			return nil
		}
		if ipVerify == 2 && city != "" && b.City == city {
			return nil
		}
		if b.Value == ip {
			return nil
		}
	}

	if multiOpenCount <= 0 {
		multiOpenCount = 1
	}
	if len(bindings) >= multiOpenCount {
		return errors.New("登录IP未绑定，请先进行IP转绑")
	}
	return db.Create(&models.Binding{
		MemberUUID: memberUUID,
		Type:       models.BindingTypeIP,
		Value:      ip,
		Province:   province,
		City:       city,
	}).Error
}

// authMemberByToken 按应用与会话令牌定位有效账号，并刷新会话活跃时间。
func authMemberByToken(db *gorm.DB, appUUID, token string) (*models.Member, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("令牌不能为空")
	}
	var session models.MemberSession
	if err := db.Where("app_uuid = ? AND token = ?", strings.TrimSpace(appUUID), token).First(&session).Error; err != nil {
		return nil, errors.New("会话无效或已被顶号")
	}
	var member models.Member
	if err := db.Where("uuid = ?", session.MemberUUID).First(&member).Error; err != nil {
		return nil, errors.New("账号不存在")
	}
	// 刷新会话活跃时间（心跳）
	db.Model(&models.MemberSession{}).Where("id = ?", session.ID).Update("last_active_at", time.Now())
	return &member, nil
}

// authActiveMember 校验令牌并要求账号正常且可用（按运营模式），返回有效账号及其应用。
// 供需要“已登录且可用”前提的接口（数据获取、改密、转绑、扣点等）复用。
func authActiveMember(db *gorm.DB, appUUID, token string) (*models.Member, *models.App, error) {
	member, err := authMemberByToken(db, appUUID, token)
	if err != nil {
		return nil, nil, err
	}
	if member.Status != models.MemberStatusNormal {
		return nil, nil, errors.New("账号状态异常")
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, nil, err
	}
	if err := checkMemberUsable(app, member); err != nil {
		return nil, nil, err
	}
	return member, app, nil
}

// authMemberByCredential 用凭据鉴权账号（不依赖会话令牌）：
// 注册账号校验用户名+密码；卡密账号以卡号为身份、无密码。供转绑等“可能登录不了”的场景用，
// 避免“设备/IP 不匹配→登不进→拿不到令牌→无法转绑”的死循环。
func authMemberByCredential(db *gorm.DB, appUUID, username, password string) (*models.Member, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("用户名不能为空")
	}
	var member models.Member
	if err := db.Where("app_uuid = ? AND username = ?", strings.TrimSpace(appUUID), username).
		First(&member).Error; err != nil {
		return nil, errors.New("账号不存在")
	}
	if member.Status == models.MemberStatusBlack {
		return nil, errors.New("账号已被拉黑")
	}
	if member.Status == models.MemberStatusDisabled {
		return nil, errors.New("账号已被封停")
	}
	// 注册账号校验密码；卡密账号以卡号即身份，无需密码
	if member.Type == models.MemberTypeRegister {
		if !utils.VerifyPasswordWithSalt(password, member.PasswordSalt, member.Password) {
			return nil, errors.New("密码错误")
		}
	}
	return &member, nil
}

// CheckMemberStatus 心跳/状态查询：校验令牌有效、账号正常且可用。
// 按时点数模式在心跳时结算：过了预扣周期则自动续扣下一周期。
// CheckMemberStatus 检测账号状态/心跳（type 41）。
// noCharge：本次心跳是否跳过扣费。点数-按时模式下心跳**默认结算扣费**，客户端传 no_charge=true 才跳过
// （用于免费功能）；免费模式/时长模式/按次模式本就不在心跳扣费，忽略该参数。
func CheckMemberStatus(appUUID, token string, noCharge bool) (*StatusResult, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	member, err := authMemberByToken(db, appUUID, token)
	if err != nil {
		return nil, err
	}
	if member.Status != models.MemberStatusNormal {
		return nil, errors.New("账号状态异常")
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}
	// 按时预扣费结算：心跳**默认结算扣费**，客户端传 no_charge=true 才跳过本次扣费（免费功能）。
	// 免费/时长/按次模式下 settlePointsTime 自身即会跳过，不受影响。
	// 尽力续期，忽略无法续期的错误，交由 usable 判定。
	if !noCharge {
		_ = settlePointsTime(db, app, member)
	}
	if err := checkMemberUsable(app, member); err != nil {
		return nil, err
	}
	return buildStatusResult(app, member), nil
}

// MemberLogout 登出：删除当前会话。
func MemberLogout(appUUID, token string) error {
	db, err := database.GetDB()
	if err != nil {
		return err
	}
	token = strings.TrimSpace(token)
	res := db.Where("app_uuid = ? AND token = ?", strings.TrimSpace(appUUID), token).
		Delete(&models.MemberSession{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("会话无效")
	}
	return nil
}

// ============================================================================
// 账号模式（注册/登录/充值/到期查询）
// ============================================================================

// loadEnabledApp 读取应用并校验其存在且启用
func loadEnabledApp(db *gorm.DB, appUUID string) (*models.App, error) {
	var app models.App
	if err := db.Where("uuid = ?", strings.TrimSpace(appUUID)).First(&app).Error; err != nil {
		return nil, errors.New("应用不存在")
	}
	if app.Status != 1 {
		return nil, errors.New("应用已停用")
	}
	return &app, nil
}

// registerInitialExpiry 注册账号的初始到期时间：注册后默认过期，需充值或独立领取试用。
func registerInitialExpiry() time.Time {
	return time.Now()
}

// enforceRegisterLimit 按应用和注册 IP 校验每天/永久注册次数限制。
func enforceRegisterLimit(db *gorm.DB, app *models.App, registerIP, machineCode string) error {
	registerIP = strings.TrimSpace(registerIP)
	machineCode = strings.TrimSpace(machineCode)
	limit := app.RegisterCount
	if limit <= 0 {
		limit = 1
	}

	// 时间窗口（每天/永久），IP 与设备维度共用
	withWindow := func(q *gorm.DB) *gorm.DB {
		if app.RegisterLimitTime == 0 {
			today := time.Now()
			startOfDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
			q = q.Where("created_at >= ?", startOfDay)
		}
		return q
	}
	countBy := func(field, value string) (int64, error) {
		var count int64
		q := withWindow(db.Model(&models.Member{}).
			Where("app_uuid = ? AND "+field+" = ?", app.UUID, value))
		err := q.Count(&count).Error
		return count, err
	}

	// IP 维度
	if app.RegisterLimitEnabled == 1 {
		if registerIP == "" {
			return errors.New("注册IP不能为空")
		}
		count, err := countBy("register_ip", registerIP)
		if err != nil {
			return err
		}
		if count >= int64(limit) {
			return errors.New("该IP注册次数已达上限")
		}
	}

	// 设备维度：开启后注册必须提交设备码
	if app.RegisterDeviceLimitEnabled == 1 {
		if machineCode == "" {
			return errors.New("注册需提供设备码")
		}
		count, err := countBy("register_machine", machineCode)
		if err != nil {
			return err
		}
		if count >= int64(limit) {
			return errors.New("该设备注册次数已达上限")
		}
	}
	return nil
}

// AccountRegister 账号注册（邮箱即账号）：邮箱作为登录名创建注册型账号。
// 应用开启邮箱验证时须校验验证码；开启卡密注册时须额外提交有效卡密，注册即核销该卡并按面值发放时长/点数。
// 不颁发会话令牌——注册账号在无试用/卡密时初始即过期，需登录（或先充值）后方可使用。
func AccountRegister(appUUID, email, password, code, card, registerIP, machineCode string) (*StatusResult, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || password == "" {
		return nil, errors.New("邮箱与密码不能为空")
	}
	if !IsValidEmail(email) {
		return nil, errors.New("邮箱格式不正确")
	}

	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}
	if app.RegisterEnabled != 1 {
		return nil, errors.New("该应用未开启账号注册")
	}

	// 卡密注册：开启后须提交卡号，注册时核销该卡并按面值发放（先校验非空，落库前占用）
	cardNo := strings.TrimSpace(card)
	if app.CardRegisterEnabled == 1 && cardNo == "" {
		return nil, errors.New("请提供注册卡密")
	}

	if err := enforceRegisterLimit(db, app, registerIP, machineCode); err != nil {
		return nil, err
	}

	// 开启邮箱验证则校验验证码
	if app.EmailVerifyEnabled == 1 {
		if err := VerifyRegisterCode(app.UUID, email, code); err != nil {
			return nil, err
		}
	}

	var dup int64
	if err := db.Model(&models.Member{}).Where("app_uuid = ? AND username = ?", app.UUID, email).Count(&dup).Error; err != nil {
		return nil, err
	}
	if dup > 0 {
		return nil, errors.New("该邮箱已注册")
	}

	salt, err := utils.GenerateRandomSalt()
	if err != nil {
		return nil, err
	}
	hashed, err := utils.HashPasswordWithSalt(password, salt)
	if err != nil {
		return nil, err
	}

	member := models.Member{
		AppUUID:         app.UUID,
		Username:        email,
		Email:           email,
		Type:            models.MemberTypeRegister,
		Password:        hashed,
		PasswordSalt:    salt,
		Status:          models.MemberStatusNormal,
		RegisterIP:      strings.TrimSpace(registerIP),
		RegisterMachine: strings.TrimSpace(machineCode),
	}
	if app.OperationMode == models.OperationModePoints {
		// 点数模式：默认注册初始 0 点，需充值；ExpiredAt 留零值
		member.Points = 0
	} else {
		member.ExpiredAt = registerInitialExpiry()
	}

	logDetail := ""
	if app.CardRegisterEnabled == 1 {
		// 事务内：校验并核销卡密、按面值发放、创建账号（一卡一号，避免并发双花）
		err = db.Transaction(func(tx *gorm.DB) error {
			var cardRec models.Card
			if err := tx.Where("app_uuid = ? AND card_no = ?", app.UUID, cardNo).First(&cardRec).Error; err != nil {
				return errors.New("卡号不存在")
			}
			if cardRec.Status == models.CardStatusFrozen {
				return errors.New("卡密已被冻结")
			}
			if cardRec.Status != models.CardStatusUnused {
				return errors.New("该卡已被使用")
			}
			// 按运营模式发放卡面值：点数模式发点数，时长模式发到期时长（-1 为永久）
			// 新号无等级、返利为 0，故按原面值发放
			if app.OperationMode == models.OperationModePoints {
				member.Points = cardRec.Points
			} else {
				member.ExpiredAt = expiryFromDuration(cardRec.Duration)
			}
			if err := tx.Create(&member).Error; err != nil {
				return errors.New("注册失败")
			}
			if err := settleMemberLevel(tx, app.UUID, &member, cardRec.Price); err != nil {
				return err
			}
			return MarkCardUsed(tx, cardRec.ID, member.UUID)
		})
		if err != nil {
			return nil, err
		}
		logDetail = "卡号 " + cardNo
	} else {
		if err := db.Create(&member).Error; err != nil {
			return nil, errors.New("注册失败")
		}
	}

	AddMemberLog(app.UUID, member.UUID, member.Username, "注册", logDetail, "")
	return buildStatusResult(app, &member), nil
}

// trialEligible 账号资源是否已耗尽——仅耗尽的账号才允许领取试用：
//   - 点数模式：点数 <= 0
//   - 时长模式：非永久且已到期
//   - 免费模式：恒可用，不允许领取
func trialEligible(app *models.App, m *models.Member) bool {
	switch app.OperationMode {
	case models.OperationModePoints:
		return m.Points <= 0
	case models.OperationModeFree:
		return false
	default:
		return !isPermanent(m.ExpiredAt) && !m.ExpiredAt.After(time.Now())
	}
}

// ClaimTrial 领取试用：仅当账号资源已耗尽（到期/点数为0）且未超每天/永久领取次数时发放。
// trial_duration 按运营模式解释——时长模式为分钟数，点数模式为点数。
func ClaimTrial(appUUID, username, password string) (*StatusResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, errors.New("用户名与密码不能为空")
	}
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}
	if app.TrialEnabled != 1 || app.TrialDuration <= 0 {
		return nil, errors.New("该应用未开启试用领取")
	}

	var member models.Member
	if err := db.Where("app_uuid = ? AND username = ?", app.UUID, username).First(&member).Error; err != nil {
		return nil, errors.New("账号或密码错误")
	}
	if member.Type != models.MemberTypeRegister || !utils.VerifyPasswordWithSalt(password, member.PasswordSalt, member.Password) {
		return nil, errors.New("账号或密码错误")
	}
	if member.Status != models.MemberStatusNormal {
		return nil, errors.New("账号状态异常")
	}
	// 「到期可领」方案：仅资源已耗尽（到期/点数为0）才可领取，避免有效期内反复叠加
	if app.TrialClaimMode == models.TrialClaimExhaustedOnly && !trialEligible(app, &member) {
		return nil, errors.New("账号仍可用，无需领取试用")
	}

	today := time.Now().Format("2006-01-02")
	used := member.TrialUsed
	if app.TrialLimitTime == 0 && member.TrialDate != today {
		used = 0
	}
	if used > 0 {
		return nil, errors.New("试用领取次数已达上限")
	}

	member.TrialUsed = used + 1
	member.TrialDate = today
	updates := map[string]interface{}{
		"trial_used": member.TrialUsed,
		"trial_date": member.TrialDate,
	}
	if app.OperationMode == models.OperationModePoints {
		// 点数模式：发放试用点数
		member.Points += app.TrialDuration
		updates["points"] = member.Points
	} else {
		// 时长模式：在当前到期时间（或现在）基础上顺延试用时长
		base := member.ExpiredAt
		if base.Before(time.Now()) || base.IsZero() {
			base = time.Now()
		}
		member.ExpiredAt = base.Add(time.Duration(app.TrialDuration) * time.Minute)
		updates["expired_at"] = member.ExpiredAt
	}
	if err := db.Model(&member).Updates(updates).Error; err != nil {
		return nil, err
	}
	return buildStatusResult(app, &member), nil
}

// AccountLogin 账号登录：校验用户名密码后颁发令牌。
func AccountLogin(appUUID, username, password, machineCode, ip, version, deviceName string) (*LoginResult, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, errors.New("用户名与密码不能为空")
	}
	if strings.TrimSpace(version) == "" {
		return nil, errors.New("请提供客户端版本号")
	}

	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}

	var member models.Member
	if err := db.Where("app_uuid = ? AND username = ?", app.UUID, username).First(&member).Error; err != nil {
		return nil, errors.New("账号或密码错误")
	}
	if !utils.VerifyPasswordWithSalt(password, member.PasswordSalt, member.Password) {
		return nil, errors.New("账号或密码错误")
	}
	// 强制更新门禁：版本过旧拒绝登录（在开会话前）
	if err := checkForceUpdate(app, version); err != nil {
		return nil, err
	}

	return finishMemberLogin(db, app, &member, machineCode, ip, version, deviceName)
}

// RechargeByCard 用一张卡为账号充值：把卡面值加到该账号到期时间，并核销卡密。
// 卡与账号须属同一应用；卡须未使用；永久卡直接将账号设为永久。
func RechargeByCard(appUUID, username, cardNo string) (*StatusResult, error) {
	username = strings.TrimSpace(username)
	cardNo = strings.TrimSpace(cardNo)
	if username == "" || cardNo == "" {
		return nil, errors.New("用户名与卡号不能为空")
	}

	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}
	if app.RechargeEnabled != 1 {
		return nil, errors.New("该应用未开启卡密充值")
	}

	var member models.Member
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("app_uuid = ? AND username = ?", app.UUID, username).First(&member).Error; err != nil {
			return errors.New("账号不存在")
		}
		var card models.Card
		if err := tx.Where("app_uuid = ? AND card_no = ?", app.UUID, cardNo).First(&card).Error; err != nil {
			return errors.New("卡号不存在")
		}
		if card.Status != models.CardStatusUnused {
			return errors.New("该卡已被使用或冻结")
		}

		// 会员返利：按「充值前」等级放大面值，之后再结算累充升级
		grantDuration, grantPoints := rebatedCardValue(tx, &member, &card)

		if app.OperationMode == models.OperationModePoints {
			// 点数模式：卡面值为点数，累加到余额
			newPoints := member.Points + grantPoints
			if err := tx.Model(&member).Update("points", newPoints).Error; err != nil {
				return err
			}
			member.Points = newPoints
			if err := settleMemberLevel(tx, app.UUID, &member, card.Price); err != nil {
				return err
			}
			return MarkCardUsed(tx, card.ID, member.UUID)
		}

		// 时长模式：把卡面值加到到期时间
		var newExpiry time.Time
		if isPermanent(member.ExpiredAt) {
			return errors.New("账号已是永久，无需充值")
		}
		if grantDuration == models.CardDurationPermanent {
			newExpiry = models.PermanentTime
		} else {
			base := member.ExpiredAt
			if base.Before(time.Now()) {
				base = time.Now()
			}
			newExpiry = base.Add(time.Duration(grantDuration) * time.Minute)
		}

		if err := tx.Model(&member).Update("expired_at", newExpiry).Error; err != nil {
			return err
		}
		member.ExpiredAt = newExpiry
		if err := settleMemberLevel(tx, app.UUID, &member, card.Price); err != nil {
			return err
		}
		return MarkCardUsed(tx, card.ID, member.UUID)
	})
	if err != nil {
		return nil, err
	}

	AddMemberLog(app.UUID, member.UUID, member.Username, "充值", "卡号 "+cardNo, "")
	return buildStatusResult(app, &member), nil
}

// GetMemberExpiry 获取到期/余额（type 40）：校验令牌有效，返回资源信息（不因已到期/点数耗尽而报错）。
func GetMemberExpiry(appUUID, token string) (*StatusResult, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	member, err := authMemberByToken(db, appUUID, token)
	if err != nil {
		return nil, err
	}
	app, err := loadEnabledApp(db, appUUID)
	if err != nil {
		return nil, err
	}
	return buildStatusResult(app, member), nil
}

// DeductPoints 显式功能扣点（点数模式）：从余额扣除 amount 点，不足则拒绝。
func DeductPoints(appUUID, token string, amount int) (*StatusResult, error) {
	if amount <= 0 {
		return nil, errors.New("扣除点数必须大于0")
	}
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	member, app, err := authActiveMember(db, appUUID, token)
	if err != nil {
		return nil, err
	}
	if app.OperationMode != models.OperationModePoints {
		return nil, errors.New("当前应用非点数模式")
	}
	if member.Points < amount {
		return nil, errors.New("点数不足")
	}
	newPoints := member.Points - amount
	if err := db.Model(member).Update("points", newPoints).Error; err != nil {
		return nil, err
	}
	member.Points = newPoints
	AddMemberLog(app.UUID, member.UUID, member.Username, "扣点", "扣"+strconv.Itoa(amount)+"点", "")
	return buildStatusResult(app, member), nil
}
