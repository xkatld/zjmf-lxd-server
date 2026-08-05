package services

import (
	"context"
	"fmt"
	"time"

	"lxdapi/config"
	"lxdapi/database"
	"lxdapi/models"
	"lxdapi/pkg/logger"

	"go.uber.org/zap"
)

func StartTrafficAutoResetChecker(ctx context.Context) {
	checkInterval := time.Duration(config.AppConfig.TrafficAPI.AutoReset.CheckInterval) * time.Second
	batchSize := config.AppConfig.TrafficAPI.AutoReset.BatchSize

	lc := &logger.Context{
		Action: "traffic_auto_reset_checker",
	}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Info(ctx, "启动自动流量重置检查器",
		zap.Duration("interval", checkInterval),
		zap.Int("batch_size", batchSize))

	containerOffset := 0

	for {
		select {
		case <-ctx.Done():
			logger.Global.Info(ctx, "自动流量重置检查器停止")
			return
		default:
			containerOffset = checkAndResetTraffic(containerOffset, batchSize)
			time.Sleep(checkInterval)
		}
	}
}

func checkAndResetTraffic(offset int, batchSize int) int {
	ctx := context.Background()
	lc := &logger.Context{
		Action: "check_and_reset_traffic",
	}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Debug(ctx, "开始检查需要重置流量的容器")

	var containers []models.ContainerConfig
	if err := database.DB.Find(&containers).Error; err != nil {
		logger.Global.Error(ctx, "获取容器列表失败", zap.Error(err))
		return 0
	}

	if len(containers) == 0 {
		logger.Global.Debug(ctx, "当前没有容器")
		return 0
	}

	if offset >= len(containers) {
		offset = 0
	}

	endOffset := offset + batchSize
	if endOffset > len(containers) {
		endOffset = len(containers)
	}

	batchContainers := containers[offset:endOffset]

	logger.Global.Debug(ctx, "检查流量重置批次",
		zap.Int("start", offset+1),
		zap.Int("end", endOffset),
		zap.Int("total", len(containers)))

	now := time.Now()
	resetCount := 0

	for _, container := range batchContainers {
		if shouldResetTraffic(container, now) {
			logger.Global.Info(ctx, "容器需要重置流量", zap.String("container", container.ContainerName))

			if err := resetContainerTraffic(container.ContainerName); err != nil {
				logger.Global.Error(ctx, "流量重置失败",
					zap.String("container", container.ContainerName),
					zap.Error(err))
			} else {
				logger.Global.Info(ctx, "流量重置成功", zap.String("container", container.ContainerName))
				resetCount++
			}
		}
	}

	logger.Global.Debug(ctx, "本批次检查完成", zap.Int("reset_count", resetCount))

	newOffset := endOffset
	if newOffset >= len(containers) {
		newOffset = 0
		logger.Global.Debug(ctx, "本轮所有容器检查完成，下次从头开始")
	}

	return newOffset
}

func shouldResetTraffic(container models.ContainerConfig, now time.Time) bool {
	ctx := context.Background()
	lc := &logger.Context{
		Container: container.ContainerName,
		Action:    "check_should_reset",
	}
	ctx = logger.NewContext(ctx, lc)

	baseDate := container.CreatedAt
	nextResetDate := addMonthsPHPStyle(baseDate, 1)

	for nextResetDate.Before(now) {
		nextResetDate = addMonthsPHPStyle(nextResetDate, 1)
	}

	lastResetDate := addMonthsPHPStyle(nextResetDate, -1)

	logger.Global.Debug(ctx, "容器重置时间检查",
		zap.Time("created", baseDate),
		zap.Time("last_reset", lastResetDate),
		zap.Time("next_reset", nextResetDate),
		zap.Time("now", now))

	var summary models.TrafficSummary
	err := database.DB.Where("container_name = ?", container.ContainerName).First(&summary).Error
	if err != nil {
		if now.After(lastResetDate) {
			logger.Global.Debug(ctx, "没有流量记录但已到重置时间")
			return true
		}
		return false
	}

	if summary.UpdatedAt.Before(lastResetDate) && now.After(lastResetDate) {
		logger.Global.Debug(ctx, "流量记录过期，需要重置", zap.Time("updated", summary.UpdatedAt))
		return true
	}

	return false
}

func addMonthsPHPStyle(date time.Time, months int) time.Time {
	return date.AddDate(0, months, 0)
}

func resetContainerTraffic(containerName string) error {
	ctx := context.Background()
	lc := &logger.Context{
		Container: containerName,
		Action:    "reset_container_traffic",
	}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Info(ctx, "开始重置容器流量")

	if err := ResetTrafficStats(containerName); err != nil {
		return fmt.Errorf("重置流量统计失败: %v", err)
	}

	task := &models.Task{
		ContainerName: containerName,
		Action:        "start",
		Status:        models.TaskQueued,
		Priority:      5,
		QueuedAt:      time.Now(),
		MaxRetries:    config.AppConfig.TaskQueue.MaxRetries,
		Steps:         []models.TaskStep{},
	}
	if err := database.DB.Create(task).Error; err != nil {
		logger.Global.Error(ctx, "创建开机任务失败", zap.Error(err))
	} else {
		logger.Global.Info(ctx, "流量重置完成，开机任务已提交", zap.Uint("task_id", task.ID))
	}

	return nil
}
