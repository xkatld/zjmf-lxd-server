package models

import (
	"gorm.io/gorm"
)


type ContainerConfig struct {
	gorm.Model
	ContainerName string `gorm:"index:uidx_container_name,unique;size:500"`
	Image         string `gorm:"size:500"`
	OriginalImage string `gorm:"size:500"`
	Password      string `gorm:"size:500" json:"-"`
	Status        string `gorm:"size:100"`
	CreatedBy     string `gorm:"size:255"`

	// 资源配置
	CPUs int

	Memory string `gorm:"size:100"`

	Disk string `gorm:"size:100"`

	TrafficLimit int

	// 高级配置
	CPUAllowance string `gorm:"size:100;default:'100%'"`

	// 内存高级配置
	MemorySwap bool `gorm:"default:true"`

	// 磁盘高级配置
	DiskIOLimit string `gorm:"size:100"`

	// 网络配置
	Ingress string `gorm:"size:100"`

	Egress string `gorm:"size:100"`

	// 进程和文件限制
	MaxProcesses int `gorm:"default:512"`

	// 安全配置
	AllowNesting bool `gorm:"default:false"`

	Privileged bool `gorm:"default:false"`
	
	// 网络模式
	NetworkMode string `gorm:"size:50;default:'mode1'"`
	
	// LXCFS 配置
	EnableLXCFS bool `gorm:"default:true"`
}


type LXCContainerState struct {
	Name            string         `json:"name"`
	Status          string         `json:"status"`
	StatusCode      int            `json:"status_code"`
	Architecture    string         `json:"architecture"`
	Type            string         `json:"type"`
	Description     string         `json:"description"`
	Ephemeral       bool           `json:"ephemeral"`
	Config          map[string]any `json:"config"`
	Devices         map[string]any `json:"devices"`
	ExpandedConfig  map[string]any `json:"expanded_config"`
	ExpandedDevices map[string]any `json:"expanded_devices"`
	Profiles        []string       `json:"profiles"`
	Stateful        bool           `json:"stateful"`
	CreatedAt       string         `json:"created_at"`
	LastUsedAt      string         `json:"last_used_at"`
	Location        string         `json:"location"`
	Project         string         `json:"project"`
}


type LXCNetworkState struct {
	Addresses []LXCNetworkAddress `json:"addresses"`
	Counters  LXCNetworkCounters  `json:"counters"`
	Hwaddr    string              `json:"hwaddr"`
	Mtu       int                 `json:"mtu"`
	State     string              `json:"state"`
	Type      string              `json:"type"`
}


type LXCNetworkAddress struct {
	Family  string `json:"family"`
	Address string `json:"address"`
	Netmask string `json:"netmask"`
	Scope   string `json:"scope"`
}


type LXCNetworkCounters struct {
	BytesReceived   int64 `json:"bytes_received"`
	BytesSent       int64 `json:"bytes_sent"`
	PacketsReceived int64 `json:"packets_received"`
	PacketsSent     int64 `json:"packets_sent"`
}


type ContainerRealTimeStats struct {

	ContainerName string `json:"container_name"`
	// 容器状态
	Status string `json:"status"`

	PID int `json:"pid"`

	Processes int `json:"processes"`



	CPUUsagePercent float64 `json:"cpu_usage_percent,omitempty"` // CPU使用率百分比


	MemoryUsedMB       float64 `json:"memory_used_mb"`       // 当前内存使用
	MemoryTotalMB      float64 `json:"memory_total_mb"`      // 内存限制总量
	MemoryUsagePercent float64 `json:"memory_usage_percent"` // 内存使用率百分比
	MemorySwapUsedMB   float64 `json:"memory_swap_used_mb"`  // Swap使用量
	MemoryUsagePeakMB  float64 `json:"memory_usage_peak_mb"` // 内存使用峰值


	DiskUsedMB       float64 `json:"disk_used_mb"`       // 磁盘已用空间
	DiskTotalGB      float64 `json:"disk_total_gb"`      // 磁盘总容量
	DiskUsagePercent float64 `json:"disk_usage_percent"` // 磁盘使用率百分比


	NetworkRxKB      float64 `json:"network_rx_kb"`      // 接收流量
	NetworkTxKB      float64 `json:"network_tx_kb"`      // 发送流量
	NetworkRxPackets int64   `json:"network_rx_packets"` // 接收包数量
	NetworkTxPackets int64   `json:"network_tx_packets"` // 发送包数量
	NetworkRxErrors  int64   `json:"network_rx_errors"`  // 接收错误包数
	NetworkTxErrors  int64   `json:"network_tx_errors"`  // 发送错误包数
	NetworkRxDropped int64   `json:"network_rx_dropped"` // 接收丢包数
	NetworkTxDropped int64   `json:"network_tx_dropped"` // 发送丢包数


	Timestamp string `json:"timestamp"` // 数据采样时间
}


type ContainerInfo struct {
	// 容器名
	Name string `json:"name"`
	// 容器状态
	Status string `json:"status"`

	StatusCode int `json:"status_code"`

	Type string `json:"type"`

	Architecture string `json:"architecture"`

	Description string `json:"description"`

	Ephemeral bool `json:"ephemeral"`

	Profiles []string `json:"profiles"`

	Stateful bool `json:"stateful"`

	Project string `json:"project"`

	Location string `json:"location"`

	IPAddresses []string `json:"ip_addresses"`

	Image string `json:"image,omitempty"` // 镜像信息

	NetworkState map[string]LXCNetworkState `json:"network_state,omitempty"`

	Resources map[string]any `json:"resources,omitempty"`

	Config map[string]any `json:"config,omitempty"`

	Devices map[string]any `json:"devices,omitempty"`

	CreatedAt string `json:"created_at"`

	LastUsedAt string `json:"last_used_at"`
}


type CreateContainerRequest struct {
	// 容器名
	Name string `json:"name" example:"test-container"`

	CPUs int `json:"cpus" example:"2"`
	// 3. 内存(支持单位)
	Memory string `json:"memory" example:"2048MB"`
	// 4. 硬盘(支持单位)
	Disk string `json:"disk" example:"10GB"`
	// 5. 上行带宽(支持单位) - 容器发送数据限制
	Egress string `json:"egress" example:"100Mbit"`
	// 6. 下行带宽(支持单位) - 容器接收数据限制
	Ingress string `json:"ingress" example:"100Mbit"`
	// 8. 允许嵌套
	AllowNesting bool `json:"allow_nesting" example:"false"`
	// 9. 镜像
	Image string `json:"image" example:"ubuntu:20.04"`
	// 流量限制(GB/月)，0表示无限制
	TrafficLimit int `json:"traffic_limit" example:"100"`
	
	// LXCFS 配置
	EnableLXCFS bool `json:"enable_lxcfs" example:"true"`
}

// LXDServerCreateRequest lxdserver插件兼容的创建容器请求结构
type LXDServerCreateRequest struct {
	// 容器主机名（对应容器名）
	Hostname string `json:"hostname" binding:"required" example:"test-container"`
	// root密码
	Password string `json:"password" example:"password123"`

	CPUs int `json:"cpus" example:"2"`
	// 内存(支持单位)
	Memory string `json:"memory" example:"256MB"`
	// 硬盘(支持单位)
	Disk string `json:"disk" example:"512MB"`
	// 镜像
	Image string `json:"image" example:"debian/12"`
	// 上行带宽(支持单位) - 容器发送数据限制
	Ingress string `json:"ingress" example:"100Mbit"`
	// 下行带宽(支持单位) - 容器接收数据限制
	Egress string `json:"egress" example:"100Mbit"`
	// 允许嵌套
	AllowNesting bool `json:"allow_nesting" example:"false"`
	TrafficLimit int `json:"traffic_limit" example:"100"`
	
	NetworkMode string `json:"network_mode" example:"mode1"`
	
	// LXCFS 配置
	EnableLXCFS bool `json:"enable_lxcfs" example:"true"`

	// CPU 高级配置
	CPUAllowance string `json:"cpu_allowance" example:"50%"` // CPU使用率限制

	// 内存高级配置
	MemorySwap bool `json:"memory_swap" example:"true"` // 是否允许使用Swap

	// 进程和文件限制
	MaxProcesses int `json:"max_processes" example:"512"` // 最大进程数

	// 磁盘IO限制
	DiskIOLimit string `json:"disk_io_limit" example:"100MB"` // 磁盘IO限速



	Privileged bool `json:"privileged" example:"false"` // 是否允许特权模式
}

// LXDServerCreateResponse lxdserver插件兼容的创建响应结构
type LXDServerCreateResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		DedicatedIP  string `json:"dedicatedip,omitempty"`
		AssignedIPs  string `json:"assignedips,omitempty"`
	} `json:"data,omitempty"`
}

// LXDServerResponse lxdserver插件兼容的通用响应结构
type LXDServerResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

// LXDServerReinstallRequest lxdserver插件兼容的重装系统请求结构
type LXDServerReinstallRequest struct {
	// 容器主机名
	Hostname string `json:"hostname" binding:"required" example:"test-container"`
	// 新的操作系统镜像
	System string `json:"system" binding:"required" example:"debian/12"`
	// 新的root密码
	Password string `json:"password" example:"newpassword123"`

	// 基础配置（重装时重新应用）
	CPUs         int    `json:"cpus" example:"2"`
	Memory       string `json:"memory" example:"256MB"`
	Disk         string `json:"disk" example:"512MB"`
	Ingress      string `json:"ingress" example:"100Mbit"`
	Egress       string `json:"egress" example:"100Mbit"`
	AllowNesting bool `json:"allow_nesting" example:"false"`
	TrafficLimit int  `json:"traffic_limit" example:"100"`

	// CPU 高级配置
	CPUAllowance string `json:"cpu_allowance" example:"50%"`

	// 内存高级配置
	MemorySwap bool `json:"memory_swap" example:"true"`

	// 进程和文件限制
	MaxProcesses int `json:"max_processes" example:"512"`

	// 磁盘IO限制
	DiskIOLimit string `json:"disk_io_limit" example:"100MB"`



	Privileged bool `json:"privileged" example:"false"`
	
	// LXCFS 配置
	EnableLXCFS bool `json:"enable_lxcfs" example:"true"`
}



// UpdateContainerConfigRequest 升降配置请求结构
type UpdateContainerConfigRequest struct {
	// 1. 核心数（可选）
	CPUs *int `json:"cpus,omitempty" example:"4"`
	// 3. 内存(支持单位)（可选）
	Memory *string `json:"memory,omitempty" example:"4096MB"`
	// 4. 硬盘(支持单位)（可选）
	Disk *string `json:"disk,omitempty" example:"20GB"`
	// 5. 上行带宽(支持单位)（可选）
	Egress *string `json:"egress,omitempty" example:"100Mbit"`
	// 6. 下行带宽(支持单位)（可选）
	Ingress *string `json:"ingress,omitempty" example:"100Mbit"`
	// 8. 允许嵌套（可选）
	AllowNesting *bool `json:"allow_nesting,omitempty" example:"true"`
}

// LXDServerResetPasswordRequest lxdserver插件兼容的重置密码请求结构
type LXDServerResetPasswordRequest struct {
	Hostname string `json:"hostname" binding:"required" example:"test-container"`
	Password string `json:"password" binding:"required" example:"newpassword123"`
}
