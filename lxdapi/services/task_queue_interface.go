package services

import (
	"lxdapi/models"
)

type TaskQueueBackend interface {
	EnqueueTask(task *models.Task) error
	FetchTask() (*models.Task, error)
	UpdateTaskStatus(taskID uint, status string, errorMsg string, errorCode int, failedFunc string, steps []models.TaskStep) error
	IncrementRetryCount(taskID uint) error
	LoadPendingTasks() ([]models.Task, error)
	Close() error
}

type TaskQueueMetrics struct {
	TotalProcessed int64
	TotalSuccess   int64
	TotalFailed    int64
	ActiveWorkers  int
}

