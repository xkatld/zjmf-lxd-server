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

type RebootContainerExecutor struct {
	*BaseExecutor
}

func NewRebootContainerExecutor(db *gorm.DB, logger *zap.Logger, task *models.Task) *RebootContainerExecutor {
	return &RebootContainerExecutor{
		BaseExecutor: NewBaseExecutor(db, logger, task),
	}
}

func (e *RebootContainerExecutor) Execute() error {
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
		e.CompleteStep(false, errors.ERR_CONTAINER_NOT_RUNNING, "Container is not running, cannot reboot")
		return fmt.Errorf("container not running")
	}
	
	e.StartStep("STEP_2", "Restart container")
	if err := e.restartContainer(ctx); err != nil {
		e.CompleteStep(false, errors.ERR_LXC_RESTART_FAIL, fmt.Sprintf("Restart container failed: %v", err))
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, "Container restarted successfully")
	
	e.updateCacheStatus("Running")
	
	return nil
}

func (e *RebootContainerExecutor) getContainerStatus(ctx context.Context) (string, error) {
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

func (e *RebootContainerExecutor) restartContainer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "lxc", "restart", e.ContainerName, "--force")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lxc restart failed: %v, output: %s", err, string(output))
	}
	
	e.LogInfo("Container restarted", zap.String("output", string(output)))
	return nil
}

