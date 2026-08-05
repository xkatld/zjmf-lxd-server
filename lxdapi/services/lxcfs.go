package services

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"lxdapi/config"
	"lxdapi/pkg/logger"

	"go.uber.org/zap"
)

func EnableLXCFSForContainer(ctx context.Context, containerName string) error {
	lc := &logger.Context{
		Container: containerName,
		Action:    "enable_lxcfs",
	}
	ctx = logger.NewContext(ctx, lc)

	lxcfsMountPath := config.GetLXCFSMountPath()
	logger.Global.Info(ctx, "开始为容器启用LXCFS", zap.String("mount_path", lxcfsMountPath))

	if !CheckLXCFSInstalled(ctx) {
		logger.Global.Warn(ctx, "LXCFS未安装或未运行，跳过LXCFS配置")
		return nil
	}

	time.Sleep(2 * time.Second)

	lxcfsPaths := []string{
		"proc/cpuinfo",
		"proc/meminfo",
		"proc/stat",
		"proc/uptime",
		"proc/diskstats",
		"proc/swaps",
		"sys/devices/system/cpu/online",
	}

	successCount := 0
	for _, path := range lxcfsPaths {
		deviceName := strings.ReplaceAll(path, "/", "-")
		sourcePath := fmt.Sprintf("%s/%s", lxcfsMountPath, path)
		targetPath := fmt.Sprintf("/%s", path)

		cmd := exec.CommandContext(ctx, "lxc", "config", "device", "add", containerName,
			deviceName, "disk",
			fmt.Sprintf("source=%s", sourcePath),
			fmt.Sprintf("path=%s", targetPath))

		output, err := cmd.CombinedOutput()
		if err != nil {
			if strings.Contains(string(output), "already exists") {
				logger.Global.Debug(ctx, "LXCFS设备已存在",
					zap.String("device", deviceName))
				successCount++
				continue
			}
			logger.Global.Warn(ctx, "添加LXCFS设备失败",
				zap.String("device", deviceName),
				zap.Error(err),
				zap.String("output", string(output)))
			continue
		}

		logger.Global.Debug(ctx, "LXCFS设备添加成功",
			zap.String("device", deviceName))
		successCount++
	}

	if successCount > 0 {
		logger.Global.Info(ctx, "LXCFS配置完成", zap.Int("success_count", successCount))
	} else {
		logger.Global.Warn(ctx, "未能添加任何LXCFS设备")
	}
	
	return nil
}

func CheckLXCFSInstalled(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "systemctl", "is-active", "lxcfs")
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		return false
	}

	status := strings.TrimSpace(string(output))
	return status == "active"
}

func GetLXCFSStatus() map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	status := map[string]interface{}{
		"installed":  false,
		"running":    false,
		"mount_path": config.GetLXCFSMountPath(),
	}

	cmd := exec.CommandContext(ctx, "systemctl", "is-active", "lxcfs")
	output, err := cmd.CombinedOutput()
	
	if err == nil && strings.TrimSpace(string(output)) == "active" {
		status["installed"] = true
		status["running"] = true
	} else {
		cmd = exec.CommandContext(ctx, "which", "lxcfs")
		if err := cmd.Run(); err == nil {
			status["installed"] = true
		}
	}

	return status
}

func CheckContainerLXCFSStatus(containerName string) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), config.GetLXCTimeout())
	defer cancel()

	cmd := exec.CommandContext(ctx, "lxc", "config", "device", "list", containerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("获取容器设备列表失败: %v", err)
	}

	deviceList := string(output)
	return strings.Contains(deviceList, "proc-cpuinfo") || 
	       strings.Contains(deviceList, "proc-meminfo"), nil
}

