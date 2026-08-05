package services

import (
	"context"
	"encoding/json"
	"time"

	"lxdapi/config"
	"lxdapi/models"
	"lxdapi/pkg/logger"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DatabaseQueue struct {
	DB *gorm.DB
}

func NewDatabaseQueue(db *gorm.DB) *DatabaseQueue {
	return &DatabaseQueue{DB: db}
}

func (dq *DatabaseQueue) EnqueueTask(task *models.Task) error {
	task.Status = models.TaskQueued
	task.QueuedAt = time.Now()
	return dq.DB.Create(task).Error
}

func (dq *DatabaseQueue) FetchTask() (*models.Task, error) {
	var task models.Task
	result := dq.DB.Where("status = ?", models.TaskQueued).
		Order("priority DESC, queued_at ASC").
		First(&task)
	
	if result.Error != nil {
		return nil, result.Error
	}
	
	updateResult := dq.DB.Model(&models.Task{}).
		Where("id = ? AND status = ?", task.ID, models.TaskQueued).
		Updates(map[string]interface{}{
			"status":     models.TaskRunning,
			"started_at": time.Now(),
		})
	
	if updateResult.Error != nil {
		return nil, updateResult.Error
	}
	
	if updateResult.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	
	return &task, nil
}

func (dq *DatabaseQueue) UpdateTaskStatus(taskID uint, status string, errorMsg string, errorCode int, failedFunc string, steps []models.TaskStep) error {
	updates := map[string]interface{}{
		"status": status,
	}

	if status == models.TaskSuccess || status == models.TaskFailed {
		updates["completed_at"] = time.Now()
	}

	if errorMsg != "" {
		updates["error_msg"] = errorMsg
	}

	if errorCode != 0 {
		updates["error_code"] = errorCode
	}

	if failedFunc != "" {
		updates["failed_function"] = failedFunc
	}

	if len(steps) > 0 {
		stepsJSON, _ := json.Marshal(steps)
		updates["steps"] = string(stepsJSON)
	}
	
	return dq.DB.Model(&models.Task{}).Where("id = ?", taskID).Updates(updates).Error
}

func (dq *DatabaseQueue) IncrementRetryCount(taskID uint) error {
	return dq.DB.Model(&models.Task{}).Where("id = ?", taskID).UpdateColumn("retry_count", gorm.Expr("retry_count + 1")).Error
}

func (dq *DatabaseQueue) LoadPendingTasks() ([]models.Task, error) {
	var tasks []models.Task
	err := dq.DB.Where("status IN ?", []string{models.TaskQueued, models.TaskRunning}).Find(&tasks).Error
	return tasks, err
}

func (dq *DatabaseQueue) Close() error {
	return nil
}

func (wp *WorkerPool) loadPendingTasks() {
	tasks, err := wp.Backend.LoadPendingTasks()
	if err != nil {
		wp.Logger.Error("Failed to load pending tasks", zap.Error(err))
		return
	}
	
	if len(tasks) == 0 {
		wp.Logger.Info("No pending tasks to clean")
		return
	}
	
	wp.Logger.Info("Found pending tasks, marking as cancelled", zap.Int("count", len(tasks)))
	
	// 将所有未完成的任务标记为cancelled，而不是重新执行
	// 这样可以避免：
	// 1. 重启后任务重复执行
	// 2. 部分完成的任务继续执行导致错误
	// 3. 旧任务堆积
	now := time.Now()
	for _, task := range tasks {
		wp.Logger.Info("Cancelling old task",
			zap.Uint("task_id", task.ID),
			zap.String("container", task.ContainerName),
			zap.String("action", task.Action),
			zap.String("old_status", task.Status),
		)
		
		errorMsg := "Task cancelled: Server restarted before task completion"
		if err := wp.Backend.UpdateTaskStatus(task.ID, models.TaskCancelled, errorMsg, 0, "", nil); err != nil {
			wp.Logger.Error("Failed to cancel task", zap.Uint("task_id", task.ID), zap.Error(err))
		}
		
		// 更新 completed_at 时间
		wp.DB.Model(&models.Task{}).Where("id = ?", task.ID).Update("completed_at", now)
	}
	
	wp.Logger.Info("Old tasks cleanup completed")
}

func StartTaskCleanupService() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		
		for range ticker.C {
			cleanupOldTasks()
		}
	}()
}

func cleanupOldTasks() {
	retentionHours := config.AppConfig.TaskStatus.RetentionHours
	if retentionHours <= 0 {
		return
	}
	
	cutoffTime := time.Now().Add(-time.Duration(retentionHours) * time.Hour)
	
	var db *gorm.DB
	if globalWorkerPool != nil && globalWorkerPool.Backend != nil {
		if dbQueue, ok := globalWorkerPool.Backend.(*DatabaseQueue); ok {
			db = dbQueue.DB
		}
	}
	
	if db == nil {
		return
	}
	
	result := db.Where("completed_at < ? AND status IN ?", cutoffTime, []string{models.TaskSuccess, models.TaskFailed}).
		Delete(&models.Task{})

	ctx := context.Background()
	lc := &logger.Context{Action: "cleanup_old_tasks"}
	ctx = logger.NewContext(ctx, lc)

	if result.Error != nil {
		logger.Global.Error(ctx, "Failed to cleanup old tasks", zap.Error(result.Error))
		return
	}

	if result.RowsAffected > 0 {
		logger.Global.Info(ctx, "Cleaned up old tasks", zap.Int64("count", result.RowsAffected))
	}
}

