package handlers

import (
	"lxdapi/config"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"lxdapi/services"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// LXDServerListHandler 获取所有容器名称列表（轻量级）
// @Summary 获取容器名称列表
// @Description 获取所有LXD容器的名称列表（仅返回hostname和status，用于轻量级查询）
// @Tags 容器管理
// @Produce json
// @Success 200 {object} map[string]interface{} "code: 200, msg: success, data: []string"
// @Router /api/list [get]
func LXDServerListHandler(c *gin.Context) {
	ctx := c.Request.Context()
	logger.Global.Info(ctx, "获取容器列表请求")

	containers, err := services.GetAllContainers()
	if err != nil {
		logger.Global.Error(ctx, "获取容器列表失败", zap.Error(err))
		respondError(c, 500, "获取容器列表失败: "+err.Error())
		return
	}

	simplifiedContainers := make([]map[string]interface{}, 0, len(containers))
	for _, container := range containers {
		simplifiedContainers = append(simplifiedContainers, map[string]interface{}{
			"hostname": container.Name,
			"status":   container.Status,
		})
	}

	logger.Global.Info(ctx, "获取容器列表成功", zap.Int("count", len(containers)))
	respondSuccess(c, simplifiedContainers)
}

// LXDServerCreateHandler 提交容器创建请求
// @Summary 创建容器
// @Description 通过魔方财务创建 LXD 容器
// @Tags 容器管理
// @Accept json
// @Produce json
// @Param data body models.LXDServerCreateRequest true "创建参数"
// @Success 200 {object} models.LXDServerResponse
// @Failure 401 {object} map[string]string "认证失败"
// @Security ApiKeyAuth
// @Router /api/create [post]
func LXDServerCreateHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)

	logger.Global.Info(ctx, "创建容器请求")

	var req models.LXDServerCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Global.Error(ctx, "参数解析失败", zap.Error(err))
		c.JSON(400, models.LXDServerCreateResponse{
			Code: 400,
			Msg:  "请求参数解析失败: " + err.Error(),
		})
		return
	}

	lc.Container = req.Hostname
	lc.Action = "create"

	logger.Global.Info(ctx, "创建容器参数",
		zap.Int("cpus", req.CPUs),
		zap.String("memory", req.Memory),
		zap.String("disk", req.Disk),
		zap.String("image", req.Image))

	if req.Hostname == "" {
		logger.Global.Warn(ctx, "主机名为空")
		c.JSON(400, models.LXDServerCreateResponse{
			Code: 400,
			Msg:  "容器主机名不能为空",
		})
		return
	}
	if req.CPUs <= 0 {
		c.JSON(400, models.LXDServerCreateResponse{
			Code: 400,
			Msg:  "CPU核心数必须大于0",
		})
		return
	}
	if req.Memory == "" {
		c.JSON(400, models.LXDServerCreateResponse{
			Code: 400,
			Msg:  "内存不能为空",
		})
		return
	}
	if req.Disk == "" {
		c.JSON(400, models.LXDServerCreateResponse{
			Code: 400,
			Msg:  "硬盘大小不能为空",
		})
		return
	}
	if req.Image == "" {
		c.JSON(400, models.LXDServerCreateResponse{
			Code: 400,
			Msg:  "镜像不能为空",
		})
		return
	}

	networkMode := req.NetworkMode
	if networkMode == "" {
		networkMode = "mode1"
	}

	task, err := CreateTask(req.Hostname, "create", 5, req)
	if err != nil {
		logger.Global.Error(ctx, "创建任务失败", zap.Error(err))
		c.JSON(500, models.LXDServerResponse{
			Code: 500,
			Msg:  "创建任务失败: " + err.Error(),
		})
		return
	}

	// 根据网络模式返回对应的IP地址
	dedicatedIP := ""
	assignedIPs := ""

	serverIPs := config.AppConfig.System.Server.TLS.ServerIPs

	switch networkMode {
	case "mode1":
		// IPv4 NAT共享：返回服务器公网IPv4
		if len(serverIPs) > 0 {
			dedicatedIP = serverIPs[0]
		}
	case "mode2":
		// IPv4 NAT + IPv6独立：返回服务器公网IPv4，IPv6需要手动添加
		if len(serverIPs) > 0 {
			dedicatedIP = serverIPs[0]
		}
	}

	logger.Global.Info(ctx, "创建任务已提交",
		zap.Uint("task_id", task.ID),
		zap.String("dedicatedip", dedicatedIP),
		zap.String("assignedips", assignedIPs))

	response := models.LXDServerCreateResponse{
		Code: 200,
		Msg:  "容器创建任务已提交",
	}
	response.Data.DedicatedIP = dedicatedIP
	response.Data.AssignedIPs = assignedIPs

	c.JSON(200, response)
}

// LXDServerBootHandler 启动容器
// @Summary 启动容器
// @Description 提交容器开机任务
// @Tags 容器管理
// @Produce json
// @Param hostname query string true "容器名称"
// @Success 200 {object} models.LXDServerResponse
// @Router /api/boot [get]
func LXDServerBootHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)
	hostname := c.Query("hostname")

	lc.Container = hostname
	lc.Action = "start"

	logger.Global.Info(ctx, "开机容器请求")

	if hostname == "" {
		logger.Global.Warn(ctx, "缺少hostname参数")
		c.JSON(400, models.LXDServerResponse{
			Code: 400,
			Msg:  "缺少hostname参数",
		})
		return
	}

	task, err := CreateTask(hostname, "start", 5, nil)
	if err != nil {
		logger.Global.Error(ctx, "创建开机任务失败", zap.Error(err))
		c.JSON(500, models.LXDServerResponse{
			Code: 500,
			Msg:  "创建任务失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "开机任务已提交", zap.Uint("task_id", task.ID))

	c.JSON(200, models.LXDServerResponse{
		Code: 200,
		Msg:  "开机任务已提交",
		Data: map[string]interface{}{
			"task_id":  task.ID,
			"trace_id": lc.TraceID,
		},
	})
}

// LXDServerStopHandler 关闭容器
// @Summary 关闭容器
// @Description 提交容器关机任务
// @Tags 容器管理
// @Produce json
// @Param hostname query string true "容器名称"
// @Success 200 {object} models.LXDServerResponse
// @Router /api/stop [get]
func LXDServerStopHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)
	hostname := c.Query("hostname")

	lc.Container = hostname
	lc.Action = "stop"

	logger.Global.Info(ctx, "关机容器请求")

	if hostname == "" {
		logger.Global.Warn(ctx, "缺少hostname参数")
		c.JSON(400, models.LXDServerResponse{
			Code: 400,
			Msg:  "缺少hostname参数",
		})
		return
	}

	task, err := CreateTask(hostname, "stop", 5, nil)
	if err != nil {
		logger.Global.Error(ctx, "创建关机任务失败", zap.Error(err))
		c.JSON(500, models.LXDServerResponse{
			Code: 500,
			Msg:  "创建任务失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "关机任务已提交", zap.Uint("task_id", task.ID))

	c.JSON(200, models.LXDServerResponse{
		Code: 200,
		Msg:  "关机任务已提交",
		Data: map[string]interface{}{
			"task_id":  task.ID,
			"trace_id": lc.TraceID,
		},
	})
}

// LXDServerRebootHandler 重启容器
// @Summary 重启容器
// @Description 提交容器重启任务
// @Tags 容器管理
// @Produce json
// @Param hostname query string true "容器名称"
// @Success 200 {object} models.LXDServerResponse
// @Router /api/reboot [get]
func LXDServerRebootHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)
	hostname := c.Query("hostname")

	lc.Container = hostname
	lc.Action = "reboot"

	logger.Global.Info(ctx, "重启容器请求")

	if hostname == "" {
		logger.Global.Warn(ctx, "缺少hostname参数")
		c.JSON(400, models.LXDServerResponse{
			Code: 400,
			Msg:  "缺少hostname参数",
		})
		return
	}

	task, err := CreateTask(hostname, "reboot", 5, nil)
	if err != nil {
		logger.Global.Error(ctx, "创建重启任务失败", zap.Error(err))
		c.JSON(500, models.LXDServerResponse{
			Code: 500,
			Msg:  "创建任务失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "重启任务已提交", zap.Uint("task_id", task.ID))

	c.JSON(200, models.LXDServerResponse{
		Code: 200,
		Msg:  "重启任务已提交",
		Data: map[string]interface{}{
			"task_id":  task.ID,
			"trace_id": lc.TraceID,
		},
	})
}

// LXDServerDeleteHandler 删除容器
// @Summary 删除容器
// @Description 提交容器删除任务
// @Tags 容器管理
// @Produce json
// @Param hostname query string true "容器名称"
// @Success 200 {object} models.LXDServerResponse
// @Router /api/delete [get]
func LXDServerDeleteHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)
	hostname := c.Query("hostname")

	lc.Container = hostname
	lc.Action = "delete"

	logger.Global.Info(ctx, "删除容器请求")

	if hostname == "" {
		logger.Global.Warn(ctx, "缺少hostname参数")
		c.JSON(400, models.LXDServerResponse{
			Code: 400,
			Msg:  "缺少hostname参数",
		})
		return
	}

	task, err := CreateTask(hostname, "delete", 5, nil)
	if err != nil {
		logger.Global.Error(ctx, "创建删除任务失败", zap.Error(err))
		c.JSON(500, models.LXDServerResponse{
			Code: 500,
			Msg:  "创建任务失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "删除任务已提交", zap.Uint("task_id", task.ID))

	c.JSON(200, models.LXDServerResponse{
		Code: 200,
		Msg:  "删除任务已提交",
		Data: map[string]interface{}{
			"task_id":  task.ID,
			"trace_id": lc.TraceID,
		},
	})
}

// LXDServerReinstallHandler 重装系统
// @Summary 重装容器系统
// @Description 提交容器重装任务
// @Tags 容器管理
// @Accept json
// @Produce json
// @Param data body models.LXDServerReinstallRequest true "重装参数"
// @Success 200 {object} models.LXDServerResponse
// @Router /api/reinstall [post]
func LXDServerReinstallHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)

	logger.Global.Info(ctx, "重装系统请求")

	var req models.LXDServerReinstallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Global.Error(ctx, "参数解析失败", zap.Error(err))
		c.JSON(400, models.LXDServerResponse{
			Code: 400,
			Msg:  "参数错误: " + err.Error(),
		})
		return
	}

	lc.Container = req.Hostname
	lc.Action = "reinstall"

	logger.Global.Info(ctx, "重装系统参数", zap.String("system", req.System))

	if !services.ContainerExists(req.Hostname) {
		logger.Global.Warn(ctx, "容器不存在")
		c.JSON(404, models.LXDServerResponse{
			Code: 404,
			Msg:  "容器不存在: " + req.Hostname,
		})
		return
	}

	task, err := CreateTask(req.Hostname, "reinstall", 5, req)
	if err != nil {
		logger.Global.Error(ctx, "创建重装任务失败", zap.Error(err))
		c.JSON(500, models.LXDServerResponse{
			Code: 500,
			Msg:  "创建任务失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "重装系统任务已提交",
		zap.String("system", req.System),
		zap.Uint("task_id", task.ID))

	c.JSON(200, models.LXDServerResponse{
		Code: 200,
		Msg:  "重装系统任务已提交",
		Data: map[string]interface{}{
			"task_id":  task.ID,
			"trace_id": lc.TraceID,
		},
	})
}

// LXDServerResetPasswordHandler 重置容器root密码
// @Summary 重置密码
// @Description 提交密码重置任务
// @Tags 容器管理
// @Accept json
// @Produce json
// @Param data body models.LXDServerResetPasswordRequest true "密码参数"
// @Success 200 {object} models.LXDServerResponse
// @Router /api/password [post]
func LXDServerResetPasswordHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)

	logger.Global.Info(ctx, "重置密码请求")

	var req models.LXDServerResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Global.Error(ctx, "参数解析失败", zap.Error(err))
		c.JSON(400, models.LXDServerResponse{
			Code: 400,
			Msg:  "参数错误: " + err.Error(),
		})
		return
	}

	lc.Container = req.Hostname
	lc.Action = "reset_password"

	logger.Global.Info(ctx, "重置密码参数")

	if !services.ContainerExists(req.Hostname) {
		logger.Global.Warn(ctx, "容器不存在")
		c.JSON(404, models.LXDServerResponse{
			Code: 404,
			Msg:  "容器不存在: " + req.Hostname,
		})
		return
	}

	passwordData := map[string]string{
		"password": req.Password,
	}

	task, err := CreateTask(req.Hostname, "reset_password", 5, passwordData)
	if err != nil {
		logger.Global.Error(ctx, "创建重置密码任务失败", zap.Error(err))
		c.JSON(500, models.LXDServerResponse{
			Code: 500,
			Msg:  "创建任务失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "重置密码任务已提交", zap.Uint("task_id", task.ID))

	c.JSON(200, models.LXDServerResponse{
		Code: 200,
		Msg:  "密码重置任务已提交",
		Data: map[string]interface{}{
			"task_id":  task.ID,
			"trace_id": lc.TraceID,
		},
	})
}

// LXDServerSuspendHandler 暂停容器
// @Summary 暂停容器
// @Description 提交容器暂停任务
// @Tags 容器管理
// @Produce json
// @Param hostname query string true "容器名称"
// @Success 200 {object} models.LXDServerResponse
// @Router /api/suspend [get]
func LXDServerSuspendHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)
	hostname := c.Query("hostname")

	lc.Container = hostname
	lc.Action = "pause"

	logger.Global.Info(ctx, "暂停容器请求")

	if hostname == "" {
		logger.Global.Warn(ctx, "缺少hostname参数")
		c.JSON(400, models.LXDServerResponse{
			Code: 400,
			Msg:  "缺少hostname参数",
		})
		return
	}

	task, err := CreateTask(hostname, "pause", 5, nil)
	if err != nil {
		logger.Global.Error(ctx, "创建暂停任务失败", zap.Error(err))
		c.JSON(500, models.LXDServerResponse{
			Code: 500,
			Msg:  "创建任务失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "暂停任务已提交", zap.Uint("task_id", task.ID))

	c.JSON(200, models.LXDServerResponse{
		Code: 200,
		Msg:  "容器暂停任务已提交",
		Data: map[string]interface{}{
			"trace_id": lc.TraceID,
		},
	})
}

// LXDServerUnsuspendHandler 恢复容器
// @Summary 恢复容器
// @Description 提交容器恢复任务
// @Tags 容器管理
// @Produce json
// @Param hostname query string true "容器名称"
// @Success 200 {object} models.LXDServerResponse
// @Router /api/unsuspend [get]
func LXDServerUnsuspendHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)
	hostname := c.Query("hostname")

	lc.Container = hostname
	lc.Action = "resume"

	logger.Global.Info(ctx, "解除暂停容器请求")

	if hostname == "" {
		logger.Global.Warn(ctx, "缺少hostname参数")
		c.JSON(400, models.LXDServerResponse{
			Code: 400,
			Msg:  "缺少hostname参数",
		})
		return
	}

	task, err := CreateTask(hostname, "resume", 5, nil)
	if err != nil {
		logger.Global.Error(ctx, "创建恢复任务失败", zap.Error(err))
		c.JSON(500, models.LXDServerResponse{
			Code: 500,
			Msg:  "创建任务失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "解除暂停任务已提交", zap.Uint("task_id", task.ID))

	c.JSON(200, models.LXDServerResponse{
		Code: 200,
		Msg:  "容器恢复任务已提交",
		Data: map[string]interface{}{
			"trace_id": lc.TraceID,
		},
	})
}

func respondSuccess(c *gin.Context, data interface{}) {
	lc := logger.FromContext(c.Request.Context())
	c.JSON(200, gin.H{
		"code":     200,
		"msg":      "success",
		"trace_id": lc.TraceID,
		"data":     data,
	})
}

func respondError(c *gin.Context, code int, msg string) {
	lc := logger.FromContext(c.Request.Context())
	c.JSON(code, gin.H{
		"code":     code,
		"msg":      msg,
		"trace_id": lc.TraceID,
		"data":     []interface{}{},
	})
}
