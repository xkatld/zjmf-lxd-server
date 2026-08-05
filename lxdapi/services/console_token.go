package services

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"lxdapi/config"
	"lxdapi/database"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"lxdapi/utils"

	"go.uber.org/zap"
)

func GenerateConsoleToken(hostname, serverIP string, userID, serviceID, expiresIn int) (*models.CreateTokenResponse, error) {
	ctx := context.Background()
	lc := &logger.Context{
		Container: hostname,
		Action:    "generate_token",
	}
	ctx = logger.NewContext(ctx, lc)

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("生成令牌失败: %v", err)
	}
	token := hex.EncodeToString(tokenBytes)

	expiresAt := time.Now().Add(time.Duration(expiresIn) * time.Second)

	consoleToken := models.ConsoleToken{
		Token:     token,
		Hostname:  hostname,
		UserID:    userID,
		ServiceID: serviceID,
		ExpiresAt: expiresAt,
		Used:      false,
	}

	if err := database.DB.Create(&consoleToken).Error; err != nil {
		return nil, fmt.Errorf("保存令牌失败: %v", err)
	}

	logger.Global.Info(ctx, "生成控制台令牌",
		zap.String("token", token),
		zap.Int("user_id", userID),
		zap.Time("expires_at", expiresAt))

	serverHost := serverIP
	if serverHost == "" {
		serverHost = utils.GetServerIP()
		if serverHost == "" {
			serverHost = "localhost"
		}
	}
	serverPort := config.AppConfig.System.Server.Port
	if serverPort == 0 {
		serverPort = 8080
	}

	protocol := "wss"
	wsUrl := fmt.Sprintf("%s://%s:%d/api/console/ws-token?token=%s", protocol, serverHost, serverPort, token)

	return &models.CreateTokenResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		WSUrl:     wsUrl,
	}, nil
}

func ValidateConsoleToken(token string) (*models.ConsoleToken, error) {
	ctx := context.Background()
	lc := &logger.Context{
		Action: "validate_token",
	}
	ctx = logger.NewContext(ctx, lc)

	var consoleToken models.ConsoleToken

	err := database.DB.Where("token = ? AND used = ? AND expires_at > ?",
		token, false, time.Now()).First(&consoleToken).Error

	if err != nil {
		logger.Global.Warn(ctx, "令牌验证失败", zap.String("token", token), zap.Error(err))
		return nil, fmt.Errorf("令牌无效或已过期")
	}

	logger.Global.Info(ctx, "令牌验证成功",
		zap.String("token", token),
		zap.String("hostname", consoleToken.Hostname))
	return &consoleToken, nil
}

func MarkTokenAsUsed(token string) error {
	ctx := context.Background()
	lc := &logger.Context{
		Action: "mark_token_used",
	}
	ctx = logger.NewContext(ctx, lc)

	err := database.DB.Model(&models.ConsoleToken{}).
		Where("token = ?", token).
		Update("used", true).Error

	if err != nil {
		logger.Global.Error(ctx, "标记令牌已使用失败", zap.String("token", token), zap.Error(err))
		return err
	}

	logger.Global.Info(ctx, "令牌已标记为使用", zap.String("token", token))
	return nil
}

func CleanupExpiredTokens() {
	ctx := context.Background()
	lc := &logger.Context{
		Action: "cleanup_tokens",
	}
	ctx = logger.NewContext(ctx, lc)

	result := database.DB.Where("expires_at < ?", time.Now()).Delete(&models.ConsoleToken{})
	if result.Error != nil {
		logger.Global.Error(ctx, "清理过期令牌失败", zap.Error(result.Error))
	} else if result.RowsAffected > 0 {
		logger.Global.Info(ctx, "清理过期令牌", zap.Int64("count", result.RowsAffected))
	}
}

func StartTokenCleanupTask() {
	ctx := context.Background()
	lc := &logger.Context{
		Action: "token_cleanup_task",
	}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Info(ctx, "启动令牌清理任务")

	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()

		for range ticker.C {
			CleanupExpiredTokens()
		}
	}()
}
