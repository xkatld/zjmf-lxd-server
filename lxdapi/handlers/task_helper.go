package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"lxdapi/models"
	"lxdapi/pkg/logger"
	"lxdapi/services"
)

func CreateTask(containerName, action string, priority int, data interface{}) (*models.Task, error) {
	dataJSON := ""
	if data != nil {
		bytes, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		dataJSON = string(bytes)
	}

	task := &models.Task{
		ContainerName: containerName,
		Action:        action,
		Status:        models.TaskQueued,
		Priority:      priority,
		QueuedAt:      time.Now(),
		MaxRetries:    0,
		Data:          dataJSON,
		Steps:         []models.TaskStep{},
	}

	wp := services.GetGlobalWorkerPool()
	if wp == nil || wp.Backend == nil {
		return nil, fmt.Errorf("worker pool not initialized")
	}

	if err := wp.Backend.EnqueueTask(task); err != nil {
		return nil, err
	}

	return task, nil
}

func CreateTaskWithContext(ctx context.Context, containerName, action string, priority int, data interface{}) (*models.Task, error) {
	lc := logger.FromContext(ctx)

	dataJSON := ""
	if data != nil {
		bytes, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		dataJSON = string(bytes)
	}

	task := &models.Task{
		ContainerName: containerName,
		Action:        action,
		Status:        models.TaskQueued,
		Priority:      priority,
		TraceID:       lc.TraceID,
		ClientIP:      lc.ClientIP,
		QueuedAt:      time.Now(),
		MaxRetries:    0,
		Data:          dataJSON,
		Steps:         []models.TaskStep{},
	}

	wp := services.GetGlobalWorkerPool()
	if wp == nil || wp.Backend == nil {
		return nil, fmt.Errorf("worker pool not initialized")
	}

	if err := wp.Backend.EnqueueTask(task); err != nil {
		return nil, err
	}

	return task, nil
}

