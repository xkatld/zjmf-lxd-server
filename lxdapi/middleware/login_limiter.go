package middleware

import (
	"sync"
	"time"
)

type loginAttempt struct {
	count      int
	lastAttempt time.Time
	lockedUntil time.Time
}

var (
	loginAttempts = make(map[string]*loginAttempt)
	attemptsMutex sync.RWMutex
	
	maxAttempts   = 5                // 最大尝试次数
	lockDuration  = 15 * time.Minute // 锁定时长
	resetDuration = 5 * time.Minute  // 失败记录重置时长
)

// CheckLoginAttempt 检查是否允许登录尝试
func CheckLoginAttempt(ip string) (allowed bool, remainingAttempts int, lockedUntil time.Time) {
	attemptsMutex.Lock()
	defer attemptsMutex.Unlock()
	
	attempt, exists := loginAttempts[ip]
	now := time.Now()
	
	if !exists {
		loginAttempts[ip] = &loginAttempt{
			count:       0,
			lastAttempt: now,
		}
		return true, maxAttempts, time.Time{}
	}
	
	// 检查是否在锁定期
	if !attempt.lockedUntil.IsZero() && now.Before(attempt.lockedUntil) {
		return false, 0, attempt.lockedUntil
	}
	
	// 如果锁定期已过，重置
	if !attempt.lockedUntil.IsZero() && now.After(attempt.lockedUntil) {
		attempt.count = 0
		attempt.lockedUntil = time.Time{}
	}
	
	// 如果距离上次尝试超过重置时长，重置计数
	if now.Sub(attempt.lastAttempt) > resetDuration {
		attempt.count = 0
	}
	
	remaining := maxAttempts - attempt.count
	if remaining <= 0 {
		return false, 0, attempt.lockedUntil
	}
	
	return true, remaining, time.Time{}
}

// RecordLoginFailure 记录登录失败
func RecordLoginFailure(ip string) {
	attemptsMutex.Lock()
	defer attemptsMutex.Unlock()
	
	attempt, exists := loginAttempts[ip]
	now := time.Now()
	
	if !exists {
		loginAttempts[ip] = &loginAttempt{
			count:       1,
			lastAttempt: now,
		}
		return
	}
	
	attempt.count++
	attempt.lastAttempt = now
	
	// 如果达到最大次数，锁定
	if attempt.count >= maxAttempts {
		attempt.lockedUntil = now.Add(lockDuration)
	}
}

// ResetLoginAttempts 重置登录尝试（成功登录后）
func ResetLoginAttempts(ip string) {
	attemptsMutex.Lock()
	defer attemptsMutex.Unlock()
	
	delete(loginAttempts, ip)
}

// CleanupExpiredAttempts 定期清理过期记录
func CleanupExpiredAttempts() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	
	for range ticker.C {
		attemptsMutex.Lock()
		now := time.Now()
		for ip, attempt := range loginAttempts {
			// 清理超过1小时的记录
			if now.Sub(attempt.lastAttempt) > time.Hour {
				delete(loginAttempts, ip)
			}
		}
		attemptsMutex.Unlock()
	}
}

