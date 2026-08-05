package handlers
import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"lxdweb/database"
	"lxdweb/models"
	"lxdweb/pkg/logger"
	"lxdweb/services"
	"net/http"
	"strconv"
	"time"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// NodesPage 节点管理页面
// @Summary 节点管理页面
// @Description 显示节点管理页面
// @Tags 节点管理
// @Produce html
// @Success 200 {string} string "HTML页面"
// @Router /nodes [get]
func NodesPage(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("username")
	c.HTML(http.StatusOK, "nodes.html", gin.H{
		"title":    "节点管理 - LXD管理后台",
		"username": username,
	})
}

// NodeDetailPage 节点详情页面
// @Summary 节点详情页面
// @Description 显示单个节点的详细信息和管理界面
// @Tags 节点管理
// @Produce html
// @Param id path string true "节点ID"
// @Success 200 {string} string "HTML页面"
// @Router /nodes/{id} [get]
func NodeDetailPage(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("username")
	nodeID := c.Param("id")
	
	c.HTML(http.StatusOK, "node_detail.html", gin.H{
		"title":    "节点详情 - LXD管理后台",
		"username": username,
		"node_id":  nodeID,
	})
}

// NodeContainersPage 节点容器列表页面
func NodeContainersPage(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("username")
	nodeID := c.Param("id")
	
	c.HTML(http.StatusOK, "node_containers.html", gin.H{
		"title":    "容器列表 - LXD管理后台",
		"username": username,
		"node_id":  nodeID,
	})
}

// ContainerDetailPage 容器详情页面
func ContainerDetailPage(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("username")
	nodeID := c.Param("id")
	containerName := c.Param("name")
	
	c.HTML(http.StatusOK, "container_detail.html", gin.H{
		"title":          "容器详情 - LXD管理后台",
		"username":       username,
		"node_id":        nodeID,
		"container_name": containerName,
	})
}

// GetNodes 获取节点列表
// @Summary 获取节点列表
// @Description 查询所有LXD节点及其系统信息，支持按状态过滤
// @Tags 节点管理
// @Produce json
// @Param status query string false "节点状态(active/inactive)"
// @Success 200 {object} map[string]interface{} "成功返回节点列表"
// @Failure 500 {object} map[string]interface{} "查询失败"
// @Router /api/nodes [get]
func GetNodes(c *gin.Context) {
	var nodes []models.Node
	if err := database.DB.Order("created_at desc").Find(&nodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "查询失败",
		})
		return
	}

	var caches []models.NodeInfoCache
	database.DB.Find(&caches)
	cacheMap := make(map[uint]models.NodeInfoCache)
	for _, cache := range caches {
		cacheMap[cache.NodeID] = cache
	}
	
	result := make([]map[string]interface{}, 0, len(nodes))
	for _, node := range nodes {
		nodeData := map[string]interface{}{
			"id":          node.ID,
			"name":        node.Name,
			"description": node.Description,
			"address":     node.Address,
			"api_key":     node.APIKey,
			"status":      node.Status,
			"last_check":  node.LastCheck,
			"created_at":  node.CreatedAt,
			"updated_at":  node.UpdatedAt,
		}

		if cache, ok := cacheMap[node.ID]; ok {
			var sysInfo map[string]interface{}
			if err := json.Unmarshal([]byte(cache.SystemInfo), &sysInfo); err == nil {
				nodeData["system_info"] = sysInfo
			}
		}
		
		result = append(result, nodeData)
	}
	
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": result,
	})
}
// GetNode 获取单个节点
// @Summary 获取单个节点
// @Description 根据ID获取节点详情
// @Tags 节点管理
// @Produce json
// @Param id path string true "节点ID"
// @Success 200 {object} map[string]interface{} "成功返回节点信息"
// @Failure 404 {object} map[string]interface{} "节点不存在"
// @Router /api/nodes/{id} [get]
func GetNode(c *gin.Context) {
	id := c.Param("id")
	var node models.Node
	if err := database.DB.First(&node, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "节点不存在",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": node,
	})
}
// CreateNode 创建节点
// @Summary 创建节点
// @Description 添加新的LXD节点到管理系统
// @Tags 节点管理
// @Accept json
// @Produce json
// @Param body body models.CreateNodeRequest true "节点配置参数"
// @Success 200 {object} map[string]interface{} "创建成功"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Router /api/nodes [post]
func CreateNode(c *gin.Context) {
	ctx := c.Request.Context()
	
	var req models.CreateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Global.Error(ctx, "创建节点参数解析失败", 
			zap.Error(err),
			zap.String("action", "create_node"))
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误: " + err.Error(),
		})
		return
	}
	
	logger.Global.Info(ctx, "收到创建节点请求",
		zap.String("name", req.Name),
		zap.String("address", req.Address),
		zap.Bool("auto_sync", req.AutoSync),
		zap.Int("sync_interval", req.SyncInterval),
		zap.Int("batch_size", req.BatchSize),
		zap.Int("batch_interval", req.BatchInterval),
		zap.String("action", "create_node"))
	
	var count int64
	database.DB.Unscoped().Model(&models.Node{}).Where("name = ?", req.Name).Count(&count)
	if count > 0 {
		logger.Global.Warn(ctx, "节点名称已存在",
			zap.String("name", req.Name),
			zap.String("action", "create_node"))
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "节点名称已存在",
		})
		return
	}
	
	syncInterval := req.SyncInterval
	if syncInterval <= 0 {
		syncInterval = 300
	}
	
	batchSize := req.BatchSize
	if batchSize <= 0 {
		batchSize = 5
	}
	
	batchInterval := req.BatchInterval
	if batchInterval <= 0 {
		batchInterval = 5
	}
	
	node := models.Node{
		Name:          req.Name,
		Description:   req.Description,
		Address:       req.Address,
		APIKey:        req.APIKey,
		Status:        "inactive",
		AutoSync:      req.AutoSync,
		SyncInterval:  syncInterval,
		BatchSize:     batchSize,
		BatchInterval: batchInterval,
	}
	
	logger.Global.Debug(ctx, "准备创建节点",
		zap.String("name", node.Name),
		zap.String("address", node.Address),
		zap.Int("batch_size", node.BatchSize),
		zap.Int("batch_interval", node.BatchInterval),
		zap.String("action", "create_node"))
	
	if err := database.DB.Create(&node).Error; err != nil {
		logger.Global.Error(ctx, "数据库创建节点失败",
			zap.Error(err),
			zap.String("name", req.Name),
			zap.String("action", "create_node"))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "创建失败: " + err.Error(),
		})
		return
	}
	
	logger.Global.Info(ctx, "节点创建成功",
		zap.Uint("node_id", node.ID),
		zap.String("name", node.Name),
		zap.String("action", "create_node"))
	
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "创建成功",
		"data": node,
	})
}
// UpdateNode 更新节点
// @Summary 更新节点
// @Description 更新节点配置信息
// @Tags 节点管理
// @Accept json
// @Produce json
// @Param id path string true "节点ID"
// @Param body body models.UpdateNodeRequest true "更新参数"
// @Success 200 {object} map[string]interface{} "更新成功"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 404 {object} map[string]interface{} "节点不存在"
// @Router /api/nodes/{id} [put]
func UpdateNode(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	
	var node models.Node
	if err := database.DB.First(&node, id).Error; err != nil {
		logger.Global.Warn(ctx, "更新节点失败-节点不存在",
			zap.String("node_id", id),
			zap.String("action", "update_node"))
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "节点不存在",
		})
		return
	}
	
	var req models.UpdateNodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Global.Error(ctx, "更新节点参数解析失败",
			zap.Error(err),
			zap.Uint("node_id", node.ID),
			zap.String("action", "update_node"))
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误: " + err.Error(),
		})
		return
	}
	
	logger.Global.Info(ctx, "收到更新节点请求",
		zap.Uint("node_id", node.ID),
		zap.String("name", req.Name),
		zap.String("action", "update_node"))
	updates := map[string]interface{}{}
	if req.Name != "" {
		var count int64
		database.DB.Unscoped().Model(&models.Node{}).Where("name = ? AND id != ?", req.Name, id).Count(&count)
		if count > 0 {
			logger.Global.Warn(ctx, "节点名称已存在",
				zap.String("name", req.Name),
				zap.Uint("node_id", node.ID),
				zap.String("action", "update_node"))
			c.JSON(http.StatusBadRequest, gin.H{
				"code": 400,
				"msg":  "节点名称已存在",
			})
			return
		}
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Address != "" {
		updates["address"] = req.Address
	}
	if req.APIKey != "" {
		updates["api_key"] = req.APIKey
	}
	
	updates["auto_sync"] = req.AutoSync
	
	if req.SyncInterval > 0 {
		updates["sync_interval"] = req.SyncInterval
	}
	
	if req.BatchSize > 0 {
		updates["batch_size"] = req.BatchSize
	}
	if req.BatchInterval > 0 {
		updates["batch_interval"] = req.BatchInterval
	}
	
	if err := database.DB.Model(&node).Updates(updates).Error; err != nil {
		logger.Global.Error(ctx, "数据库更新节点失败",
			zap.Error(err),
			zap.Uint("node_id", node.ID),
			zap.String("action", "update_node"))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "更新失败: " + err.Error(),
		})
		return
	}
	
	database.DB.First(&node, id)
	logger.Global.Info(ctx, "节点更新成功",
		zap.Uint("node_id", node.ID),
		zap.String("name", node.Name),
		zap.String("action", "update_node"))
	
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "更新成功",
		"data": node,
	})
}
// DeleteNode 删除节点
// @Summary 删除节点
// @Description 从管理系统中删除指定节点及其所有关联数据
// @Tags 节点管理
// @Produce json
// @Param id path string true "节点ID"
// @Success 200 {object} map[string]interface{} "删除成功"
// @Failure 500 {object} map[string]interface{} "删除失败"
// @Router /api/nodes/{id} [delete]
func DeleteNode(c *gin.Context) {
	ctx := c.Request.Context()
	id := c.Param("id")
	nodeID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		logger.Global.Warn(ctx, "删除节点失败-无效ID",
			zap.String("node_id", id),
			zap.Error(err),
			zap.String("action", "delete_node"))
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "无效的节点ID",
		})
		return
	}
	
	logger.Global.Info(ctx, "开始删除节点及关联数据",
		zap.Uint64("node_id", nodeID),
		zap.String("action", "delete_node"))
	
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("node_id = ?", nodeID).Delete(&models.Container{}).Error; err != nil {
			return fmt.Errorf("删除容器数据失败: %w", err)
		}
		
		if err := tx.Unscoped().Where("node_id = ?", nodeID).Delete(&models.ContainerCache{}).Error; err != nil {
			return fmt.Errorf("删除容器缓存失败: %w", err)
		}
		
		if err := tx.Unscoped().Where("node_id = ?", nodeID).Delete(&models.NodeInfoCache{}).Error; err != nil {
			return fmt.Errorf("删除节点缓存失败: %w", err)
		}
		
		if err := tx.Unscoped().Where("node_id = ?", nodeID).Delete(&models.SyncTask{}).Error; err != nil {
			return fmt.Errorf("删除同步任务失败: %w", err)
		}
		
		if err := tx.Unscoped().Delete(&models.Node{}, id).Error; err != nil {
			return fmt.Errorf("删除节点失败: %w", err)
		}
		
		return nil
	})
	
	if err != nil {
		logger.Global.Error(ctx, "数据库删除节点失败",
			zap.Error(err),
			zap.Uint64("node_id", nodeID),
			zap.String("action", "delete_node"))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "删除失败: " + err.Error(),
		})
		return
	}
	
	logger.Global.Info(ctx, "节点删除成功",
		zap.Uint64("node_id", nodeID),
		zap.String("action", "delete_node"))
	
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "删除成功",
	})
}
// TestNode 测试节点连接
// @Summary 测试节点连接
// @Description 测试与LXD节点的连接状态
// @Tags 节点管理
// @Produce json
// @Param id path string true "节点ID"
// @Success 200 {object} map[string]interface{} "测试成功"
// @Failure 404 {object} map[string]interface{} "节点不存在"
// @Failure 500 {object} map[string]interface{} "连接失败"
// @Router /api/nodes/{id}/test [post]
func TestNode(c *gin.Context) {
	id := c.Param("id")
	idInt, _ := strconv.ParseUint(id, 10, 32)
	var node models.Node
	if err := database.DB.First(&node, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "节点不存在",
		})
		return
	}
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	req, err := http.NewRequest("GET", node.Address+"/api/check", nil)
	if err != nil {
		updateNodeStatus(uint(idInt), "error")
		c.JSON(http.StatusOK, gin.H{
			"code": 500,
			"msg":  "连接失败: " + err.Error(),
		})
		return
	}
	if node.APIKey != "" {
		req.Header.Set("apikey", node.APIKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		updateNodeStatus(uint(idInt), "error")
		c.JSON(http.StatusOK, gin.H{
			"code": 500,
			"msg":  "连接失败: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		updateNodeStatus(uint(idInt), "active")
		go services.RefreshNodeCache(uint(idInt))
		c.JSON(http.StatusOK, gin.H{
			"code": 200,
			"msg":  "连接成功",
		})
	} else {
		updateNodeStatus(uint(idInt), "error")
		c.JSON(http.StatusOK, gin.H{
			"code": 500,
			"msg":  fmt.Sprintf("连接失败: HTTP %d", resp.StatusCode),
		})
	}
}
// RefreshNodeCache 刷新节点缓存
// @Summary 刷新节点缓存
// @Description 刷新指定节点的系统信息缓存
// @Tags 节点管理
// @Produce json
// @Param id path string true "节点ID"
// @Success 200 {object} map[string]interface{} "刷新成功"
// @Failure 404 {object} map[string]interface{} "节点不存在"
// @Router /api/nodes/{id}/refresh [post]
func RefreshNodeCache(c *gin.Context) {
	id := c.Param("id")
	idInt, _ := strconv.ParseUint(id, 10, 32)
	
	// 更新节点系统信息缓存
	if err := services.RefreshNodeCache(uint(idInt)); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "节点不存在",
		})
		return
	}
	
	// 刷新容器缓存（从lxdapi缓存快速复制）
	go services.RefreshNodeContainers(uint(idInt), true)
	
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "刷新任务已启动",
	})
}

func updateNodeStatus(nodeID uint, status string) {
	now := time.Now()
	database.DB.Model(&models.Node{}).Where("id = ?", nodeID).Updates(map[string]interface{}{
		"status":     status,
		"last_check": now,
	})
}

func ExportNodes(c *gin.Context) {
	var nodes []models.Node
	if err := database.DB.Find(&nodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "查询失败",
		})
		return
	}

	exportData := make([]map[string]interface{}, 0, len(nodes))
	for _, node := range nodes {
		exportData = append(exportData, map[string]interface{}{
			"name":           node.Name,
			"address":        node.Address,
			"api_key":        node.APIKey,
			"description":    node.Description,
			"auto_sync":      node.AutoSync,
			"sync_interval":  node.SyncInterval,
			"batch_size":     node.BatchSize,
			"batch_interval": node.BatchInterval,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": exportData,
	})
}

func ImportNodes(c *gin.Context) {
	ctx := c.Request.Context()
	
	var importData []map[string]interface{}
	if err := c.ShouldBindJSON(&importData); err != nil {
		logger.Global.Error(ctx, "导入节点参数解析失败",
			zap.Error(err),
			zap.String("action", "import_nodes"))
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "开始批量导入节点",
		zap.Int("total", len(importData)),
		zap.String("action", "import_nodes"))
	
	successCount := 0
	failedCount := 0
	var errors []string

	for _, nodeData := range importData {
		name, _ := nodeData["name"].(string)
		address, _ := nodeData["address"].(string)
		
		if name == "" || address == "" {
			failedCount++
			errors = append(errors, fmt.Sprintf("节点数据缺少必填字段"))
			continue
		}

		var existingNode models.Node
		if err := database.DB.Unscoped().Where("name = ?", name).First(&existingNode).Error; err == nil {
			failedCount++
			errors = append(errors, fmt.Sprintf("节点 %s 已存在", name))
			continue
		}

		autoSync, _ := nodeData["auto_sync"].(bool)
		syncInterval, _ := nodeData["sync_interval"].(float64)
		if syncInterval == 0 {
			syncInterval = 300
		}

		batchSize, _ := nodeData["batch_size"].(float64)
		if batchSize == 0 {
			batchSize = 5
		}
		
		batchInterval, _ := nodeData["batch_interval"].(float64)
		if batchInterval == 0 {
			batchInterval = 5
		}

		apiKey, _ := nodeData["api_key"].(string)
		description, _ := nodeData["description"].(string)

		node := models.Node{
			Name:          name,
			Address:       address,
			APIKey:        apiKey,
			Description:   description,
			Status:        "inactive",
			AutoSync:      autoSync,
			SyncInterval:  int(syncInterval),
			BatchSize:     int(batchSize),
			BatchInterval: int(batchInterval),
		}

		if err := database.DB.Create(&node).Error; err != nil {
			failedCount++
			errors = append(errors, fmt.Sprintf("节点 %s 创建失败: %v", name, err))
			continue
		}

		successCount++
	}

	logger.Global.Info(ctx, "批量导入节点完成",
		zap.Int("success", successCount),
		zap.Int("failed", failedCount),
		zap.String("action", "import_nodes"))
	
	c.JSON(http.StatusOK, gin.H{
		"code":          200,
		"msg":           fmt.Sprintf("导入完成：成功 %d 个，失败 %d 个", successCount, failedCount),
		"success_count": successCount,
		"failed_count":  failedCount,
		"errors":        errors,
	})
}

func BatchDeleteNodes(c *gin.Context) {
	ctx := c.Request.Context()
	
	var req struct {
		IDs []uint `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Global.Error(ctx, "批量删除节点参数解析失败",
			zap.Error(err),
			zap.String("action", "batch_delete_nodes"))
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误: " + err.Error(),
		})
		return
	}

	logger.Global.Info(ctx, "开始批量删除节点",
		zap.Int("count", len(req.IDs)),
		zap.String("action", "batch_delete_nodes"))
	
	successCount := 0
	failedCount := 0
	var failedErrors []string

	for _, nodeID := range req.IDs {
		err := database.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Unscoped().Where("node_id = ?", nodeID).Delete(&models.Container{}).Error; err != nil {
				return fmt.Errorf("删除容器数据失败: %w", err)
			}
			
			if err := tx.Unscoped().Where("node_id = ?", nodeID).Delete(&models.ContainerCache{}).Error; err != nil {
				return fmt.Errorf("删除容器缓存失败: %w", err)
			}
			
			if err := tx.Unscoped().Where("node_id = ?", nodeID).Delete(&models.NodeInfoCache{}).Error; err != nil {
				return fmt.Errorf("删除节点缓存失败: %w", err)
			}
			
			if err := tx.Unscoped().Where("node_id = ?", nodeID).Delete(&models.SyncTask{}).Error; err != nil {
				return fmt.Errorf("删除同步任务失败: %w", err)
			}
			
			if err := tx.Unscoped().Delete(&models.Node{}, nodeID).Error; err != nil {
				return fmt.Errorf("删除节点失败: %w", err)
			}
			
			return nil
		})
		
		if err != nil {
			failedCount++
			failedErrors = append(failedErrors, fmt.Sprintf("节点 %d: %s", nodeID, err.Error()))
			logger.Global.Error(ctx, "批量删除单个节点失败",
				zap.Uint("node_id", nodeID),
				zap.Error(err),
				zap.String("action", "batch_delete_nodes"))
		} else {
			successCount++
			logger.Global.Info(ctx, "批量删除单个节点成功",
				zap.Uint("node_id", nodeID),
				zap.String("action", "batch_delete_nodes"))
		}
	}

	logger.Global.Info(ctx, "批量删除节点完成",
		zap.Int("success", successCount),
		zap.Int("failed", failedCount),
		zap.String("action", "batch_delete_nodes"))
	
	response := gin.H{
		"code":          200,
		"msg":           fmt.Sprintf("批量删除完成：成功 %d 个，失败 %d 个", successCount, failedCount),
		"success_count": successCount,
		"failed_count":  failedCount,
	}
	
	if len(failedErrors) > 0 {
		response["errors"] = failedErrors
	}
	
	c.JSON(http.StatusOK, response)
}
