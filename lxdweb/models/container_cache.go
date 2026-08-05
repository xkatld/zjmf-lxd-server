package models

import (
	"time"
	"gorm.io/gorm"
)

type ContainerCache struct {
	ID             uint           `json:"id" gorm:"primaryKey"`
	NodeID         uint           `json:"node_id" gorm:"not null;index:idx_node_hostname_cache;uniqueIndex:idx_unique_container"`
	NodeName       string         `json:"node_name" gorm:"size:200"`
	Hostname       string         `json:"hostname" gorm:"size:200;not null;index:idx_node_hostname_cache;uniqueIndex:idx_unique_container"`
	Status         string         `json:"status" gorm:"size:50"`
	IPv4           string         `json:"ipv4" gorm:"size:50"`
	IPv6           string         `json:"ipv6" gorm:"size:200"`
	Image          string         `json:"image" gorm:"size:200"`
	
	CPUs           int            `json:"cpus"`
	Memory         string         `json:"memory" gorm:"size:50"`
	Disk           string         `json:"disk" gorm:"size:50"`
	TrafficLimit   int            `json:"traffic_limit"`
	Ingress        string         `json:"ingress" gorm:"size:50"`
	Egress         string         `json:"egress" gorm:"size:50"` 
	
	CPUUsage       float64        `json:"cpu_usage"`
	MemoryUsage    uint64         `json:"memory_usage"`
	MemoryTotal    uint64         `json:"memory_total"`
	DiskUsage      uint64         `json:"disk_usage"`
	DiskTotal      uint64         `json:"disk_total"`
	TrafficTotal   uint64         `json:"traffic_total"`
	TrafficIn      uint64         `json:"traffic_in"`
	TrafficOut     uint64         `json:"traffic_out"`
	
	LastSync       time.Time      `json:"last_sync"`
	SyncError      string         `json:"sync_error" gorm:"type:text"`
	
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

type SyncTask struct {
	ID             uint           `json:"id" gorm:"primaryKey"`
	NodeID         uint           `json:"node_id" gorm:"index"`
	NodeName       string         `json:"node_name" gorm:"size:200"`
	Status         string         `json:"status" gorm:"size:50;default:'pending'"` 
	TotalCount     int            `json:"total_count"`
	SuccessCount   int            `json:"success_count"`
	FailedCount    int            `json:"failed_count"`
	StartTime      *time.Time     `json:"start_time"`
	EndTime        *time.Time     `json:"end_time"`
	ErrorMessage   string         `json:"error_message" gorm:"type:text"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
}

func (ContainerCache) TableName() string {
	return "container_cache"
}

func (SyncTask) TableName() string {
	return "sync_tasks"
}

