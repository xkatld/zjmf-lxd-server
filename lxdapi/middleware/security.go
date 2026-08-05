package middleware

import "github.com/gin-gonic/gin"

// SecurityHeaders 添加安全响应头中间件
func SecurityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 防止页面被iframe嵌入（点击劫持防护）
		c.Header("X-Frame-Options", "DENY")
		
		// 防止MIME类型嗅探
		c.Header("X-Content-Type-Options", "nosniff")
		
		// 启用浏览器XSS过滤器
		c.Header("X-XSS-Protection", "1; mode=block")
		
		// 控制Referrer信息
		c.Header("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// 内容安全策略（基础配置）
		c.Header("Content-Security-Policy", "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval' https://cdn.tailwindcss.com https://cdn.jsdelivr.net https://code.jquery.com https://code.iconify.design; style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; img-src 'self' data: https:; font-src 'self' data: https://cdn.jsdelivr.net; connect-src 'self' https://cdn.jsdelivr.net https://code.iconify.design;")
		
		c.Next()
	}
}

