package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"lxdapi/database"
	"lxdapi/models"
	"lxdapi/pkg/logger"

	"go.uber.org/zap"
)

func ContainerExists(containerName string) bool {
	cmd := exec.Command("lxc", "info", containerName)
	err := cmd.Run()
	return err == nil
}

func SetupContainerSystem(ctx context.Context, containerName, password string) (string, error) {
	var logBuilder strings.Builder

	logBuilder.WriteString("=== 开始设置容器系统 ===\n")
	logBuilder.WriteString(fmt.Sprintf("容器: %s\n", containerName))

	logBuilder.WriteString("1. 等待容器启动完成\n")
	for i := 0; i < 30; i++ {
		cmd := exec.CommandContext(ctx, "lxc", "exec", containerName, "--", "systemctl", "is-system-running")
		if err := cmd.Run(); err == nil {
			break
		}
		time.Sleep(1 * time.Second)
		if i == 29 {
			logBuilder.WriteString("⚠️ 容器启动超时，继续执行设置\n")
		}
	}
	logBuilder.WriteString("容器启动检查完成\n")

	if password != "" {
		logBuilder.WriteString("2. 设置root密码\n")
		passwordCmd := fmt.Sprintf("echo 'root:%s' | chpasswd", password)
		cmd := exec.CommandContext(ctx, "lxc", "exec", containerName, "--", "bash", "-c", passwordCmd)
		output, err := cmd.CombinedOutput()
		if err != nil {
			logBuilder.WriteString(fmt.Sprintf("设置密码失败: %v\n输出: %s\n", err, string(output)))
			return logBuilder.String(), fmt.Errorf("设置密码失败: %v", err)
		}
		logBuilder.WriteString("成功设置root密码\n")
	}

	logBuilder.WriteString("3. 设置主机名\n")
	hostnameCmd := exec.CommandContext(ctx, "lxc", "exec", containerName, "--", "hostnamectl", "set-hostname", containerName)
	output, err := hostnameCmd.CombinedOutput()
	if err != nil {
		logBuilder.WriteString(fmt.Sprintf("设置主机名失败: %v\n输出: %s\n", err, string(output)))

		logBuilder.WriteString("尝试备用方法设置主机名\n")
		fallbackCmd := exec.CommandContext(ctx, "lxc", "exec", containerName, "--", "bash", "-c", fmt.Sprintf("echo '%s' > /etc/hostname && hostname %s", containerName, containerName))
		fallbackOutput, fallbackErr := fallbackCmd.CombinedOutput()
		if fallbackErr != nil {
			logBuilder.WriteString(fmt.Sprintf("备用方法也失败: %v\n输出: %s\n", fallbackErr, string(fallbackOutput)))
			return logBuilder.String(), fmt.Errorf("设置主机名失败: %v", err)
		}
		logBuilder.WriteString("使用备用方法成功设置主机名\n")
	} else {
		logBuilder.WriteString("成功设置主机名\n")
	}

	logBuilder.WriteString("=== 容器系统设置完成 ===\n")
	return logBuilder.String(), nil
}

func CleanupBackupContainers(containerName string) error {
	ctx := context.Background()
	lc := &logger.Context{
		Container: containerName,
		Action:    "cleanup_backup_containers",
	}
	ctx = logger.NewContext(ctx, lc)

	cmd := exec.Command("lxc", "list", "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("获取容器列表失败: %v", err)
	}

	var containers []map[string]interface{}
	if err := json.Unmarshal(output, &containers); err != nil {
		return fmt.Errorf("解析容器列表失败: %v", err)
	}

	backupPatterns := []string{
		fmt.Sprintf("%s-backup-", containerName),
		fmt.Sprintf("%s-reinstall-backup-", containerName),
	}

	for _, container := range containers {
		if name, ok := container["name"].(string); ok {
			for _, pattern := range backupPatterns {
				if strings.HasPrefix(name, pattern) {
					logger.Global.Info(ctx, "发现遗留备份容器", zap.String("backup_container", name))

					stopCmd := exec.Command("lxc", "stop", name, "--force")
					stopCmd.Run()

					deleteCmd := exec.Command("lxc", "delete", name)
					if err := deleteCmd.Run(); err != nil {
						logger.Global.Error(ctx, "删除备份容器失败",
							zap.String("backup_container", name),
							zap.Error(err))
					} else {
						logger.Global.Info(ctx, "成功删除备份容器", zap.String("backup_container", name))
					}
				}
			}
		}
	}

	return nil
}

func RunUpdateConfigLXCCommand(ctx context.Context, containerName string, req models.UpdateContainerConfigRequest) (string, error) {
	var logBuilder strings.Builder

	logBuilder.WriteString("=== 开始更新容器配置 ===\n")
	logBuilder.WriteString(fmt.Sprintf("容器: %s\n", containerName))

	statusCmd := exec.CommandContext(ctx, "lxc", "info", containerName)
	statusOutput, err := statusCmd.CombinedOutput()
	if err != nil {
		logBuilder.WriteString(fmt.Sprintf("获取容器状态失败: %v\n", err))
		return logBuilder.String(), fmt.Errorf("获取容器状态失败: %v", err)
	}

	isRunning := strings.Contains(string(statusOutput), "Status: Running")
	logBuilder.WriteString(fmt.Sprintf("容器状态: %s\n", func() string {
		if isRunning {
			return "运行中"
		}
		return "已停止"
	}()))

	if req.CPUs != nil {
		logBuilder.WriteString(fmt.Sprintf("更新CPU核心数: %d\n", *req.CPUs))
		cmd := exec.CommandContext(ctx, "lxc", "config", "set", containerName, "limits.cpu", fmt.Sprintf("%d", *req.CPUs))
		output, err := cmd.CombinedOutput()
		if err != nil {
			logBuilder.WriteString(fmt.Sprintf("更新CPU配置失败: %v\n输出: %s\n", err, string(output)))
			return logBuilder.String(), fmt.Errorf("更新CPU配置失败: %v", err)
		}
		logBuilder.WriteString("CPU配置更新成功\n")
	}

	if req.Memory != nil {
		logBuilder.WriteString(fmt.Sprintf("更新内存大小: %s\n", *req.Memory))
		cmd := exec.CommandContext(ctx, "lxc", "config", "set", containerName, "limits.memory", *req.Memory)
		output, err := cmd.CombinedOutput()
		if err != nil {
			logBuilder.WriteString(fmt.Sprintf("更新内存配置失败: %v\n输出: %s\n", err, string(output)))
			return logBuilder.String(), fmt.Errorf("更新内存配置失败: %v", err)
		}
		logBuilder.WriteString("内存配置更新成功\n")
	}

	if req.AllowNesting != nil {
		nestingValue := "false"
		if *req.AllowNesting {
			nestingValue = "true"
		}
		logBuilder.WriteString(fmt.Sprintf("更新嵌套虚拟化: %s\n", nestingValue))
		cmd := exec.CommandContext(ctx, "lxc", "config", "set", containerName, "security.nesting", nestingValue)
		output, err := cmd.CombinedOutput()
		if err != nil {
			logBuilder.WriteString(fmt.Sprintf("更新嵌套配置失败: %v\n输出: %s\n", err, string(output)))
			return logBuilder.String(), fmt.Errorf("更新嵌套配置失败: %v", err)
		}
		logBuilder.WriteString("嵌套虚拟化配置更新成功\n")
	}

	if req.Egress != nil || req.Ingress != nil {
		logBuilder.WriteString("更新网络带宽配置\n")

		if req.Ingress != nil {
			logBuilder.WriteString(fmt.Sprintf("更新下行带宽: %s\n", *req.Ingress))
			cmd := exec.CommandContext(ctx, "lxc", "config", "device", "set", containerName, "eth0", "limits.ingress", *req.Ingress)
			output, err := cmd.CombinedOutput()
			if err != nil {
				logBuilder.WriteString(fmt.Sprintf("更新下行带宽失败: %v\n输出: %s\n", err, string(output)))
				return logBuilder.String(), fmt.Errorf("更新下行带宽失败: %v", err)
			}
			logBuilder.WriteString("下行带宽配置更新成功\n")
		}

		if req.Egress != nil {
			logBuilder.WriteString(fmt.Sprintf("更新上行带宽: %s\n", *req.Egress))
			cmd := exec.CommandContext(ctx, "lxc", "config", "device", "set", containerName, "eth0", "limits.egress", *req.Egress)
			output, err := cmd.CombinedOutput()
			if err != nil {
				logBuilder.WriteString(fmt.Sprintf("更新上行带宽失败: %v\n输出: %s\n", err, string(output)))
				return logBuilder.String(), fmt.Errorf("更新上行带宽失败: %v", err)
			}
			logBuilder.WriteString("上行带宽配置更新成功\n")
		}
	}

	if req.Disk != nil {
		logBuilder.WriteString(fmt.Sprintf("更新磁盘大小: %s\n", *req.Disk))
		cmd := exec.CommandContext(ctx, "lxc", "config", "device", "set", containerName, "root", "size", *req.Disk)
		output, err := cmd.CombinedOutput()
		if err != nil {
			logBuilder.WriteString(fmt.Sprintf("更新磁盘配置失败: %v\n输出: %s\n", err, string(output)))
			return logBuilder.String(), fmt.Errorf("更新磁盘配置失败: %v", err)
		}
		logBuilder.WriteString("磁盘配置更新成功\n")
	}

	var containerConfig models.ContainerConfig
	if err := database.DB.Where("container_name = ?", containerName).First(&containerConfig).Error; err == nil {
		if req.CPUs != nil {
			containerConfig.CPUs = *req.CPUs
		}
		if req.Memory != nil {
			containerConfig.Memory = *req.Memory
		}
		if req.Disk != nil {
			containerConfig.Disk = *req.Disk
		}
		if req.Egress != nil {
			containerConfig.Egress = *req.Egress
		}
		if req.Ingress != nil {
			containerConfig.Ingress = *req.Ingress
		}
		if req.AllowNesting != nil {
			containerConfig.AllowNesting = *req.AllowNesting
		}

		if err := database.DB.Save(&containerConfig).Error; err != nil {
			logBuilder.WriteString(fmt.Sprintf("更新数据库配置失败: %v\n", err))
		} else {
			logBuilder.WriteString("数据库配置更新成功\n")
		}
	}

	logBuilder.WriteString("=== 配置更新完成 ===\n")
	return logBuilder.String(), nil
}
