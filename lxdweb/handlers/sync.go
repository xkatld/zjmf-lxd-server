package handlers

import (
	"lxdweb/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

// GetAutoSyncStatus 获取自动同步状态
// @Summary 获取自动同步状态
// @Description 查询自动同步服务的启用状态
// @Tags 系统管理
// @Produce json
// @Success 200 {object} map[string]interface{} "返回自动同步状态"
// @Router /api/auto-sync/status [get]
func GetAutoSyncStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    200,
		"msg":     "success",
		"enabled": services.IsAutoSyncEnabled(),
	})
}

// EnableAutoSync 启用自动同步
// @Summary 启用自动同步
// @Description 启用自动同步服务，每5分钟自动同步所有节点数据
// @Tags 系统管理
// @Produce json
// @Success 200 {object} map[string]interface{} "启用成功"
// @Router /api/auto-sync/enable [post]
func EnableAutoSync(c *gin.Context) {
	services.EnableAutoSync()
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "自动同步已启用",
	})
}

// DisableAutoSync 禁用自动同步
// @Summary 禁用自动同步
// @Description 禁用自动同步服务，停止定时同步
// @Tags 系统管理
// @Produce json
// @Success 200 {object} map[string]interface{} "禁用成功"
// @Router /api/auto-sync/disable [post]
func DisableAutoSync(c *gin.Context) {
	services.DisableAutoSync()
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "自动同步已禁用",
	})
}
