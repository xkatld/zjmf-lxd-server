package middleware

import (
	"time"

	"lxdapi/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func Trace() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = uuid.New().String()
		}
		
		lc := &logger.Context{
			TraceID:  traceID,
			ClientIP: c.ClientIP(),
		}
		
		ctx := logger.NewContext(c.Request.Context(), lc)
		c.Request = c.Request.WithContext(ctx)
		
		c.Header("X-Trace-ID", traceID)
		
		logger.Global.Info(ctx, "请求开始",
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.String("query", c.Request.URL.RawQuery))
		
		c.Next()
		
		duration := time.Since(start)
		status := c.Writer.Status()
		
		logger.Global.Info(ctx, "请求完成",
			zap.Int("status", status),
			zap.Duration("duration", duration))
	}
}

