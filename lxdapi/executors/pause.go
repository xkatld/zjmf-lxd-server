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

type PauseContainerExecutor struct {
	*BaseExecutor
}

func NewPauseContainerExecutor(db *gorm.DB, logger *zap.Logger, task *models.Task) *PauseContainerExecutor {
	return &PauseContainerExecutor{
		BaseExecutor: NewBaseExecutor(db, logger, task),
	}
}

func (e *PauseContainerExecutor) Execute() error {
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
		e.CompleteStep(false, errors.ERR_CONTAINER_NOT_RUNNING, "Container must be running to pause")
		return fmt.Errorf("container is not running")
	}
	
	e.StartStep("STEP_2", "Pause container")
	if err := e.pauseContainer(ctx); err != nil {
		e.CompleteStep(false, errors.ERR_LXC_PAUSE_FAIL, fmt.Sprintf("Pause container failed: %v", err))
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, "Container paused successfully")
	
	e.updateCacheStatus("Frozen")
	
	return nil
}

func (e *PauseContainerExecutor) getContainerStatus(ctx context.Context) (string, error) {
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
		return "NotFound", nil
	}
	
	return containers[0].Status, nil
}

func (e *PauseContainerExecutor) pauseContainer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "lxc", "pause", e.ContainerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lxc pause failed: %v, output: %s", err, string(output))
	}
	
	e.LogInfo("Container paused", zap.String("output", string(output)))
	return nil
}

func (e *PauseContainerExecutor) GetSteps() []models.TaskStep {
	return e.Task.Steps
}

func (e *PauseContainerExecutor) GetErrorCode() int {
	return e.Task.ErrorCode
}

