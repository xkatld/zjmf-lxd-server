package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"lxdapi/config"
	"lxdapi/database"
	"lxdapi/models"
	"lxdapi/pkg/logger"
	"lxdapi/utils"

	"go.uber.org/zap"
)

func GetContainerRealTimeStats(containerName string, state ...map[string]interface{}) (*models.ContainerRealTimeStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), config.GetLXCTimeout())
	defer cancel()

	lc := &logger.Context{
		Container: containerName,
		Action:    "get_realtime_stats",
	}
	ctx = logger.NewContext(ctx, lc)

	var stateData map[string]interface{}
	if len(state) > 0 && state[0] != nil {
		stateData = state[0]
		logger.Global.Debug(ctx, "复用已有state数据")
	} else {
		cmd := exec.CommandContext(ctx, "lxc", "query", fmt.Sprintf("/1.0/containers/%s/state", containerName))
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("获取容器状态失败: %v, 输出: %s", err, string(output))
		}

		if err := json.Unmarshal(output, &stateData); err != nil {
			return nil, fmt.Errorf("解析状态JSON失败: %v", err)
		}
	}

	var containerConfig models.ContainerConfig
	database.DB.Where("container_name = ?", containerName).First(&containerConfig)

	stats := &models.ContainerRealTimeStats{
		ContainerName: containerName,
		Timestamp:     time.Now().Format(time.RFC3339),
	}

	if status, ok := stateData["status"].(string); ok {
		stats.Status = status
	}
	if pid, ok := stateData["pid"].(float64); ok {
		stats.PID = int(pid)
	}
	if processes, ok := stateData["processes"].(float64); ok {
		stats.Processes = int(processes)
	}

	if stats.Status == "Running" {
		cpuUsage := getContainerCPUUsage(ctx, containerName)
		stats.CPUUsagePercent = cpuUsage
		logger.Global.Debug(ctx, "CPU使用率", zap.Float64("percent", stats.CPUUsagePercent))
	} else {
		stats.CPUUsagePercent = 0
	}

	if memory, ok := stateData["memory"].(map[string]interface{}); ok {
		if usage, ok := memory["usage"].(float64); ok {
			stats.MemoryUsedMB = usage / 1024 / 1024
		}
		if total, ok := memory["total"].(float64); ok {
			stats.MemoryTotalMB = total / 1024 / 1024
		}
		if swapUsage, ok := memory["swap_usage"].(float64); ok {
			stats.MemorySwapUsedMB = swapUsage / 1024 / 1024
		}
		if usagePeak, ok := memory["usage_peak"].(float64); ok {
			stats.MemoryUsagePeakMB = usagePeak / 1024 / 1024
		}

		if stats.MemoryTotalMB > 0 {
			stats.MemoryUsagePercent = (stats.MemoryUsedMB / stats.MemoryTotalMB) * 100
		}
	}

	if disk, ok := stateData["disk"].(map[string]interface{}); ok {
		if root, ok := disk["root"].(map[string]interface{}); ok {
			if usage, ok := root["usage"].(float64); ok {
				stats.DiskUsedMB = usage / 1024 / 1024
			}
			if total, ok := root["total"].(float64); ok && total > 0 {
				stats.DiskTotalGB = total / 1024 / 1024 / 1024
			}
		}
	}

	if containerConfig.ID > 0 && containerConfig.Disk != "" {
		diskSizeMB, err := utils.ParseSizeToMB(containerConfig.Disk)
		if err == nil {
			stats.DiskTotalGB = float64(diskSizeMB) / 1024
			logger.Global.Debug(ctx, "从数据库获取磁盘大小",
				zap.String("disk", containerConfig.Disk),
				zap.Float64("total_gb", stats.DiskTotalGB))
		}
	} else {
		if disk, ok := stateData["disk"].(map[string]interface{}); ok {
			if root, ok := disk["root"].(map[string]interface{}); ok {
				if total, ok := root["total"].(float64); ok && total > 0 {
					logger.Global.Debug(ctx, "使用state中的磁盘总大小", zap.Float64("total_gb", stats.DiskTotalGB))
				}
			}
		}
	}

	if stats.DiskTotalGB > 0 {
		stats.DiskUsagePercent = (stats.DiskUsedMB / 1024 / stats.DiskTotalGB) * 100
	}

	if network, ok := stateData["network"].(map[string]interface{}); ok {
		if eth0, ok := network["eth0"].(map[string]interface{}); ok {
			if counters, ok := eth0["counters"].(map[string]interface{}); ok {
				if bytesReceived, ok := counters["bytes_received"].(float64); ok {
					stats.NetworkRxKB = bytesReceived / 1024
				}
				if bytesSent, ok := counters["bytes_sent"].(float64); ok {
					stats.NetworkTxKB = bytesSent / 1024
				}
				if packetsReceived, ok := counters["packets_received"].(float64); ok {
					stats.NetworkRxPackets = int64(packetsReceived)
				}
				if packetsSent, ok := counters["packets_sent"].(float64); ok {
					stats.NetworkTxPackets = int64(packetsSent)
				}
				if errorsReceived, ok := counters["errors_received"].(float64); ok {
					stats.NetworkRxErrors = int64(errorsReceived)
				}
				if errorsSent, ok := counters["errors_sent"].(float64); ok {
					stats.NetworkTxErrors = int64(errorsSent)
				}
				if packetsDroppedInbound, ok := counters["packets_dropped_inbound"].(float64); ok {
					stats.NetworkRxDropped = int64(packetsDroppedInbound)
				}
				if packetsDroppedOutbound, ok := counters["packets_dropped_outbound"].(float64); ok {
					stats.NetworkTxDropped = int64(packetsDroppedOutbound)
				}
			}
		}
	}

	return stats, nil
}

func getContainerCPUUsage(ctx context.Context, containerName string) float64 {
	lc := &logger.Context{
		Container: containerName,
		Action:    "get_cpu_usage",
	}
	ctx = logger.NewContext(ctx, lc)

	cmd := exec.CommandContext(ctx, "lxc", "exec", containerName, "--", "sh", "-c", "vmstat 1 2 | tail -1 | awk '{print $15}'")
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Global.Debug(ctx, "获取CPU使用率失败，使用默认值0", zap.Error(err))
		return 0
	}

	idleStr := strings.TrimSpace(string(output))
	idlePercent, err := strconv.ParseFloat(idleStr, 64)
	if err != nil {
		logger.Global.Debug(ctx, "解析CPU idle值失败",
			zap.Error(err),
			zap.String("output", idleStr))
		return 0
	}

	cpuUsage := 100 - idlePercent
	if cpuUsage < 0 {
		cpuUsage = 0
	}
	if cpuUsage > 100 {
		cpuUsage = 100
	}

	return cpuUsage
}
