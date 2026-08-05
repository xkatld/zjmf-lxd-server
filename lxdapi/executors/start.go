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

type StartContainerExecutor struct {
	*BaseExecutor
}

func NewStartContainerExecutor(db *gorm.DB, logger *zap.Logger, task *models.Task) *StartContainerExecutor {
	return &StartContainerExecutor{
		BaseExecutor: NewBaseExecutor(db, logger, task),
	}
}

func (e *StartContainerExecutor) Execute() error {
	ctx := context.Background()
	
	e.StartStep("STEP_1", "Check container status")
	status, err := e.getContainerStatus(ctx)
	if err != nil {
		e.CompleteStep(false, errors.ERR_LXC_QUERY_FAIL, fmt.Sprintf("Get container status failed: %v", err))
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, fmt.Sprintf("Container status: %s", status))
	
	if status == "Running" {
		e.StartStep("STEP_2", "Container already running")
		e.CompleteStep(true, errors.ERR_SUCCESS, "Container is already running, no action needed")
		return nil
	}
	
	e.StartStep("STEP_2", "Start container")
	if err := e.startContainer(ctx); err != nil {
		e.CompleteStep(false, errors.ERR_LXC_START_FAIL, fmt.Sprintf("Start container failed: %v", err))
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, "Container started successfully")
	
	e.updateCacheStatus("Running")
	
	return nil
}

func (e *StartContainerExecutor) getContainerStatus(ctx context.Context) (string, error) {
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

func (e *StartContainerExecutor) startContainer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "lxc", "start", e.ContainerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lxc start failed: %v, output: %s", err, string(output))
	}
	
	e.LogInfo("Container started", zap.String("output", string(output)))
	return nil
}

