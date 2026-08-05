package executors

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"lxdapi/config"
	"lxdapi/errors"
	"lxdapi/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// IPv6BindingManager 接口定义
type IPv6BindingManager interface {
	AddIPv6Binding(containerName, containerIPv6 string) (string, error)
	RemoveIPv6Binding(publicIPv6 string) error
	GetIPv6BindingsByContainer(containerName string) ([]models.IPv6BindingRule, error)
	ActivatePreAllocatedIPv6(containerName, containerIPv6 string) error
}

// 全局管理器变量（由 main.go 初始化）
var (
	GlobalIPv6Manager IPv6BindingManager
)

type CreateContainerExecutor struct {
	*BaseExecutor
	CreateRequest models.LXDServerCreateRequest
}

func NewCreateContainerExecutor(db *gorm.DB, logger *zap.Logger, task *models.Task) (*CreateContainerExecutor, error) {
	var req models.LXDServerCreateRequest
	if task.Data != "" {
		if err := json.Unmarshal([]byte(task.Data), &req); err != nil {
			return nil, fmt.Errorf("parse create request failed: %v", err)
		}
	}
	
	return &CreateContainerExecutor{
		BaseExecutor:  NewBaseExecutor(db, logger, task),
		CreateRequest: req,
	}, nil
}

func (e *CreateContainerExecutor) Execute() error {
	ctx := context.Background()
	
	e.StartStep("STEP_1", "Check if container already exists")
	exists, err := e.checkContainerExists(ctx)
	if err != nil {
		e.CompleteStep(false, errors.ERR_LXC_LIST_FAIL, fmt.Sprintf("Check container failed: %v", err))
		return err
	}
	if exists {
		e.CompleteStep(false, errors.ERR_CONTAINER_ALREADY_EXISTS, fmt.Sprintf("Container %s already exists", e.ContainerName))
		return fmt.Errorf("container already exists")
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, "Container does not exist, can proceed")
	
	e.StartStep("STEP_2", "Create LXD container")
	if err := e.createLXDContainer(ctx); err != nil {
		e.CompleteStep(false, errors.ERR_LXC_CREATE_FAIL, fmt.Sprintf("Create container failed: %v", err))
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, "Container created successfully")
	
	e.StartStep("STEP_3", "Start container")
	if err := e.startContainer(ctx); err != nil {
		e.CompleteStep(false, errors.ERR_LXC_START_FAIL, fmt.Sprintf("Start container failed: %v", err))
		e.cleanupContainer(ctx)
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, "Container started successfully")
	
	e.StartStep("STEP_4", "Wait for container IP address")
	time.Sleep(5 * time.Second)
	e.CompleteStep(true, errors.ERR_SUCCESS, "Wait completed")
	
	e.StartStep("STEP_5", "Configure password and hostname")
	time.Sleep(5 * time.Second)
	if err := e.configureContainer(ctx); err != nil {
		e.CompleteStep(false, errors.ERR_PASSWORD_RESET_FAIL, fmt.Sprintf("Configure container failed: %v", err))
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, "Container configured successfully")
	}
	
	e.StartStep("STEP_6", "Save container configuration to database")
	if err := e.saveContainerConfig(); err != nil {
		e.CompleteStep(false, errors.ERR_DB_INSERT_FAIL, fmt.Sprintf("Save config failed: %v", err))
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, "Configuration saved to database")
	
	e.StartStep("STEP_7", "Initialize container info cache")
	if err := e.initializeContainerCache(ctx); err != nil {
		e.LogInfo("Initialize cache failed (non-critical)", zap.Error(err))
		e.CompleteStep(false, errors.ERR_SUCCESS, "Cache initialization failed but not critical")
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, "Container cache initialized")
	}
	
	e.StartStep("STEP_8", "Allocate independent IP addresses")
	if err := e.allocateIndependentIPs(); err != nil {
		e.LogInfo("Allocate independent IPs failed (non-critical)", zap.Error(err))
		e.CompleteStep(false, errors.ERR_SUCCESS, "Independent IP allocation failed but not critical")
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, "Independent IPs allocated successfully")
	}
	
	return nil
}

func (e *CreateContainerExecutor) checkContainerExists(ctx context.Context) (bool, error) {
	cmd := exec.CommandContext(ctx, "lxc", "list", e.ContainerName, "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("lxc list failed: %v, output: %s", err, string(output))
	}
	
	var containers []map[string]interface{}
	if err := json.Unmarshal(output, &containers); err != nil {
		return false, fmt.Errorf("parse lxc list output failed: %v", err)
	}
	
	return len(containers) > 0, nil
}

func (e *CreateContainerExecutor) createLXDContainer(ctx context.Context) error {
	storagePools := config.GetStoragePools()
	var lastErr error
	
	for _, pool := range storagePools {
		e.LogInfo("Trying storage pool", zap.String("pool", pool))
		
		args := []string{"init", e.CreateRequest.Image, e.ContainerName}
		
		if e.CreateRequest.CPUs > 0 {
			args = append(args, "--config", fmt.Sprintf("limits.cpu=%d", e.CreateRequest.CPUs))
		}
		
		if e.CreateRequest.Memory != "" {
			args = append(args, "--config", fmt.Sprintf("limits.memory=%s", e.CreateRequest.Memory))
		}
		
		if e.CreateRequest.CPUAllowance != "" {
			args = append(args, "--config", fmt.Sprintf("limits.cpu.allowance=%s", e.CreateRequest.CPUAllowance))
		}
		
		args = append(args, "--config", fmt.Sprintf("limits.memory.swap=%v", e.CreateRequest.MemorySwap))
		
		if e.CreateRequest.MaxProcesses > 0 {
			args = append(args, "--config", fmt.Sprintf("limits.processes=%d", e.CreateRequest.MaxProcesses))
		}
		
		args = append(args, "--config", fmt.Sprintf("security.privileged=%v", e.CreateRequest.Privileged))
		args = append(args, "--config", fmt.Sprintf("security.nesting=%v", e.CreateRequest.AllowNesting))
		
		args = append(args, "--device", "eth0,type=nic")
		args = append(args, "--device", "eth0,network=lxdbr0")
		
		if e.CreateRequest.Ingress != "" {
			args = append(args, "--device", fmt.Sprintf("eth0,limits.ingress=%s", e.CreateRequest.Ingress))
		}
		if e.CreateRequest.Egress != "" {
			args = append(args, "--device", fmt.Sprintf("eth0,limits.egress=%s", e.CreateRequest.Egress))
		}
		
		if e.CreateRequest.Disk != "" {
			args = append(args, "--device", "root,path=/")
			args = append(args, "--device", fmt.Sprintf("root,pool=%s", pool))
			args = append(args, "--device", "root,type=disk")
			args = append(args, "--device", fmt.Sprintf("root,size=%s", e.CreateRequest.Disk))
			
			if e.CreateRequest.DiskIOLimit != "" {
				args = append(args, "--device", fmt.Sprintf("root,limits.read=%s", e.CreateRequest.DiskIOLimit))
				args = append(args, "--device", fmt.Sprintf("root,limits.write=%s", e.CreateRequest.DiskIOLimit))
			}
		}
		
		cmd := exec.CommandContext(ctx, "lxc", args...)
		output, err := cmd.CombinedOutput()
		
		if err == nil {
			e.LogInfo("Container created with storage pool", zap.String("pool", pool), zap.String("output", string(output)))
			return nil
		}
		
		lastErr = fmt.Errorf("pool %s failed: %v, output: %s", pool, err, string(output))
		e.LogError("Storage pool failed", zap.String("pool", pool), zap.Error(err), zap.String("output", string(output)))
	}
	
	return fmt.Errorf("all storage pools failed, last error: %v", lastErr)
}

func (e *CreateContainerExecutor) startContainer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "lxc", "start", e.ContainerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lxc start failed: %v, output: %s", err, string(output))
	}
	
	e.LogInfo("Container started", zap.String("output", string(output)))
	return nil
}

func (e *CreateContainerExecutor) getContainerIP() (string, error) {
	cmd := exec.Command("lxc", "list", e.ContainerName, "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("lxc list failed: %v", err)
	}
	
	var containers []struct {
		State struct {
			Network map[string]struct {
				Addresses []struct {
					Family  string `json:"family"`
					Address string `json:"address"`
				} `json:"addresses"`
			} `json:"network"`
		} `json:"state"`
	}
	
	if err := json.Unmarshal(output, &containers); err != nil {
		return "", fmt.Errorf("parse json failed: %v", err)
	}
	
	if len(containers) == 0 {
		return "", fmt.Errorf("container not found")
	}
	
	for _, netInfo := range containers[0].State.Network {
		for _, addr := range netInfo.Addresses {
			if addr.Family == "inet" && !strings.HasPrefix(addr.Address, "127.") {
				return addr.Address, nil
			}
		}
	}
	
	return "", fmt.Errorf("no IPv4 address found")
}

func (e *CreateContainerExecutor) getContainerIPv6() (string, error) {
	cmd := exec.Command("lxc", "list", e.ContainerName, "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("lxc list failed: %v", err)
	}
	
	var containers []struct {
		State struct {
			Network map[string]struct {
				Addresses []struct {
					Family  string `json:"family"`
					Address string `json:"address"`
					Scope   string `json:"scope"`
				} `json:"addresses"`
			} `json:"network"`
		} `json:"state"`
	}
	
	if err := json.Unmarshal(output, &containers); err != nil {
		return "", fmt.Errorf("parse json failed: %v", err)
	}
	
	if len(containers) == 0 {
		return "", fmt.Errorf("container not found")
	}
	
	// 查找全局IPv6地址（非link-local）
	for _, netInfo := range containers[0].State.Network {
		for _, addr := range netInfo.Addresses {
			if addr.Family == "inet6" && addr.Scope == "global" {
				return addr.Address, nil
			}
		}
	}
	
	return "", fmt.Errorf("no global IPv6 address found")
}

func (e *CreateContainerExecutor) configureContainer(ctx context.Context) error {
	if e.CreateRequest.Password == "" {
		return nil
	}
	
	hostnameCmd := exec.CommandContext(ctx, "lxc", "exec", e.ContainerName, "--", "sh", "-c",
		fmt.Sprintf("hostname %s && echo %s > /etc/hostname 2>/dev/null || true", e.ContainerName, e.ContainerName))
	if output, err := hostnameCmd.CombinedOutput(); err != nil {
		e.LogInfo("Set hostname skipped (non-critical)", zap.String("output", string(output)))
	} else {
		e.LogInfo("Hostname set", zap.String("hostname", e.ContainerName))
	}
	
	passwdCmd := exec.CommandContext(ctx, "lxc", "exec", e.ContainerName, "--", "sh", "-c",
		fmt.Sprintf("echo 'root:%s' | chpasswd", e.CreateRequest.Password))
	output, err := passwdCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("set password failed: %v, output: %s", err, string(output))
	}
	
	e.LogInfo("Password configured", zap.String("output", string(output)))
	return nil
}

func (e *CreateContainerExecutor) saveContainerConfig() error {
	containerConfig := models.ContainerConfig{
		ContainerName:  e.ContainerName,
		CPUs:           e.CreateRequest.CPUs,
		Memory:         e.CreateRequest.Memory,
		Disk:           e.CreateRequest.Disk,
		Egress:         e.CreateRequest.Egress,
		Ingress:        e.CreateRequest.Ingress,
		AllowNesting:   e.CreateRequest.AllowNesting,
		Image:          e.CreateRequest.Image,
		OriginalImage:  e.CreateRequest.Image,
		Password:       e.CreateRequest.Password,
		Status:         "Active",
		CreatedBy:      "lxdserver",
		TrafficLimit:   e.CreateRequest.TrafficLimit,
		CPUAllowance:   e.CreateRequest.CPUAllowance,
		MemorySwap:     e.CreateRequest.MemorySwap,
		MaxProcesses:   e.CreateRequest.MaxProcesses,
		DiskIOLimit:    e.CreateRequest.DiskIOLimit,
		Privileged:     e.CreateRequest.Privileged,
		NetworkMode:    e.CreateRequest.NetworkMode,
	}
	
	if err := e.DB.Create(&containerConfig).Error; err != nil {
		return fmt.Errorf("save container config failed: %v", err)
	}
	
	e.LogInfo("Container config saved to database")
	return nil
}

func (e *CreateContainerExecutor) cleanupContainer(ctx context.Context) {
	cmd := exec.CommandContext(ctx, "lxc", "delete", e.ContainerName, "--force")
	if output, err := cmd.CombinedOutput(); err != nil {
		e.LogError("Cleanup container failed", zap.Error(err), zap.String("output", string(output)))
	} else {
		e.LogInfo("Container cleaned up")
	}
}

func (e *CreateContainerExecutor) initializeContainerCache(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "lxc", "list", e.ContainerName, "--format", "json")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lxc list failed: %v", err)
	}
	
	var containers []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		State  struct {
			Network map[string]struct {
				Addresses []struct {
					Family  string `json:"family"`
					Address string `json:"address"`
					Scope   string `json:"scope"`
				} `json:"addresses"`
			} `json:"network"`
		} `json:"state"`
	}
	
	if err := json.Unmarshal(output, &containers); err != nil {
		return fmt.Errorf("parse json failed: %v", err)
	}
	
	if len(containers) == 0 {
		return fmt.Errorf("container not found")
	}
	
	container := containers[0]
	
	var ipv4, ipv6 string
	for _, netInfo := range container.State.Network {
		for _, addr := range netInfo.Addresses {
			if addr.Family == "inet" && !strings.HasPrefix(addr.Address, "127.") {
				ipv4 = addr.Address
			}
			if addr.Family == "inet6" && addr.Scope == "global" {
				ipv6 = addr.Address
			}
		}
	}
	
	cache := models.ContainerInfoCache{
		Hostname:     e.ContainerName,
		Status:       container.Status,
		Image:        e.CreateRequest.Image,
		IPv4:         ipv4,
		IPv6:         ipv6,
		CPUs:         e.CreateRequest.CPUs,
		Memory:       e.CreateRequest.Memory,
		Disk:         e.CreateRequest.Disk,
		TrafficLimit: e.CreateRequest.TrafficLimit,
		LastUpdate:   time.Now(),
	}
	
	result := e.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "hostname"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"status", "image", "ipv4", "ipv6",
			"cpus", "memory", "disk", "traffic_limit", "last_update",
		}),
	}).Create(&cache)
	
	if result.Error != nil {
		return fmt.Errorf("update cache failed: %v", result.Error)
	}
	
	e.LogInfo("Container cache initialized", zap.String("status", container.Status))
	return nil
}

func (e *CreateContainerExecutor) allocateIndependentIPs() error {
	networkMode := e.CreateRequest.NetworkMode
	
	if networkMode == "" {
		networkMode = "mode1"
	}
	
	e.LogInfo("Allocating independent IPs", zap.String("network_mode", networkMode))
	
	// 只支持 mode2: IPv4 NAT + IPv6 独立
	if networkMode == "mode2" {
		return e.allocateIPv6Only()
	}
	
	e.LogInfo("Network mode does not require independent IP allocation", zap.String("mode", networkMode))
	return nil
}

func (e *CreateContainerExecutor) allocateIPv6Only() error {
	e.LogInfo("Activating pre-allocated independent IPv6 for mode2")
	
	ipv6Manager := e.getIPv6Manager()
	if ipv6Manager == nil {
		return fmt.Errorf("IPv6 binding manager not initialized")
	}
	
	containerIPv6, err := e.getContainerIPv6()
	if err != nil {
		return fmt.Errorf("get container IPv6 failed: %v", err)
	}
	
	err = ipv6Manager.ActivatePreAllocatedIPv6(e.ContainerName, containerIPv6)
	if err != nil {
		return fmt.Errorf("activate pre-allocated IPv6 failed: %v", err)
	}
	
	e.LogInfo("Pre-allocated IPv6 activated successfully")
	return nil
}

func (e *CreateContainerExecutor) getIPv6Manager() IPv6BindingManager {
	return GlobalIPv6Manager
}

