package config

import (
	"lxdapi/pkg/logger"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		
		logger.Global.Debug(ctx, "认证请求",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path))

		if c.Request.Method == "OPTIONS" {
			logger.Global.Debug(ctx, "跳过OPTIONS预检")
			c.Next()
			return
		}

		apiHash := getAPIHash(c)
		logger.Global.Debug(ctx, "请求详情",
			zap.String("hash", apiHash),
			zap.String("user_agent", c.GetHeader("User-Agent")))

		if apiHash == "" {
			logger.Global.Warn(ctx, "认证失败: 缺少API Hash")
			c.JSON(401, gin.H{"error": "缺少API Hash"})
			c.Abort()
			return
		}

		expectedHash := AppConfig.System.Security.APIHash
		logger.Global.Debug(ctx, "Hash验证",
			zap.String("received", apiHash),
			zap.String("expected", expectedHash),
			zap.Bool("match", apiHash == expectedHash))

		if apiHash != expectedHash {
			logger.Global.Warn(ctx, "认证失败: Hash不匹配")
			c.JSON(401, gin.H{"error": "无效的API Hash"})
			c.Abort()
			return
		}

		logger.Global.Debug(ctx, "认证成功")
		c.Next()
	}
}

func getAPIHash(c *gin.Context) string {
	if hash := c.GetHeader("apikey"); hash != "" {
		return hash
	}
	return ""
}

func ValidateAPIHash(c *gin.Context) bool {
	ctx := c.Request.Context()
	apiHash := getAPIHash(c)
	
	logger.Global.Debug(ctx, "Hash验证请求", zap.String("hash", apiHash))

	if apiHash == "" {
		logger.Global.Warn(ctx, "认证失败: 缺少API Hash")
		c.JSON(401, gin.H{"error": "缺少API Hash"})
		return false
	}

	expectedHash := AppConfig.System.Security.APIHash
	if apiHash != expectedHash {
		logger.Global.Warn(ctx, "认证失败: Hash不匹配")
		c.JSON(401, gin.H{"error": "无效的API Hash"})
		return false
	}

	logger.Global.Debug(ctx, "认证成功")
	return true
}
