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
	"github.com/gin-gonic/gin"
)
// CreateConsoleToken 创建控制台令牌
// @Summary 创建控制台令牌
// @Description 为指定容器创建Web控制台访问令牌
// @Tags 容器管理
// @Accept json
// @Produce json
// @Param body body object true "容器信息(hostname, node_id)"
// @Success 200 {object} map[string]interface{} "成功返回令牌和控制台URL"
// @Failure 400 {object} map[string]interface{} "参数错误"
// @Failure 404 {object} map[string]interface{} "节点不存在"
// @Failure 500 {object} map[string]interface{} "创建失败"
// @Router /api/console/create-token [post]
func CreateConsoleToken(c *gin.Context) {
	var req struct {
		Hostname string `json:"hostname" binding:"required"`
		NodeID   uint   `json:"node_id" binding:"required"`
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
	tokenReq := map[string]interface{}{
		"hostname":   req.Hostname,
		"user_id":    1, 
		"service_id": 0,
		"server_ip":  c.ClientIP(),
		"expires_in": 3600, 
	}
	reqBody, _ := json.Marshal(tokenReq)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	url := fmt.Sprintf("%s/api/console/create-token", node.Address)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "创建请求失败: " + err.Error(),
		})
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("apikey", node.APIKey)
	resp, err := client.Do(httpReq)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code": 500,
			"msg":  "调用节点API失败: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	if result["code"] != float64(200) {
		c.JSON(http.StatusOK, result)
		return
	}
	data := result["data"].(map[string]interface{})
	token := data["token"].(string)
	consoleURL := fmt.Sprintf("%s/console?token=%s", node.Address, token)
	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "success",
		"data": gin.H{
			"token":       token,
			"console_url": consoleURL,
			"node_url":    node.Address,
			"hostname":    req.Hostname,
		},
	})
}
