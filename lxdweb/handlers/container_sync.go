package handlers

import (
	"net/http"
	"strconv"

	"lxdweb/database"
	"lxdweb/models"
	"lxdweb/services"

	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)

// SyncAllNodes 同步所有节点
// @Summary 同步所有节点的容器信息
// @Description 启动所有活跃节点的容器信息同步任务
// @Tags 容器同步
// @Produce json
// @Success 200 {object} map[string]interface{} "同步任务已启动"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Failure 500 {object} map[string]interface{} "查询失败"
// @Router /api/sync/all [post]
func SyncAllNodes(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("username")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 401,
			"msg":  "未登录",
		})
		return
	}

	var nodes []models.Node
	if err := database.DB.Where("status = ?", "active").Find(&nodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "查询节点失败",
		})
		return
	}

	for _, node := range nodes {
		go services.SyncNodeContainers(node.ID, true)
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "容器同步任务已启动，请稍后刷新页面查看结果",
	})
}

// SyncNode 同步指定节点
// @Summary 同步指定节点的容器信息
// @Description 启动指定节点的容器信息同步任务
// @Tags 容器同步
// @Produce json
// @Param id path string true "节点ID"
// @Success 200 {object} map[string]interface{} "同步任务已启动"
// @Failure 400 {object} map[string]interface{} "节点ID格式错误"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Failure 404 {object} map[string]interface{} "节点不存在"
// @Router /api/sync/node/{id} [post]
func SyncNode(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("username")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 401,
			"msg":  "未登录",
		})
		return
	}

	nodeIDStr := c.Param("id")
	nodeID, err := strconv.ParseUint(nodeIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "节点ID格式错误",
		})
		return
	}

	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "节点不存在",
		})
		return
	}

	go services.SyncNodeContainers(uint(nodeID), true)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "容器同步任务已启动，请稍后刷新页面查看结果",
	})
}

// GetSyncTasks 获取同步任务列表
// @Summary 获取容器同步任务列表
// @Description 查询最近50条容器同步任务记录
// @Tags 容器同步
// @Produce json
// @Success 200 {object} map[string]interface{} "成功返回任务列表"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Router /api/sync/tasks [get]
func GetSyncTasks(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("username")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 401,
			"msg":  "未登录",
		})
		return
	}

	var tasks []models.SyncTask
	database.DB.Order("created_at DESC").Limit(50).Find(&tasks)

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": tasks,
	})
}

// GetSyncStatus 获取同步状态
// @Summary 获取容器同步状态
// @Description 查询指定节点或所有节点的容器同步状态
// @Tags 容器同步
// @Produce json
// @Param node_id query string false "节点ID"
// @Success 200 {object} map[string]interface{} "成功返回同步状态"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Router /api/sync/status [get]
func GetSyncStatus(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("username")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 401,
			"msg":  "未登录",
		})
		return
	}

	nodeIDStr := c.Query("node_id")
	
	var status []map[string]interface{}
	
	if nodeIDStr != "" {
		nodeID, err := strconv.ParseUint(nodeIDStr, 10, 32)
		if err == nil {
			var lastTask models.SyncTask
			database.DB.Where("node_id = ?", nodeID).Order("created_at DESC").First(&lastTask)
			
			status = append(status, map[string]interface{}{
				"node_id":   uint(nodeID),
				"last_task": lastTask,
			})
		}
	} else {
		var nodes []models.Node
		database.DB.Find(&nodes)
		
		for _, node := range nodes {
			var lastTask models.SyncTask
			database.DB.Where("node_id = ?", node.ID).Order("created_at DESC").First(&lastTask)
			
			status = append(status, map[string]interface{}{
				"node_id":   node.ID,
				"node_name": node.Name,
				"last_task": lastTask,
			})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": status,
	})
}

