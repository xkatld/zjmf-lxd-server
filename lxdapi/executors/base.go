package executors

import (
	"context"
	"time"

	"lxdapi/errors"
	"lxdapi/models"
	pkglogger "lxdapi/pkg/logger"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type TaskExecutor interface {
	Execute() error
	GetSteps() []models.TaskStep
	GetErrorCode() int
	GetFailedStep() *models.TaskStep
}

type BaseExecutor struct {
	DB            *gorm.DB
	Logger        *zap.Logger
	Task          *models.Task
	Steps         []models.TaskStep
	CurrentStep   *models.TaskStep
	ErrorCode     int
	ContainerName string
	Data          map[string]interface{}
	Context       context.Context
}

func NewBaseExecutor(db *gorm.DB, logger *zap.Logger, task *models.Task) *BaseExecutor {
	lc := &pkglogger.Context{
		TraceID:   task.TraceID,
		TaskID:    task.ID,
		Container: task.ContainerName,
		Action:    task.Action,
		ClientIP:  task.ClientIP,
	}
	ctx := pkglogger.NewContext(context.Background(), lc)

	return &BaseExecutor{
		DB:            db,
		Logger:        logger,
		Task:          task,
		Steps:         []models.TaskStep{},
		ErrorCode:     0,
		ContainerName: task.ContainerName,
		Data:          make(map[string]interface{}),
		Context:       ctx,
	}
}

func (e *BaseExecutor) StartStep(stepID, stepName string) {
	now := time.Now()
	e.CurrentStep = &models.TaskStep{
		StepID:    stepID,
		StepName:  stepName,
		Status:    "running",
		ErrorCode: 0,
		Message:   "",
		StartTime: now,
	}

	e.Logger.Info("Task step started",
		zap.Uint("task_id", e.Task.ID),
		zap.String("container", e.ContainerName),
		zap.String("action", e.Task.Action),
		zap.String("step_id", stepID),
		zap.String("step_name", stepName),
	)
}

func (e *BaseExecutor) CompleteStep(success bool, errorCode int, message string) {
	if e.CurrentStep == nil {
		return
	}

	now := time.Now()
	e.CurrentStep.EndTime = now
	e.CurrentStep.Duration = now.Sub(e.CurrentStep.StartTime).Milliseconds()
	e.CurrentStep.ErrorCode = errorCode
	e.CurrentStep.Message = message
	e.CurrentStep.Suggestion = errors.GetSuggestion(errorCode)

	if success {
		e.CurrentStep.Status = "success"
	} else {
		e.CurrentStep.Status = "failed"
		e.ErrorCode = errorCode
	}

	e.Steps = append(e.Steps, *e.CurrentStep)

	logLevel := zap.InfoLevel
	if !success {
		logLevel = zap.ErrorLevel
	}

	e.Logger.Log(logLevel, "Task step completed",
		zap.Uint("task_id", e.Task.ID),
		zap.String("container", e.ContainerName),
		zap.String("action", e.Task.Action),
		zap.String("step_id", e.CurrentStep.StepID),
		zap.String("step_name", e.CurrentStep.StepName),
		zap.String("status", e.CurrentStep.Status),
		zap.Int("error_code", errorCode),
		zap.String("message", message),
		zap.String("suggestion", e.CurrentStep.Suggestion),
		zap.Int64("duration_ms", e.CurrentStep.Duration),
	)

	e.CurrentStep = nil
}

func (e *BaseExecutor) GetSteps() []models.TaskStep {
	return e.Steps
}

func (e *BaseExecutor) GetErrorCode() int {
	return e.ErrorCode
}

func (e *BaseExecutor) GetFailedStep() *models.TaskStep {
	for i := range e.Steps {
		if e.Steps[i].Status == "failed" {
			return &e.Steps[i]
		}
	}
	return nil
}

func (e *BaseExecutor) LogInfo(msg string, fields ...zap.Field) {
	baseFields := []zap.Field{
		zap.Uint("task_id", e.Task.ID),
		zap.String("container", e.ContainerName),
		zap.String("action", e.Task.Action),
	}
	e.Logger.Info(msg, append(baseFields, fields...)...)
}

func (e *BaseExecutor) LogError(msg string, fields ...zap.Field) {
	baseFields := []zap.Field{
		zap.Uint("task_id", e.Task.ID),
		zap.String("container", e.ContainerName),
		zap.String("action", e.Task.Action),
	}
	e.Logger.Error(msg, append(baseFields, fields...)...)
}

func (e *BaseExecutor) updateCacheStatus(status string) {
	if e.ContainerName == "" {
		return
	}

	result := e.DB.Model(&models.ContainerInfoCache{}).
		Where("hostname = ?", e.ContainerName).
		Updates(map[string]interface{}{
			"status":      status,
			"last_update": time.Now(),
		})

	if result.Error != nil {
		e.LogError("Failed to update container cache status",
			zap.String("status", status),
			zap.Error(result.Error))
		return
	}

	if result.RowsAffected == 0 {
		e.LogInfo("Cache record not found, creating new one",
			zap.String("status", status))
		
		cache := models.ContainerInfoCache{
			Hostname:   e.ContainerName,
			Status:     status,
			LastUpdate: time.Now(),
		}
		if err := e.DB.Create(&cache).Error; err != nil {
			e.LogError("Failed to create container cache record",
				zap.String("status", status),
				zap.Error(err))
		} else {
			e.LogInfo("Container cache record created",
				zap.String("status", status))
		}
	} else {
		e.LogInfo("Container cache status updated",
			zap.String("status", status),
			zap.Int64("rows_affected", result.RowsAffected))
	}
}
