package handlers

import (
	"net/http"
	"strconv"

	"lxdapi/database"
	"lxdapi/models"

	"github.com/gin-gonic/gin"
)

// GetTaskListHandler 获取任务列表
func GetTaskListHandler(c *gin.Context) {
	// 获取分页参数
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	// 获取筛选参数
	status := c.Query("status")
	action := c.Query("action")
	containerName := c.Query("container_name")

	// 构建查询
	query := database.DB.Model(&models.Task{}).Order("created_at DESC").Limit(limit)

	if status != "" {
		query = query.Where("status = ?", status)
	}
	if action != "" {
		query = query.Where("action = ?", action)
	}
	if containerName != "" {
		query = query.Where("container_name = ?", containerName)
	}

	// 查询任务列表
	var tasks []models.Task
	if err := query.Find(&tasks).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 500,
			"msg":  "查询任务列表失败: " + err.Error(),
			"data": nil,
		})
		return
	}

	// 转换为响应格式
	taskResponses := make([]models.TaskResponse, 0, len(tasks))
	for _, task := range tasks {
		taskResponses = append(taskResponses, models.TaskResponse{
			ID:             task.ID,
			CreatedAt:      task.CreatedAt,
			UpdatedAt:      task.UpdatedAt,
			ContainerName:  task.ContainerName,
			Action:         task.Action,
			Status:         task.Status,
			Priority:       task.Priority,
			BatchID:        task.BatchID,
			TraceID:        task.TraceID,
			ClientIP:       task.ClientIP,
			QueuedAt:       task.QueuedAt,
			StartedAt:      task.StartedAt,
			CompletedAt:    task.CompletedAt,
			RetryCount:     task.RetryCount,
			MaxRetries:     task.MaxRetries,
			Steps:          task.Steps,
			ErrorCode:      task.ErrorCode,
			ErrorMsg:       task.ErrorMsg,
			FailedFunction: task.FailedFunction,
			Log:            task.Log,
			Result:         task.Result,
			StartTime:      task.StartTime,
			EndTime:        task.EndTime,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 200,
		"msg":  "获取任务列表成功",
		"data": taskResponses,
	})
}

