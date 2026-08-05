package handlers

import (
	"net/http"
	"strconv"

	"lxdapi/config"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"lxdapi/services"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// LXDServerAddPortHandler 添加用户 NAT 规则
// @Summary 添加NAT端口转发
// @Description 为容器创建外部端口到内部端口的映射
// @Tags NAT管理
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param hostname formData string true "容器名称"
// @Param dport formData int false "外网端口" minimum(10000) maximum(65535)
// @Param sport formData int true "内网端口" minimum(1) maximum(65535)
// @Param dtype formData string true "协议" Enums(tcp,udp)
// @Param description formData string false "描述"
// @Success 200 {object} models.NATOperationResponse
// @Failure 401 {object} map[string]string "认证失败"
// @Security ApiKeyAuth
// @Router /api/addport [post]
func LXDServerAddPortHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)

	logger.Global.Info(ctx, "添加NAT规则请求")

	// 检查NAT功能是否启用
	ipv4NATEnabled := config.IsNATEnabled()
	
	if !ipv4NATEnabled {
		logger.Global.Warn(ctx, "NAT功能已禁用")
		c.JSON(http.StatusServiceUnavailable, models.NATOperationResponse{
			Code: 503,
			Msg:  "NAT功能已禁用，请联系管理员启用此功能",
		})
		return
	}

	var req models.AddNATRequest
	if err := c.ShouldBind(&req); err != nil {
		logger.Global.Error(ctx, "参数解析失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.NATOperationResponse{
			Code: 400,
			Msg:  "参数错误: " + err.Error(),
		})
		return
	}

	lc.Container = req.Hostname
	lc.Action = "add_nat"

	// 设置默认协议为both（TCP+UDP）
	if req.Protocol == "" {
		req.Protocol = "both"
	}

	// 验证端口范围参数
	isPortRange := req.ExternalPortEnd > 0 || req.InternalPortEnd > 0
	if isPortRange {
		// 端口段模式：必须同时指定起止端口
		if req.ExternalPort == 0 || req.InternalPort == 0 || req.ExternalPortEnd == 0 || req.InternalPortEnd == 0 {
			c.JSON(http.StatusBadRequest, models.NATOperationResponse{
				Code: 400,
				Msg:  "端口段模式需要同时指定起始和结束端口",
			})
			return
		}
		
		// 验证端口范围有效性
		if req.ExternalPort > req.ExternalPortEnd {
			c.JSON(http.StatusBadRequest, models.NATOperationResponse{
				Code: 400,
				Msg:  "外部端口起始值不能大于结束值",
			})
			return
		}
		
		if req.InternalPort > req.InternalPortEnd {
			c.JSON(http.StatusBadRequest, models.NATOperationResponse{
				Code: 400,
				Msg:  "内部端口起始值不能大于结束值",
			})
			return
		}
		
		// 验证端口范围大小一致
		externalRange := req.ExternalPortEnd - req.ExternalPort + 1
		internalRange := req.InternalPortEnd - req.InternalPort + 1
		if externalRange != internalRange {
			c.JSON(http.StatusBadRequest, models.NATOperationResponse{
				Code: 400,
				Msg:  "外部和内部端口范围大小必须一致",
			})
			return
		}
		
		logger.Global.Info(ctx, "添加NAT端口段规则",
			zap.Int("external_port_start", req.ExternalPort),
			zap.Int("external_port_end", req.ExternalPortEnd),
			zap.Int("internal_port_start", req.InternalPort),
			zap.Int("internal_port_end", req.InternalPortEnd),
			zap.Int("range_size", externalRange),
			zap.String("protocol", req.Protocol))
	} else {
		logger.Global.Info(ctx, "添加NAT单端口规则",
			zap.Int("external_port", req.ExternalPort),
			zap.Int("internal_port", req.InternalPort),
			zap.String("protocol", req.Protocol))
	}

	if err := services.AddNATRuleForContainerV2(req); err != nil {
		logger.Global.Error(ctx, "添加NAT规则失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.NATOperationResponse{
			Code: 500,
			Msg:  "添加NAT规则失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "NAT规则添加成功", zap.String("protocol", req.Protocol))

	c.JSON(http.StatusOK, models.NATOperationResponse{
		Code: 200,
		Msg:  "NAT规则添加成功",
	})
}

// LXDServerCheckPortHandler 检查端口可用性
// @Summary 检查外网端口是否可用
// @Description 校验指定端口是否被占用
// @Tags NAT管理
// @Produce json
// @Param hostname query string false "容器名称"
// @Param port query int true "外网端口" minimum(10000) maximum(65535)
// @Param protocol query string false "协议" Enums(tcp,udp)
// @Success 200 {object} models.NATPortCheckResponse
// @Failure 401 {object} map[string]string "认证失败"
// @Security ApiKeyAuth
// @Router /api/nat/check [get]
func LXDServerCheckPortHandler(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Global.Info(ctx, "检查NAT端口可用性请求")

	portStr := c.Query("port")
	protocol := c.Query("protocol")
	container := c.Query("hostname")

	if portStr == "" {
		c.JSON(http.StatusBadRequest, models.NATPortCheckResponse{
			Code: 400,
			Msg:  "缺少port参数",
			Data: models.NATPortCheckData{Available: false, Reason: "缺少port参数"},
		})
		return
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, models.NATPortCheckResponse{
			Code: 400,
			Msg:  "port参数必须为整数",
			Data: models.NATPortCheckData{Available: false, Reason: "端口必须为整数"},
		})
		return
	}

	available, reason := services.CheckExternalPortAvailability(port, protocol, container)
	
	logger.Global.Info(ctx, "端口可用性检查结果",
		zap.Int("port", port),
		zap.String("protocol", protocol),
		zap.Bool("available", available))

	if available {
		c.JSON(http.StatusOK, models.NATPortCheckResponse{
			Code: 200,
			Msg:  "端口可用",
			Data: models.NATPortCheckData{Available: true},
		})
		return
	}

	message := reason
	if message == "" {
		message = "端口不可用"
	}

	statusCode := http.StatusOK
	if port < 10000 || port > 65535 {
		statusCode = http.StatusBadRequest
	}

	c.JSON(statusCode, models.NATPortCheckResponse{
		Code: 200,
		Msg:  message,
		Data: models.NATPortCheckData{Available: false, Reason: reason},
	})
}

// LXDServerAllNATListHandler 获取所有NAT规则列表（不限容器）
// @Summary 获取所有NAT规则列表
// @Description 列出所有NAT端口转发规则
// @Tags NAT管理
// @Produce json
// @Success 200 {object} models.NATListResponse
// @Router /api/nat/list [get]
func LXDServerAllNATListHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)

	logger.Global.Info(ctx, "查询所有NAT规则列表请求")

	rules, err := services.ListAllNATRules()
	if err != nil {
		logger.Global.Error(ctx, "查询所有NAT规则列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.NATListResponse{
			Code:    500,
			Msg:     "查询所有NAT规则失败: " + err.Error(),
			TraceID: lc.TraceID,
			Data:    []models.NATRule{},
		})
		return
	}

	logger.Global.Info(ctx, "查询所有NAT规则列表成功", zap.Int("count", len(rules)))

	c.JSON(http.StatusOK, models.NATListResponse{
		Code:    200,
		Msg:     "获取所有NAT规则列表成功",
		TraceID: lc.TraceID,
		Data:    rules,
	})
}

// LXDServerNATListHandler 获取NAT规则列表（单个容器）
// @Summary 获取NAT规则列表
// @Description 列出容器所有NAT端口转发
// @Tags NAT管理
// @Produce json
// @Param hostname query string true "容器名称"
// @Success 200 {object} models.NATListResponse
// @Router /api/natlist [get]
func LXDServerNATListHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)
	hostname := c.Query("hostname")

	lc.Container = hostname

	logger.Global.Info(ctx, "查询NAT规则列表请求")

	if hostname == "" {
		logger.Global.Warn(ctx, "缺少hostname参数")
		c.JSON(http.StatusBadRequest, models.NATListResponse{
			Code:    400,
			Msg:     "缺少hostname参数",
			TraceID: lc.TraceID,
			Data:    []models.NATRule{},
		})
		return
	}

	rules, err := services.ListNATRules(hostname)
	if err != nil {
		logger.Global.Error(ctx, "查询NAT规则列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.NATListResponse{
			Code:    500,
			Msg:     "查询NAT规则失败: " + err.Error(),
			TraceID: lc.TraceID,
			Data:    []models.NATRule{},
		})
		return
	}

	logger.Global.Info(ctx, "查询NAT规则列表成功", zap.Int("count", len(rules)))

	c.JSON(http.StatusOK, models.NATListResponse{
		Code:    200,
		Msg:     "获取NAT规则列表成功",
		TraceID: lc.TraceID,
		Data:    rules,
	})
}

// LXDServerDelPortHandler 删除NAT规则
// @Summary 删除NAT端口转发
// @Description 删除容器的NAT端口映射
// @Tags NAT管理
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param hostname formData string true "容器名称"
// @Param dport formData int true "外网端口" minimum(10000) maximum(65535)
// @Param sport formData int true "内网端口" minimum(1) maximum(65535)
// @Param dtype formData string true "协议" Enums(tcp,udp)
// @Success 200 {object} models.NATOperationResponse
// @Router /api/delport [post]
func LXDServerDelPortHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)

	logger.Global.Info(ctx, "删除NAT规则请求")

	var req models.DeleteNATRequest
	if err := c.ShouldBind(&req); err != nil {
		logger.Global.Error(ctx, "参数解析失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.NATOperationResponse{
			Code: 400,
			Msg:  "参数错误: " + err.Error(),
		})
		return
	}

	lc.Container = req.Hostname
	lc.Action = "delete_nat"

	isPortRange := req.ExternalPortEnd > 0 && req.InternalPortEnd > 0
	
	if isPortRange {
		logger.Global.Info(ctx, "删除NAT端口段规则参数",
			zap.Int("external_port_start", req.ExternalPort),
			zap.Int("external_port_end", req.ExternalPortEnd),
			zap.Int("internal_port_start", req.InternalPort),
			zap.Int("internal_port_end", req.InternalPortEnd),
			zap.String("protocol", req.Protocol))

		if err := services.DeleteNATRuleRange(req.Hostname, req.ExternalPort, req.ExternalPortEnd, req.InternalPort, req.InternalPortEnd, req.Protocol); err != nil {
			logger.Global.Error(ctx, "删除NAT端口段规则失败", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.NATOperationResponse{
				Code: 500,
				Msg:  "删除NAT端口段规则失败: " + err.Error(),
			})
			return
		}

		logger.Global.Info(ctx, "删除NAT端口段规则成功",
			zap.Int("external_port_start", req.ExternalPort),
			zap.Int("external_port_end", req.ExternalPortEnd),
			zap.Int("internal_port_start", req.InternalPort),
			zap.Int("internal_port_end", req.InternalPortEnd),
			zap.String("protocol", req.Protocol))
	} else {
		logger.Global.Info(ctx, "删除NAT规则参数",
			zap.Int("external_port", req.ExternalPort),
			zap.Int("internal_port", req.InternalPort),
			zap.String("protocol", req.Protocol))

		if err := services.DeleteNATRule(req.Hostname, req.ExternalPort, req.InternalPort, req.Protocol); err != nil {
			logger.Global.Error(ctx, "删除NAT规则失败", zap.Error(err))
			c.JSON(http.StatusInternalServerError, models.NATOperationResponse{
				Code: 500,
				Msg:  "删除NAT规则失败: " + err.Error(),
			})
			return
		}

		logger.Global.Info(ctx, "删除NAT规则成功",
			zap.Int("external_port", req.ExternalPort),
			zap.Int("internal_port", req.InternalPort),
			zap.String("protocol", req.Protocol))
	}

	c.JSON(http.StatusOK, models.NATOperationResponse{
		Code: 200,
		Msg:  "NAT规则删除成功",
	})
}
