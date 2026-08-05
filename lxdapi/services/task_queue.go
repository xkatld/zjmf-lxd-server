package services

import (
	"fmt"
	"sync"
	"time"

	"lxdapi/errors"
	"lxdapi/executors"
	"lxdapi/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type WorkerPool struct {
	Backend      TaskQueueBackend
	DB           *gorm.DB
	Logger       *zap.Logger
	WorkerCount  int
	PollInterval time.Duration
	StopChan     chan struct{}
	WG           sync.WaitGroup
	Metrics      *QueueMetrics
}

type QueueMetrics struct {
	mu                sync.RWMutex
	TasksQueuedTotal  int64
	TasksRunningNow   int64
	TasksSuccessTotal int64
	TasksFailedTotal  int64
	AvgDurationMs     int64
	Uptime            time.Time
}

func NewWorkerPool(backend TaskQueueBackend, db *gorm.DB, logger *zap.Logger, workerCount int, pollInterval time.Duration) *WorkerPool {
	wp := &WorkerPool{
		Backend:      backend,
		DB:           db,
		Logger:       logger,
		WorkerCount:  workerCount,
		PollInterval: pollInterval,
		StopChan:     make(chan struct{}),
		Metrics: &QueueMetrics{
			Uptime: time.Now(),
		},
	}
	
	wp.loadPendingTasks()
	
	return wp
}

func (wp *WorkerPool) Start() {
	wp.Logger.Info("Worker pool starting", zap.Int("worker_count", wp.WorkerCount), zap.Duration("poll_interval", wp.PollInterval))
	
	for i := 0; i < wp.WorkerCount; i++ {
		wp.WG.Add(1)
		go wp.worker(i)
	}
	
	wp.Logger.Info("Worker pool started", zap.Int("workers", wp.WorkerCount))
}

func (wp *WorkerPool) Stop() {
	wp.Logger.Info("Worker pool stopping")
	close(wp.StopChan)
	wp.WG.Wait()
	if wp.Backend != nil {
		wp.Backend.Close()
	}
	wp.Logger.Info("Worker pool stopped")
}

func (wp *WorkerPool) worker(workerID int) {
	defer wp.WG.Done()
	
	wp.Logger.Info("Worker started", zap.Int("worker_id", workerID))
	
	for {
		select {
		case <-wp.StopChan:
			wp.Logger.Info("Worker stopping", zap.Int("worker_id", workerID))
			return
		default:
			task, err := wp.Backend.FetchTask()
			if err != nil {
				if err != gorm.ErrRecordNotFound {
					wp.Logger.Error("Fetch task failed", zap.Error(err))
				}
				time.Sleep(wp.PollInterval)
				continue
			}
			
			if task == nil {
				time.Sleep(wp.PollInterval)
				continue
			}
			
			wp.Logger.Info("Worker picked up task",
				zap.Int("worker_id", workerID),
				zap.Uint("task_id", task.ID),
				zap.String("container", task.ContainerName),
				zap.String("action", task.Action),
			)
			
			wp.Metrics.mu.Lock()
			wp.Metrics.TasksRunningNow++
			wp.Metrics.mu.Unlock()
			
			wp.executeTask(task)
		}
	}
}

func (wp *WorkerPool) executeTask(task *models.Task) {
	startTime := time.Now()
	
	var executor executors.TaskExecutor
	var err error
	
	switch task.Action {
	case "create":
		executor, err = executors.NewCreateContainerExecutor(wp.DB, wp.Logger, task)
	case "start":
		executor = executors.NewStartContainerExecutor(wp.DB, wp.Logger, task)
	case "stop":
		executor = executors.NewStopContainerExecutor(wp.DB, wp.Logger, task)
	case "reboot", "restart":
		executor = executors.NewRebootContainerExecutor(wp.DB, wp.Logger, task)
	case "pause":
		executor = executors.NewPauseContainerExecutor(wp.DB, wp.Logger, task)
	case "resume":
		executor = executors.NewResumeContainerExecutor(wp.DB, wp.Logger, task)
	case "delete":
		executor = executors.NewDeleteContainerExecutor(wp.DB, wp.Logger, task)
	case "reinstall":
		executor, err = executors.NewReinstallContainerExecutor(wp.DB, wp.Logger, task)
	case "reset_password":
		executor, err = executors.NewResetPasswordExecutor(wp.DB, wp.Logger, task)
	default:
		err = fmt.Errorf("unknown action: %s", task.Action)
	}
	
	if err != nil {
		wp.Logger.Error("Create executor failed",
			zap.Uint("task_id", task.ID),
			zap.String("action", task.Action),
			zap.Error(err),
		)
		wp.finishTask(task, false, errors.ERR_TASK_CREATE_FAIL, err.Error(), "", nil, startTime)
		return
	}
	
	execErr := executor.Execute()

	steps := executor.GetSteps()
	errorCode := executor.GetErrorCode()
	failedStep := executor.GetFailedStep()

	success := execErr == nil
	errorMsg := ""
	failedFunc := ""
	if execErr != nil {
		errorMsg = execErr.Error()
		if failedStep != nil {
			failedFunc = failedStep.StepName
		}
	}

	wp.finishTask(task, success, errorCode, errorMsg, failedFunc, steps, startTime)
}

func (wp *WorkerPool) finishTask(task *models.Task, success bool, errorCode int, errorMsg string, failedFunc string, steps []models.TaskStep, startTime time.Time) {
	duration := time.Since(startTime)
	
	var status string
	if success {
		status = models.TaskSuccess
	} else {
		status = models.TaskFailed
		// 移除自动重试机制
		// 原因：
		// 1. LXD操作大多不是幂等的（删除、重装等）
		// 2. 自动重试可能造成更严重的问题
		// 3. 失败应该让用户检查原因后手动重试
		wp.Logger.Error("Task failed",
			zap.Uint("task_id", task.ID),
			zap.String("container", task.ContainerName),
			zap.String("action", task.Action),
			zap.Int("error_code", errorCode),
			zap.String("error_msg", errorMsg),
		)
	}
	
	if err := wp.Backend.UpdateTaskStatus(task.ID, status, errorMsg, errorCode, failedFunc, steps); err != nil {
		wp.Logger.Error("Update task status failed",
			zap.Uint("task_id", task.ID),
			zap.Error(err),
		)
	}
	
	wp.Metrics.mu.Lock()
	wp.Metrics.TasksRunningNow--
	if success {
		wp.Metrics.TasksSuccessTotal++
	} else {
		if status == models.TaskFailed {
			wp.Metrics.TasksFailedTotal++
		}
	}
	if wp.Metrics.AvgDurationMs == 0 {
		wp.Metrics.AvgDurationMs = duration.Milliseconds()
	} else {
		wp.Metrics.AvgDurationMs = (wp.Metrics.AvgDurationMs + duration.Milliseconds()) / 2
	}
	wp.Metrics.mu.Unlock()
	
	wp.Logger.Info("Task completed",
		zap.Uint("task_id", task.ID),
		zap.String("container", task.ContainerName),
		zap.String("action", task.Action),
		zap.String("status", status),
		zap.Int("error_code", errorCode),
		zap.Duration("duration", duration),
	)
}

func (wp *WorkerPool) GetMetrics() map[string]interface{} {
	wp.Metrics.mu.RLock()
	defer wp.Metrics.mu.RUnlock()
	
	var queueLength int64
	wp.DB.Model(&models.Task{}).Where("status = ?", models.TaskQueued).Count(&queueLength)
	
	return map[string]interface{}{
		"tasks_queued":         queueLength,
		"tasks_running":        wp.Metrics.TasksRunningNow,
		"tasks_success_total":  wp.Metrics.TasksSuccessTotal,
		"tasks_failed_total":   wp.Metrics.TasksFailedTotal,
		"queue_length":         queueLength,
		"worker_count":         wp.WorkerCount,
		"avg_task_duration_ms": wp.Metrics.AvgDurationMs,
		"uptime_seconds":       int64(time.Since(wp.Metrics.Uptime).Seconds()),
	}
}

var globalWorkerPool *WorkerPool

func SetGlobalWorkerPool(wp *WorkerPool) {
	globalWorkerPool = wp
}

func GetGlobalWorkerPool() *WorkerPool {
	return globalWorkerPool
}

