package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"lxdapi/config"
	"lxdapi/models"
	"lxdapi/pkg/logger"

	"go.uber.org/zap"
)


func GetAllContainers() ([]models.ContainerInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), config.GetLXCTimeout())
	defer cancel()


	cmd := exec.CommandContext(ctx, "lxc", "list", "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		lc := &logger.Context{Action: "get_all_containers"}
		logCtx := logger.NewContext(context.Background(), lc)
		logger.Global.Error(logCtx, "LXC命令执行失败，返回空容器列表", zap.Error(err))
		return []models.ContainerInfo{}, nil
	}

	var lxcContainers []models.LXCContainerState
	if err := json.Unmarshal(output, &lxcContainers); err != nil {
		lc := &logger.Context{Action: "get_all_containers"}
		logCtx := logger.NewContext(context.Background(), lc)
		logger.Global.Error(logCtx, "JSON解析失败，返回空容器列表", zap.Error(err))
		return []models.ContainerInfo{}, nil
	}

	var containers []models.ContainerInfo
	for _, lxcContainer := range lxcContainers {
		container := convertLXCContainerToContainerInfo(lxcContainer)

		// 优化: 只查询一次state，获取所有需要的数据
		stateData, err := getContainerState(ctx, lxcContainer.Name)
		if err == nil {
			// 从state中提取网络状态
			if network, ok := stateData["network"].(map[string]interface{}); ok {
				networkState := make(map[string]models.LXCNetworkState)
				for iface, data := range network {
					if ifaceData, ok := data.(map[string]interface{}); ok {
						netState := models.LXCNetworkState{}
						
						// 解析地址列表
						if addresses, ok := ifaceData["addresses"].([]interface{}); ok {
							for _, addr := range addresses {
								if addrMap, ok := addr.(map[string]interface{}); ok {
									address := models.LXCNetworkAddress{}
									if family, ok := addrMap["family"].(string); ok {
										address.Family = family
									}
									if addrStr, ok := addrMap["address"].(string); ok {
										address.Address = addrStr
									}
									if scope, ok := addrMap["scope"].(string); ok {
										address.Scope = scope
									}
									netState.Addresses = append(netState.Addresses, address)
								}
							}
						}
						networkState[iface] = netState
					}
				}
				container.NetworkState = networkState
				
				// 提取IP地址
				var ips []string
				for _, netState := range networkState {
					for _, addr := range netState.Addresses {
						if addr.Family == "inet" && addr.Scope == "global" {
							ips = append(ips, addr.Address)
						}
					}
				}
				container.IPAddresses = ips
			}

			// 从state中提取资源信息
			resources := make(map[string]interface{})
			if cpu, ok := stateData["cpu"].(map[string]interface{}); ok {
				resources["cpu"] = cpu
			}
			if memory, ok := stateData["memory"].(map[string]interface{}); ok {
				resources["memory"] = memory
			}
			if disk, ok := stateData["disk"].(map[string]interface{}); ok {
				resources["disk"] = disk
			}
			if len(resources) > 0 {
				container.Resources = resources
			}
		}

		containers = append(containers, container)
	}

	return containers, nil
}


func GetContainerStatus(containerName string) (string, error) {
	ctx := context.Background()


	cmd := exec.CommandContext(ctx, "lxc", "list", containerName, "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("获取容器状态失败: %v", err)
	}

	var lxcContainers []models.LXCContainerState
	if err := json.Unmarshal(output, &lxcContainers); err != nil {
		return "", fmt.Errorf("解析容器状态失败: %v", err)
	}

	if len(lxcContainers) == 0 {
		return "", fmt.Errorf("容器不存在")
	}

	return lxcContainers[0].Status, nil
}


func GetLXDVersion() (string, error) {
	ctx := context.Background()


	cmd := exec.CommandContext(ctx, "lxc", "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("获取LXD版本失败: %v", err)
	}


	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Server version:") {
			// 提取服务器版本，格式如 "Server version: 5.0.2"
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				return parts[2], nil
			}
		}
	}

	// 如果没有找到服务器版本，返回客户端版本
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Client version:") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				return parts[2], nil
			}
		}
	}

	return "unknown", nil
}


func GetContainerInfo(containerName string) (*models.ContainerInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), config.GetLXCTimeout())
	defer cancel()


	cmd := exec.CommandContext(ctx, "lxc", "list", containerName, "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("容器不存在: %s", containerName)
	}

	var lxcContainers []models.LXCContainerState
	if err := json.Unmarshal(output, &lxcContainers); err != nil {
		return nil, fmt.Errorf("容器不存在: %s", containerName)
	}

	if len(lxcContainers) == 0 {
		return nil, fmt.Errorf("容器不存在: %s", containerName)
	}

	lxcContainer := lxcContainers[0]
	container := convertLXCContainerToContainerInfo(lxcContainer)

	// 优化: 只查询一次state，获取所有需要的数据
	stateData, err := getContainerState(ctx, containerName)
	if err == nil {
		// 从state中提取网络状态
		if network, ok := stateData["network"].(map[string]interface{}); ok {
			networkState := make(map[string]models.LXCNetworkState)
			for iface, data := range network {
				if ifaceData, ok := data.(map[string]interface{}); ok {
					netState := models.LXCNetworkState{}
					
					// 解析地址列表
					if addresses, ok := ifaceData["addresses"].([]interface{}); ok {
						for _, addr := range addresses {
							if addrMap, ok := addr.(map[string]interface{}); ok {
								address := models.LXCNetworkAddress{}
								if family, ok := addrMap["family"].(string); ok {
									address.Family = family
								}
								if addrStr, ok := addrMap["address"].(string); ok {
									address.Address = addrStr
								}
								if netmask, ok := addrMap["netmask"].(string); ok {
									address.Netmask = netmask
								}
								if scope, ok := addrMap["scope"].(string); ok {
									address.Scope = scope
								}
								netState.Addresses = append(netState.Addresses, address)
							}
						}
					}
					
					// 解析计数器
					if counters, ok := ifaceData["counters"].(map[string]interface{}); ok {
						counter := models.LXCNetworkCounters{}
						if bytesReceived, ok := counters["bytes_received"].(float64); ok {
							counter.BytesReceived = int64(bytesReceived)
						}
						if bytesSent, ok := counters["bytes_sent"].(float64); ok {
							counter.BytesSent = int64(bytesSent)
						}
						if packetsReceived, ok := counters["packets_received"].(float64); ok {
							counter.PacketsReceived = int64(packetsReceived)
						}
						if packetsSent, ok := counters["packets_sent"].(float64); ok {
							counter.PacketsSent = int64(packetsSent)
						}
						netState.Counters = counter
					}
					
					// 其他网络属性
					if hwaddr, ok := ifaceData["hwaddr"].(string); ok {
						netState.Hwaddr = hwaddr
					}
					if mtu, ok := ifaceData["mtu"].(float64); ok {
						netState.Mtu = int(mtu)
					}
					if state, ok := ifaceData["state"].(string); ok {
						netState.State = state
					}
					if ifaceType, ok := ifaceData["type"].(string); ok {
						netState.Type = ifaceType
					}
					
					networkState[iface] = netState
				}
			}
			container.NetworkState = networkState
			
			// 提取IP地址
			var ips []string
			for _, netState := range networkState {
				for _, addr := range netState.Addresses {
					if (addr.Family == "inet" || addr.Family == "inet6") && addr.Scope == "global" {
						ips = append(ips, addr.Address)
					}
				}
			}
			container.IPAddresses = ips
		}

		// 从state中提取资源信息
		resources := make(map[string]interface{})
		if cpu, ok := stateData["cpu"].(map[string]interface{}); ok {
			resources["cpu"] = cpu
		}
		if memory, ok := stateData["memory"].(map[string]interface{}); ok {
			resources["memory"] = memory
		}
		if disk, ok := stateData["disk"].(map[string]interface{}); ok {
			resources["disk"] = disk
		}
		if len(resources) > 0 {
			container.Resources = resources
		}
	}

	return &container, nil
}


func convertLXCContainerToContainerInfo(lxcContainer models.LXCContainerState) models.ContainerInfo {
	// 提取镜像信息
	image := extractImageInfo(lxcContainer)
	
	return models.ContainerInfo{
		Name:         lxcContainer.Name,
		Status:       lxcContainer.Status,
		StatusCode:   lxcContainer.StatusCode,
		Type:         lxcContainer.Type,
		Architecture: lxcContainer.Architecture,
		Description:  lxcContainer.Description,
		Ephemeral:    lxcContainer.Ephemeral,
		Profiles:     lxcContainer.Profiles,
		Stateful:     lxcContainer.Stateful,
		Project:      lxcContainer.Project,
		Location:     lxcContainer.Location,
		Config:       lxcContainer.Config,
		Devices:      lxcContainer.Devices,
		CreatedAt:    lxcContainer.CreatedAt,
		LastUsedAt:   lxcContainer.LastUsedAt,
		Image:        image,
	}
}

// extractImageInfo 从容器配置中提取镜像信息 - 简化版，只获取 image.description
func extractImageInfo(lxcContainer models.LXCContainerState) string {
	// 优先从 expanded_config 获取 image.description
	if expandedConfig := lxcContainer.ExpandedConfig; expandedConfig != nil {
		if imageDesc, ok := expandedConfig["image.description"].(string); ok && imageDesc != "" {
			return imageDesc
		}
	}
	
	// 其次从 config 获取 image.description
	if containerConfig := lxcContainer.Config; containerConfig != nil {
		if imageDesc, ok := containerConfig["image.description"].(string); ok && imageDesc != "" {
			return imageDesc
		}
	}
	
	// 如果都没有，返回空字符串
	return ""
}


// getContainerState 获取容器的完整状态信息 (网络、资源等) - 统一查询，避免重复
func getContainerState(ctx context.Context, containerName string) (map[string]interface{}, error) {
	cmd := exec.CommandContext(ctx, "lxc", "query", fmt.Sprintf("/1.0/containers/%s/state", containerName))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("获取容器状态失败: %v", err)
	}

	var state map[string]interface{}
	if err := json.Unmarshal(output, &state); err != nil {
		return nil, fmt.Errorf("解析状态JSON失败: %v", err)
	}

	return state, nil
}

// GetContainerState 导出的获取容器状态的函数（用于handler等外部调用）
func GetContainerState(containerName string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), config.GetLXCTimeout())
	defer cancel()
	return getContainerState(ctx, containerName)
}

