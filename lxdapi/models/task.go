package models

import (
	"time"

	"gorm.io/gorm"
)

const (
	TaskQueued    = "queued"
	TaskPending   = "pending"
	TaskRunning   = "running"
	TaskSuccess   = "success"
	TaskFailed    = "failed"
	TaskCancelled = "cancelled"
)

type TaskStep struct {
	StepID     string                 `json:"step_id"`
	StepName   string                 `json:"step_name"`
	Status     string                 `json:"status"`
	ErrorCode  int                    `json:"error_code"`
	Message    string                 `json:"message"`
	Suggestion string                 `json:"suggestion,omitempty"`
	Data       map[string]interface{} `json:"data,omitempty"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    time.Time              `json:"end_time"`
	Duration   int64                  `json:"duration_ms"`
}

type Task struct {
	gorm.Model
	ContainerName string     `gorm:"index;size:500"`
	Action        string     `gorm:"index;size:255"`
	Status        string     `gorm:"index;size:100"`
	Priority      int        `gorm:"index;default:5"`
	BatchID       string     `gorm:"index;size:500"`
	TraceID       string     `gorm:"type:varchar(36);index"`
	ClientIP      string     `gorm:"type:varchar(45)"`
	QueuedAt      time.Time  `gorm:"index"`
	StartedAt     *time.Time
	CompletedAt   *time.Time
	RetryCount    int        `gorm:"default:0"`
	MaxRetries    int        `gorm:"default:3"`
	Steps          []TaskStep `gorm:"serializer:json;type:longtext"`
	ErrorCode      int        `gorm:"default:0"`
	ErrorMsg       string     `gorm:"type:longtext"`
	FailedFunction string     `gorm:"type:varchar(255)"`
	Log            string     `gorm:"type:longtext"`
	Result         string     `gorm:"type:longtext"`
	Data           string     `gorm:"type:longtext" json:"data,omitempty"`
	StartTime      time.Time
	EndTime        time.Time
}


type TaskResponse struct {
	ID             uint        `json:"id" example:"1"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	ContainerName  string      `json:"container_name" example:"test-container"`
	Action         string      `json:"action" example:"start"`
	Status         string      `json:"status" example:"success"`
	Priority       int         `json:"priority" example:"5"`
	BatchID        string      `json:"batch_id" example:"batch-123"`
	TraceID        string      `json:"trace_id" example:"550e8400-e29b-41d4-a716-446655440000"`
	ClientIP       string      `json:"client_ip" example:"192.168.1.100"`
	QueuedAt       time.Time   `json:"queued_at"`
	StartedAt      *time.Time  `json:"started_at,omitempty"`
	CompletedAt    *time.Time  `json:"completed_at,omitempty"`
	RetryCount     int         `json:"retry_count" example:"0"`
	MaxRetries     int         `json:"max_retries" example:"3"`
	Steps          []TaskStep  `json:"steps,omitempty"`
	ErrorCode      int         `json:"error_code" example:"0"`
	ErrorMsg       string      `json:"error_msg" example:""`
	FailedFunction string      `json:"failed_function,omitempty" example:"CheckStoragePool"`
	Log            string      `json:"log" example:"容器启动成功"`
	Result         string      `json:"result" example:"成功"`
	StartTime      time.Time   `json:"start_time"`
	EndTime        time.Time   `json:"end_time"`
}


type BatchRequest struct {
	Containers []string `json:"containers" example:"test1,test2,test3"`
}


func (t *Task) IsCompleted() bool {
	return t.Status == TaskSuccess || t.Status == TaskFailed
}


func (t *Task) IsRunning() bool {
	return t.Status == TaskRunning
}


func (t *Task) SetCompleted(success bool, result, errorMsg string) {
	t.EndTime = time.Now()
	t.Result = result
	t.ErrorMsg = errorMsg
	if success {
		t.Status = TaskSuccess
	} else {
		t.Status = TaskFailed
	}
}
