package services

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"lxdapi/pkg/logger"

	"go.uber.org/zap"
)

const (
	ActionStart  = "start"
	ActionStop   = "stop"
	ActionDelete = "delete"
	ActionReboot = "restart"
	ActionPause  = "pause"
	ActionResume = "resume"
)

func RunLXCCommand(ctx context.Context, container, action string) (string, error) {
	cmd, err := buildLXCCommand(ctx, container, action)
	if err != nil {
		return "", err
	}

	logger.Global.Debug(ctx, "执行LXC命令", zap.String("command", strings.Join(cmd.Args, " ")))

	output, err := cmd.CombinedOutput()
	result := string(output)

	if err != nil {
		logger.Global.Error(ctx, "命令执行失败", zap.String("output", result), zap.Error(err))
		return result, err
	}

	logger.Global.Debug(ctx, "命令执行成功", zap.String("output", result))
	return result, nil
}

func buildLXCCommand(ctx context.Context, container, action string) (*exec.Cmd, error) {
	switch action {
	case ActionStart:
		return exec.CommandContext(ctx, "lxc", "start", container), nil
	case ActionStop:
		return exec.CommandContext(ctx, "lxc", "stop", container, "--force"), nil
	case ActionReboot:
		return exec.CommandContext(ctx, "lxc", "restart", container), nil
	case ActionDelete:
		return exec.CommandContext(ctx, "lxc", "delete", "--force", container), nil
	case ActionPause:
		return exec.CommandContext(ctx, "lxc", "pause", container), nil
	case ActionResume:
		return exec.CommandContext(ctx, "lxc", "start", container), nil
	default:
		return nil, fmt.Errorf("不支持的操作: %s", action)
	}
}

func IsValidAction(action string) bool {
	validActions := []string{ActionStart, ActionStop, ActionReboot, ActionDelete, ActionPause, ActionResume}
	for _, validAction := range validActions {
		if action == validAction {
			return true
		}
	}
	return false
}
