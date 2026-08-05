package handlers

import (
	"net/http"

	"lxdapi/config"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"lxdapi/services"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// LXDServerAddIPv6Handler 添加IPv6独立绑定
// @Summary 添加IPv6独立绑定
// @Description 为容器添加独立的公网IPv6地址绑定
// @Tags IPv6绑定
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param hostname formData string true "容器名称"
// @Param description formData string false "绑定描述"
// @Success 200 {object} models.IPv6BindingResponse
// @Router /api/ipv6/add [post]
func LXDServerAddIPv6Handler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)

	logger.Global.Info(ctx, "IPv6绑定添加请求")

	var req models.IPv6BindingRequest
	if err := c.ShouldBind(&req); err != nil {
		logger.Global.Error(ctx, "参数解析失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.IPv6BindingResponse{
			Code: 400,
			Msg:  "请求参数错误: " + err.Error(),
		})
		return
	}

	lc.Container = req.Hostname
	lc.Action = "add_ipv6"

	logger.Global.Info(ctx, "处理IPv6绑定请求")

	if services.IPv6Manager == nil || !services.IPv6Manager.IsEnabled() {
		logger.Global.Warn(ctx, "IPv6绑定管理器未启用")
		c.JSON(http.StatusServiceUnavailable, models.IPv6BindingResponse{
			Code: 503,
			Msg:  "独立IPv6功能已禁用，请联系管理员启用此功能",
		})
		return
	}

	binding, err := services.IPv6Manager.AllocateIPv6ForContainer(req.Hostname, req.Description)
	if err != nil {
		logger.Global.Error(ctx, "IPv6绑定添加失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.IPv6BindingResponse{
			Code: 500,
			Msg:  "IPv6绑定添加失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "IPv6绑定添加成功", zap.String("ipv6", binding.PublicIPv6))

	c.JSON(http.StatusOK, models.IPv6BindingResponse{
		Code: 200,
		Msg:  "IPv6绑定添加成功",
		Data: binding,
	})
}

// LXDServerDeleteIPv6Handler 删除IPv6独立绑定
// @Summary 删除IPv6独立绑定
// @Description 删除容器的独立公网IPv6地址绑定
// @Tags IPv6绑定
// @Accept application/x-www-form-urlencoded
// @Produce json
// @Param hostname formData string true "容器名称"
// @Param public_ipv6 formData string true "公网IPv6地址"
// @Success 200 {object} models.IPv6BindingResponse
// @Router /api/ipv6/delete [post]
func LXDServerDeleteIPv6Handler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)

	logger.Global.Info(ctx, "IPv6绑定删除请求")

	var req models.DeleteIPv6BindingRequest
	if err := c.ShouldBind(&req); err != nil {
		logger.Global.Error(ctx, "参数解析失败", zap.Error(err))
		c.JSON(http.StatusBadRequest, models.IPv6BindingResponse{
			Code: 400,
			Msg:  "请求参数错误: " + err.Error(),
		})
		return
	}

	lc.Container = req.Hostname
	lc.Action = "delete_ipv6"

	logger.Global.Info(ctx, "处理IPv6绑定删除请求", zap.String("ipv6", req.PublicIPv6))

	if err := services.IPv6Manager.DeleteIPv6Binding(req.Hostname, req.PublicIPv6); err != nil {
		logger.Global.Error(ctx, "IPv6绑定删除失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.IPv6BindingResponse{
			Code: 500,
			Msg:  "IPv6绑定删除失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "IPv6绑定删除成功", zap.String("ipv6", req.PublicIPv6))

	c.JSON(http.StatusOK, models.IPv6BindingResponse{
		Code: 200,
		Msg:  "IPv6绑定删除成功",
	})
}

// LXDServerIPv6ListHandler 获取IPv6绑定列表
// @Summary 获取IPv6绑定列表
// @Description 获取指定容器的IPv6绑定列表
// @Tags IPv6绑定
// @Accept json
// @Produce json
// @Param hostname query string true "容器名称"
// @Success 200 {object} models.IPv6BindingListResponse
// @Router /api/ipv6/list [get]
func LXDServerIPv6ListHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)
	hostname := c.Query("hostname")

	lc.Container = hostname

	if hostname == "" {
		c.JSON(http.StatusBadRequest, models.IPv6BindingListResponse{
			Code: 400,
			Msg:  "缺少hostname参数",
		})
		return
	}

	logger.Global.Info(ctx, "获取IPv6绑定列表")

	bindings, err := services.GetIPv6BindingList(hostname)
	if err != nil {
		logger.Global.Error(ctx, "获取IPv6绑定列表失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, models.IPv6BindingListResponse{
			Code: 500,
			Msg:  "获取IPv6绑定列表失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "获取IPv6绑定列表成功", zap.Int("count", len(bindings)))

	c.JSON(http.StatusOK, models.IPv6BindingListResponse{
		Code: 200,
		Msg:  "获取成功",
		Data: bindings,
	})
}

// LXDServerIPv6StatusHandler 检查IPv6绑定状态
// @Summary 检查IPv6绑定状态
// @Description 检查系统IPv6绑定功能状态
// @Tags IPv6绑定
// @Accept json
// @Produce json
// @Success 200 {object} models.IPv6BindingStatusResponse
// @Router /api/ipv6/status [get]
func LXDServerIPv6StatusHandler(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Global.Info(ctx, "检查IPv6绑定状态")

	status := models.IPv6BindingStatusData{
		Enabled:   false,
		Interface: "",
		PoolInfo:  models.IPv6PoolStatus{},
	}

	if services.IPv6Manager != nil && services.IPv6Manager.IsEnabled() {
		status.Enabled = true
		status.Interface = config.AppConfig.IPv6Binding.Interface
		status.PoolInfo = models.IPv6PoolStatus{
			Start:        config.AppConfig.IPv6Binding.IPv6Pool.Start,
			PrefixLength: config.AppConfig.IPv6Binding.IPv6Pool.PrefixLength,
			PoolSize:     config.AppConfig.IPv6Binding.IPv6Pool.PoolSize,
		}
	}

	c.JSON(http.StatusOK, models.IPv6BindingStatusResponse{
		Code: 200,
		Msg:  "状态检查成功",
		Data: status,
	})
}
