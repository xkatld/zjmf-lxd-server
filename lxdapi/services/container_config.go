package services

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"lxdapi/database"
	"lxdapi/models"
)


func ConfigureContainerAfterCreation(ctx context.Context, containerName, password string) (string, error) {
	var logBuilder strings.Builder

	logBuilder.WriteString("开始配置容器\n")

	// 等待容器启动
	time.Sleep(5 * time.Second)

	if password != "" {
		logBuilder.WriteString("设置root密码\n")
		passwordCmd := exec.CommandContext(ctx, "lxc", "exec", containerName, "--", "bash", "-c", fmt.Sprintf("echo 'root:%s' | chpasswd", password))
		passwordOutput, err := passwordCmd.CombinedOutput()
		if err != nil {
			logBuilder.WriteString(fmt.Sprintf("设置密码失败: %v, 输出: %s\n", err, string(passwordOutput)))
			return logBuilder.String(), fmt.Errorf("设置密码失败: %v", err)
		}
		logBuilder.WriteString("设置root密码成功\n")
	} else {
		logBuilder.WriteString("密码为空，跳过密码设置\n")
	}

	logBuilder.WriteString("设置主机名\n")
	hostnameScript := fmt.Sprintf(`hostname %s && echo '%s' > /etc/hostname && (sed -i 's/127.0.1.1.*/127.0.1.1\t%s/' /etc/hosts || echo '127.0.1.1\t%s' >> /etc/hosts)`, 
		containerName, containerName, containerName, containerName)
	hostnameCmd := exec.CommandContext(ctx, "lxc", "exec", containerName, "--", "bash", "-c", hostnameScript)
	hostnameOutput, err := hostnameCmd.CombinedOutput()
	if err != nil {
		logBuilder.WriteString(fmt.Sprintf("设置主机名失败: %v, 输出: %s\n", err, string(hostnameOutput)))
	} else {
		logBuilder.WriteString("设置主机名成功\n")
	}

	logBuilder.WriteString("容器配置完成\n")
	return logBuilder.String(), nil
}


func ResetContainerPassword(ctx context.Context, containerName, newPassword string) (string, error) {
	var logBuilder strings.Builder

	logBuilder.WriteString("=== 重置容器密码 ===\n")
	logBuilder.WriteString(fmt.Sprintf("容器: %s\n", containerName))


	status, err := GetContainerStatus(containerName)
	if err != nil {
		logBuilder.WriteString(fmt.Sprintf("获取容器状态失败: %v\n", err))
		return logBuilder.String(), fmt.Errorf("获取容器状态失败: %v", err)
	}

	if status != "Running" {
		logBuilder.WriteString(fmt.Sprintf("容器状态为 %s，需要运行状态才能重置密码\n", status))
		return logBuilder.String(), fmt.Errorf("容器状态为 %s，需要运行状态才能重置密码", status)
	}


	logBuilder.WriteString("重置root密码...\n")
	passwordCmd := exec.CommandContext(ctx, "lxc", "exec", containerName, "--", "bash", "-c", fmt.Sprintf("echo 'root:%s' | chpasswd", newPassword))
	passwordOutput, err := passwordCmd.CombinedOutput()
	if err != nil {
		logBuilder.WriteString(fmt.Sprintf("重置密码失败: %v\n输出: %s\n", err, string(passwordOutput)))
		return logBuilder.String(), fmt.Errorf("重置密码失败: %v", err)
	}
	logBuilder.WriteString("成功重置root密码\n")


	logBuilder.WriteString("更新数据库中的密码...\n")
	if err := database.DB.Model(&models.ContainerConfig{}).Where("container_name = ?", containerName).Update("password", newPassword).Error; err != nil {
		logBuilder.WriteString(fmt.Sprintf("更新数据库密码失败: %v\n", err))
		logBuilder.WriteString("⚠️ 密码重置成功，但数据库更新失败\n")
	} else {
		logBuilder.WriteString("成功更新数据库中的密码\n")
	}

	logBuilder.WriteString("=== 密码重置完成 ===\n")
	return logBuilder.String(), nil
}

