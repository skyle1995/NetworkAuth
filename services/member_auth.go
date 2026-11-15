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
	Type              int       `json:"type"` // 来源类型：0 注册 / 1 卡密
	Permanent         bool      `json:"permanent"`
	ExpiredAt         time.Time `json:"expired_at"`         // 时长模式有效
	Points            float64   `json:"points"`             // 点数模式有效
	HeartbeatInterval int       `json:"heartbeat_interval"` // 心跳间隔（分钟），客户端据此周期心跳
	// 会员信息：累计充值（分）+ 当前会员等级（权限等级值、名称、返利比例）；空等级即默认「免费账号」
	TotalRecharge int    `json:"total_recharge"` // 累计充值金额（单位：分）
	Level         int    `json:"level"`          // 权限等级数值（越大权限越高；默认档=1），客户端据此做等级判断
	LevelName     string `json:"level_name"`     // 会员等级名（默认档为 level=1 等级名或「默认会员」）
	LevelColor    string `json:"level_color"`    // 等级显示颜色（hex，默认档灰 #909399）
	RebateRate    int    `json:"rebate_rate"`    // 当前等级充值返利比例（%）

	// 绑定与转绑：设备与 IP 各自一套额度，客户端据此显示
	// 「已绑 2/3 台」「本窗口还能免费换设备 1 次 / 换 IP 2 次」
	MaxDevices   int           `json:"max_devices"`   // 本账号可绑定设备数
	BoundDevices []BoundDevice `json:"bound_devices"` // 已绑定设备列表（按绑定时间升序）
	BoundIPs     []string      `json:"bound_ips"`     // 已绑定 IP 列表

	MachineRebindFreeLeft  int `json:"machine_rebind_free_left"`  // 设备转绑：本窗口剩余免费次数
	MachineRebindUsed      int `json:"machine_rebind_used"`       // 设备转绑：本窗口已用
	MachineRebindMaxCount  int `json:"machine_rebind_max_count"`  // 设备转绑：上限，0=不限
	MachineRebindLimitTime int `json:"machine_rebind_limit_time"` // 设备转绑：0=每天 / 1=永久

	IPRebindFreeLeft  int `json:"ip_rebind_free_left"`  // IP 转绑：本窗口剩余免费次数
	IPRebindUsed      int `json:"ip_rebind_used"`       // IP 转绑：本窗口已用
	IPRebindMaxCount  int `json:"ip_rebind_max_count"`  // IP 转绑：上限，0=不限
	IPRebindLimitTime int `json:"ip_rebind_limit_time"` // IP 转绑：0=每天 / 1=永久

	// Update：更新判断结果。仅当应用更新方式非「不启用」时返回；据登录提交的版本号判断是否需要更新。
	Update *LoginUpdate `json:"update,omitempty"`
}

// applyBindingInfo 把绑定快照写进登录返回（两个结果结构共用同一批字段）
func (r *LoginResult) applyBindingInfo(b BindingInfo) {
	r.MaxDevices = b.MaxDevices
	r.BoundDevices = b.BoundDevices
	r.BoundIPs = b.BoundIPs
	r.MachineRebindFreeLeft = b.MachineFreeLeft
	r.MachineRebindUsed = b.MachineUsed
	r.MachineRebindMaxCount = b.MachineMaxCount
	r.MachineRebindLimitTime = b.MachineLimitTime
	r.IPRebindFreeLeft = b.IPFreeAllLeft
	r.IPRebindUsed = b.IPUsed
	r.IPRebindMaxCount = b.IPMaxCount
	r.IPRebindLimitTime = b.IPLimitTime
}

// applyBindingInfo 同上，用于心跳/状态查询返回
func (r *StatusResult) applyBindingInfo(b BindingInfo) {
	r.MaxDevices = b.MaxDevices
	r.BoundDevices = b.BoundDevices
	r.BoundIPs = b.BoundIPs
	r.MachineRebindFreeLeft = b.MachineFreeLeft
	r.MachineRebindUsed = b.MachineUsed
	r.MachineRebindMaxCount = b.MachineMaxCount
	r.MachineRebindLimitTime = b.MachineLimitTime
	r.IPRebindFreeLeft = b.IPFreeAllLeft
	r.IPRebindUsed = b.IPUsed
	r.IPRebindMaxCount = b.IPMaxCount
	r.IPRebindLimitTime = b.IPLimitTime
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

// SessionInfo 在线会话信息（手动顶号满员时返回，供客户端展示并选择踢出）
type SessionInfo struct {
	ID           uint      `json:"id"`             // 会话ID（踢下线时作为 session_id 提交）
	DeviceName   string    `json:"device_name"`    // 设备名称
	MachineCode  string    `json:"machine_code"`   // 机器码
	IP           string    `json:"ip"`             // 登录IP
	Province     string    `json:"province"`       // IP归属省
	City         string    `json:"city"`           // IP归属市
	Version      string    `json:"version"`        // 客户端版本号
	LastActiveAt time.Time `json:"last_active_at"` // 最近活跃时间
	CreatedAt    time.Time `json:"created_at"`     // 登录时间
}

// SessionsFullError 手动顶号满员拦截：在线数已达上限，附带当前会话列表，
// 客户端据此弹窗让用户选择踢出哪个会话后再重试登录。
// 类似 ForceUpdateError，经 errors.As 在公开 API 入口识别为专用响应 code。
type SessionsFullError struct {
	Sessions []SessionInfo
}

func (e *SessionsFullError) Error() string {
	return "已达最大同时在线数，请选择要下线的设备"
}

// StatusResult 账号状态查询返回的信息
type StatusResult struct {
	Username          string    `json:"username"`
	Type              int       `json:"type"` // 来源类型：0注册/1卡密
	Status            int       `json:"status"`
	Permanent         bool      `json:"permanent"`
	ExpiredAt         time.Time `json:"expired_at"`
	Points            float64   `json:"points"`
	HeartbeatInterval int       `json:"heartbeat_interval"` // 心跳间隔（分钟），客户端可据此动态调整

	// 续期后的租约签名（与 70 同格式），客户端用于刷新本地租约文件
	LeaseExpire int64  `json:"lease_expire"`
	LeaseSig    string `json:"lease_sig"`
	// 会员信息：随心跳/状态查询实时下发，客户端据此同步权限，避免中途改等级后仍用旧权限
	TotalRecharge  int    `json:"total_recharge"`  // 累计充值金额（单位：分）
	Level         int    `json:"level"`          // 权限等级数值（越大权限越高；默认档=1）
	LevelName     string `json:"level_name"`     // 会员等级名（默认档为 level=1 等级名或「默认会员」）
	LevelColor    string `json:"level_color"`    // 等级显示颜色（hex，默认档灰 #909399）
	RebateRate    int    `json:"rebate_rate"`    // 当前等级充值返利比例（%）

	// 绑定与转绑：随心跳实时刷新，客户端不必另发请求
	MaxDevices   int           `json:"max_devices"`
	BoundDevices []BoundDevice `json:"bound_devices"`
	BoundIPs     []string      `json:"bound_ips"`

	MachineRebindFreeLeft  int `json:"machine_rebind_free_left"`
	MachineRebindUsed      int `json:"machine_rebind_used"`
	MachineRebindMaxCount  int `json:"machine_rebind_max_count"`
	MachineRebindLimitTime int `json:"machine_rebind_limit_time"`

	IPRebindFreeLeft  int `json:"ip_rebind_free_left"`
	IPRebindUsed      int `json:"ip_rebind_used"`
	IPRebindMaxCount  int `json:"ip_rebind_max_count"`
	IPRebindLimitTime int `json:"ip_rebind_limit_time"`
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

// checkMemberUsable 校验账号对本应用的可用性。
//
// 判定维度由 Billing 决定：
//   - 计时模式：账号未到期（永久放行）
//   - 点数模式：不校验等级与到期，只看余额是否够一次消耗
//   - 高级模式：等级达标 + 余额够一次消耗
//
// 免费豁免时全部跳过。
func checkMemberUsable(db *gorm.DB, billing *Billing, m *models.Member) error {
	if billing.Free {
		return nil
	}
	// 等级门槛（计时模式与高级模式）
	if billing.RequiresLevel() {
		cur := memberPermissionLevel(db, m)
		if cur < billing.RequiredLevel {
			name := levelNameByValue(db, billing.RequiredLevel)
			return &EntitlementError{NeedLevel: billing.RequiredLevel, NeedName: name}
		}
	}
	// 有效期门槛（仅计时模式）
	if billing.RequiresValidPeriod() && !isPermanent(m.ExpiredAt) && m.ExpiredAt.Before(time.Now()) {
		return &LicenseExpiredError{}
	}
	// 点数门槛：按次看单次价，按时看一个周期价
	if billing.UsesPoints() {
		cost := billing.PointsCostForLevel(m.LevelUUID, !billing.IsPointsChargeMode())
		if cost > 0 && m.Points+1e-6 < cost {
			return &InsufficientCreditsError{Need: cost, Have: m.Points}
		}
	}
	return nil
}

// effectiveMaxConcurrent 账号级并发上限 = 账号策略 + 等级加量，下限 1。
//
// 并发是**账号**资源：一个账号同时最多几台设备在线（跨所有应用合计），
// 按设备去重，同一台电脑开多个插件不重复占位。
func effectiveMaxConcurrent(db *gorm.DB, m *models.Member) int {
	return ResolveAccountQuota(db, m).MaxConcurrent
}

// pointsPerPeriod 按时模式每周期扣点（至少1）
func pointsPerPeriod(app *models.App) float64 {
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
func settlePointsTime(tx *gorm.DB, app *models.App, m *models.Member) error {
	if app.BillingMode != models.BillingModePoints || app.PointsChargeMode != models.PointsChargePerTime {
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
	// 查当前会员等级信息随状态实时下发（在事务外调用，用全局连接安全）
	var levelName, levelColor string
	var levelValue, rebateRate int
	if db, err := database.GetDB(); err == nil {
		levelName, levelValue, rebateRate, levelColor = memberLevelInfo(db, m)
	}
	return &StatusResult{
		Username:          m.Username,
		Type:              m.Type,
		Status:            m.Status,
		Permanent:         isPermanent(m.ExpiredAt),
		ExpiredAt:         m.ExpiredAt,
		Points:            m.Points,
		HeartbeatInterval: heartbeatMinutes(app),
		TotalRecharge:     m.TotalRecharge,
		Level:             levelValue,
		LevelName:         levelName,
		LevelColor:        levelColor,
		RebateRate:        rebateRate,
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
	if !GetAccountPolicy().CardLoginEnabled {
		return nil, errors.New("卡密登录已关闭")
	}
	// 强制更新门禁：须在核销卡密/建号前拦截，避免拒绝登录却已消费卡
	if err := checkForceUpdate(&app, version); err != nil {
		return nil, err
	}

	var member models.Member
	err = db.Transaction(func(tx *gorm.DB) error {
		var card models.Card
		if err := tx.Where("card_no = ?", cardNo).First(&card).Error; err != nil {
			return errors.New("卡号不存在")
		}
		if card.Status == models.CardStatusFrozen {
			return errors.New("卡密已被冻结")
		}

		if card.Status == models.CardStatusUnused {
			// 首次使用：激活并创建绑定该卡的全局账号
			member = models.Member{
				Username: cardNo,
				Type:     models.MemberTypeCard,
				CardUUID: card.UUID,
				Status:   models.MemberStatusNormal,
			}
			// 新号无等级、返利为 0，故按原面值发放。
			// 一号通：卡面值可能同时含时长与点数，各自按非零项发放
			if card.Points > 0 {
				member.Points = card.Points
			}
			if card.Duration != 0 {
				member.ExpiredAt = expiryFromDuration(card.Duration)
			}
			if err := tx.Create(&member).Error; err != nil {
				return errors.New("激活卡密失败")
			}
			if err := settleMemberLevel(tx, &member, card.Price); err != nil {
				return err
			}
			if err := MarkCardUsed(tx, card.ID, member.UUID); err != nil {
				return errors.New("核销卡密失败")
			}
			return nil
		}

		// 已使用：定位其绑定的账号（卡号即用户名，全站唯一）
		if err := tx.Where("username = ?", cardNo).First(&member).Error; err != nil {
			return errors.New("卡密账号不存在")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// 二级令牌：只做身份认证，计费与绑定留给 70 激活
	return issueAccountTokenResult(db, &app, &member, machineCode, ip, version, deviceName)
}
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
// ensureMachineBinding 确保机器码已绑定。
//
// 未绑定时在**账号级设备配额**内新增（账号最多绑几台机器），超出则拒绝；
// 配额来自「账号策略·可绑定设备」+ 会员等级额外设备。
// 须在持有账号锁的事务内调用（见 lockMemberRow），否则并发的先查后写会击穿上限。
func ensureMachineBinding(db *gorm.DB, memberUUID, machineCode, deviceName string, maxDevices int) error {
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
	if maxDevices <= 0 {
		maxDevices = 1
	}
	if int(count) >= maxDevices {
		return errors.New("机器码未绑定，请先进行机器码转绑")
	}
	return db.Create(&models.Binding{
		MemberUUID: memberUUID,
		Type:       models.BindingTypeMachine,
		Value:      machineCode,
		DeviceName: deviceName,
	}).Error
}

// lockMemberRow 在登录事务内对账号行加排他锁，串行化同一账号的并发登录。
// 采用等值主键 UPDATE 实现：MySQL 取行级 X 锁（持锁期间后到的并发事务等待，其后的快照读
// 建立在锁释放之后，可见锁持有者已提交的绑定/会话，保证计数一致）；SQLite 连接池为单连接、
// 写操作本就全局串行，该语句仅作多连接配置下的兜底。UpdateColumn 跳过更新时间自动维护与
// 钩子，不产生任何数据副作用。
func lockMemberRow(tx *gorm.DB, memberUUID string) error {
	return tx.Model(&models.Member{}).
		Where("uuid = ?", memberUUID).
		UpdateColumn("id", gorm.Expr("id")).Error
}


// ensureIPBinding 确保登录 IP 满足应用 IP 验证配置；首次登录会自动绑定当前 IP。
// 须在持有账号锁的事务内调用（见 lockMemberRow），否则并发的先查后写会击穿绑定数量上限。
func ensureIPBinding(db *gorm.DB, memberUUID, ip string, ipVerify, maxDevices int) error {
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

	if maxDevices <= 0 {
		maxDevices = 1
	}
	if len(bindings) >= maxDevices {
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
func authActiveMember(db *gorm.DB, appUUID, token string) (*models.Member, *models.App, error) {
	// 激活令牌走正常路径；账号令牌会隐式激活（幂等，不重复扣费）
	member, app, _, err := AuthForApp(db, appUUID, token)
	if err != nil {
		return nil, nil, err
	}
	if member.Status != models.MemberStatusNormal {
		return nil, nil, errors.New("账号状态异常")
	}
	billing := EffectiveBilling(app)
	if err := checkMemberUsable(db, billing, member); err != nil {
		return nil, nil, err
	}
	return member, app, nil
}

// authMemberByCredential 用凭据鉴权账号（不依赖会话令牌）：
// 注册账号校验用户名+密码；卡密账号以卡号为身份、无密码。供转绑等“可能登录不了”的场景用，
// 避免“设备/IP 不匹配→登不进→拿不到令牌→无法转绑”的死循环。
func authMemberByCredential(db *gorm.DB, username, password, ip string) (*models.Member, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("用户名不能为空")
	}
	guard := GetLoginGuard()
	guardUsername := "public:" + username
	if allowed, _ := guard.Allow(ip, guardUsername); !allowed {
		return nil, errors.New("登录尝试过多，请稍后重试")
	}
	var member models.Member
	if err := db.Where("username = ?", username).
		First(&member).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		guard.OnCredentialFailure(ip, guardUsername)
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
			guard.OnCredentialFailure(ip, guardUsername)
			return nil, errors.New("密码错误")
		}
	}
	guard.OnSuccess(ip, guardUsername)
	return &member, nil
}

// CheckMemberStatus 心跳（type 41）：续租 + 按应用的计费配置结算。
//
// 扣不扣点完全由应用配置决定，客户端无权干预：
//   - 免费豁免 / 计时模式 / 按次模式：心跳不扣费，只续租
//   - 按时模式 + 登录预扣（charge_basis=0）：激活时已扣，心跳只在周期走完后续下一个
//   - 按时模式 + 首次心跳起算（charge_basis=1）：首次心跳扣一次，之后按周期续
//
// 续费失败（余额不足）会返回错误，激活已被置死——客户端应退出并提示充值。
func CheckMemberStatus(appUUID, token string) (*StatusResult, error) {
	db, err := database.GetDB()
	if err != nil {
		return nil, err
	}
	// 应用级接口只接受激活令牌（账号令牌返回 ACTIVATION_REQUIRED）
	member, app, act, err := AuthForApp(db, appUUID, token)
	if err != nil {
		return nil, err
	}
	if _, _, _, err := RenewLease(db, act.Token); err != nil {
		return nil, err
	}
	// 重新读一次，拿到扣费后的最新余额/到期
	if err := db.Where("uuid = ?", member.UUID).First(member).Error; err != nil {
		return nil, err
	}
	out := buildStatusResult(app, member)
	// 续期成功：把新的租约到期时间与签名一起下发，客户端据此刷新本地租约文件
	if act != nil {
		expire := act.LeaseExpiresAt.Unix()
		sig, sigErr := SignLease(app.UUID, act.Token, act.Fingerprint, expire)
		if sigErr != nil {
			return nil, sigErr
		}
		out.LeaseExpire = expire
		out.LeaseSig = sig
	}
	return out, nil
}

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
	policy := GetAccountPolicy()
	limit := policy.RegisterCount
	if limit <= 0 {
		limit = 1
	}

	// 时间窗口（每天/永久），IP 与设备维度共用
	withWindow := func(q *gorm.DB) *gorm.DB {
		if policy.RegisterLimitTime == 0 {
			today := time.Now()
			startOfDay := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
			q = q.Where("created_at >= ?", startOfDay)
		}
		return q
	}
	countBy := func(field, value string) (int64, error) {
		var count int64
		q := withWindow(db.Model(&models.Member{}).
			Where(field+" = ?", value))
		err := q.Count(&count).Error
		return count, err
	}

	// IP 维度
	if GetAccountPolicy().RegisterLimitEnabled {
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
	if GetAccountPolicy().RegisterDeviceLimitEnabled {
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
	if !GetAccountPolicy().RegisterEnabled {
		return nil, errors.New("该应用未开启账号注册")
	}

	// 卡密注册：开启后须提交卡号，注册时核销该卡并按面值发放（先校验非空，落库前占用）
	cardNo := strings.TrimSpace(card)
	if GetAccountPolicy().CardRegisterEnabled && cardNo == "" {
		return nil, errors.New("请提供注册卡密")
	}

	if err := enforceRegisterLimit(db, app, registerIP, machineCode); err != nil {
		return nil, err
	}

	// 开启邮箱验证则校验验证码
	if GetAccountPolicy().EmailVerifyEnabled {
		if err := VerifyRegisterCode(email, code); err != nil {
			return nil, err
		}
	}

	var dup int64
	if err := db.Model(&models.Member{}).Where("username = ?", email).Count(&dup).Error; err != nil {
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
		Username:        email,
		Email:           email,
		Type:            models.MemberTypeRegister,
		Password:        hashed,
		PasswordSalt:    salt,
		Status:          models.MemberStatusNormal,
		RegisterIP:      strings.TrimSpace(registerIP),
		RegisterMachine: strings.TrimSpace(machineCode),
	}
	if app.BillingMode == models.BillingModePoints {
		// 点数模式：默认注册初始 0 点，需充值；ExpiredAt 留零值
		member.Points = 0
	} else {
		member.ExpiredAt = registerInitialExpiry()
	}

	logDetail := ""
	if GetAccountPolicy().CardRegisterEnabled {
		// 事务内：校验并核销卡密、按面值发放、创建账号（一卡一号，避免并发双花）
		err = db.Transaction(func(tx *gorm.DB) error {
			var cardRec models.Card
			if err := tx.Where("card_no = ?", cardNo).First(&cardRec).Error; err != nil {
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
			if app.BillingMode == models.BillingModePoints {
				member.Points = cardRec.Points
			} else {
				member.ExpiredAt = expiryFromDuration(cardRec.Duration)
			}
			if err := tx.Create(&member).Error; err != nil {
				return errors.New("注册失败")
			}
			if err := settleMemberLevel(tx, &member, cardRec.Price); err != nil {
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
		// 无卡密注册：同样要落到默认档，避免 level_uuid 为空
		if err := settleMemberLevel(db, &member, 0); err != nil {
			return nil, err
		}
	}

	AddMemberLog(app.UUID, member.UUID, member.Username, "注册", logDetail, "")
	return buildStatusResult(app, &member), nil
}

// trialEligible 账号资源是否已耗尽——仅耗尽的账号才允许领取试用：
//   - 点数模式：点数 <= 0
//   - 时长模式：非永久且已到期
//   - 免费模式：恒可用，不允许领取
// trialEligible 「资源耗尽」判定：到期没时长 或 点数见底，任一成立即可领。
//
// 一号通下同一账号可能既有时长又有点数，因此不再按计费模式二选一，
// 而是看两类资源是不是都耗光了（有点数或有到期时间 = 还能用，不该再领）。
func trialEligible(app *models.App, m *models.Member) bool {
	hasTime := isPermanent(m.ExpiredAt) || m.ExpiredAt.After(time.Now())
	hasPoints := m.Points > 0
	return !hasTime && !hasPoints
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
	if !GetAccountPolicy().TrialEnabled || !GetAccountPolicy().HasTrialQuota() {
		return nil, errors.New("该应用未开启试用领取")
	}

	guard := GetLoginGuard()
	guardUsername := "public:" + username
	if allowed, _ := guard.Allow("", guardUsername); !allowed {
		return nil, errors.New("登录尝试过多，请稍后重试")
	}
	var member models.Member
	if err := db.Where("username = ?", username).First(&member).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		guard.OnCredentialFailure("", guardUsername)
		return nil, errors.New("账号或密码错误")
	}
	if member.Type != models.MemberTypeRegister || !utils.VerifyPasswordWithSalt(password, member.PasswordSalt, member.Password) {
		guard.OnCredentialFailure("", guardUsername)
		return nil, errors.New("账号或密码错误")
	}
	guard.OnSuccess("", guardUsername)
	if member.Status != models.MemberStatusNormal {
		return nil, errors.New("账号状态异常")
	}
	// 「到期可领」方案：仅资源已耗尽（到期/点数为0）才可领取，避免有效期内反复叠加
	if GetAccountPolicy().TrialClaimMode == models.TrialClaimExhaustedOnly && !trialEligible(app, &member) {
		return nil, errors.New("账号仍可用，无需领取试用")
	}

	today := time.Now().Format("2006-01-02")
	used := member.TrialUsed
	if GetAccountPolicy().TrialLimitTime == 0 && member.TrialDate != today {
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
	// 试用额度是全局策略：点数与时长可同时发放，各自独立判断
	trialPolicy := GetAccountPolicy()
	if trialPolicy.TrialPoints > 0 {
		member.Points += trialPolicy.TrialPoints
		updates["points"] = member.Points
	}
	if trialPolicy.TrialDurationMinutes > 0 {
		base := member.ExpiredAt
		if base.Before(time.Now()) || base.IsZero() {
			base = time.Now()
		}
		member.ExpiredAt = base.Add(time.Duration(trialPolicy.TrialDurationMinutes) * time.Minute)
		updates["expired_at"] = member.ExpiredAt
	}
	if err := db.Model(&member).Updates(updates).Error; err != nil {
		return nil, err
	}
	return buildStatusResult(app, &member), nil
}

// AccountLogin 账号登录：先检查账号/IP 失败锁定，校验凭据后颁发令牌。
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

	guard := GetLoginGuard()
	guardUsername := "public:" + username
	if allowed, _ := guard.Allow(ip, guardUsername); !allowed {
		return nil, errors.New("登录尝试过多，请稍后重试")
	}
	var member models.Member
	if err := db.Where("username = ?", username).First(&member).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		guard.OnCredentialFailure(ip, guardUsername)
		return nil, errors.New("账号或密码错误")
	}
	if !utils.VerifyPasswordWithSalt(password, member.PasswordSalt, member.Password) {
		guard.OnCredentialFailure(ip, guardUsername)
		return nil, errors.New("账号或密码错误")
	}
	guard.OnSuccess(ip, guardUsername)
	// 强制更新门禁：版本过旧拒绝登录（在开会话前）
	if err := checkForceUpdate(app, version); err != nil {
		return nil, err
	}

	// 二级令牌：只做身份认证，计费与绑定留给 70 激活
	return issueAccountTokenResult(db, app, &member, machineCode, ip, version, deviceName)
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
	if !GetAccountPolicy().RechargeEnabled {
		return nil, errors.New("该应用未开启卡密充值")
	}

	var member models.Member
	err = db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("username = ?", username).First(&member).Error; err != nil {
			return errors.New("账号不存在")
		}
		var card models.Card
		if err := tx.Where("card_no = ?", cardNo).First(&card).Error; err != nil {
			return errors.New("卡号不存在")
		}
		if card.Status != models.CardStatusUnused {
			return errors.New("该卡已被使用或冻结")
		}

		// 会员返利：按「充值前」等级放大面值，之后再结算累充升级
		grantDuration, grantPoints := rebatedCardValue(tx, &member, &card)

		if app.BillingMode == models.BillingModePoints {
			// 点数模式：卡面值为点数，累加到余额
			newPoints := member.Points + grantPoints
			if err := tx.Model(&member).Update("points", newPoints).Error; err != nil {
				return err
			}
			member.Points = newPoints
			if err := settleMemberLevel(tx, &member, card.Price); err != nil {
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
		if err := settleMemberLevel(tx, &member, card.Price); err != nil {
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
	// 账号级查询：接受账号令牌（激活令牌也放行，方便客户端复用）。
	// 只回报资源，不因已到期/点数耗尽而失败——客户端要能查到「为什么不能用」。
	member, err := AuthForAccountAny(db, token)
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
func DeductPoints(appUUID, token string, amount float64) (*StatusResult, error) {
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
	if app.BillingMode != models.BillingModePoints {
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
	AddMemberLog(app.UUID, member.UUID, member.Username, "扣点", "扣"+strconv.FormatFloat(amount, 'f', -1, 64)+"点", "")
	return buildStatusResult(app, member), nil
}

// issueAccountTokenResult 登录收尾：只校验账号状态，然后颁发**账号令牌**。
//
// 二级令牌模型下，登录只回答「你是谁」：
//   - 不扣费、不校验等级与到期、不做设备绑定、不占在线名额
//   - 到期账号也能登录（否则用户没法进去充值续费）
//
// 计费与绑定统一下放到 70 应用激活。
func issueAccountTokenResult(db *gorm.DB, app *models.App, member *models.Member, machineCode, ip, version, deviceName string) (*LoginResult, error) {
	if member.Status == models.MemberStatusBlack {
		return nil, errors.New("账号已被拉黑")
	}
	if member.Status == models.MemberStatusDisabled {
		return nil, errors.New("账号已被封停")
	}

	machineCode = strings.TrimSpace(machineCode)
	deviceName = strings.TrimSpace(deviceName)

	// 设备与 IP 绑定：**登录时立即建立**，不等到 70 激活。
	//
	// 理由：设备白名单是账号级资产（上限取 account_max_devices），登录时就能算，
	// 不依赖具体应用。用户"登录"本身就代表这台机器开始使用，白名单该在这一刻成型；
	// 70 激活时的绑定校验只是复核（幂等放行）。
	//
	// 与应用的 machine_verify / ip_verify 解耦：那两个开关只决定「激活时是否复核」，
	// 不影响登录是否把设备记进白名单。
	quota := ResolveAccountQuota(db, member)
	policy := GetAccountPolicy()
	if err := db.Transaction(func(tx *gorm.DB) error {
		// 账号级互斥：串行化同账号并发登录，防止并发的「先查后写」击穿设备上限
		if err := lockMemberRow(tx, member.UUID); err != nil {
			return err
		}
		// 只有开启「设备校验」才建立绑定；关闭时登录不写白名单，
		// 也不受「可绑定设备」上限约束（与原应用级开关语义一致）
		if policy.MachineVerify == 1 && machineCode != "" {
			if err := ensureMachineBinding(tx, member.UUID, machineCode, deviceName, quota.MaxDevices); err != nil {
				return &FingerprintMismatchError{Cause: err}
			}
		}
		// IP 绑定**不在登录时校验**：用户换网络（家用/公司/移动热点）很常见，
		// 登录就拦会把人锁在门外。IP 复核放在 70 激活时做。
		return nil
	}); err != nil {
		return nil, err
	}

	acct, err := IssueAccountToken(db, member.UUID, machineCode, ip, version, deviceName)
	if err != nil {
		return nil, err
	}

	levelName, level, rebate, levelColor := memberLevelInfo(db, member)

	res := &LoginResult{
		Token:             acct.Token,
		Username:          member.Username,
		Type:              member.Type,

		Permanent:         isPermanent(member.ExpiredAt),
		ExpiredAt:         member.ExpiredAt,
		Points:            member.Points,
		HeartbeatInterval: heartbeatMinutes(app),
		TotalRecharge:     member.TotalRecharge,
		Level:             level,
		LevelName:         levelName,
		LevelColor:        levelColor,
		RebateRate:        rebate,
	}
	res.Update = buildLoginUpdate(app, version)
	res.applyBindingInfo(BuildBindingInfo(db, member))

	// 登录痕迹：不计费，仅供风控与后台展示。
	// 注册时没采集设备码的账号（如邮箱注册），首次带设备码登录时补上，供后台与风控使用。
	trace := map[string]any{
		"last_login_at": time.Now(),
		"last_login_ip": ip,
	}
	if strings.TrimSpace(member.RegisterMachine) == "" && strings.TrimSpace(machineCode) != "" {
		trace["register_machine"] = strings.TrimSpace(machineCode)
		member.RegisterMachine = strings.TrimSpace(machineCode)
	}
	db.Model(&models.Member{}).Where("uuid = ?", member.UUID).Updates(trace)
	AddMemberLog(app.UUID, member.UUID, member.Username, "账号登录", machineCode, ip)
	return res, nil
}
