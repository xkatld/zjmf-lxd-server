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

type ResumeContainerExecutor struct {
	*BaseExecutor
}

func NewResumeContainerExecutor(db *gorm.DB, logger *zap.Logger, task *models.Task) *ResumeContainerExecutor {
	return &ResumeContainerExecutor{
		BaseExecutor: NewBaseExecutor(db, logger, task),
	}
}

func (e *ResumeContainerExecutor) Execute() error {
	ctx := context.Background()

	e.StartStep("STEP_1", "Check container status")
	status, err := e.getContainerStatus(ctx)
	if err != nil {
		e.CompleteStep(false, errors.ERR_LXC_QUERY_FAIL, fmt.Sprintf("Get container status failed: %v", err))
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, fmt.Sprintf("Container status: %s", status))

	if status != "Frozen" {
		e.StartStep("STEP_2", "Container not paused")
		e.CompleteStep(false, errors.ERR_CONTAINER_NOT_PAUSED, "Container is not paused")
		return fmt.Errorf("container is not frozen/paused")
	}

	e.StartStep("STEP_2", "Resume container")
	if err := e.resumeContainer(ctx); err != nil {
		e.CompleteStep(false, errors.ERR_LXC_RESUME_FAIL, fmt.Sprintf("Resume container failed: %v", err))
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, "Container resumed successfully")

	e.updateCacheStatus("Running")

	return nil
}

func (e *ResumeContainerExecutor) getContainerStatus(ctx context.Context) (string, error) {
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

func (e *ResumeContainerExecutor) resumeContainer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "lxc", "start", e.ContainerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lxc start failed: %v, output: %s", err, string(output))
	}

	e.LogInfo("Container resumed", zap.String("output", string(output)))
	return nil
}

func (e *ResumeContainerExecutor) GetSteps() []models.TaskStep {
	return e.Task.Steps
}

func (e *ResumeContainerExecutor) GetErrorCode() int {
	return e.Task.ErrorCode
}
