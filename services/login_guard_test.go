package services

import (
	"testing"
	"time"
)

// 账号维度的连续失败达到阈值后应被锁定（即使来自不同 IP，可防御分布式爆破）
func TestLoginGuardLocksAccountAcrossIPs(t *testing.T) {
	g := newLoginGuard(3, 1000, time.Minute, 10*time.Minute, time.Minute)

	for i := 0; i < 3; i++ {
		ip := "10.0.0." + string(rune('1'+i))
		if allowed, _ := g.Allow(ip, "admin"); !allowed {
			t.Fatalf("第 %d 次尝试应被放行（阈值内）", i+1)
		}
		g.OnCredentialFailure(ip, "admin")
	}

	allowed, retryAfter := g.Allow("10.0.0.99", "admin")
	if allowed {
		t.Fatal("同一账号在多个 IP 连续失败达到阈值后应被锁定")
	}
	if retryAfter <= 0 {
		t.Fatalf("锁定后 retryAfter 应为正数，实际 %v", retryAfter)
	}
}

// 登录成功后应清除该账号与 IP 的失败计数
func TestLoginGuardSuccessResetsCounters(t *testing.T) {
	g := newLoginGuard(3, 1000, time.Minute, 10*time.Minute, time.Minute)

	g.OnCredentialFailure("1.1.1.1", "admin")
	g.OnCredentialFailure("1.1.1.1", "admin")
	g.OnSuccess("1.1.1.1", "admin")

	for i := 0; i < 2; i++ {
		g.OnCredentialFailure("1.1.1.1", "admin")
	}
	if allowed, _ := g.Allow("1.1.1.1", "admin"); !allowed {
		t.Fatal("成功登录后失败计数应已清零，不应立即触发锁定")
	}
}

// 验证码错误只锁定 IP，不允许把账号锁死（防止攻击者用错误验证码 DoS 管理员）
func TestLoginGuardCaptchaFailureOnlyAffectsIP(t *testing.T) {
	g := newLoginGuard(1000, 2, time.Minute, 10*time.Minute, 5*time.Minute)

	g.OnCaptchaFailure("1.2.3.4")
	g.OnCaptchaFailure("1.2.3.4")

	if allowed, _ := g.Allow("1.2.3.4", "admin"); allowed {
		t.Fatal("同一 IP 验证码连续失败达到阈值后应被临时锁定")
	}
	if allowed, _ := g.Allow("5.6.7.8", "admin"); !allowed {
		t.Fatal("验证码失败不应累加账号维度，其他 IP 仍应放行")
	}
}

// 重复触发锁定应指数退避，并且不超过上限
func TestLoginGuardLockEscalatesWithCap(t *testing.T) {
	g := newLoginGuard(1, 1000, time.Minute, 10*time.Minute, time.Minute)

	g.OnCredentialFailure("1.1.1.1", "admin")
	entry := g.entries[accountKey("admin")]
	if entry == nil || entry.lockLevel != 1 {
		t.Fatalf("首次锁定等级应为 1，实际 %+v", entry)
	}
	firstLock := entry.lockedUntil

	g.OnCredentialFailure("2.2.2.2", "admin")
	if entry.lockLevel != 2 {
		t.Fatalf("第二次锁定等级应为 2，实际 %d", entry.lockLevel)
	}
	if !entry.lockedUntil.After(firstLock) {
		t.Fatal("重复触发的锁定时长应递增")
	}

	if d := scaleLockDuration(time.Hour, 100); d > loginGuardLockCap {
		t.Fatalf("锁定时长不得超过上限 %v，实际 %v", loginGuardLockCap, d)
	}
}

// 用户名大小写/空白变体应共用同一账号计数，避免绕过
func TestLoginGuardNormalizesUsername(t *testing.T) {
	g := newLoginGuard(2, 1000, time.Minute, 10*time.Minute, time.Minute)

	g.OnCredentialFailure("1.1.1.1", " Admin ")
	g.OnCredentialFailure("2.2.2.2", "ADMIN")

	if allowed, _ := g.Allow("3.3.3.3", "admin"); allowed {
		t.Fatal("用户名大小写/空白变体应被归一化为同一账号并触发锁定")
	}
}
