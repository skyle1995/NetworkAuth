package services

import (
	"strings"
	"sync"
	"time"

	"NetworkAuth/config"
)

// ============================================================================
// 登录防爆破守卫（LoginGuard）
//
// 背景：验证码只能提高自动化成本，无法作为唯一防线（滑块/点击类验证码可被脚本
// 图像识别或撞库绕过）。当后台只有一个 admin 账号时，一旦验证码被绕开，
// 攻击者就能无限次猜密码。本守卫提供第二道防线：
//
//   - 账号维度：连续失败达到阈值后临时锁定该账号（可防御分布式 IP 轮换）；
//   - IP 维度：同一 IP 的失败（含验证码错误）达到阈值后临时锁定该 IP；
//   - 指数退避：重复触发锁定时，锁定时长按 2 倍递增，最高 6 小时；
//   - 验证码错误只累加 IP 维度，避免攻击者用错误验证码把正常账号锁死（DoS）。
//
// 状态保存在进程内存中：服务重启后计数清零；多实例部署时各实例独立计数。
// 对于 HTTP 后台登录这种低频场景，内存实现已足够，且不引入额外数据库压力。
// ============================================================================

const (
	// loginGuardMaxEntries 内存中 key 上限，防止被海量随机 IP 撑爆
	loginGuardMaxEntries = 50000
	// loginGuardEntryTTL 空闲条目保留时长（超过后清理，锁定等级随之重置）
	loginGuardEntryTTL = 2 * time.Hour
	// loginGuardLockCap 单次锁定最大时长
	loginGuardLockCap = 6 * time.Hour
	// loginGuardCleanupInterval 定期清理间隔
	loginGuardCleanupInterval = 5 * time.Minute

	// 缺省策略（配置文件 security.* 未配置或非法时使用）
	defaultLoginMaxFailures   = 10
	defaultLoginIPMaxFailures = 30
	defaultLoginWindowMinute  = 15
	defaultLoginLockMinute    = 15
	defaultLoginIPLockMinute  = 10
)

// loginGuardEntry 单个 key（账号或 IP）的失败计数与锁定状态
type loginGuardEntry struct {
	failures    int       // 当前统计窗口内的失败次数
	windowStart time.Time // 当前统计窗口起始时间
	lockedUntil time.Time // 锁定截止时间
	lockLevel   int       // 已触发的锁定次数（用于指数退避）
	lastSeen    time.Time // 最近一次更新时间（用于清理）
}

// LoginGuard 登录失败保护器
type LoginGuard struct {
	mu          sync.Mutex
	entries     map[string]*loginGuardEntry
	maxFail     int           // 账号维度阈值
	ipMaxFail   int           // IP 维度阈值
	window      time.Duration // 失败统计窗口
	lock        time.Duration // 账号锁定基础时长
	ipLock      time.Duration // IP 锁定基础时长
	lastCleanup time.Time
}

var (
	loginGuardOnce sync.Once
	loginGuardInst *LoginGuard
)

// GetLoginGuard 获取全局登录守卫单例（策略从配置文件 security.* 读取）
func GetLoginGuard() *LoginGuard {
	loginGuardOnce.Do(func() {
		maxFail, ipMaxFail, window, lock, ipLock := loadLoginGuardPolicy()
		loginGuardInst = newLoginGuard(maxFail, ipMaxFail, window, lock, ipLock)
	})
	return loginGuardInst
}

// newLoginGuard 创建登录守卫（便于测试注入策略）
func newLoginGuard(maxFail, ipMaxFail int, window, lock, ipLock time.Duration) *LoginGuard {
	if window <= 0 {
		window = defaultLoginWindowMinute * time.Minute
	}
	if lock <= 0 {
		lock = defaultLoginLockMinute * time.Minute
	}
	if ipLock <= 0 {
		ipLock = defaultLoginIPLockMinute * time.Minute
	}
	return &LoginGuard{
		entries:   make(map[string]*loginGuardEntry),
		maxFail:   maxFail,
		ipMaxFail: ipMaxFail,
		window:    window,
		lock:      lock,
		ipLock:    ipLock,
	}
}

// loadLoginGuardPolicy 从配置文件读取策略，非法值回退到安全缺省值
func loadLoginGuardPolicy() (maxFail, ipMaxFail int, window, lock, ipLock time.Duration) {
	sec := config.SecurityConfig{}
	if cfg := config.GetRuntimeConfig(); cfg != nil {
		sec = cfg.Security
	}

	maxFail = sec.LoginMaxFailures
	if maxFail <= 0 {
		maxFail = defaultLoginMaxFailures
	}
	ipMaxFail = sec.LoginIPMaxFailures
	if ipMaxFail <= 0 {
		ipMaxFail = defaultLoginIPMaxFailures
	}
	windowMin := sec.LoginWindowMinutes
	if windowMin <= 0 {
		windowMin = defaultLoginWindowMinute
	}
	lockMin := sec.LoginLockMinutes
	if lockMin <= 0 {
		lockMin = defaultLoginLockMinute
	}
	ipLockMin := sec.LoginIPLockMinutes
	if ipLockMin <= 0 {
		ipLockMin = defaultLoginIPLockMinute
	}

	return maxFail, ipMaxFail,
		time.Duration(windowMin) * time.Minute,
		time.Duration(lockMin) * time.Minute,
		time.Duration(ipLockMin) * time.Minute
}

// Allow 判断当前账号/IP 是否允许尝试登录。
// 返回 false 时，retryAfter 为还需要等待的时长。
// 应在验证码校验与密码校验之前调用，被锁定的请求直接拒绝，避免浪费 CPU 与验证码。
func (g *LoginGuard) Allow(ip, username string) (bool, time.Duration) {
	now := time.Now()

	g.mu.Lock()
	defer g.mu.Unlock()

	g.cleanupLocked(now)

	var remain time.Duration
	for _, key := range loginGuardKeys(ip, username) {
		e := g.entries[key]
		if e == nil {
			continue
		}
		if now.Before(e.lockedUntil) {
			if d := e.lockedUntil.Sub(now); d > remain {
				remain = d
			}
		}
	}

	if remain > 0 {
		return false, remain
	}
	return true, 0
}

// OnCredentialFailure 记录一次「账号不存在/密码错误」：同时累加账号与 IP 维度。
func (g *LoginGuard) OnCredentialFailure(ip, username string) {
	now := time.Now()

	g.mu.Lock()
	defer g.mu.Unlock()

	g.cleanupLocked(now)

	g.failLocked(accountKey(username), now, g.maxFail, g.lock)
	g.failLocked(ipKey(ip), now, g.ipMaxFail, g.ipLock)
}

// OnCaptchaFailure 记录一次验证码错误：只累加当前 IP，不触碰账号计数，
// 防止攻击者故意提交错误验证码把正常账号锁死。
func (g *LoginGuard) OnCaptchaFailure(ip string) {
	now := time.Now()

	g.mu.Lock()
	defer g.mu.Unlock()

	g.cleanupLocked(now)

	g.failLocked(ipKey(ip), now, g.ipMaxFail, g.ipLock)
}

// OnSuccess 登录成功：清除该账号与 IP 的失败计数和锁定状态。
func (g *LoginGuard) OnSuccess(ip, username string) {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.entries, accountKey(username))
	delete(g.entries, ipKey(ip))
}

// failLocked 累加失败次数，达到阈值时锁定（指数退避）。调用方必须持有锁。
func (g *LoginGuard) failLocked(key string, now time.Time, threshold int, baseLock time.Duration) {
	if key == "" {
		return
	}

	e := g.entries[key]
	if e == nil {
		e = &loginGuardEntry{windowStart: now}
		g.entries[key] = e
	}

	// 窗口过期则重新开始计数
	if now.Sub(e.windowStart) > g.window {
		e.failures = 0
		e.windowStart = now
	}

	e.failures++
	e.lastSeen = now

	if threshold <= 0 || e.failures < threshold {
		return
	}

	// 达到阈值：锁定并升级退避等级
	e.lockLevel++
	lockFor := scaleLockDuration(baseLock, e.lockLevel)
	e.lockedUntil = now.Add(lockFor)
	e.failures = 0
	e.windowStart = now
}

// scaleLockDuration 按锁定等级指数放大锁定时长，最高不超过 loginGuardLockCap
func scaleLockDuration(base time.Duration, level int) time.Duration {
	d := base
	for i := 1; i < level; i++ {
		d *= 2
		if d >= loginGuardLockCap {
			return loginGuardLockCap
		}
	}
	if d > loginGuardLockCap {
		return loginGuardLockCap
	}
	return d
}

// cleanupLocked 定期清理空闲条目，并在条目过多时回收未锁定条目。调用方必须持有锁。
func (g *LoginGuard) cleanupLocked(now time.Time) {
	if now.Sub(g.lastCleanup) < loginGuardCleanupInterval && len(g.entries) <= loginGuardMaxEntries {
		return
	}
	g.lastCleanup = now

	for k, e := range g.entries {
		if now.After(e.lockedUntil) && now.Sub(e.lastSeen) > loginGuardEntryTTL {
			delete(g.entries, k)
		}
	}

	if len(g.entries) > loginGuardMaxEntries {
		// 遭遇海量随机 key（如伪造 IP 扫描）时释放内存，但保留仍在锁定中的条目。
		for k, e := range g.entries {
			if !now.Before(e.lockedUntil) {
				delete(g.entries, k)
			}
		}
	}
}

// loginGuardKeys 返回一次登录尝试涉及的计数 key（账号 + IP）
func loginGuardKeys(ip, username string) []string {
	return []string{accountKey(username), ipKey(ip)}
}

// accountKey 账号维度 key（用户名统一小写，避免大小写变体绕过）
func accountKey(username string) string {
	username = strings.ToLower(strings.TrimSpace(username))
	if username == "" {
		return ""
	}
	return "u:" + username
}

// ipKey IP 维度 key
func ipKey(ip string) string {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return ""
	}
	return "i:" + ip
}
