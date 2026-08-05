package handlers

import (
	"math"
	"strings"
	"time"

	"lxdapi/database"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"lxdapi/services"
	"lxdapi/utils"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type TrafficUsageResponse struct {
	ContainerName    string `json:"container_name"`
	TotalReceived    string `json:"total_received"`
	TotalSent        string `json:"total_sent"`
	TotalBytes       string `json:"total_bytes"`
	TotalReceivedRaw uint64 `json:"total_received_raw"`
	TotalSentRaw     uint64 `json:"total_sent_raw"`
	TotalBytesRaw    uint64 `json:"total_bytes_raw"`
	LastRecordTime   string `json:"last_record_time"`
}

type LXDServerTrafficResponse struct {
	Code    int         `json:"code"`
	Msg     string      `json:"msg"`
	TraceID string      `json:"trace_id,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// LXDServerTrafficHandler 获取流量使用情况
// @Summary 获取容器流量
// @Description 获取容器当前的流量统计信息
// @Tags 流量管理
// @Produce json
// @Param hostname query string true "容器名称"
// @Success 200 {object} LXDServerTrafficResponse
// @Router /api/traffic [get]
func LXDServerTrafficHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)
	hostname := c.Query("hostname")

	lc.Container = hostname

	logger.Global.Info(ctx, "流量查询请求")

	if hostname == "" {
		logger.Global.Warn(ctx, "缺少hostname参数")
		c.JSON(400, LXDServerTrafficResponse{
			Code:    400,
			Msg:     "缺少hostname参数",
			TraceID: lc.TraceID,
		})
		return
	}

	if err := services.UpdateTrafficStats(hostname); err != nil {
		logger.Global.Error(ctx, "更新流量统计失败", zap.Error(err))
		c.JSON(500, LXDServerTrafficResponse{
			Code:    500,
			Msg:     "更新流量统计失败: " + err.Error(),
			TraceID: lc.TraceID,
		})
		return
	}

	summary, err := services.GetTrafficSummary(hostname)
	if err != nil {
		logger.Global.Error(ctx, "获取流量统计失败", zap.Error(err))
		c.JSON(404, LXDServerTrafficResponse{
			Code:    404,
			Msg:     "获取流量统计失败: " + err.Error(),
			TraceID: lc.TraceID,
		})
		return
	}

	data := map[string]interface{}{
		"used": services.FormatBytes(float64(summary.TotalBytes)),
	}

	logger.Global.Info(ctx, "流量查询成功",
		zap.String("used", services.FormatBytes(float64(summary.TotalBytes))))

	c.JSON(200, LXDServerTrafficResponse{
		Code:    200,
		Msg:     "获取流量使用量成功",
		TraceID: lc.TraceID,
		Data:    data,
	})
}

// LXDServerResetTrafficHandler 重置流量统计
// @Summary 重置流量统计
// @Description 重置容器流量并重新启动统计
// @Tags 流量管理
// @Produce json
// @Param hostname query string true "容器名称"
// @Success 200 {object} LXDServerTrafficResponse
// @Router /api/traffic/reset [post]
func LXDServerResetTrafficHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)
	hostname := c.Query("hostname")

	lc.Container = hostname
	lc.Action = "reset_traffic"

	logger.Global.Info(ctx, "流量重置请求")

	if hostname == "" {
		logger.Global.Warn(ctx, "缺少hostname参数")
		c.JSON(400, LXDServerTrafficResponse{
			Code:    400,
			Msg:     "缺少hostname参数",
			TraceID: lc.TraceID,
		})
		return
	}

	if err := services.ResetTrafficStats(hostname); err != nil {
		logger.Global.Error(ctx, "流量重置失败", zap.Error(err))
		c.JSON(500, LXDServerTrafficResponse{
			Code:    500,
			Msg:     "重置流量统计失败: " + err.Error(),
			TraceID: lc.TraceID,
		})
		return
	}

	task, taskErr := CreateTask(hostname, "start", 5, nil)
	if taskErr != nil {
		logger.Global.Error(ctx, "创建开机任务失败", zap.Error(taskErr))
	} else {
		logger.Global.Info(ctx, "流量重置后开机任务已提交", zap.Uint("task_id", task.ID))
	}

	logger.Global.Info(ctx, "流量重置成功")

	c.JSON(200, LXDServerTrafficResponse{
		Code:    200,
		Msg:     "流量统计重置成功，开机任务已提交",
		TraceID: lc.TraceID,
	})
}

type LXDServerStatusResponse struct {
	Code    int                    `json:"code" example:"200"`
	Msg     string                 `json:"msg" example:"获取容器状态成功"`
	TraceID string                 `json:"trace_id,omitempty"`
	Data    map[string]interface{} `json:"data"`
}

// LXDServerStatusHandler 查询容器状态
// @Summary 查询容器状态
// @Description 获取容器当前状态
// @Tags 容器管理
// @Produce json
// @Param hostname query string true "容器名称"
// @Success 200 {object} LXDServerStatusResponse
// @Router /api/status [get]
func LXDServerStatusHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)
	hostname := c.Query("hostname")

	lc.Container = hostname

	logger.Global.Info(ctx, "容器状态查询请求")

	if hostname == "" {
		logger.Global.Warn(ctx, "缺少hostname参数")
		c.JSON(400, LXDServerStatusResponse{
			Code:    400,
			Msg:     "缺少hostname参数",
			TraceID: lc.TraceID,
		})
		return
	}

	status, err := services.GetContainerStatus(hostname)
	if err != nil {
		logger.Global.Error(ctx, "获取容器状态失败", zap.Error(err))
		c.JSON(404, LXDServerStatusResponse{
			Code:    404,
			Msg:     "容器不存在或获取状态失败: " + err.Error(),
			TraceID: lc.TraceID,
		})
		return
	}

	var zjmfStatus string
	switch status {
	case "Running":
		zjmfStatus = "RUNNING"
	case "Stopped":
		zjmfStatus = "STOPPED"
	case "Frozen":
		zjmfStatus = "FROZEN"
	default:
		zjmfStatus = status
	}

	logger.Global.Info(ctx, "容器状态查询成功", zap.String("status", zjmfStatus))

	data := map[string]interface{}{
		"status": zjmfStatus,
	}

	c.JSON(200, LXDServerStatusResponse{
		Code:    200,
		Msg:     "获取容器状态成功",
		TraceID: lc.TraceID,
		Data:    data,
	})
}

type LXDServerInfoResponse struct {
	Code    int                    `json:"code" example:"200"`
	Msg     string                 `json:"msg" example:"获取容器信息成功"`
	TraceID string                 `json:"trace_id,omitempty"`
	Data    map[string]interface{} `json:"data"`
}

// LXDServerInfoHandler 查询容器信息
// @Summary 查询容器信息
// @Description 获取容器基础信息和实时统计
// @Tags 容器管理
// @Produce json
// @Param hostname query string true "容器名称"
// @Success 200 {object} LXDServerInfoResponse
// @Router /api/info [get]
func LXDServerInfoHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)
	hostname := c.Query("hostname")

	lc.Container = hostname

	logger.Global.Info(ctx, "容器信息查询请求")

	if hostname == "" {
		logger.Global.Warn(ctx, "缺少hostname参数")
		c.JSON(400, LXDServerInfoResponse{
			Code:    400,
			Msg:     "缺少hostname参数",
			TraceID: lc.TraceID,
		})
		return
	}

	containerInfo, err := services.GetContainerInfo(hostname)
	if err != nil {
		logger.Global.Error(ctx, "获取基本信息失败", zap.Error(err))
		c.JSON(404, LXDServerInfoResponse{
			Code:    404,
			Msg:     "容器不存在或获取信息失败: " + err.Error(),
			TraceID: lc.TraceID,
		})
		return
	}

	stateData, err := services.GetContainerState(hostname)
	if err != nil {
		logger.Global.Error(ctx, "获取状态数据失败", zap.Error(err))
		c.JSON(500, LXDServerInfoResponse{
			Code:    500,
			Msg:     "获取容器状态失败: " + err.Error(),
			TraceID: lc.TraceID,
		})
		return
	}

	stats, err := services.GetContainerRealTimeStats(hostname, stateData)
	if err != nil {
		logger.Global.Error(ctx, "获取实时统计失败", zap.Error(err))
		c.JSON(500, LXDServerInfoResponse{
			Code:    500,
			Msg:     "获取容器实时统计失败: " + err.Error(),
			TraceID: lc.TraceID,
		})
		return
	}

	trafficSummary, err := services.GetTrafficSummary(hostname)
	if err != nil {
		trafficSummary = &models.TrafficSummary{
			ContainerName: hostname,
			TotalBytes:    0,
		}
	}

	var containerConfig models.ContainerConfig
	database.DB.Where("container_name = ?", hostname).First(&containerConfig)

	cpus := containerConfig.CPUs
	memory := 1024
	disk := 10240
	trafficLimit := containerConfig.TrafficLimit

	if memMB, err := utils.ParseSizeToMB(containerConfig.Memory); err == nil {
		memory = memMB
	}
	if diskMB, err := utils.ParseSizeToMB(containerConfig.Disk); err == nil {
		disk = diskMB
	}

	// 直接获取容器内网IP地址（不受网络模式影响）
	// containerInfo.IPAddresses 已经在 container_info.go 中过滤了 scope=global
	// 自动排除了回环地址（127.0.0.1, ::1）和链路本地地址（fe80::）
	var ipv4, ipv6 string
	if len(containerInfo.IPAddresses) > 0 {
		for _, ip := range containerInfo.IPAddresses {
			if strings.Contains(ip, ".") && ipv4 == "" {
				ipv4 = ip
			} else if strings.Contains(ip, ":") && ipv6 == "" {
				ipv6 = ip
			}

			if ipv4 != "" && ipv6 != "" {
				break
			}
		}
	}

	image := ""
	if containerConfig.Image != "" {
		image = containerConfig.Image
	} else if containerInfo.Image != "" {
		image = containerInfo.Image
	} else {
		if containerInfo.Config != nil {
			if imgDesc, ok := containerInfo.Config["image.description"].(string); ok && imgDesc != "" {
				image = imgDesc
			} else if imgOS, ok := containerInfo.Config["image.os"].(string); ok {
				if imgRelease, ok := containerInfo.Config["image.release"].(string); ok {
					image = imgOS + "/" + imgRelease
				} else {
					image = imgOS
				}
			}
		}
	}

	data := map[string]interface{}{
		"hostname":   hostname,
		"status":     containerInfo.Status,
		"ipv4":       ipv4,
		"ipv6":       ipv6,
		"image":      image,
		"type":       containerInfo.Type,
		"created_at": containerInfo.CreatedAt,

		"config": map[string]interface{}{
			"cpus":          cpus,
			"memory":        containerConfig.Memory,
			"disk":          containerConfig.Disk,
			"traffic_limit": trafficLimit,
		},

		"cpus":   cpus,
		"memory": memory,
		"disk":   disk,

		"usage": map[string]interface{}{
			"cpu_usage":         int(stats.CPUUsagePercent),
			"memory_usage":      services.FormatBytes(float64(stats.MemoryUsedMB * 1024 * 1024)),
			"memory_usage_raw":  uint64(stats.MemoryUsedMB * 1024 * 1024),
			"disk_usage":        services.FormatBytes(float64(stats.DiskUsedMB * 1024 * 1024)),
			"disk_usage_raw":    uint64(stats.DiskUsedMB * 1024 * 1024),
			"traffic_usage":     services.FormatBytes(float64(trafficSummary.TotalBytes)),
			"traffic_usage_raw": trafficSummary.TotalBytes,
		},

		"cpu_usage":         int(stats.CPUUsagePercent),
		"memory_usage":      services.FormatBytes(float64(stats.MemoryUsedMB * 1024 * 1024)),
		"memory_usage_raw":  uint64(stats.MemoryUsedMB * 1024 * 1024),
		"disk_usage":        services.FormatBytes(float64(stats.DiskUsedMB * 1024 * 1024)),
		"disk_usage_raw":    uint64(stats.DiskUsedMB * 1024 * 1024),
		"traffic_usage":     services.FormatBytes(float64(trafficSummary.TotalBytes)),
		"traffic_usage_raw": trafficSummary.TotalBytes,

		"cpu_percent":    int(stats.CPUUsagePercent),
		"memory_percent": math.Round(calculateMemoryPercent(uint64(stats.MemoryUsedMB*1024*1024), memory)*100) / 100,
		"disk_percent":   int(calculateDiskPercent(uint64(stats.DiskUsedMB*1024*1024), disk)),

		"last_update": stats.Timestamp,
		"timestamp":   time.Now().Unix(),
	}

	logger.Global.Info(ctx, "容器信息查询成功",
		zap.String("status", containerInfo.Status),
		zap.Int("cpus", cpus),
		zap.Int("memory", memory))

	go func() {
		if err := services.UpdateContainerCache(hostname, data); err != nil {
			logger.Global.Error(ctx, "缓存更新失败",
				zap.String("hostname", hostname),
				zap.Error(err))
		} else {
			logger.Global.Debug(ctx, "缓存更新成功", zap.String("hostname", hostname))
		}
	}()

	c.JSON(200, LXDServerInfoResponse{
		Code:    200,
		Msg:     "获取容器信息成功",
		TraceID: lc.TraceID,
		Data:    data,
	})
}

type LXDServerCheckResponse struct {
	Code    int                    `json:"code" example:"200"`
	Msg     string                 `json:"msg" example:"API连接正常"`
	TraceID string                 `json:"trace_id,omitempty"`
	Data    map[string]interface{} `json:"data"`
}

// LXDServerCheckHandler API连通性检查
// @Summary API检查
// @Description 检查API连接与LXD服务状态
// @Tags 容器管理
// @Produce json
// @Success 200 {object} LXDServerCheckResponse
// @Router /api/check [get]
func LXDServerCheckHandler(c *gin.Context) {
	ctx := c.Request.Context()
	lc := logger.FromContext(ctx)

	logger.Global.Info(ctx, "API连接检查请求")

	lxdVersion, err := services.GetLXDVersion()
	if err != nil {
		logger.Global.Error(ctx, "获取LXD版本失败", zap.Error(err))
		c.JSON(500, LXDServerCheckResponse{
			Code:    500,
			Msg:     "获取LXD版本失败: " + err.Error(),
			TraceID: lc.TraceID,
		})
		return
	}

	logger.Global.Info(ctx, "API连接检查成功", zap.String("lxd_version", lxdVersion))

	c.JSON(200, gin.H{
		"code":     200,
		"msg":      "API连接检查成功",
		"trace_id": lc.TraceID,
		"data": gin.H{
			"server_ip":   c.ClientIP(),
			"lxd_version": lxdVersion,
			"api_version": "1.0.2",
		},
	})
}

func calculateMemoryPercent(used uint64, total int) float64 {
	if total <= 0 {
		return 0.0
	}
	totalBytes := uint64(total) * 1024 * 1024
	if totalBytes == 0 {
		return 0.0
	}
	percent := float64(used) / float64(totalBytes) * 100
	if percent > 100 {
		percent = 100
	}
	return math.Round(percent*100) / 100
}

func calculateDiskPercent(used uint64, total int) float64 {
	if total <= 0 {
		return 0.0
	}
	totalBytes := uint64(total) * 1024 * 1024
	if totalBytes == 0 {
		return 0.0
	}
	percent := float64(used) / float64(totalBytes) * 100
	if percent > 100 {
		percent = 100
	}
	return math.Round(percent*100) / 100
}
