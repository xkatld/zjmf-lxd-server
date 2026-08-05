package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"lxdapi/config"
	"lxdapi/database"
	"lxdapi/models"
	"lxdapi/pkg/logger"

	"go.uber.org/zap"
)

type NetworkCounters struct {
	BytesReceived   uint64 `json:"bytes_received"`
	BytesSent       uint64 `json:"bytes_sent"`
	PacketsReceived uint64 `json:"packets_received"`
	PacketsSent     uint64 `json:"packets_sent"`
}

type NetworkInterface struct {
	Counters NetworkCounters `json:"counters"`
}

type ContainerNetworkState struct {
	Eth0 NetworkInterface `json:"eth0"`
}

type ContainerState struct {
	Network ContainerNetworkState `json:"network"`
}

func GetContainerTrafficStats(containerName string) (*NetworkCounters, error) {
	cmd := exec.Command("lxc", "query", fmt.Sprintf("/1.0/instances/%s/state", containerName))
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取容器状态失败: %v", err)
	}

	var state ContainerState
	if err := json.Unmarshal(output, &state); err != nil {
		return nil, fmt.Errorf("解析容器状态失败: %v", err)
	}

	return &state.Network.Eth0.Counters, nil
}

func UpdateTrafficStats(containerName string) error {
	stats, err := GetContainerTrafficStats(containerName)
	if err != nil {
		return fmt.Errorf("获取流量数据失败: %v", err)
	}

	now := time.Now()

	var summary models.TrafficSummary
	result := database.DB.Where("container_name = ?", containerName).First(&summary)

	if result.Error != nil {
		summary = models.TrafficSummary{
			ContainerName:   containerName,
			TotalReceived:   stats.BytesReceived,
			TotalSent:       stats.BytesSent,
			TotalBytes:      stats.BytesReceived + stats.BytesSent,
			LastRecordTime:  now,
			LastReceived:    stats.BytesReceived,
			LastSent:        stats.BytesSent,
			DailyReceived:   stats.BytesReceived,
			DailySent:       stats.BytesSent,
			MonthlyReceived: stats.BytesReceived,
			MonthlySent:     stats.BytesSent,
		}

		if err := database.DB.Create(&summary).Error; err != nil {
			return fmt.Errorf("创建流量汇总记录失败: %v", err)
		}
	} else {
		var receivedInc, sentInc uint64

		if stats.BytesReceived >= summary.LastReceived {
			receivedInc = stats.BytesReceived - summary.LastReceived
		} else {
			receivedInc = stats.BytesReceived
		}

		if stats.BytesSent >= summary.LastSent {
			sentInc = stats.BytesSent - summary.LastSent
		} else {
			sentInc = stats.BytesSent
		}

		if !isSameDay(summary.LastRecordTime, now) {
			summary.DailyReceived = receivedInc
			summary.DailySent = sentInc
		} else {
			summary.DailyReceived += receivedInc
			summary.DailySent += sentInc
		}

		if !isSameMonth(summary.LastRecordTime, now) {
			summary.MonthlyReceived = receivedInc
			summary.MonthlySent = sentInc
		} else {
			summary.MonthlyReceived += receivedInc
			summary.MonthlySent += sentInc
		}

		summary.TotalReceived += receivedInc
		summary.TotalSent += sentInc
		summary.TotalBytes = summary.TotalReceived + summary.TotalSent
		summary.LastRecordTime = now
		summary.LastReceived = stats.BytesReceived
		summary.LastSent = stats.BytesSent

		if err := database.DB.Save(&summary).Error; err != nil {
			return fmt.Errorf("更新流量汇总记录失败: %v", err)
		}
	}

	return nil
}

func GetTrafficSummary(containerName string) (*models.TrafficSummary, error) {
	var summary models.TrafficSummary
	if err := database.DB.Where("container_name = ?", containerName).First(&summary).Error; err != nil {
		return nil, fmt.Errorf("获取流量汇总失败: %v", err)
	}
	return &summary, nil
}

func GetAllTrafficSummaries() ([]models.TrafficSummary, error) {
	var summaries []models.TrafficSummary
	if err := database.DB.Find(&summaries).Error; err != nil {
		return nil, fmt.Errorf("获取所有流量汇总失败: %v", err)
	}
	return summaries, nil
}

func isSameDay(t1, t2 time.Time) bool {
	y1, m1, d1 := t1.Date()
	y2, m2, d2 := t2.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func isSameMonth(t1, t2 time.Time) bool {
	y1, m1, _ := t1.Date()
	y2, m2, _ := t2.Date()
	return y1 == y2 && m1 == m2
}

func StartTrafficMonitoring(ctx context.Context) {
	interval := time.Duration(config.AppConfig.TrafficAPI.Interval) * time.Second
	batchSize := config.AppConfig.TrafficAPI.BatchSize
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	lc := &logger.Context{
		Action: "traffic_monitoring",
	}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Info(ctx, "流量监控已启动",
		zap.Duration("interval", interval),
		zap.Int("batch_size", batchSize))

	containerOffset := 0

	for {
		select {
		case <-ctx.Done():
			logger.Global.Info(ctx, "流量监控已停止")
			return
		case <-ticker.C:
			containers, err := GetRunningContainers()
			if err != nil {
				logger.Global.Error(ctx, "获取运行中容器列表失败", zap.Error(err))
				continue
			}

			if len(containers) == 0 {
				logger.Global.Debug(ctx, "当前没有运行中的容器")
				containerOffset = 0
				continue
			}

			if containerOffset >= len(containers) {
				containerOffset = 0
			}

			endOffset := containerOffset + batchSize
			if endOffset > len(containers) {
				endOffset = len(containers)
			}

			batchContainers := containers[containerOffset:endOffset]

			logger.Global.Debug(ctx, "开始更新流量统计",
				zap.Int("start", containerOffset+1),
				zap.Int("end", endOffset),
				zap.Int("total", len(containers)))

			for _, containerName := range batchContainers {
				if err := UpdateTrafficStats(containerName); err != nil {
					logger.Global.Error(ctx, "更新容器流量统计失败",
						zap.String("container", containerName),
						zap.Error(err))
				} else {
					logger.Global.Debug(ctx, "容器流量统计更新成功", zap.String("container", containerName))
				}
			}

			containerOffset = endOffset

			if containerOffset >= len(containers) {
				containerOffset = 0
				logger.Global.Debug(ctx, "本轮所有容器处理完成，下次从头开始")
			}
		}
	}
}

func GetRunningContainers() ([]string, error) {
	cmd := exec.Command("lxc", "list", "--format=csv", "--columns=n,s")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("获取容器列表失败: %v", err)
	}

	var containers []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) >= 2 {
			name := strings.TrimSpace(parts[0])
			status := strings.TrimSpace(parts[1])

			if status == "RUNNING" {
				containers = append(containers, name)
			}
		}
	}

	return containers, nil
}

func ResetTrafficStats(containerName string) error {
	ctx := context.Background()
	lc := &logger.Context{
		Container: containerName,
		Action:    "reset_traffic",
	}
	ctx = logger.NewContext(ctx, lc)

	currentStats, err := GetContainerTrafficStats(containerName)
	if err != nil {
		return fmt.Errorf("获取当前流量数据失败: %v", err)
	}

	if err := database.DB.Unscoped().Where("container_name = ?", containerName).Delete(&models.TrafficSummary{}).Error; err != nil {
		return fmt.Errorf("重置流量汇总失败: %v", err)
	}

	now := time.Now()
	summary := models.TrafficSummary{
		ContainerName:   containerName,
		TotalReceived:   0,
		TotalSent:       0,
		TotalBytes:      0,
		LastRecordTime:  now,
		LastReceived:    currentStats.BytesReceived,
		LastSent:        currentStats.BytesSent,
		DailyReceived:   0,
		DailySent:       0,
		MonthlyReceived: 0,
		MonthlySent:     0,
	}

	if err := database.DB.Create(&summary).Error; err != nil {
		return fmt.Errorf("创建重置基准记录失败: %v", err)
	}

	logger.Global.Info(ctx, "流量统计已重置",
		zap.Uint64("base_received", currentStats.BytesReceived),
		zap.Uint64("base_sent", currentStats.BytesSent))

	return nil
}
