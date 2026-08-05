package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"lxdapi/config"
	"lxdapi/database"
	"lxdapi/models"
	"lxdapi/pkg/logger"

	"go.uber.org/zap"
)

func StartTrafficLimitChecker(ctx context.Context) {
	checkInterval := time.Duration(config.AppConfig.TrafficAPI.LimitCheck.CheckInterval) * time.Second
	batchSize := config.AppConfig.TrafficAPI.LimitCheck.BatchSize
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	lc := &logger.Context{
		Action: "traffic_limit_checker",
	}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Info(ctx, "流量限制检测服务启动",
		zap.Duration("interval", checkInterval),
		zap.Int("batch_size", batchSize))

	containerOffset := 0

	for {
		select {
		case <-ctx.Done():
			logger.Global.Info(ctx, "流量限制检测服务停止")
			return
		case <-ticker.C:
			containerOffset = checkAllContainersTrafficLimit(containerOffset, batchSize)
		}
	}
}

func checkAllContainersTrafficLimit(offset int, batchSize int) int {
	ctx := context.Background()
	lc := &logger.Context{
		Action: "check_traffic_limits",
	}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Debug(ctx, "开始检查容器流量限制")

	var containers []models.ContainerConfig
	if err := database.DB.Where("traffic_limit > 0 AND status = ?", "Active").Find(&containers).Error; err != nil {
		logger.Global.Error(ctx, "获取容器配置失败", zap.Error(err))
		return 0
	}

	if len(containers) == 0 {
		logger.Global.Debug(ctx, "当前没有需要检查流量限制的容器")
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

	logger.Global.Debug(ctx, "检查流量限制批次",
		zap.Int("start", offset+1),
		zap.Int("end", endOffset),
		zap.Int("total", len(containers)))

	for _, container := range batchContainers {
		checkContainerTrafficLimit(container)
	}

	newOffset := endOffset
	if newOffset >= len(containers) {
		newOffset = 0
		logger.Global.Debug(ctx, "本轮所有容器检查完成，下次从头开始")
	}

	return newOffset
}

func checkContainerTrafficLimit(container models.ContainerConfig) {
	ctx := context.Background()
	lc := &logger.Context{
		Container: container.ContainerName,
		Action:    "check_traffic_limit",
	}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Debug(ctx, "检查容器流量限制", zap.Int64("limit_gb", int64(container.TrafficLimit)))

	monthlyUsageBytes, err := GetContainerMonthlyTraffic(container.ContainerName)
	if err != nil {
		logger.Global.Error(ctx, "获取月流量失败", zap.Error(err))
		return
	}

	monthlyUsageGB := float64(monthlyUsageBytes) / (1024 * 1024 * 1024)
	limitGB := float64(container.TrafficLimit)

	logger.Global.Debug(ctx, "容器流量使用情况",
		zap.Float64("used_gb", monthlyUsageGB),
		zap.Float64("limit_gb", limitGB))

	if monthlyUsageGB >= limitGB {
		logger.Global.Warn(ctx, "容器流量超限，准备暂停")

		if err := suspendContainerForTrafficLimit(container.ContainerName); err != nil {
			logger.Global.Error(ctx, "暂停容器失败", zap.Error(err))
		} else {
			logger.Global.Info(ctx, "容器因流量超限被暂停")
		}
	}
}

func suspendContainerForTrafficLimit(containerName string) error {
	ctx := context.Background()
	lc := &logger.Context{
		Container: containerName,
		Action:    "suspend_for_traffic",
	}
	ctx = logger.NewContext(ctx, lc)

	status, err := GetContainerStatus(containerName)
	if err != nil {
		return fmt.Errorf("获取容器状态失败: %v", err)
	}

	statusUpper := strings.ToUpper(status)

	if statusUpper == "FROZEN" {
		logger.Global.Debug(ctx, "容器已经是暂停状态，跳过")
		return nil
	}

	if statusUpper != "RUNNING" {
		logger.Global.Debug(ctx, "容器不是运行状态，跳过暂停", zap.String("status", status))
		return nil
	}

	logger.Global.Info(ctx, "执行暂停容器", zap.String("status", status))
	ctx, cancel := context.WithTimeout(context.Background(), config.GetLXCTimeout())
	defer cancel()

	_, err = RunLXCCommand(ctx, containerName, "pause")
	return err
}

func GetContainerMonthlyTraffic(containerName string) (int64, error) {
	var summary models.TrafficSummary
	err := database.DB.Where("container_name = ?", containerName).First(&summary).Error
	if err != nil {
		return 0, fmt.Errorf("获取容器流量汇总失败: %v", err)
	}

	monthlyTraffic := int64(summary.MonthlyReceived + summary.MonthlySent)
	return monthlyTraffic, nil
}
