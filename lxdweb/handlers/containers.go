package handlers
import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"lxdweb/database"
	"lxdweb/models"
	"net/http"
	"time"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
)
// ContainersPage 容器管理页面
// @Summary 容器管理页面
// @Description 显示容器管理页面
// @Tags 容器管理
// @Produce html
// @Success 200 {string} string "HTML页面"
// @Router /containers [get]
func ContainersPage(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("username")
	c.HTML(http.StatusOK, "containers.html", gin.H{
		"title":    "容器管理 - LXD管理后台",
		"username": username,
	})
}
// GetContainers 获取容器列表
// @Summary 获取容器列表
// @Description 从本地数据库获取所有容器缓存信息
// @Tags 容器管理
// @Produce json
// @Success 200 {object} map[string]interface{} "成功返回容器列表"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Router /api/containers [get]
func GetContainers(c *gin.Context) {
	var containers []models.ContainerCache
	database.DB.Order("node_id ASC, hostname ASC").Find(&containers)
	
	allContainers := make([]map[string]interface{}, 0, len(containers))
	for _, container := range containers {
		allContainers = append(allContainers, map[string]interface{}{
			"node_id":       container.NodeID,
			"node_name":     container.NodeName,
			"hostname":      container.Hostname,
			"status":        container.Status,
			"ipv4":          container.IPv4,
			"ipv6":          container.IPv6,
			"image":         container.Image,
			"cpus":          container.CPUs,
			"memory":        container.Memory,
			"disk":          container.Disk,
			"traffic_limit": container.TrafficLimit,
			"cpu_usage":     container.CPUUsage,
			"memory_usage":  container.MemoryUsage,
			"memory_total":  container.MemoryTotal,
			"disk_usage":    container.DiskUsage,
			"disk_total":    container.DiskTotal,
			"traffic_total": container.TrafficTotal,
			"traffic_in":    container.TrafficIn,
			"traffic_out":   container.TrafficOut,
			"last_sync":     container.LastSync,
		})
	}
	
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": allContainers,
	})
}

// GetContainersFromCache 从本地数据库缓存获取容器列表
// @Summary 从本地数据库缓存获取容器列表
// @Description 从本地数据库缓存获取容器列表，支持按node_id筛选
// @Tags 容器管理
// @Produce json
// @Param node_id query string false "节点ID"
// @Success 200 {object} map[string]interface{} "成功返回容器列表"
// @Failure 401 {object} map[string]interface{} "未登录"
// @Router /api/containers/cache [get]
func GetContainersFromCache(c *gin.Context) {
	session := sessions.Default(c)
	username := session.Get("username")
	if username == nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"code": 401,
			"msg":  "未登录",
		})
		return
	}

	// 从本地数据库缓存查询
	var containers []models.ContainerCache
	query := database.DB.Order("node_id ASC, hostname ASC")
	
	// 支持按 node_id 筛选
	if nodeID := c.Query("node_id"); nodeID != "" {
		query = query.Where("node_id = ?", nodeID)
	}
	
	if err := query.Find(&containers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "查询失败",
			"data": []interface{}{},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "查询成功",
		"data": containers,
	})
}
// GetContainerDetail 获取容器详细信息
// @Summary 获取容器详细信息
// @Description 获取指定容器的详细信息
// @Tags 容器管理
// @Produce json
// @Param name path string true "容器名称"
// @Param node_id query string true "节点ID"
// @Success 200 {object} map[string]interface{} "成功返回容器详情"
// @Failure 404 {object} map[string]interface{} "节点或容器不存在"
// @Router /api/containers/{name} [get]
func GetContainerDetail(c *gin.Context) {
	name := c.Param("name")
	nodeID := c.Query("node_id")
	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "节点不存在",
		})
		return
	}
	detail := fetchContainerDetail(node, name)
	if detail == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "容器不存在",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": detail,
	})
}
// StartContainer 启动容器
// @Summary 启动容器
// @Description 启动指定的容器
// @Tags 容器管理
// @Produce json
// @Param name path string true "容器名称"
// @Param node_id query string true "节点ID"
// @Success 200 {object} map[string]interface{} "启动成功"
// @Failure 404 {object} map[string]interface{} "节点不存在"
// @Router /api/containers/{name}/start [post]
func StartContainer(c *gin.Context) {
	name := c.Param("name")
	nodeID := c.Query("node_id")
	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "节点不存在",
		})
		return
	}
	result := callNodeAPI(node, "GET", "/api/boot?hostname="+name, nil)
	if result["code"] == float64(200) {
		time.Sleep(1 * time.Second)
		callNodeAPI(node, "GET", fmt.Sprintf("/api/info?hostname=%s", name), nil)
	}
	c.JSON(http.StatusOK, result)
}
// StopContainer 停止容器
// @Summary 停止容器
// @Description 停止指定的容器
// @Tags 容器管理
// @Produce json
// @Param name path string true "容器名称"
// @Param node_id query string true "节点ID"
// @Success 200 {object} map[string]interface{} "停止成功"
// @Failure 404 {object} map[string]interface{} "节点不存在"
// @Router /api/containers/{name}/stop [post]
func StopContainer(c *gin.Context) {
	name := c.Param("name")
	nodeID := c.Query("node_id")
	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "节点不存在",
		})
		return
	}
	result := callNodeAPI(node, "GET", "/api/stop?hostname="+name, nil)
	if result["code"] == float64(200) {
		time.Sleep(1 * time.Second)
		callNodeAPI(node, "GET", fmt.Sprintf("/api/info?hostname=%s", name), nil)
	}
	c.JSON(http.StatusOK, result)
}
// RestartContainer 重启容器
// @Summary 重启容器
// @Description 重启指定的容器
// @Tags 容器管理
// @Produce json
// @Param name path string true "容器名称"
// @Param node_id query string true "节点ID"
// @Success 200 {object} map[string]interface{} "重启成功"
// @Failure 404 {object} map[string]interface{} "节点不存在"
// @Router /api/containers/{name}/restart [post]
func RestartContainer(c *gin.Context) {
	name := c.Param("name")
	nodeID := c.Query("node_id")
	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "节点不存在",
		})
		return
	}
	result := callNodeAPI(node, "GET", "/api/reboot?hostname="+name, nil)
	if result["code"] == float64(200) {
		time.Sleep(2 * time.Second)
		callNodeAPI(node, "GET", fmt.Sprintf("/api/info?hostname=%s", name), nil)
	}
	c.JSON(http.StatusOK, result)
}
// DeleteContainer 删除容器
// @Summary 删除容器
// @Description 删除指定的容器并清理数据库缓存
// @Tags 容器管理
// @Produce json
// @Param name path string true "容器名称"
// @Param node_id query string true "节点ID"
// @Success 200 {object} map[string]interface{} "删除成功"
// @Failure 404 {object} map[string]interface{} "节点不存在"
// @Router /api/containers/{name}/delete [post]
func DeleteContainer(c *gin.Context) {
	name := c.Param("name")
	nodeID := c.Query("node_id")
	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "节点不存在",
		})
		return
	}
	result := callNodeAPI(node, "GET", "/api/delete?hostname="+name, nil)
	if result["code"] == float64(200) {
		database.DB.Unscoped().Where("node_id = ? AND hostname = ?", node.ID, name).Delete(&models.Container{})
		database.DB.Unscoped().Where("node_id = ? AND hostname = ?", node.ID, name).Delete(&models.ContainerCache{})
	}
	c.JSON(http.StatusOK, result)
}

// RefreshSingleContainer 刷新单个容器信息
// @Summary 刷新单个容器信息
// @Description 实时获取指定容器的最新信息并更新缓存
// @Tags 容器管理
// @Produce json
// @Param name path string true "容器名称"
// @Param node_id query string true "节点ID"
// @Success 200 {object} map[string]interface{} "刷新成功"
// @Failure 404 {object} map[string]interface{} "节点不存在"
// @Router /api/containers/{name}/refresh [post]
func RefreshSingleContainer(c *gin.Context) {
	name := c.Param("name")
	nodeID := c.Query("node_id")
	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "节点不存在",
		})
		return
	}
	result := callNodeAPI(node, "GET", fmt.Sprintf("/api/info?hostname=%s", name), nil)
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "刷新成功",
		"data": result,
	})
}

// ReinstallContainer 重装容器系统
// @Summary 重装容器系统
// @Description 重装指定容器的操作系统
// @Tags 容器管理
// @Accept json
// @Produce json
// @Param name path string true "容器名称"
// @Param body body object true "重装参数"
// @Success 200 {object} map[string]interface{} "重装成功"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 404 {object} map[string]interface{} "节点不存在"
// @Router /api/containers/{name}/reinstall [post]
func ReinstallContainer(c *gin.Context) {
	name := c.Param("name")
	
	var req struct {
		NodeID       uint   `json:"node_id" binding:"required"`
		Image        string `json:"image" binding:"required"`
		Password     string `json:"password" binding:"required"`
		CPUs         int    `json:"cpus"`
		Memory       string `json:"memory"`
		Disk         string `json:"disk"`
		Ingress      string `json:"ingress"`
		Egress       string `json:"egress"`
		TrafficLimit int    `json:"traffic_limit"`
		AllowNesting bool   `json:"allow_nesting"`
		MemorySwap   bool   `json:"memory_swap"`
		MaxProcesses int    `json:"max_processes"`
		CPUAllowance string `json:"cpu_allowance"`
		DiskIOLimit  string `json:"disk_io_limit"`
		Privileged   bool   `json:"privileged"`
		EnableLXCFS  bool   `json:"enable_lxcfs"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误: " + err.Error(),
		})
		return
	}

	var node models.Node
	if err := database.DB.First(&node, req.NodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "节点不存在",
		})
		return
	}

	reinstallData := map[string]interface{}{
		"hostname":      name,
		"system":        req.Image,
		"password":      req.Password,
		"cpus":          req.CPUs,
		"memory":        req.Memory,
		"disk":          req.Disk,
		"ingress":       req.Ingress,
		"egress":        req.Egress,
		"traffic_limit": req.TrafficLimit,
		"allow_nesting": req.AllowNesting,
		"memory_swap":   req.MemorySwap,
		"max_processes": req.MaxProcesses,
		"cpu_allowance": req.CPUAllowance,
		"privileged":    req.Privileged,
		"enable_lxcfs":  req.EnableLXCFS,
	}
	
	if req.DiskIOLimit != "" {
		reinstallData["disk_io_limit"] = req.DiskIOLimit
	}

	result := callNodeAPI(node, "POST", "/api/reinstall", reinstallData)
	if result["code"] == float64(200) {
		time.Sleep(2 * time.Second)
		callNodeAPI(node, "GET", fmt.Sprintf("/api/info?hostname=%s", name), nil)
	}
	c.JSON(http.StatusOK, result)
}

// ResetContainerPassword 重置容器密码
// @Summary 重置容器密码
// @Description 重置指定容器的root密码
// @Tags 容器管理
// @Accept json
// @Produce json
// @Param name path string true "容器名称"
// @Param body body object true "密码参数"
// @Success 200 {object} map[string]interface{} "重置成功"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 404 {object} map[string]interface{} "节点不存在"
// @Router /api/containers/{name}/password [post]
func ResetContainerPassword(c *gin.Context) {
	name := c.Param("name")
	
	var req struct {
		NodeID   uint   `json:"node_id" binding:"required"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误: " + err.Error(),
		})
		return
	}

	var node models.Node
	if err := database.DB.First(&node, req.NodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "节点不存在",
		})
		return
	}

	passwordData := map[string]interface{}{
		"hostname": name,
		"password": req.Password,
	}

	result := callNodeAPI(node, "POST", "/api/password", passwordData)
	c.JSON(http.StatusOK, result)
}

// SuspendContainer 暂停容器
// @Summary 暂停容器
// @Description 暂停指定的容器（Frozen状态）
// @Tags 容器管理
// @Produce json
// @Param name path string true "容器名称"
// @Param node_id query string true "节点ID"
// @Success 200 {object} map[string]interface{} "暂停成功"
// @Failure 404 {object} map[string]interface{} "节点不存在"
// @Router /api/containers/{name}/suspend [post]
func SuspendContainer(c *gin.Context) {
	name := c.Param("name")
	nodeID := c.Query("node_id")
	
	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "节点不存在",
		})
		return
	}
	
	result := callNodeAPI(node, "GET", "/api/suspend?hostname="+name, nil)
	if result["code"] == float64(200) {
		time.Sleep(1 * time.Second)
		callNodeAPI(node, "GET", fmt.Sprintf("/api/info?hostname=%s", name), nil)
	}
	c.JSON(http.StatusOK, result)
}

// UnsuspendContainer 恢复容器
// @Summary 恢复容器
// @Description 恢复被暂停的容器
// @Tags 容器管理
// @Produce json
// @Param name path string true "容器名称"
// @Param node_id query string true "节点ID"
// @Success 200 {object} map[string]interface{} "恢复成功"
// @Failure 404 {object} map[string]interface{} "节点不存在"
// @Router /api/containers/{name}/unsuspend [post]
func UnsuspendContainer(c *gin.Context) {
	name := c.Param("name")
	nodeID := c.Query("node_id")
	
	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "节点不存在",
		})
		return
	}
	
	result := callNodeAPI(node, "GET", "/api/unsuspend?hostname="+name, nil)
	if result["code"] == float64(200) {
		time.Sleep(1 * time.Second)
		callNodeAPI(node, "GET", fmt.Sprintf("/api/info?hostname=%s", name), nil)
	}
	c.JSON(http.StatusOK, result)
}

// ResetContainerTraffic 重置容器流量
// @Summary 重置容器流量
// @Description 重置指定容器的流量统计
// @Tags 容器管理
// @Produce json
// @Param name path string true "容器名称"
// @Param node_id query string true "节点ID"
// @Success 200 {object} map[string]interface{} "重置成功"
// @Failure 404 {object} map[string]interface{} "节点不存在"
// @Router /api/containers/{name}/traffic/reset [post]
func ResetContainerTraffic(c *gin.Context) {
	name := c.Param("name")
	nodeID := c.Query("node_id")
	
	var node models.Node
	if err := database.DB.First(&node, nodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "节点不存在",
		})
		return
	}
	
	result := callNodeAPI(node, "POST", "/api/traffic/reset?hostname="+name, nil)
	c.JSON(http.StatusOK, result)
}
// CreateContainer 创建容器
// @Summary 创建容器
// @Description 在指定节点创建新的LXD容器
// @Tags 容器管理
// @Accept json
// @Produce json
// @Param body body object true "容器配置参数"
// @Success 200 {object} map[string]interface{} "创建成功"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 404 {object} map[string]interface{} "节点不存在"
// @Router /api/containers/create [post]
func CreateContainer(c *gin.Context) {
	var req struct {
		NodeID        uint   `json:"node_id" binding:"required"`
		Hostname      string `json:"hostname" binding:"required"`
		Password      string `json:"password" binding:"required"`
		Image         string `json:"image" binding:"required"`
		CPUs          int    `json:"cpus"`
		Memory        string `json:"memory"`
		Disk          string `json:"disk"`
		Ingress       string `json:"ingress"`
		Egress        string `json:"egress"`
		TrafficLimit  int    `json:"traffic_limit"`
		AllowNesting  bool   `json:"allow_nesting"`
		MemorySwap    bool   `json:"memory_swap"`
		MaxProcesses  int    `json:"max_processes"`
		CPUAllowance  string `json:"cpu_allowance"`
		DiskIOLimit   string `json:"disk_io_limit"`
		Privileged    bool   `json:"privileged"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code": 400,
			"msg":  "参数错误: " + err.Error(),
		})
		return
	}

	var node models.Node
	if err := database.DB.First(&node, req.NodeID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code": 404,
			"msg":  "节点不存在",
		})
		return
	}

	if req.CPUs == 0 {
		req.CPUs = 1
	}
	if req.Memory == "" {
		req.Memory = "512MB"
	}
	if req.Disk == "" {
		req.Disk = "10GB"
	}
	if req.Ingress == "" {
		req.Ingress = "100Mbit"
	}
	if req.Egress == "" {
		req.Egress = "100Mbit"
	}
	if req.MaxProcesses == 0 {
		req.MaxProcesses = 512
	}
	if req.CPUAllowance == "" {
		req.CPUAllowance = "100%"
	}

	createData := map[string]interface{}{
		"hostname":       req.Hostname,
		"password":       req.Password,
		"image":          req.Image,
		"cpus":           req.CPUs,
		"memory":         req.Memory,
		"disk":           req.Disk,
		"ingress":        req.Ingress,
		"egress":         req.Egress,
		"traffic_limit":  req.TrafficLimit,
		"allow_nesting":  req.AllowNesting,
		"memory_swap":    req.MemorySwap,
		"max_processes":  req.MaxProcesses,
		"cpu_allowance":  req.CPUAllowance,
		"privileged":     req.Privileged,
	}
	if req.DiskIOLimit != "" {
		createData["disk_io_limit"] = req.DiskIOLimit
	}

	result := callNodeAPI(node, "POST", "/api/create", createData)
	if result["code"] == float64(200) {
		time.Sleep(2 * time.Second)
		callNodeAPI(node, "GET", fmt.Sprintf("/api/info?hostname=%s", req.Hostname), nil)
	}
	c.JSON(http.StatusOK, result)
}
func fetchContainersFromNode(node models.Node) []map[string]interface{} {
	result := callNodeAPI(node, "GET", "/api/list", nil)
	if result["code"] != float64(200) {
		return []map[string]interface{}{}
	}
	data, ok := result["data"].([]interface{})
	if !ok {
		return []map[string]interface{}{}
	}
	containers := make([]map[string]interface{}, 0, len(data))
	for _, item := range data {
		if container, ok := item.(map[string]interface{}); ok {
			hostname, _ := container["hostname"].(string)
			if hostname != "" {
				detailResult := callNodeAPI(node, "GET", fmt.Sprintf("/api/info?hostname=%s", hostname), nil)
				if detailResult["code"] == float64(200) {
					if detailData, ok := detailResult["data"].(map[string]interface{}); ok {
						if cpuUsage, ok := detailData["cpu_percent"].(float64); ok {
							container["cpu_usage"] = cpuUsage
						} else if cpuUsage, ok := detailData["cpu_usage"].(float64); ok {
							container["cpu_usage"] = cpuUsage
						}
						if memUsageRaw, ok := detailData["memory_usage_raw"].(float64); ok {
							container["memory_usage"] = uint64(memUsageRaw)
						}
						if memTotal, ok := detailData["memory"].(float64); ok {
							container["memory_total"] = uint64(memTotal * 1024 * 1024) 
						}
						if diskUsageRaw, ok := detailData["disk_usage_raw"].(float64); ok {
							container["disk_usage"] = uint64(diskUsageRaw)
						}
						if diskTotal, ok := detailData["disk"].(float64); ok {
							container["disk_total"] = uint64(diskTotal * 1024 * 1024) 
						}
						if trafficRaw, ok := detailData["traffic_usage_raw"].(float64); ok {
							container["traffic_total"] = uint64(trafficRaw)
							container["traffic_in"] = uint64(trafficRaw * 0.5)
							container["traffic_out"] = uint64(trafficRaw * 0.5)
						}
					}
				}
			}
			containers = append(containers, container)
		}
	}
	return containers
}
func fetchContainerDetail(node models.Node, name string) map[string]interface{} {
	result := callNodeAPI(node, "GET", fmt.Sprintf("/api/info?hostname=%s", name), nil)
	if result["code"] == float64(200) {
		if data, ok := result["data"].(map[string]interface{}); ok {
			return data
		}
	}
	return nil
}
func callNodeAPI(node models.Node, method, path string, data interface{}) map[string]interface{} {
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	var body io.Reader
	if data != nil {
		jsonData, _ := json.Marshal(data)
		body = bytes.NewBuffer(jsonData)
	}
	req, err := http.NewRequest(method, node.Address+path, body)
	if err != nil {
		return map[string]interface{}{
			"code": 500,
			"msg":  "请求创建失败: " + err.Error(),
		}
	}
	if node.APIKey != "" {
		req.Header.Set("apikey", node.APIKey)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return map[string]interface{}{
			"code": 500,
			"msg":  "请求失败: " + err.Error(),
		}
	}
	defer resp.Body.Close()
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return map[string]interface{}{
			"code": 500,
			"msg":  "响应解析失败: " + err.Error(),
		}
	}
	return result
}
