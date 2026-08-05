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

type StopContainerExecutor struct {
	*BaseExecutor
}

func NewStopContainerExecutor(db *gorm.DB, logger *zap.Logger, task *models.Task) *StopContainerExecutor {
	return &StopContainerExecutor{
		BaseExecutor: NewBaseExecutor(db, logger, task),
	}
}

func (e *StopContainerExecutor) Execute() error {
	ctx := context.Background()
	
	e.StartStep("STEP_1", "Check container status")
	status, err := e.getContainerStatus(ctx)
	if err != nil {
		e.CompleteStep(false, errors.ERR_LXC_QUERY_FAIL, fmt.Sprintf("Get container status failed: %v", err))
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, fmt.Sprintf("Container status: %s", status))
	
	if status == "Stopped" {
		e.StartStep("STEP_2", "Container already stopped")
		e.CompleteStep(true, errors.ERR_SUCCESS, "Container is already stopped, no action needed")
		return nil
	}
	
	e.StartStep("STEP_2", "Stop container")
	if err := e.stopContainer(ctx); err != nil {
		e.CompleteStep(false, errors.ERR_LXC_STOP_FAIL, fmt.Sprintf("Stop container failed: %v", err))
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, "Container stopped successfully")
	
	e.updateCacheStatus("Stopped")
	
	return nil
}

func (e *StopContainerExecutor) getContainerStatus(ctx context.Context) (string, error) {
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

func (e *StopContainerExecutor) stopContainer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "lxc", "stop", e.ContainerName, "--force")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lxc stop failed: %v, output: %s", err, string(output))
	}
	
	e.LogInfo("Container stopped", zap.String("output", string(output)))
	return nil
}

