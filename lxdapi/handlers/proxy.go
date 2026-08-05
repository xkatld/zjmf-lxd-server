package handlers

import (
	"lxdapi/database"
	"lxdapi/errors"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"lxdapi/services"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// AddProxyHandler 添加反向代理规则
// @Summary 添加反向代理
// @Description 为容器添加Nginx反向代理规则
// @Tags 反向代理
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param hostname formData string true "容器名称"
// @Param domain formData string true "域名"
// @Param container_port formData int false "容器端口" default(80)
// @Param description formData string false "描述"
// @Success 200 {object} models.ProxyResponse
// @Failure 400 {object} models.ProxyResponse
// @Failure 500 {object} models.ProxyResponse
// @Security ApiKeyAuth
// @Router /api/proxy/add [post]
func AddProxyHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)

	logger.Global.Info(ctx, "添加反向代理请求")

	var req models.AddProxyRequest
	if err := c.ShouldBind(&req); err != nil {
		logger.Global.Error(ctx, "参数解析失败", zap.Error(err))
		c.JSON(400, models.ProxyResponse{
			Code: 400,
			Msg:  "请求参数解析失败: " + err.Error(),
		})
		return
	}

	lc.Container = req.Hostname
	lc.Action = "add_proxy"

	if !services.ValidateDomain(req.Domain) {
		logger.Global.Warn(ctx, "域名格式无效", zap.String("domain", req.Domain))
		c.JSON(400, models.ProxyResponse{
			Code: errors.ERR_PROXY_DOMAIN_INVALID,
			Msg:  "域名格式无效",
		})
		return
	}

	var existingRule models.ProxyRule
	if err := database.DB.Where("domain = ?", req.Domain).First(&existingRule).Error; err == nil {
		logger.Global.Warn(ctx, "域名已被使用", zap.String("domain", req.Domain))
		c.JSON(400, models.ProxyResponse{
			Code: errors.ERR_PROXY_RULE_EXISTS,
			Msg:  "域名已被使用",
		})
		return
	}

	if req.ContainerPort == 0 {
		req.ContainerPort = 80
	}

	if !services.ContainerExists(req.Hostname) {
		logger.Global.Warn(ctx, "容器不存在")
		c.JSON(404, models.ProxyResponse{
			Code: errors.ERR_CONTAINER_NOT_FOUND,
			Msg:  "容器不存在: " + req.Hostname,
		})
		return
	}

	rule := models.ProxyRule{
		ContainerName: req.Hostname,
		Domain:        req.Domain,
		ContainerPort: req.ContainerPort,
		Status:        "active",
		Description:   req.Description,
		SSLEnabled:    req.SSLEnabled,
		SSLType:       "none",
	}

	if req.SSLEnabled {
		if req.SSLType == "" {
			req.SSLType = "self-signed"
		}
		
		rule.SSLType = req.SSLType

		var certPath, keyPath string
		var err error

		if req.SSLType == "self-signed" {
			logger.Global.Info(ctx, "生成自签名证书", zap.String("domain", req.Domain))
			certPath, keyPath, err = services.CertManager.GenerateSelfSignedCert(req.Domain)
			if err != nil {
				logger.Global.Error(ctx, "生成自签名证书失败", zap.Error(err))
				c.JSON(500, models.ProxyResponse{
					Code: 500,
					Msg:  "生成自签名证书失败: " + err.Error(),
				})
				return
			}
		} else if req.SSLType == "custom" {
			if req.SSLCert == "" || req.SSLKey == "" {
				logger.Global.Warn(ctx, "自定义证书内容为空")
				c.JSON(400, models.ProxyResponse{
					Code: 400,
					Msg:  "启用自定义证书时，必须提供证书和私钥内容",
				})
				return
			}
			
			logger.Global.Info(ctx, "保存自定义证书", zap.String("domain", req.Domain))
			certPath, keyPath, err = services.CertManager.SaveCustomCert(req.Domain, req.SSLCert, req.SSLKey)
			if err != nil {
				logger.Global.Error(ctx, "保存自定义证书失败", zap.Error(err))
				c.JSON(500, models.ProxyResponse{
					Code: 500,
					Msg:  "保存自定义证书失败: " + err.Error(),
				})
				return
			}
		} else {
			c.JSON(400, models.ProxyResponse{
				Code: 400,
				Msg:  "不支持的SSL类型: " + req.SSLType,
			})
			return
		}

		rule.SSLCertPath = certPath
		rule.SSLKeyPath = keyPath
		logger.Global.Info(ctx, "SSL证书准备完成", zap.String("cert", certPath), zap.String("key", keyPath))
	}

	if err := services.ProxyManager.AddProxyRule(&rule); err != nil {
		logger.Global.Error(ctx, "添加代理规则失败", zap.Error(err))
		if req.SSLEnabled && services.CertManager != nil {
			services.CertManager.DeleteCert(req.Domain)
		}
		c.JSON(500, models.ProxyResponse{
			Code: errors.ERR_PROXY_CONFIG_GENERATE,
			Msg:  "添加反向代理失败: " + err.Error(),
		})
		return
	}

	if err := database.DB.Create(&rule).Error; err != nil {
		logger.Global.Error(ctx, "保存代理规则失败", zap.Error(err))
		services.ProxyManager.DeleteProxyRule(&rule)
		c.JSON(500, models.ProxyResponse{
			Code: errors.ERR_DB_INSERT_FAIL,
			Msg:  "保存规则失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "添加反向代理成功",
		zap.String("domain", rule.Domain),
		zap.Int("port", rule.ContainerPort))

	c.JSON(200, models.ProxyResponse{
		Code: 200,
		Msg:  "反向代理添加成功",
		Data: rule,
	})
}

// DeleteProxyHandler 删除反向代理规则
// @Summary 删除反向代理
// @Description 删除容器的反向代理规则
// @Tags 反向代理
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param hostname formData string true "容器名称"
// @Param domain formData string true "域名"
// @Success 200 {object} models.ProxyResponse
// @Failure 400 {object} models.ProxyResponse
// @Failure 404 {object} models.ProxyResponse
// @Failure 500 {object} models.ProxyResponse
// @Security ApiKeyAuth
// @Router /api/proxy/delete [post]
func DeleteProxyHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)

	logger.Global.Info(ctx, "删除反向代理请求")

	var req models.DeleteProxyRequest
	if err := c.ShouldBind(&req); err != nil {
		logger.Global.Error(ctx, "参数解析失败", zap.Error(err))
		c.JSON(400, models.ProxyResponse{
			Code: 400,
			Msg:  "请求参数解析失败: " + err.Error(),
		})
		return
	}

	lc.Container = req.Hostname
	lc.Action = "delete_proxy"

	var rule models.ProxyRule
	if err := database.DB.Where("container_name = ? AND domain = ?", req.Hostname, req.Domain).First(&rule).Error; err != nil {
		logger.Global.Warn(ctx, "代理规则不存在", zap.String("domain", req.Domain))
		c.JSON(404, models.ProxyResponse{
			Code: errors.ERR_PROXY_RULE_NOT_FOUND,
			Msg:  "代理规则不存在",
		})
		return
	}

	if err := services.ProxyManager.DeleteProxyRule(&rule); err != nil {
		logger.Global.Error(ctx, "删除代理规则失败", zap.Error(err))
		c.JSON(500, models.ProxyResponse{
			Code: errors.ERR_PROXY_FILE_WRITE,
			Msg:  "删除反向代理失败: " + err.Error(),
		})
		return
	}

	if err := database.DB.Delete(&rule).Error; err != nil {
		logger.Global.Error(ctx, "删除数据库记录失败", zap.Error(err))
		c.JSON(500, models.ProxyResponse{
			Code: errors.ERR_DB_DELETE_FAIL,
			Msg:  "删除规则失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "删除反向代理成功", zap.String("domain", rule.Domain))

	c.JSON(200, models.ProxyResponse{
		Code: 200,
		Msg:  "反向代理删除成功",
	})
}

// ListProxyHandler 获取容器的反向代理列表
// @Summary 获取容器代理列表
// @Description 获取指定容器的所有反向代理规则
// @Tags 反向代理
// @Produce json
// @Param hostname query string true "容器名"
// @Success 200 {object} models.ProxyListResponse
// @Failure 400 {object} models.ProxyResponse
// @Security ApiKeyAuth
// @Router /api/proxy/list [get]
func ListProxyHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)
	hostname := c.Query("hostname")

	lc.Container = hostname

	if hostname == "" {
		c.JSON(400, models.ProxyResponse{
			Code: 400,
			Msg:  "缺少hostname参数",
		})
		return
	}

	logger.Global.Info(ctx, "查询容器代理列表")

	var rules []models.ProxyRule
	if err := database.DB.Where("container_name = ?", hostname).Find(&rules).Error; err != nil {
		logger.Global.Error(ctx, "查询失败", zap.Error(err))
		c.JSON(500, models.ProxyResponse{
			Code: errors.ERR_DB_QUERY_FAIL,
			Msg:  "查询失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "查询成功", zap.Int("count", len(rules)))

	c.JSON(200, models.ProxyListResponse{
		Code: 200,
		Msg:  "success",
		Data: rules,
	})
}

// AllProxyHandler 获取所有反向代理规则
// @Summary 获取所有代理规则
// @Description 获取系统中的所有反向代理规则
// @Tags 反向代理
// @Produce json
// @Success 200 {object} models.ProxyListResponse
// @Failure 500 {object} models.ProxyResponse
// @Security ApiKeyAuth
// @Router /api/proxy/all [get]
func AllProxyHandler(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Global.Info(ctx, "查询所有代理规则")

	var rules []models.ProxyRule
	if err := database.DB.Order("created_at desc").Find(&rules).Error; err != nil {
		logger.Global.Error(ctx, "查询失败", zap.Error(err))
		c.JSON(500, models.ProxyResponse{
			Code: errors.ERR_DB_QUERY_FAIL,
			Msg:  "查询失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "查询成功", zap.Int("count", len(rules)))

	c.JSON(200, models.ProxyListResponse{
		Code: 200,
		Msg:  "success",
		Data: rules,
	})
}

// CheckProxyDomainHandler 检查域名是否可用
// @Summary 检查域名可用性
// @Description 检查域名是否已被使用
// @Tags 反向代理
// @Produce json
// @Param domain query string true "域名"
// @Success 200 {object} models.ProxyCheckResponse
// @Failure 400 {object} models.ProxyResponse
// @Security ApiKeyAuth
// @Router /api/proxy/check [get]
func CheckProxyDomainHandler(c *gin.Context) {
	ctx := c.Request.Context()
	domain := c.Query("domain")

	if domain == "" {
		c.JSON(400, models.ProxyResponse{
			Code: 400,
			Msg:  "缺少domain参数",
		})
		return
	}

	logger.Global.Info(ctx, "检查域名可用性", zap.String("domain", domain))

	if !services.ValidateDomain(domain) {
		c.JSON(200, models.ProxyCheckResponse{
			Code: 200,
			Msg:  "success",
			Data: models.ProxyCheckData{
				Available: false,
				Reason:    "域名格式无效",
			},
		})
		return
	}

	var existingRule models.ProxyRule
	if err := database.DB.Where("domain = ?", domain).First(&existingRule).Error; err == nil {
		c.JSON(200, models.ProxyCheckResponse{
			Code: 200,
			Msg:  "success",
			Data: models.ProxyCheckData{
				Available: false,
				Reason:    "域名已被使用",
			},
		})
		return
	}

	c.JSON(200, models.ProxyCheckResponse{
		Code: 200,
		Msg:  "success",
		Data: models.ProxyCheckData{
			Available: true,
		},
	})
}

// RegenerateProxyHandler 重新生成所有配置文件
// @Summary 重新生成配置
// @Description 重新生成所有反向代理配置文件（启动时恢复功能）
// @Tags 反向代理
// @Produce json
// @Success 200 {object} models.ProxyResponse
// @Failure 500 {object} models.ProxyResponse
// @Security ApiKeyAuth
// @Router /api/proxy/regenerate [post]
func RegenerateProxyHandler(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Global.Info(ctx, "重新生成配置请求")

	if err := services.ProxyManager.RestoreAllProxyRules(); err != nil {
		logger.Global.Error(ctx, "重新生成配置失败", zap.Error(err))
		c.JSON(500, models.ProxyResponse{
			Code: 500,
			Msg:  "重新生成配置失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "重新生成配置成功")

	c.JSON(200, models.ProxyResponse{
		Code: 200,
		Msg:  "配置重新生成成功，请手动reload nginx",
	})
}
