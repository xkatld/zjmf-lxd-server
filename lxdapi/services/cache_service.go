package services

import (
	"encoding/json"
	"lxdapi/database"
	"lxdapi/models"
	"time"

	"gorm.io/gorm/clause"
)

// UpdateContainerCache 更新容器信息缓存
func UpdateContainerCache(hostname string, data map[string]interface{}) error {
	if hostname == "" {
		return nil
	}

	fullDataJSON, _ := json.Marshal(data)

	cache := models.ContainerInfoCache{
		Hostname:   hostname,
		FullData:   string(fullDataJSON),
		LastUpdate: time.Now(),
	}

	if status, ok := data["status"].(string); ok {
		cache.Status = status
	}
	if image, ok := data["image"].(string); ok {
		cache.Image = image
	}
	if ipv4, ok := data["ipv4"].(string); ok {
		cache.IPv4 = ipv4
	}
	if ipv6, ok := data["ipv6"].(string); ok {
		cache.IPv6 = ipv6
	}
	if cpus, ok := data["cpus"].(float64); ok {
		cache.CPUs = int(cpus)
	}
	if memory, ok := data["memory"].(string); ok {
		cache.Memory = memory
	}
	if disk, ok := data["disk"].(string); ok {
		cache.Disk = disk
	}
	if trafficLimit, ok := data["traffic_limit"].(float64); ok {
		cache.TrafficLimit = int(trafficLimit)
	}
	if cpuUsage, ok := data["cpu_usage"].(float64); ok {
		cache.CPUUsage = cpuUsage
	} else if cpuUsage, ok := data["cpu_percent"].(float64); ok {
		cache.CPUUsage = cpuUsage
	}
	if memUsage, ok := data["memory_usage"].(float64); ok {
		cache.MemoryUsage = uint64(memUsage)
	} else if memUsage, ok := data["memory_usage_raw"].(float64); ok {
		cache.MemoryUsage = uint64(memUsage)
	}
	if memTotal, ok := data["memory_total"].(float64); ok {
		cache.MemoryTotal = uint64(memTotal)
	}
	if diskUsage, ok := data["disk_usage"].(float64); ok {
		cache.DiskUsage = uint64(diskUsage)
	} else if diskUsage, ok := data["disk_usage_raw"].(float64); ok {
		cache.DiskUsage = uint64(diskUsage)
	}
	if diskTotal, ok := data["disk_total"].(float64); ok {
		cache.DiskTotal = uint64(diskTotal)
	}
	if trafficIn, ok := data["traffic_in"].(float64); ok {
		cache.TrafficIn = uint64(trafficIn)
	}
	if trafficOut, ok := data["traffic_out"].(float64); ok {
		cache.TrafficOut = uint64(trafficOut)
	}
	if trafficTotal, ok := data["traffic_total"].(float64); ok {
		cache.TrafficTotal = uint64(trafficTotal)
	} else if trafficUsage, ok := data["traffic_usage_raw"].(float64); ok {
		cache.TrafficTotal = uint64(trafficUsage)
	}

	result := database.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "hostname"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"status", "image", "ipv4", "ipv6",
			"cpus", "memory", "disk", "traffic_limit",
			"cpu_usage", "memory_usage", "memory_total",
			"disk_usage", "disk_total",
			"traffic_in", "traffic_out", "traffic_total",
			"full_data", "last_update",
		}),
	}).Create(&cache)

	return result.Error
}

// GetAllContainersFromCache 获取所有容器缓存
func GetAllContainersFromCache() ([]map[string]interface{}, error) {
	var caches []models.ContainerInfoCache
	if err := database.DB.Order("last_update DESC").Find(&caches).Error; err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0, len(caches))
	for _, cache := range caches {
		result = append(result, cache.ToMap())
	}

	return result, nil
}

// UpdateContainerStatusCache 快速更新容器状态到缓存
func UpdateContainerStatusCache(hostname string, status string) error {
	if hostname == "" {
		return nil
	}

	return database.DB.Model(&models.ContainerInfoCache{}).
		Where("hostname = ?", hostname).
		Updates(map[string]interface{}{
			"status":      status,
			"last_update": time.Now(),
		}).Error
}

