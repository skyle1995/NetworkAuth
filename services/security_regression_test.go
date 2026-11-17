package services

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"NetworkAuth/models"
	"NetworkAuth/utils"

	"gorm.io/gorm"
)

// TestSecurityPasswordRevocation 验证后台重置与正常改密均吊销所有账号令牌和应用激活。
func TestSecurityPasswordRevocation(t *testing.T) {
	for _, operation := range []string{"admin", "change"} {
		t.Run(operation, func(t *testing.T) {
			db := setupActivationDB(t)
			app := seedApp(t, db, "APP-RESET", nil)
			member := seedAccountMember(t, db, "security-reset", 10, 24*time.Hour)
			if err := ResetMemberPassword(member.ID, "old-password"); err != nil {
				t.Fatal(err)
			}
			tokens := make([]*models.MemberAccountToken, 0, 2)
			for _, fingerprint := range []string{"FP-A", "FP-B"} {
				token, err := IssueAccountToken(db, member.UUID, fingerprint, "127.0.0.1", "1", "test")
				if err != nil {
					t.Fatal(err)
				}
				tokens = append(tokens, token)
				if _, err := Activate(db, token, app, fingerprint, "127.0.0.1", "1", "test"); err != nil {
					t.Fatal(err)
				}
			}
			if operation == "admin" {
				if err := ResetMemberPassword(member.ID, "new-password"); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := ChangeMemberPassword(app.UUID, tokens[0].Token, "old-password", "new-password"); err != nil {
					t.Fatal(err)
				}
			}
			for _, token := range tokens {
				if _, err := AuthAccountToken(db, token.Token); err == nil {
					t.Fatal("旧账号令牌仍有效")
				}
				if _, err := RefreshAccountToken(db, token.RefreshToken); err == nil {
					t.Fatal("旧刷新令牌仍有效")
				}
			}
			if ActiveActivationCount(db, member.UUID, app.UUID) != 0 {
				t.Fatal("旧激活未全部失效")
			}
			if err := db.First(member, member.ID).Error; err != nil {
				t.Fatal(err)
			}
			if !utils.VerifyPasswordWithSalt("new-password", member.PasswordSalt, member.Password) {
				t.Fatal("新密码没有生效")
			}
		})
	}
}

// TestSecurityPasswordRollback 验证吊销失败时密码与已更新的激活一并回滚。
func TestSecurityPasswordRollback(t *testing.T) {
	db := setupActivationDB(t)
	app := seedApp(t, db, "APP-ROLLBACK", nil)
	member := seedAccountMember(t, db, "security-rollback", 10, 24*time.Hour)
	if err := ResetMemberPassword(member.ID, "old-password"); err != nil {
		t.Fatal(err)
	}
	token, err := IssueAccountToken(db, member.UUID, "FP", "127.0.0.1", "1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Activate(db, token, app, "FP", "127.0.0.1", "1", "test"); err != nil {
		t.Fatal(err)
	}
	injected := errors.New("test revocation failure")
	if err := db.Callback().Delete().Before("gorm:delete").Register("security:fail_token_delete", func(tx *gorm.DB) {
		if tx.Statement.Table == "member_account_tokens" {
			tx.AddError(injected)
		}
	}); err != nil {
		t.Fatal(err)
	}
	if err := ResetMemberPassword(member.ID, "new-password"); !errors.Is(err, injected) {
		t.Fatalf("未返回注入的吊销错误：%v", err)
	}
	if err := db.First(member, member.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !utils.VerifyPasswordWithSalt("old-password", member.PasswordSalt, member.Password) {
		t.Fatal("吊销失败却更新了密码")
	}
	if _, err := AuthAccountToken(db, token.Token); err != nil {
		t.Fatal("回滚后原账号令牌应保留")
	}
	if ActiveActivationCount(db, member.UUID, app.UUID) != 1 {
		t.Fatal("回滚后原激活应保持有效")
	}
}

// TestSecurityRefreshTokenConditions 验证过期、封停、删除账号与旧刷新令牌重用均被拒绝。
func TestSecurityRefreshTokenConditions(t *testing.T) {
	for _, condition := range []string{"expired", "disabled", "black", "deleted", "reused"} {
		t.Run(condition, func(t *testing.T) {
			db := setupActivationDB(t)
			member := seedAccountMember(t, db, "security-refresh", 10, 24*time.Hour)
			token, err := IssueAccountToken(db, member.UUID, "FP", "127.0.0.1", "1", "test")
			if err != nil {
				t.Fatal(err)
			}
			switch condition {
			case "expired":
				err = db.Model(token).Update("expires_at", time.Now().Add(-time.Second)).Error
			case "disabled":
				err = db.Model(member).Update("status", models.MemberStatusDisabled).Error
			case "black":
				err = db.Model(member).Update("status", models.MemberStatusBlack).Error
			case "deleted":
				err = db.Delete(member).Error
			case "reused":
				oldToken := token.Token
				newToken, refreshErr := RefreshAccountToken(db, token.RefreshToken)
				if refreshErr != nil {
					t.Fatal(refreshErr)
				}
				if newToken.Token == oldToken || newToken.RefreshToken == token.RefreshToken {
					t.Fatal("令牌未轮换")
				}
				if _, authErr := AuthAccountToken(db, oldToken); authErr == nil {
					t.Fatal("轮换后旧访问令牌仍有效")
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := RefreshAccountToken(db, token.RefreshToken); !errors.Is(err, ErrAccountTokenInvalid) {
				t.Fatalf("非法刷新没有被拒绝：%v", err)
			}
		})
	}
}

// TestSecurityRefreshTokenSingleConsumer 验证并发使用同一个刷新令牌只有一次成功。
func TestSecurityRefreshTokenSingleConsumer(t *testing.T) {
	db := setupActivationDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	// 内存 SQLite 共用一个连接，确保所有协程访问同一个测试库。
	sqlDB.SetMaxOpenConns(1)
	member := seedAccountMember(t, db, "security-refresh-concurrent", 10, 24*time.Hour)
	token, err := IssueAccountToken(db, member.UUID, "FP", "127.0.0.1", "1", "test")
	if err != nil {
		t.Fatal(err)
	}
	var success int32
	var workers sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < 8; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			if _, err := RefreshAccountToken(db, token.RefreshToken); err == nil {
				atomic.AddInt32(&success, 1)
			} else if !errors.Is(err, ErrAccountTokenInvalid) {
				t.Errorf("并发刷新返回非预期错误：%v", err)
			}
		}()
	}
	close(start)
	workers.Wait()
	if success != 1 {
		t.Fatalf("同一刷新令牌成功次数=%d，期望1", success)
	}
}

// TestSecurityPublicLoginGuard 验证公开登录失败跨 IP 累计，并阻止被锁定账号的正确密码登录。
func TestSecurityPublicLoginGuard(t *testing.T) {
	db := setupActivationDB(t)
	app := seedApp(t, db, "APP-GUARD", nil)
	member := seedAccountMember(t, db, "security-login", 10, 24*time.Hour)
	if err := ResetMemberPassword(member.ID, "correct-password"); err != nil {
		t.Fatal(err)
	}
	oldGuard := GetLoginGuard()
	loginGuardInst = newLoginGuard(3, 100, time.Minute, time.Minute, time.Minute)
	t.Cleanup(func() { loginGuardInst = oldGuard })
	for i := 0; i < 3; i++ {
		var err error
		if i == 1 {
			// 转绑与顶号使用同一凭据守卫，不应成为绕开登录锁定的入口。
			_, err = authMemberByCredential(db, member.Username, "wrong", fmt.Sprintf("192.0.2.%d", i+1))
		} else {
			_, err = AccountLogin(app.UUID, member.Username, "wrong", "FP",
				fmt.Sprintf("192.0.2.%d", i+1), "1", "test")
		}
		if err == nil {
			t.Fatal("错误密码被接受")
		}
	}
	_, err := AccountLogin(app.UUID, member.Username, "correct-password", "FP", "192.0.2.99", "1", "test")
	if err == nil || err.Error() != "登录尝试过多，请稍后重试" {
		t.Fatalf("被锁定账号未拦截：%v", err)
	}
	var count int64
	if err := db.Model(&models.MemberAccountToken{}).Where("member_uuid = ?", member.UUID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("登录失败或被锁定时不应颁发令牌")
	}
}

// TestSecurityLeaseDeadline 验证缺失期限与截止时刻均失效，未来期限正常有效。
func TestSecurityLeaseDeadline(t *testing.T) {
	now := time.Now()
	for _, deadline := range []time.Time{{}, now.Add(-time.Second), now} {
		act := models.AppActivation{LeaseExpiresAt: deadline}
		if !act.IsLeaseExpired(now) || act.IsAlive(now) {
			t.Fatalf("无效期限被放行：%v", deadline)
		}
	}
	act := models.AppActivation{LeaseExpiresAt: now.Add(time.Second)}
	if act.IsLeaseExpired(now) || !act.IsAlive(now) {
		t.Fatal("未来期限应有效")
	}
}

// TestSecurityLeaseRenewalGate 验证无效租约、异常账号和停用应用不能续期或扣费。
func TestSecurityLeaseRenewalGate(t *testing.T) {
	for _, condition := range []string{"zero", "expired", "released", "disabled", "black", "app-disabled", "valid"} {
		t.Run(condition, func(t *testing.T) {
			db := setupActivationDB(t)
			app := seedApp(t, db, "APP-LEASE", func(a *models.App) {
				a.ApplyBillingFlags(models.BillingModePoints)
				a.PointsChargeMode = models.PointsChargePerTime
				a.PointsPerPeriod = 2
				a.PointsPeriodMinutes = 10
			})
			member := seedAccountMember(t, db, "security-lease", 10, 24*time.Hour)
			acct, err := IssueAccountToken(db, member.UUID, "FP", "127.0.0.1", "1", "test")
			if err != nil {
				t.Fatal(err)
			}
			info, err := Activate(db, acct, app, "FP", "127.0.0.1", "1", "test")
			if err != nil {
				t.Fatal(err)
			}
			var act models.AppActivation
			if err := db.Where("token = ?", info.Token).First(&act).Error; err != nil {
				t.Fatal(err)
			}
			// 让下一次续期必须结算，确认被拒绝的请求不会扣点。
			if err := db.Model(&act).Update("period_end", time.Now().Add(-time.Minute)).Error; err != nil {
				t.Fatal(err)
			}
			switch condition {
			case "zero":
				err = db.Model(&act).Update("lease_expires_at", time.Time{}).Error
			case "expired":
				err = db.Model(&act).Update("lease_expires_at", time.Now().Add(-time.Second)).Error
			case "released":
				err = Deactivate(db, act.Token)
			case "disabled":
				err = db.Model(member).Update("status", models.MemberStatusDisabled).Error
			case "black":
				err = db.Model(member).Update("status", models.MemberStatusBlack).Error
			case "app-disabled":
				err = db.Model(app).Update("status", 0).Error
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := db.First(member, member.ID).Error; err != nil {
				t.Fatal(err)
			}
			points := member.Points
			if err := db.First(&act, act.ID).Error; err != nil {
				t.Fatal(err)
			}
			deadline := act.LeaseExpiresAt
			_, _, _, renewErr := RenewLease(db, act.Token)
			if condition == "valid" {
				if renewErr != nil {
					t.Fatalf("有效租约续期失败：%v", renewErr)
				}
			} else if renewErr == nil {
				t.Fatal("无效授权被续期")
			}
			if err := db.First(member, member.ID).Error; err != nil {
				t.Fatal(err)
			}
			if condition != "valid" {
				if member.Points != points {
					t.Fatal("拒绝续期却扣了点数")
				}
				if err := db.First(&act, act.ID).Error; err != nil {
					t.Fatal(err)
				}
				if !act.LeaseExpiresAt.Equal(deadline) {
					t.Fatal("拒绝续期却延长了期限")
				}
				if condition != "app-disabled" {
					if _, _, err := AuthActivation(db, act.Token); err == nil {
						t.Fatal("异常租约或账号仍通过令牌校验")
					}
				}
			} else if member.Points != points-2 {
				t.Fatalf("有效续期应只扣一次周期费用：before=%v after=%v", points, member.Points)
			}
		})
	}
}

// TestSecurityRenewalRechecksState 验证锁前快照已读取后发生撤权、轮换或封停时仍拒绝续期。
func TestSecurityRenewalRechecksState(t *testing.T) {
	for _, condition := range []string{"released", "rotated", "expired", "disabled", "app-disabled"} {
		t.Run(condition, func(t *testing.T) {
			db := setupActivationDB(t)
			app := seedApp(t, db, "APP-RECHECK", nil)
			member := seedAccountMember(t, db, "security-recheck", 10, 24*time.Hour)
			acct, err := IssueAccountToken(db, member.UUID, "FP", "127.0.0.1", "1", "test")
			if err != nil {
				t.Fatal(err)
			}
			info, err := Activate(db, acct, app, "FP", "127.0.0.1", "1", "test")
			if err != nil {
				t.Fatal(err)
			}
			injected := false
			if err := db.Callback().Update().Before("gorm:update").Register("security:renewal_state_change", func(tx *gorm.DB) {
				if injected || tx.Statement.Table != "members" {
					return
				}
				injected = true
				fresh := tx.Session(&gorm.Session{NewDB: true})
				var updateErr error
				switch condition {
				case "released":
					updateErr = fresh.Model(&models.AppActivation{}).Where("token = ?", info.Token).
						Update("deactivated_at", time.Now()).Error
				case "rotated":
					updateErr = fresh.Model(&models.AppActivation{}).Where("token = ?", info.Token).
						Update("token", "TEST-ROTATED-TOKEN").Error
				case "expired":
					updateErr = fresh.Model(&models.AppActivation{}).Where("token = ?", info.Token).
						Update("lease_expires_at", time.Now().Add(-time.Second)).Error
				case "disabled":
					updateErr = fresh.Model(&models.Member{}).Where("uuid = ?", member.UUID).
						Update("status", models.MemberStatusDisabled).Error
				case "app-disabled":
					updateErr = fresh.Model(&models.App{}).Where("uuid = ?", app.UUID).Update("status", 0).Error
				}
				tx.AddError(updateErr)
			}); err != nil {
				t.Fatal(err)
			}
			if _, _, _, err := RenewLease(db, info.Token); err == nil {
				t.Fatal("锁前快照中的旧授权被续期")
			}
			if !injected {
				t.Fatal("未触发状态变化测试")
			}
			var act models.AppActivation
			if err := db.Where("token = ?", info.Token).First(&act).Error; err != nil {
				t.Fatal(err)
			}
			if !act.LeaseExpiresAt.Equal(info.ExpiresAt) {
				t.Fatal("拒绝续期后原期限发生变化")
			}
		})
	}
}

// TestSecurityRenewalAudience 验证兼容心跳入口与公开心跳使用相同应用归属检查。
func TestSecurityRenewalAudience(t *testing.T) {
	db := setupActivationDB(t)
	app := seedApp(t, db, "APP-AUDIENCE", nil)
	member := seedAccountMember(t, db, "security-renew-audience", 10, 24*time.Hour)
	acct, err := IssueAccountToken(db, member.UUID, "FP", "127.0.0.1", "1", "test")
	if err != nil {
		t.Fatal(err)
	}
	info, err := Activate(db, acct, app, "FP", "127.0.0.1", "1", "test")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := renewLeaseOrImplicit(db, "APP-OTHER", info.Token); !errors.Is(err, ErrTokenAppMismatch) {
		t.Fatalf("跨应用续期未拒绝：%v", err)
	}
	if _, _, _, err := renewLeaseOrImplicit(db, app.UUID, acct.Token); !errors.Is(err, ErrActivationRequiredMarker) {
		t.Fatalf("账号令牌必须先激活：%v", err)
	}
	if _, _, _, err := renewLeaseOrImplicit(db, app.UUID, info.Token); err != nil {
		t.Fatalf("正确应用续期失败：%v", err)
	}
}
