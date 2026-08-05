// @title 魔方财务-LXD对接API
// @version 1.1.0
// @description 魔方财务系统专用LXD容器管理后端API服务
// @termsOfService https://github.com/xkatld/zjmf-lxd-server

// @contact.name API支持
// @contact.url https://github.com/xkatld/zjmf-lxd-server
// @contact.email support@example.com

// @license.name MIT
// @license.url https://github.com/xkatld/zjmf-lxd-server/blob/main/LICENSE

// @host localhost:8080
// @BasePath /

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name apikey
// @description 在header中添加apikey字段，值为配置文件中的api_hash

package main

import (
	"context"
	"log"
	"strconv"
	"time"

	"lxdapi/config"
	"lxdapi/database"
	_ "lxdapi/docs"
	"lxdapi/executors"
	"lxdapi/handlers"
	"lxdapi/middleware"
	"lxdapi/pkg/logger"
	"lxdapi/services"
	"lxdapi/utils"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func main() {
	initApp()
	r := setupRouter()

	ctx := context.Background()
	go services.StartTrafficMonitoring(ctx)
	go services.StartConsoleCleanupTask()
	go services.StartTokenCleanupTask()
	go services.StartTaskCleanupService()
	go services.StartTrafficLimitChecker(ctx)
	go services.StartTrafficAutoResetChecker(ctx)
	go middleware.CleanupExpiredAttempts()

	if err := services.InitIptablesPersistent(); err != nil {
		log.Printf("⚠️ iptables持久化管理器初始化失败: %v", err)
	}

	if err := services.InitIptablesNAT(); err != nil {
		log.Printf("⚠️ NAT管理器初始化失败: %v", err)
	}

	if err := services.InitIPv6BindingManager(); err != nil {
		log.Printf("⚠️ IPv6绑定管理器初始化失败: %v", err)
	} else {
		executors.GlobalIPv6Manager = services.IPv6Manager
	}

	if err := services.InitSSLCertManager(); err != nil {
		log.Printf("⚠️ SSL证书管理器初始化失败: %v", err)
	}

	if err := services.InitNginxProxyManager(); err != nil {
		log.Printf("⚠️ Nginx反向代理管理器初始化失败: %v", err)
	}

	startServer(r)
}

func initApp() {
	if err := config.LoadConfig(); err != nil {
		log.Fatalf("⚠️ 配置加载失败: %v", err)
	}
	database.InitDB()
	gin.SetMode(config.AppConfig.System.Server.Mode)

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
		log.Fatalf("⚠️ 日志系统初始化失败: %v", err)
	}

	logger.Init(zapLogger)
	log.Printf("全局日志器初始化完成: 级别=%s", config.AppConfig.Logging.Level)

	backend := services.NewDatabaseQueue(database.DB)
	log.Printf("使用数据库队列后端")

	workerPool := services.NewWorkerPool(
		backend,
		database.DB,
		zapLogger,
		config.AppConfig.TaskQueue.WorkerCount,
		time.Duration(config.AppConfig.TaskQueue.PollInterval)*time.Millisecond,
	)
	services.SetGlobalWorkerPool(workerPool)
	workerPool.Start()

	log.Printf("Worker Pool启动完成: %d个worker", config.AppConfig.TaskQueue.WorkerCount)
}

func setupRouter() *gin.Engine {
	r := gin.Default()
	r.Use(middleware.Trace())
	r.Use(middleware.SecurityHeaders())
	setupCORS(r)

	setupRoutes(r)
	return r
}

func setupCORS(r *gin.Engine) {
	corsConfig := cors.Config{
		AllowOrigins: config.AppConfig.System.CORS.AllowOrigins,
		AllowMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
		AllowHeaders: []string{
			"Origin", "Content-Type", "Accept", "Authorization",
			"X-API-Hash", "X-Requested-With", "apikey", "User-Agent",
			"Cache-Control", "Pragma", "Expires",
		},
		ExposeHeaders:    []string{"Content-Length", "X-Total-Count"},
		AllowCredentials: config.AppConfig.System.CORS.AllowCredentials,
		MaxAge:           24 * time.Hour,
	}
	r.Use(cors.New(corsConfig))
}

func setupRoutes(r *gin.Engine) {
	r.GET("/", func(c *gin.Context) {
		systemInfo := utils.GetSystemInfo()
		c.JSON(200, gin.H{
			"description": "魔方财务系统专用LXD容器管理后端",
			"version":     "1.1.0",
			"docs":        "/swagger/index.html",
			"github":      "https://github.com/xkatld/zjmf-lxd-server",
			"system": gin.H{
				"os":           systemInfo.OS,
				"arch":         systemInfo.Arch,
				"kernel":       systemInfo.Kernel,
				"distribution": systemInfo.Distribution,
				"summary":      utils.GetSystemSummary(),
			},
			"lxd_version": systemInfo.LXDVersion,
		})
	})

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/console", handlers.ConsolePageHandler)
	r.GET("/api/console/ws-token", handlers.ConsoleWebSocketWithToken)
	r.GET("/metrics", handlers.MetricsHandler)

	apiGroup := r.Group("/api")
	apiGroup.Use(config.AuthMiddleware())
	{
		apiGroup.POST("/create", handlers.LXDServerCreateHandler)
		apiGroup.GET("/boot", handlers.LXDServerBootHandler)
		apiGroup.GET("/stop", handlers.LXDServerStopHandler)
		apiGroup.GET("/reboot", handlers.LXDServerRebootHandler)
		apiGroup.GET("/delete", handlers.LXDServerDeleteHandler)
		apiGroup.POST("/reinstall", handlers.LXDServerReinstallHandler)
		apiGroup.POST("/password", handlers.LXDServerResetPasswordHandler)

		apiGroup.GET("/traffic", handlers.LXDServerTrafficHandler)
		apiGroup.POST("/traffic/reset", handlers.LXDServerResetTrafficHandler)

		apiGroup.GET("/check", handlers.LXDServerCheckHandler)
		apiGroup.GET("/status", handlers.LXDServerStatusHandler)
		apiGroup.GET("/info", handlers.LXDServerInfoHandler)

		apiGroup.POST("/addport", handlers.LXDServerAddPortHandler)
		apiGroup.POST("/delport", handlers.LXDServerDelPortHandler)
		apiGroup.GET("/natlist", handlers.LXDServerNATListHandler)
		apiGroup.GET("/nat/check", handlers.LXDServerCheckPortHandler)

		apiGroup.POST("/console/create-token", handlers.CreateConsoleToken)

		apiGroup.GET("/suspend", handlers.LXDServerSuspendHandler)
		apiGroup.GET("/unsuspend", handlers.LXDServerUnsuspendHandler)

		apiGroup.POST("/ipv6/add", handlers.LXDServerAddIPv6Handler)
		apiGroup.POST("/ipv6/delete", handlers.LXDServerDeleteIPv6Handler)
		apiGroup.GET("/ipv6/list", handlers.LXDServerIPv6ListHandler)
		apiGroup.GET("/ipv6/status", handlers.LXDServerIPv6StatusHandler)

		apiGroup.POST("/proxy/add", handlers.AddProxyHandler)
		apiGroup.POST("/proxy/delete", handlers.DeleteProxyHandler)
		apiGroup.GET("/proxy/list", handlers.ListProxyHandler)
		apiGroup.GET("/proxy/check", handlers.CheckProxyDomainHandler)
		apiGroup.POST("/proxy/regenerate", handlers.RegenerateProxyHandler)

		apiGroup.GET("/cache/containers", handlers.GetContainersCacheHandler)
		apiGroup.POST("/cache/containers/refresh", handlers.RefreshContainersCacheHandler)

		apiGroup.GET("/task/list", handlers.GetTaskListHandler)
	}
}

func startBackgroundServices() {
	ctx := context.Background()
	go services.StartTrafficMonitoring(ctx)
	go services.StartConsoleCleanupTask()
	go services.StartTokenCleanupTask()
	go services.StartTaskCleanupService()
	go services.StartTrafficLimitChecker(ctx)
	go services.StartTrafficAutoResetChecker(ctx)
}

func startServer(r *gin.Engine) {
	port := config.AppConfig.System.Server.Port
	addr := "0.0.0.0:" + strconv.Itoa(port)

	startHTTPSServer(r, port, addr)
}

func startHTTPSServer(r *gin.Engine, port int, addr string) {
	certFile := config.AppConfig.System.Server.TLS.CertFile
	keyFile := config.AppConfig.System.Server.TLS.KeyFile

	if config.AppConfig.System.Server.TLS.AutoGen {
		setupTLSCertificate(certFile, keyFile)
	}

	log.Printf("服务启动: 端口%d (HTTPS/WSS)", port)
	log.Printf("TLS证书: %s", certFile)
	log.Printf("TLS私钥: %s", keyFile)

	if err := r.RunTLS(addr, certFile, keyFile); err != nil {
		log.Fatalf("⚠️ HTTPS服务器启动失败: %v", err)
	}
}

func setupTLSCertificate(certFile, keyFile string) {
	if !utils.CertExists(certFile, keyFile) || !utils.ValidateCert(certFile) {
		log.Printf("生成自签证书...")

		var hosts []string
		if len(config.AppConfig.System.Server.TLS.ServerIPs) > 0 {
			hosts = config.AppConfig.System.Server.TLS.ServerIPs
			log.Printf("使用配置的IP列表: %v", hosts)
		} else {
			if serverIP := utils.GetServerIP(); serverIP != "" {
				hosts = append(hosts, serverIP)
				log.Printf("自动检测到IP: %s", serverIP)
			}
		}

		if err := utils.GenerateSelfSignedCert(certFile, keyFile, hosts); err != nil {
			log.Fatalf("⚠️ 生成自签证书失败: %v", err)
		}

		log.Printf("自签证书生成成功: %s, %s", certFile, keyFile)
		log.Printf("证书包含的主机: %v", hosts)
	} else {
		log.Printf("使用现有证书: %s", certFile)
	}
}
