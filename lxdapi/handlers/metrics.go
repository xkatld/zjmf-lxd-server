package handlers

import (
	"net/http"
	"runtime"

	"lxdapi/services"

	"github.com/gin-gonic/gin"
)

// MetricsHandler 获取系统性能指标
// @Summary 获取系统指标
// @Tags System
// @Produce json
// @Success 200 {object} map[string]interface{} "性能指标"
// @Router /metrics [get]
func MetricsHandler(c *gin.Context) {
	wp := services.GetGlobalWorkerPool()
	if wp == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error": "Worker pool not initialized",
		})
		return
	}
	
	metrics := wp.GetMetrics()
	
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	
	metrics["go_routines"] = runtime.NumGoroutine()
	metrics["memory_usage_mb"] = m.Alloc / 1024 / 1024
	metrics["memory_sys_mb"] = m.Sys / 1024 / 1024
	
	c.JSON(http.StatusOK, metrics)
}

