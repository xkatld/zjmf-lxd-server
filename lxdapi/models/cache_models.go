package models

import (
	"encoding/json"
	"time"
)

// ContainerInfoCache 容器信息缓存表
type ContainerInfoCache struct {
	ID       uint   `gorm:"primaryKey"`
	Hostname string `gorm:"uniqueIndex;size:200;not null"`

	Status string `gorm:"size:50"`
	Image  string `gorm:"size:200"`
	IPv4   string `gorm:"size:100"`
	IPv6   string `gorm:"size:100"`

	CPUs         int
	Memory       string `gorm:"size:50"`
	Disk         string `gorm:"size:50"`
	TrafficLimit int

	CPUUsage    float64
	MemoryUsage uint64
	MemoryTotal uint64
	DiskUsage   uint64
	DiskTotal   uint64
	TrafficIn   uint64
	TrafficOut  uint64
	TrafficTotal uint64

	FullData   string    `gorm:"type:text"`
	LastUpdate time.Time `gorm:"index"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (ContainerInfoCache) TableName() string {
	return "container_info_cache"
}

func (c *ContainerInfoCache) ToMap() map[string]interface{} {
	var result map[string]interface{}
	
	if c.FullData != "" {
		if err := json.Unmarshal([]byte(c.FullData), &result); err == nil {
			result["status"] = c.Status
			result["last_update"] = c.LastUpdate
			if c.Image != "" {
				result["image"] = c.Image
			}
			if c.IPv4 != "" {
				result["ipv4"] = c.IPv4
			}
			if c.IPv6 != "" {
				result["ipv6"] = c.IPv6
			}
			return result
		}
	}

	return map[string]interface{}{
		"hostname":       c.Hostname,
		"status":         c.Status,
		"image":          c.Image,
		"ipv4":           c.IPv4,
		"ipv6":           c.IPv6,
		"cpus":           c.CPUs,
		"memory":         c.Memory,
		"disk":           c.Disk,
		"traffic_limit":  c.TrafficLimit,
		"cpu_usage":      c.CPUUsage,
		"memory_usage":   c.MemoryUsage,
		"memory_total":   c.MemoryTotal,
		"disk_usage":     c.DiskUsage,
		"disk_total":     c.DiskTotal,
		"traffic_in":     c.TrafficIn,
		"traffic_out":    c.TrafficOut,
		"traffic_total":  c.TrafficTotal,
		"last_update":    c.LastUpdate,
	}
}

