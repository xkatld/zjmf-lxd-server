package executors

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"lxdapi/errors"
	"lxdapi/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ResetPasswordExecutor struct {
	*BaseExecutor
	NewPassword string
}

func NewResetPasswordExecutor(db *gorm.DB, logger *zap.Logger, task *models.Task) (*ResetPasswordExecutor, error) {
	var data map[string]string
	if task.Data != "" {
		if err := json.Unmarshal([]byte(task.Data), &data); err != nil {
			return nil, fmt.Errorf("parse password data failed: %v", err)
		}
	}
	
	password, ok := data["password"]
	if !ok || password == "" {
		return nil, fmt.Errorf("password not provided")
	}
	
	return &ResetPasswordExecutor{
		BaseExecutor: NewBaseExecutor(db, logger, task),
		NewPassword:  password,
	}, nil
}

func (e *ResetPasswordExecutor) Execute() error {
	ctx := context.Background()
	
	e.StartStep("STEP_1", "Check container status")
	status, err := e.getContainerStatus(ctx)
	if err != nil {
		e.CompleteStep(false, errors.ERR_LXC_QUERY_FAIL, fmt.Sprintf("Get container status failed: %v", err))
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, fmt.Sprintf("Container status: %s", status))
	
	if status != "Running" {
		e.StartStep("STEP_2", "Container not running")
		e.CompleteStep(false, errors.ERR_CONTAINER_NOT_RUNNING, "Container is not running, cannot reset password")
		return fmt.Errorf("container not running")
	}
	
	e.StartStep("STEP_2", "Reset root password")
	if err := e.resetPassword(ctx); err != nil {
		e.CompleteStep(false, errors.ERR_PASSWORD_RESET_FAIL, fmt.Sprintf("Reset password failed: %v", err))
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, "Password reset successfully")
	
	e.StartStep("STEP_3", "Update password in database")
	if err := e.updatePasswordInDB(); err != nil {
		e.CompleteStep(false, errors.ERR_DB_UPDATE_FAIL, fmt.Sprintf("Update database failed: %v", err))
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, "Password updated in database")
	
	return nil
}

func (e *ResetPasswordExecutor) getContainerStatus(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "lxc", "list", e.ContainerName, "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("lxc list failed: %v, output: %s", err, string(output))
	}
	
	var containers []struct {
		Status string `json:"status"`
	}
	
	if err := json.Unmarshal(output, &containers); err != nil {
		return "", fmt.Errorf("parse json failed: %v", err)
	}
	
	if len(containers) == 0 {
		return "", fmt.Errorf("container not found")
	}
	
	return containers[0].Status, nil
}

func (e *ResetPasswordExecutor) resetPassword(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "lxc", "exec", e.ContainerName, "--", "sh", "-c",
		fmt.Sprintf("echo 'root:%s' | chpasswd", e.NewPassword))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("chpasswd failed: %v, output: %s", err, string(output))
	}
	
	e.LogInfo("Password reset", zap.String("output", string(output)))
	return nil
}

func (e *ResetPasswordExecutor) updatePasswordInDB() error {
	result := e.DB.Model(&models.ContainerConfig{}).
		Where("container_name = ?", e.ContainerName).
		Update("password", e.NewPassword)
	
	if result.Error != nil {
		return fmt.Errorf("update password in database failed: %v", result.Error)
	}
	
	if result.RowsAffected == 0 {
		e.LogInfo("No container config found in database, skip password update")
	} else {
		e.LogInfo("Password updated in database")
	}
	
	return nil
}

