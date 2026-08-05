// @title LXD Web 管理平台
// @version 1.1.0
// @description LXD 容器管理 Web 平台，提供多节点管理、容器监控等功能
// @termsOfService https://github.com/xkatld/zjmf-lxd-server

// @contact.name xkatld
// @contact.url https://github.com/xkatld/zjmf-lxd-server
// @contact.email xkatld@example.com

// @license.name MIT
// @license.url https://github.com/xkatld/zjmf-lxd-server/blob/main/LICENSE

// @host localhost:3000
// @BasePath /

// @securityDefinitions.basic BasicAuth
// @in header
// @name Authorization
// @description 基于 Session 的认证

package main
import (
	"log"
	"lxdweb/config"
	"lxdweb/database"
	_ "lxdweb/docs"
	"lxdweb/handlers"
	"lxdweb/middleware"
	"lxdweb/pkg/logger"
	"lxdweb/services"
	"lxdweb/utils"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)
func main() {
	startWebServer()
}
func startWebServer() {
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("[ERROR] 配置加载失败: %v", err)
	}
	
	zapLogger, err := utils.InitLogger(
		config.AppConfig.Logging.File,
		config.AppConfig.Logging.MaxSize,
		config.AppConfig.Logging.MaxBackups,
		config.AppConfig.Logging.MaxAge,
		config.AppConfig.Logging.Compress,
		config.AppConfig.Logging.Level,
		config.AppConfig.Logging.DevMode,
	)
	if err != nil {
		log.Fatalf("[ERROR] 日志系统初始化失败: %v", err)
	}
	
	logger.Init(zapLogger)
	log.Printf("[LOGGER] 日志系统初始化完成: 级别=%s, 文件=%s", config.AppConfig.Logging.Level, config.AppConfig.Logging.File)
	
	database.InitDB()
	database.CheckAdminExists()

	go services.StartContainerSyncService()
	go services.StartAutoSyncService()
	go services.StartNodeCacheService()
	
	gin.SetMode(config.AppConfig.Server.Mode)
	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	store := cookie.NewStore([]byte(config.AppConfig.Server.SessionSecret))
	r.Use(sessions.Sessions("lxdweb_session", store))
	
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	
	r.GET("/", handlers.LoginPage)
	r.GET("/login", handlers.LoginPage)
	r.POST("/login", handlers.Login)
	r.GET("/logout", handlers.Logout)
	r.GET("/api/captcha", handlers.GetCaptcha)
	auth := r.Group("/")
	auth.Use(middleware.AuthRequired())
	{
		auth.GET("/dashboard", handlers.DashboardPage)
		auth.GET("/nodes", handlers.NodesPage)
		auth.GET("/nodes/:id", handlers.NodeDetailPage)
		auth.GET("/nodes/:id/containers", handlers.NodeContainersPage)
		auth.GET("/nodes/:id/containers/:name", handlers.ContainerDetailPage)
		auth.GET("/api/nodes", handlers.GetNodes)
		auth.GET("/api/nodes/:id", handlers.GetNode)
		auth.POST("/api/nodes", handlers.CreateNode)
		auth.PUT("/api/nodes/:id", handlers.UpdateNode)
		auth.DELETE("/api/nodes/:id", handlers.DeleteNode)
		auth.POST("/api/nodes/:id/test", handlers.TestNode)
		auth.POST("/api/nodes/:id/refresh", handlers.RefreshNodeCache)
		auth.GET("/api/nodes/export/all", handlers.ExportNodes)
		auth.POST("/api/nodes/import/batch", handlers.ImportNodes)
		auth.POST("/api/nodes/delete/batch", handlers.BatchDeleteNodes)
		
		// 容器API（保留API接口，但去掉全局页面入口）
		auth.GET("/api/containers", handlers.GetContainers)
		auth.GET("/api/containers/cache", handlers.GetContainersFromCache)
		auth.GET("/api/containers/:name", handlers.GetContainerDetail)
		auth.POST("/api/containers/:name/start", handlers.StartContainer)
		auth.POST("/api/containers/:name/stop", handlers.StopContainer)
		auth.POST("/api/containers/:name/restart", handlers.RestartContainer)
		auth.POST("/api/containers/:name/delete", handlers.DeleteContainer)
		auth.POST("/api/containers/:name/refresh", handlers.RefreshSingleContainer)
		auth.POST("/api/containers/:name/reinstall", handlers.ReinstallContainer)
		auth.POST("/api/containers/:name/password", handlers.ResetContainerPassword)
		auth.POST("/api/containers/:name/suspend", handlers.SuspendContainer)
		auth.POST("/api/containers/:name/unsuspend", handlers.UnsuspendContainer)
		auth.POST("/api/containers/:name/traffic/reset", handlers.ResetContainerTraffic)
		auth.POST("/api/containers/create", handlers.CreateContainer)
		
		auth.POST("/api/console/create-token", handlers.CreateConsoleToken)

		auth.POST("/api/sync/all", handlers.SyncAllNodes)
		auth.POST("/api/sync/node/:id", handlers.SyncNode)
		auth.GET("/api/sync/tasks", handlers.GetSyncTasks)
		auth.GET("/api/sync/status", handlers.GetSyncStatus)

		auth.GET("/api/auto-sync/status", handlers.GetAutoSyncStatus)
		auth.POST("/api/auto-sync/enable", handlers.EnableAutoSync)
		auth.POST("/api/auto-sync/disable", handlers.DisableAutoSync)
	}
	r.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path
		if path == "/favicon.ico" {
			c.Status(204)
			return
		}
		c.Redirect(302, "/login")
	})
	addr := config.AppConfig.Server.Address
	
	if config.AppConfig.Server.EnableHTTPS {
		certFile := config.AppConfig.Server.CertFile
		keyFile := config.AppConfig.Server.KeyFile
		
		if err := utils.GenerateSelfSignedCert(certFile, keyFile); err != nil {
			log.Fatalf("[ERROR] 证书生成失败: %v", err)
		}
		
		log.Printf("[SERVER] HTTPS 服务器启动: https://%s", addr)
		if err := r.RunTLS(addr, certFile, keyFile); err != nil {
			log.Fatalf("[ERROR] HTTPS 服务器启动失败: %v", err)
		}
	} else {
		log.Printf("[WARN] HTTP 服务器启动 (不安全): http://%s", addr)
		if err := r.Run(addr); err != nil {
			log.Fatalf("[ERROR] HTTP 服务器启动失败: %v", err)
		}
	}
}
