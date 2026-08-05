package handlers

import (
	"net/http"

	"lxdapi/database"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"lxdapi/services"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// GetContainersCacheHandler 获取所有容器缓存
// @Summary 获取所有容器缓存
// @Description 从缓存中获取所有容器信息（lxdweb专用接口）
// @Tags 缓存接口
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /api/cache/containers [get]
func GetContainersCacheHandler(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Global.Info(ctx, "获取容器缓存请求")

	containers, err := services.GetAllContainersFromCache()
	if err != nil {
		logger.Global.Error(ctx, "获取容器缓存失败", zap.Error(err))
		c.JSON(http.StatusOK, gin.H{
			"code": 500,
			"msg":  "获取缓存失败: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "获取容器缓存成功", zap.Int("count", len(containers)))

	c.JSON(http.StatusOK, gin.H{
		"code":       200,
		"msg":        "success",
		"data":       containers,
		"from_cache": true,
		"count":      len(containers),
	})
}

// RefreshContainersCacheHandler 从LXD刷新所有容器到缓存
// @Summary 刷新容器列表缓存
// @Description 从LXD重新获取所有容器并更新到缓存数据库
// @Tags 缓存接口
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Security ApiKeyAuth
// @Router /api/cache/containers/refresh [post]
func RefreshContainersCacheHandler(c *gin.Context) {
	ctx := c.Request.Context()

	logger.Global.Info(ctx, "开始刷新容器列表缓存")

	// 从LXD获取所有容器
	containers, err := services.GetAllContainers()
	if err != nil {
		logger.Global.Error(ctx, "从LXD获取容器列表失败", zap.Error(err))
		c.JSON(http.StatusOK, gin.H{
			"code": 500,
			"msg":  "获取容器列表失败: " + err.Error(),
		})
		return
	}

	// 更新每个容器到缓存
	successCount := 0
	failCount := 0
	for _, container := range containers {
		// 获取容器详细信息和状态
		info, err := services.GetContainerInfo(container.Name)
		if err != nil {
			logger.Global.Warn(ctx, "获取容器信息失败", zap.String("hostname", container.Name), zap.Error(err))
			failCount++
			continue
		}

		stateData, err := services.GetContainerState(container.Name)
		if err != nil {
			logger.Global.Warn(ctx, "获取容器状态失败", zap.String("hostname", container.Name), zap.Error(err))
			failCount++
			continue
		}

		stats, err := services.GetContainerRealTimeStats(container.Name, stateData)
		if err != nil {
			logger.Global.Warn(ctx, "获取容器统计失败", zap.String("hostname", container.Name), zap.Error(err))
			// 继续处理，不统计为失败
		}

		// 构建缓存数据
		infoMap := convertContainerInfoToMap(info, stats)
		
		// 添加流量限制和统计数据
		enrichContainerCacheData(container.Name, infoMap)

		// 更新到缓存
		if err := services.UpdateContainerCache(container.Name, infoMap); err != nil {
			logger.Global.Error(ctx, "更新容器缓存失败", zap.String("hostname", container.Name), zap.Error(err))
			failCount++
		} else {
			successCount++
		}
	}

	logger.Global.Info(ctx, "容器列表缓存刷新完成",
		zap.Int("total", len(containers)),
		zap.Int("success", successCount),
		zap.Int("failed", failCount))

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "容器列表刷新完成",
		"data": gin.H{
			"total":   len(containers),
			"success": successCount,
			"failed":  failCount,
		},
	})
}

// enrichContainerCacheData 从数据库中补充容器配置和流量数据
func enrichContainerCacheData(hostname string, infoMap map[string]interface{}) {
	// 查询容器配置获取流量限制
	var config models.ContainerConfig
	if err := database.DB.Where("container_name = ?", hostname).First(&config).Error; err == nil {
		infoMap["traffic_limit"] = config.TrafficLimit
		// 同时补充配置表的其他信息
		infoMap["cpus"] = config.CPUs
		infoMap["memory"] = config.Memory
		infoMap["disk"] = config.Disk
	}
	
	// 查询流量汇总获取已使用流量
	var traffic models.TrafficSummary
	if err := database.DB.Where("container_name = ?", hostname).First(&traffic).Error; err == nil {
		infoMap["traffic_in"] = traffic.TotalReceived
		infoMap["traffic_out"] = traffic.TotalSent
		infoMap["traffic_total"] = traffic.TotalBytes
	}
}

func convertContainerInfoToMap(info *models.ContainerInfo, stats *models.ContainerRealTimeStats) map[string]interface{} {
	result := make(map[string]interface{})

	if info != nil {
		result["hostname"] = info.Name
		result["status"] = info.Status
		result["image"] = info.Image

		// IP地址
		if len(info.IPAddresses) > 0 {
			for _, ip := range info.IPAddresses {
				if len(ip) > 0 && ip[0] != ':' { // IPv4
					result["ipv4"] = ip
					break
				}
			}
			for _, ip := range info.IPAddresses {
				if len(ip) > 0 && ip[0] == ':' { // IPv6
					result["ipv6"] = ip
					break
				}
			}
		}
	}

	if stats != nil {
		result["cpu_usage"] = stats.CPUUsagePercent
		result["memory_usage"] = stats.MemoryUsedMB
		result["memory_total"] = stats.MemoryTotalMB
		result["disk_usage"] = stats.DiskUsedMB
		result["disk_total"] = stats.DiskTotalGB * 1024 // 转换为MB
	}

	return result
}
