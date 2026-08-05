package executors

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"lxdapi/config"
	"lxdapi/errors"
	"lxdapi/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type ReinstallContainerExecutor struct {
	*BaseExecutor
	ContainerConfig  models.ContainerConfig
	ReinstallRequest models.LXDServerReinstallRequest
}

func NewReinstallContainerExecutor(db *gorm.DB, logger *zap.Logger, task *models.Task) (*ReinstallContainerExecutor, error) {
	executor := &ReinstallContainerExecutor{
		BaseExecutor: NewBaseExecutor(db, logger, task),
	}
	
	if err := db.Where("container_name = ?", task.ContainerName).First(&executor.ContainerConfig).Error; err != nil {
		return nil, fmt.Errorf("get container config failed: %v", err)
	}
	
	// 解析重装请求数据
	if task.Data != "" {
		if err := json.Unmarshal([]byte(task.Data), &executor.ReinstallRequest); err != nil {
			return nil, fmt.Errorf("parse reinstall request failed: %v", err)
		}
	} else {
		// 如果没有传递数据，使用默认值（保持原配置）
		executor.ReinstallRequest = models.LXDServerReinstallRequest{
			Hostname:       task.ContainerName,
			System:         executor.ContainerConfig.Image,
			Password:       executor.ContainerConfig.Password,
			CPUs:           executor.ContainerConfig.CPUs,
			Memory:         executor.ContainerConfig.Memory,
			Disk:           executor.ContainerConfig.Disk,
			Ingress:        executor.ContainerConfig.Ingress,
			Egress:         executor.ContainerConfig.Egress,
			AllowNesting:   executor.ContainerConfig.AllowNesting,
			TrafficLimit:   executor.ContainerConfig.TrafficLimit,
			CPUAllowance:   executor.ContainerConfig.CPUAllowance,
			MemorySwap:     executor.ContainerConfig.MemorySwap,
			MaxProcesses:   executor.ContainerConfig.MaxProcesses,
			DiskIOLimit:    executor.ContainerConfig.DiskIOLimit,
			Privileged:     executor.ContainerConfig.Privileged,
		}
	}
	
	return executor, nil
}

func (e *ReinstallContainerExecutor) Execute() error {
	ctx := context.Background()
	
	e.StartStep("STEP_1", "Get old container IP")
	oldIP, err := e.getContainerIP()
	if err != nil {
		e.CompleteStep(false, errors.ERR_CONTAINER_NOT_FOUND, fmt.Sprintf("Get old IP failed: %v", err))
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, fmt.Sprintf("Old IP: %s", oldIP))
	
	e.StartStep("STEP_2", "Stop container")
	if err := e.stopContainer(ctx); err != nil {
		e.CompleteStep(false, errors.ERR_LXC_STOP_FAIL, fmt.Sprintf("Stop container failed: %v", err))
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, "Container stopped")
	}
	
	e.StartStep("STEP_3", "Query NAT rules")
	natRules, err := e.getNATRules()
	if err != nil {
		e.CompleteStep(false, errors.ERR_NAT_QUERY_FAIL, fmt.Sprintf("Query NAT rules failed: %v", err))
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, fmt.Sprintf("Found %d NAT rules", len(natRules)))
	}
	
	e.StartStep("STEP_4", "Delete NAT iptables rules")
	if err := e.deleteIptablesRules(natRules, oldIP); err != nil {
		e.CompleteStep(false, errors.ERR_NAT_CLEANUP_FAIL, fmt.Sprintf("Delete iptables rules failed: %v", err))
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, "Iptables rules deleted")
	}
	
	e.StartStep("STEP_5", "Query IPv6 bindings")
	ipv6Bindings, err := e.getIPv6Bindings()
	if err != nil {
		e.CompleteStep(false, errors.ERR_IPV6_QUERY_FAIL, fmt.Sprintf("Query IPv6 bindings failed: %v", err))
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, fmt.Sprintf("Found %d IPv6 bindings", len(ipv6Bindings)))
	}
	
	e.StartStep("STEP_6", "Delete IPv6 routes")
	if err := e.deleteIPv6Routes(ipv6Bindings); err != nil {
		e.CompleteStep(false, errors.ERR_IPV6_CLEANUP_FAIL, fmt.Sprintf("Delete IPv6 routes failed: %v", err))
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, "IPv6 routes deleted")
	}
	
	e.StartStep("STEP_7", "Query proxy rules")
	proxyRules, err := e.getProxyRules()
	if err != nil {
		e.CompleteStep(false, errors.ERR_PROXY_RULE_NOT_FOUND, fmt.Sprintf("Query proxy rules failed: %v", err))
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, fmt.Sprintf("Found %d proxy rules", len(proxyRules)))
	}
	
	e.StartStep("STEP_8", "Delete old container")
	if err := e.deleteContainer(ctx); err != nil {
		e.CompleteStep(false, errors.ERR_LXC_DELETE_FAIL, fmt.Sprintf("Delete container failed: %v", err))
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, "Old container deleted")
	
	e.StartStep("STEP_9", "Create new container")
	if err := e.createNewContainer(ctx); err != nil {
		e.CompleteStep(false, errors.ERR_LXC_CREATE_FAIL, fmt.Sprintf("Create new container failed: %v", err))
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, "New container created")
	
	e.StartStep("STEP_10", "Start new container")
	if err := e.startContainer(ctx); err != nil {
		e.CompleteStep(false, errors.ERR_LXC_START_FAIL, fmt.Sprintf("Start container failed: %v", err))
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, "Container started")
	
	e.StartStep("STEP_11", "Wait for container IP")
	time.Sleep(5 * time.Second)
	e.CompleteStep(true, errors.ERR_SUCCESS, "Wait completed")
	
	e.StartStep("STEP_12", "Recreate NAT rules")
	if err := e.recreateNATRules(natRules); err != nil {
		e.CompleteStep(false, errors.ERR_NAT_RULE_ADD_FAIL, fmt.Sprintf("Recreate NAT rules failed: %v", err))
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, "NAT rules recreated")
	}
	
	e.StartStep("STEP_13", "Recreate IPv6 bindings")
	if err := e.recreateIPv6Bindings(ipv6Bindings); err != nil {
		e.CompleteStep(false, errors.ERR_IPV6_ROUTE_ADD_FAIL, fmt.Sprintf("Recreate IPv6 bindings failed: %v", err))
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, "IPv6 bindings recreated")
	}
	
	e.StartStep("STEP_14", "Recreate proxy rules")
	if err := e.recreateProxyRules(proxyRules); err != nil {
		e.CompleteStep(false, errors.ERR_PROXY_CONFIG_GENERATE, fmt.Sprintf("Recreate proxy rules failed: %v", err))
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, "Proxy rules recreated")
	}
	
	e.StartStep("STEP_15", "Configure password and hostname")
	time.Sleep(5 * time.Second)
	if err := e.configureContainer(ctx); err != nil {
		e.CompleteStep(false, errors.ERR_PASSWORD_RESET_FAIL, fmt.Sprintf("Configure container failed: %v", err))
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, "Container configured")
	}
	
	return nil
}

func (e *ReinstallContainerExecutor) stopContainer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "lxc", "stop", e.ContainerName, "--force")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lxc stop failed: %v, output: %s", err, string(output))
	}
	e.LogInfo("Container stopped", zap.String("output", string(output)))
	return nil
}

func (e *ReinstallContainerExecutor) getNATRules() ([]models.NATRule, error) {
	var natRules []models.NATRule
	if err := e.DB.Where("container_name = ?", e.ContainerName).Find(&natRules).Error; err != nil {
		return nil, fmt.Errorf("query NAT rules failed: %v", err)
	}
	return natRules, nil
}

func (e *ReinstallContainerExecutor) deleteIptablesRules(natRules []models.NATRule, containerIP string) error {
	for _, rule := range natRules {
		isPortRange := rule.ExternalPortEnd > 0 && rule.InternalPortEnd > 0
		
		if isPortRange {
			e.LogInfo("Deleting old NAT port range rule by comment", 
				zap.Int("external_port_start", rule.ExternalPort),
				zap.Int("external_port_end", rule.ExternalPortEnd),
				zap.Int("internal_port_start", rule.InternalPort),
				zap.Int("internal_port_end", rule.InternalPortEnd),
				zap.String("protocol", rule.Protocol),
				zap.String("ip_version", rule.IPVersion))
			
			comment := fmt.Sprintf("lxdapinat_%s_%d-%d_%d-%d_%s", e.ContainerName, rule.ExternalPort, rule.ExternalPortEnd, rule.InternalPort, rule.InternalPortEnd, rule.Protocol)
			
			e.cleanupRulesByCommentWithCmd("nat", "PREROUTING", comment, "iptables")
			e.cleanupRulesByCommentWithCmd("filter", "FORWARD", comment, "iptables")
			
			if rule.IPVersion == "ipv6" || rule.IPVersion == "dual" {
				comment6 := fmt.Sprintf("lxdapinat_ipv6_%s_%d-%d_%d-%d_%s", e.ContainerName, rule.ExternalPort, rule.ExternalPortEnd, rule.InternalPort, rule.InternalPortEnd, rule.Protocol)
				
				e.cleanupRulesByCommentWithCmd("nat", "PREROUTING", comment6, "ip6tables")
				e.cleanupRulesByCommentWithCmd("filter", "FORWARD", comment6, "ip6tables")
			}
		} else {
			e.LogInfo("Deleting old NAT rule by comment", 
				zap.Int("external_port", rule.ExternalPort),
				zap.Int("internal_port", rule.InternalPort),
				zap.String("protocol", rule.Protocol),
				zap.String("ip_version", rule.IPVersion))
			
			comment := fmt.Sprintf("lxdapinat_%s_%d_%d_%s", e.ContainerName, rule.ExternalPort, rule.InternalPort, rule.Protocol)
			
			e.cleanupRulesByCommentWithCmd("nat", "PREROUTING", comment, "iptables")
			e.cleanupRulesByCommentWithCmd("filter", "FORWARD", comment, "iptables")
			
			if rule.IPVersion == "ipv6" || rule.IPVersion == "dual" {
				comment6 := fmt.Sprintf("lxdapinat_ipv6_%s_%d_%d_%s", e.ContainerName, rule.ExternalPort, rule.InternalPort, rule.Protocol)
				
				e.cleanupRulesByCommentWithCmd("nat", "PREROUTING", comment6, "ip6tables")
				e.cleanupRulesByCommentWithCmd("filter", "FORWARD", comment6, "ip6tables")
			}
		}
	}
	
	e.LogInfo("All old NAT iptables rules deleted", zap.Int("count", len(natRules)))
	return nil
}

// cleanupRulesByCommentWithCmd 按comment查找并删除所有匹配的规则（按行号删除）
func (e *ReinstallContainerExecutor) cleanupRulesByCommentWithCmd(table, chain, comment, iptablesCmd string) {
	// 列出规则并获取行号
	cmd := exec.Command(iptablesCmd, "-t", table, "-L", chain, "--line-numbers", "-v")
	output, err := cmd.Output()
	if err != nil {
		e.LogInfo("List rules failed", zap.Error(err))
		return
	}
	
	lines := strings.Split(string(output), "\n")
	var lineNumbers []int
	
	for _, line := range lines {
		if strings.Contains(line, comment) {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				if num, err := strconv.Atoi(fields[0]); err == nil {
					lineNumbers = append(lineNumbers, num)
				}
			}
		}
	}
	
	// 从后往前删除，避免行号变化
	for i := len(lineNumbers) - 1; i >= 0; i-- {
		lineNum := strconv.Itoa(lineNumbers[i])
		delCmd := exec.Command(iptablesCmd, "-t", table, "-D", chain, lineNum)
		if output, err := delCmd.CombinedOutput(); err != nil {
			e.LogInfo("Delete rule by line number failed", 
				zap.String("line", lineNum),
				zap.String("output", string(output)))
		} else {
			e.LogInfo("Deleted rule by line number", 
				zap.String("table", table),
				zap.String("chain", chain),
				zap.String("line", lineNum),
				zap.String("comment", comment))
		}
	}
}

func (e *ReinstallContainerExecutor) getIPv6Bindings() ([]models.IPv6BindingRule, error) {
	var bindings []models.IPv6BindingRule
	if err := e.DB.Where("container_name = ?", e.ContainerName).Find(&bindings).Error; err != nil {
		return nil, fmt.Errorf("query IPv6 bindings failed: %v", err)
	}
	return bindings, nil
}

func (e *ReinstallContainerExecutor) deleteIPv6Routes(bindings []models.IPv6BindingRule) error {
	ipv6Config := config.AppConfig.IPv6Binding
	prefixLength := ipv6Config.IPv6Pool.PrefixLength
	
	for _, binding := range bindings {
		e.LogInfo("Deleting old IPv6 binding", 
			zap.String("public_ipv6", binding.PublicIPv6),
			zap.String("container_ipv6", binding.ContainerIPv6))
		
		dnatCmd := exec.Command("ip6tables", "-t", "nat", "-D", "PREROUTING", 
			"-d", binding.PublicIPv6, "-j", "DNAT", "--to-destination", binding.ContainerIPv6,
			"-m", "comment", "--comment", fmt.Sprintf("IPv6-BINDING-%s", binding.PublicIPv6))
		if output, err := dnatCmd.CombinedOutput(); err != nil {
			e.LogInfo("Delete IPv6 DNAT rule (ignore if not found)", zap.String("output", string(output)))
		}
		
		forwardCmd := exec.Command("ip6tables", "-D", "FORWARD", 
			"-d", binding.ContainerIPv6, "-j", "ACCEPT",
			"-m", "comment", "--comment", fmt.Sprintf("IPv6-BINDING-%s", binding.PublicIPv6))
		if output, err := forwardCmd.CombinedOutput(); err != nil {
			e.LogInfo("Delete IPv6 FORWARD rule (ignore if not found)", zap.String("output", string(output)))
		}
		
		cmdDel := exec.Command("ip", "-6", "route", "del", binding.PublicIPv6)
		if output, err := cmdDel.CombinedOutput(); err != nil {
			outputStr := string(output)
			if !strings.Contains(outputStr, "No such process") && !strings.Contains(outputStr, "not found") {
				e.LogInfo("Delete IPv6 route (ignore if not found)", zap.String("output", outputStr))
			}
		}
		
		ipv6WithPrefix := fmt.Sprintf("%s/%d", binding.PublicIPv6, prefixLength)
		cmdAddr := exec.Command("ip", "-6", "addr", "del", ipv6WithPrefix, "dev", binding.Interface)
		if output, err := cmdAddr.CombinedOutput(); err != nil {
			outputStr := string(output)
			if !strings.Contains(outputStr, "not found") && !strings.Contains(outputStr, "Cannot assign") {
				e.LogInfo("Delete IPv6 address (ignore if not found)", zap.String("output", outputStr))
			}
		}
	}
	
	e.LogInfo("All old IPv6 routes deleted", zap.Int("count", len(bindings)))
	return nil
}

func (e *ReinstallContainerExecutor) deleteContainer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "lxc", "delete", e.ContainerName, "--force")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lxc delete failed: %v, output: %s", err, string(output))
	}
	e.LogInfo("Container deleted", zap.String("output", string(output)))
	return nil
}

func (e *ReinstallContainerExecutor) createNewContainer(ctx context.Context) error {
	storagePools := config.GetStoragePools()
	var lastErr error
	
	// 使用重装请求中的镜像，而不是旧配置中的镜像
	newImage := e.ReinstallRequest.System
	if newImage == "" {
		newImage = e.ContainerConfig.Image
	}
	
	for _, pool := range storagePools {
		args := []string{"init", newImage, e.ContainerName}
		
		// 使用重装请求中的配置，如果没有则使用旧配置
		cpus := e.ReinstallRequest.CPUs
		if cpus == 0 {
			cpus = e.ContainerConfig.CPUs
		}
		if cpus > 0 {
			args = append(args, "--config", fmt.Sprintf("limits.cpu=%d", cpus))
		}
		
		memory := e.ReinstallRequest.Memory
		if memory == "" {
			memory = e.ContainerConfig.Memory
		}
		if memory != "" {
			args = append(args, "--config", fmt.Sprintf("limits.memory=%s", memory))
		}
		
		cpuAllowance := e.ReinstallRequest.CPUAllowance
		if cpuAllowance == "" {
			cpuAllowance = e.ContainerConfig.CPUAllowance
		}
		if cpuAllowance != "" {
			args = append(args, "--config", fmt.Sprintf("limits.cpu.allowance=%s", cpuAllowance))
		}
		
		memorySwap := e.ReinstallRequest.MemorySwap || e.ContainerConfig.MemorySwap
		args = append(args, "--config", fmt.Sprintf("limits.memory.swap=%v", memorySwap))
		
		maxProcesses := e.ReinstallRequest.MaxProcesses
		if maxProcesses == 0 {
			maxProcesses = e.ContainerConfig.MaxProcesses
		}
		if maxProcesses > 0 {
			args = append(args, "--config", fmt.Sprintf("limits.processes=%d", maxProcesses))
		}
		
		privileged := e.ReinstallRequest.Privileged || e.ContainerConfig.Privileged
		allowNesting := e.ReinstallRequest.AllowNesting || e.ContainerConfig.AllowNesting
		args = append(args, "--config", fmt.Sprintf("security.privileged=%v", privileged))
		args = append(args, "--config", fmt.Sprintf("security.nesting=%v", allowNesting))
		
		args = append(args, "--device", "eth0,type=nic")
		args = append(args, "--device", "eth0,network=lxdbr0")
		
		ingress := e.ReinstallRequest.Ingress
		if ingress == "" {
			ingress = e.ContainerConfig.Ingress
		}
		if ingress != "" {
			args = append(args, "--device", fmt.Sprintf("eth0,limits.ingress=%s", ingress))
		}
		
		egress := e.ReinstallRequest.Egress
		if egress == "" {
			egress = e.ContainerConfig.Egress
		}
		if egress != "" {
			args = append(args, "--device", fmt.Sprintf("eth0,limits.egress=%s", egress))
		}
		
		disk := e.ReinstallRequest.Disk
		if disk == "" {
			disk = e.ContainerConfig.Disk
		}
		if disk != "" {
			args = append(args, "--device", "root,path=/")
			args = append(args, "--device", fmt.Sprintf("root,pool=%s", pool))
			args = append(args, "--device", "root,type=disk")
			args = append(args, "--device", fmt.Sprintf("root,size=%s", disk))
			
			diskIOLimit := e.ReinstallRequest.DiskIOLimit
			if diskIOLimit == "" {
				diskIOLimit = e.ContainerConfig.DiskIOLimit
			}
			if diskIOLimit != "" {
				args = append(args, "--device", fmt.Sprintf("root,limits.read=%s", diskIOLimit))
				args = append(args, "--device", fmt.Sprintf("root,limits.write=%s", diskIOLimit))
			}
		}
		
		cmd := exec.CommandContext(ctx, "lxc", args...)
		output, err := cmd.CombinedOutput()
		
		if err == nil {
			e.LogInfo("Container created with storage pool", zap.String("pool", pool), zap.String("output", string(output)))
			return nil
		}
		
		lastErr = fmt.Errorf("pool %s failed: %v, output: %s", pool, err, string(output))
		e.LogError("Storage pool failed", zap.String("pool", pool), zap.Error(err))
	}
	
	return fmt.Errorf("all storage pools failed, last error: %v", lastErr)
}

func (e *ReinstallContainerExecutor) startContainer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "lxc", "start", e.ContainerName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lxc start failed: %v, output: %s", err, string(output))
	}
	e.LogInfo("Container started", zap.String("output", string(output)))
	return nil
}

func (e *ReinstallContainerExecutor) recreateNATRules(oldRules []models.NATRule) error {
	natConfig := config.AppConfig.NATAPI.Network
	externalInterface := natConfig.ExternalInterface
	internalInterface := natConfig.InternalInterface
	
	for _, rule := range oldRules {
		isPortRange := rule.ExternalPortEnd > 0 && rule.InternalPortEnd > 0
		
		if isPortRange {
			e.LogInfo("Recreating NAT port range rule with new IP", 
				zap.Int("external_port_start", rule.ExternalPort),
				zap.Int("external_port_end", rule.ExternalPortEnd),
				zap.Int("internal_port_start", rule.InternalPort),
				zap.Int("internal_port_end", rule.InternalPortEnd),
				zap.String("protocol", rule.Protocol),
				zap.String("ip_version", rule.IPVersion))
		} else {
			e.LogInfo("Recreating NAT rule with new IP", 
				zap.Int("external_port", rule.ExternalPort),
				zap.Int("internal_port", rule.InternalPort),
				zap.String("protocol", rule.Protocol),
				zap.String("ip_version", rule.IPVersion))
		}
		
		// 根据rule.IPVersion决定创建哪种规则
		if rule.IPVersion == "ipv4" || rule.IPVersion == "dual" {
			// 创建IPv4规则
			if len(natConfig.ExternalIPs) == 0 {
				e.LogError("No external IPv4 configured, skip IPv4 NAT rule")
				continue
			}
			
			newIP, err := e.getContainerIP()
			if err != nil {
				e.LogError("Get new container IPv4 failed", zap.Error(err))
				if rule.IPVersion == "ipv4" {
					continue
				}
			} else {
				externalIP := natConfig.ExternalIPs[0]
				var comment, dnatCmd, forwardCmd string
				
				if isPortRange {
					comment = fmt.Sprintf("lxdapinat_%s_%d-%d_%d-%d_%s", e.ContainerName, rule.ExternalPort, rule.ExternalPortEnd, rule.InternalPort, rule.InternalPortEnd, rule.Protocol)
					
					dnatCmd = fmt.Sprintf("iptables -t nat -A PREROUTING -i %s -d %s -p %s --dport %d:%d -j DNAT --to-destination %s:%d-%d -m comment --comment '%s'",
						externalInterface, externalIP, rule.Protocol, rule.ExternalPort, rule.ExternalPortEnd, newIP, rule.InternalPort, rule.InternalPortEnd, comment)
					
					if internalInterface != "" {
						forwardCmd = fmt.Sprintf("iptables -A FORWARD -i %s -o %s -d %s -p %s --dport %d:%d -j ACCEPT -m comment --comment '%s'",
							externalInterface, internalInterface, newIP, rule.Protocol, rule.InternalPort, rule.InternalPortEnd, comment)
					} else {
						forwardCmd = fmt.Sprintf("iptables -A FORWARD -i %s -d %s -p %s --dport %d:%d -j ACCEPT -m comment --comment '%s'",
							externalInterface, newIP, rule.Protocol, rule.InternalPort, rule.InternalPortEnd, comment)
					}
				} else {
					comment = fmt.Sprintf("lxdapinat_%s_%d_%d_%s", e.ContainerName, rule.ExternalPort, rule.InternalPort, rule.Protocol)
					
					dnatCmd = fmt.Sprintf("iptables -t nat -A PREROUTING -i %s -d %s -p %s --dport %d -j DNAT --to-destination %s:%d -m comment --comment '%s'",
						externalInterface, externalIP, rule.Protocol, rule.ExternalPort, newIP, rule.InternalPort, comment)
					
					if internalInterface != "" {
						forwardCmd = fmt.Sprintf("iptables -A FORWARD -i %s -o %s -d %s -p %s --dport %d -j ACCEPT -m comment --comment '%s'",
							externalInterface, internalInterface, newIP, rule.Protocol, rule.InternalPort, comment)
					} else {
						forwardCmd = fmt.Sprintf("iptables -A FORWARD -i %s -d %s -p %s --dport %d -j ACCEPT -m comment --comment '%s'",
							externalInterface, newIP, rule.Protocol, rule.InternalPort, comment)
					}
				}
				
				cmd := exec.Command("sh", "-c", dnatCmd)
				if output, err := cmd.CombinedOutput(); err != nil {
					e.LogError("Create IPv4 DNAT rule failed", zap.Error(err), zap.String("output", string(output)))
					if rule.IPVersion == "ipv4" {
						continue
					}
				} else {
					cmd2 := exec.Command("sh", "-c", forwardCmd)
					if output, err := cmd2.CombinedOutput(); err != nil {
						e.LogError("Create IPv4 FORWARD rule failed", zap.Error(err), zap.String("output", string(output)))
					} else {
						e.LogInfo("IPv4 NAT rule recreated", zap.String("new_ipv4", newIP))
					}
				}
			}
		}
	}
	
	e.LogInfo("All NAT rules recreated", zap.Int("count", len(oldRules)))
	return nil
}

func (e *ReinstallContainerExecutor) getContainerIP() (string, error) {
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

func (e *ReinstallContainerExecutor) recreateIPv6Bindings(oldBindings []models.IPv6BindingRule) error {
	if len(oldBindings) == 0 {
		e.LogInfo("No IPv6 bindings to recreate")
		return nil
	}
	
	newIPv6, err := e.getContainerIPv6()
	if err != nil {
		e.LogError("Get new container IPv6 failed", zap.Error(err))
		return err
	}
	
	// 获取前缀长度
	ipv6Config := config.AppConfig.IPv6Binding
	prefixLength := ipv6Config.IPv6Pool.PrefixLength
	
	for _, binding := range oldBindings {
		e.LogInfo("Recreating IPv6 binding with new IP", 
			zap.String("public_ipv6", binding.PublicIPv6),
			zap.String("new_ipv6", newIPv6))
		
		// 添加 IPv6 地址时带上前缀长度
		ipv6WithPrefix := fmt.Sprintf("%s/%d", binding.PublicIPv6, prefixLength)
		cmdAddr := fmt.Sprintf("ip -6 addr add %s dev %s 2>&1", ipv6WithPrefix, binding.Interface)
		cmd := exec.Command("sh", "-c", cmdAddr)
		if output, err := cmd.CombinedOutput(); err != nil {
			outputStr := string(output)
			if !strings.Contains(outputStr, "already assigned") && !strings.Contains(outputStr, "File exists") {
				e.LogError("Add IPv6 address failed", zap.Error(err), zap.String("output", outputStr))
				continue
			}
		}
		
		cmdRoute := fmt.Sprintf("ip -6 route add %s dev %s 2>&1", binding.PublicIPv6, binding.Interface)
		cmd2 := exec.Command("sh", "-c", cmdRoute)
		if output, err := cmd2.CombinedOutput(); err != nil {
			outputStr := string(output)
			if !strings.Contains(outputStr, "File exists") {
				e.LogError("Add IPv6 route failed", zap.Error(err), zap.String("output", outputStr))
			}
		}
		
		dnatCmd := fmt.Sprintf("ip6tables -t nat -A PREROUTING -d %s -j DNAT --to-destination %s -m comment --comment 'IPv6-BINDING-%s'",
			binding.PublicIPv6, newIPv6, binding.PublicIPv6)
		cmd3 := exec.Command("sh", "-c", dnatCmd)
		if output, err := cmd3.CombinedOutput(); err != nil {
			e.LogError("Add IPv6 DNAT rule failed", zap.Error(err), zap.String("output", string(output)))
		}
		
		forwardCmd := fmt.Sprintf("ip6tables -A FORWARD -d %s -j ACCEPT -m comment --comment 'IPv6-BINDING-%s'",
			newIPv6, binding.PublicIPv6)
		cmd4 := exec.Command("sh", "-c", forwardCmd)
		if output, err := cmd4.CombinedOutput(); err != nil {
			e.LogError("Add IPv6 FORWARD rule failed", zap.Error(err), zap.String("output", string(output)))
		}
		
		if err := e.DB.Model(&binding).Update("container_ipv6", newIPv6).Error; err != nil {
			e.LogError("Update IPv6 binding in database failed", zap.Error(err))
		}
	}
	
	e.LogInfo("All IPv6 bindings recreated with new IP", zap.Int("count", len(oldBindings)), zap.String("new_ipv6", newIPv6))
	return nil
}

func (e *ReinstallContainerExecutor) getContainerIPv6() (string, error) {
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
			if addr.Family == "inet6" {
				// 过滤掉链路本地地址 (fe80::) 和回环地址 (::1)
				if strings.HasPrefix(addr.Address, "fe80") {
					continue
				}
				if addr.Address == "::1" || strings.HasPrefix(addr.Address, "::1") {
					e.LogInfo("Skipping loopback IPv6 address", zap.String("address", addr.Address))
					continue
				}
				return addr.Address, nil
			}
		}
	}
	
	return "", fmt.Errorf("no valid IPv6 address found (filtered loopback and link-local)")
}

func (e *ReinstallContainerExecutor) configureContainer(ctx context.Context) error {
	// 使用重装请求中的密码，如果没有则使用旧密码
	password := e.ReinstallRequest.Password
	if password == "" {
		password = e.ContainerConfig.Password
	}
	
	if password == "" {
		return nil
	}
	
	hostnameCmd := exec.CommandContext(ctx, "lxc", "exec", e.ContainerName, "--", "hostnamectl", "set-hostname", e.ContainerName)
	if output, err := hostnameCmd.CombinedOutput(); err != nil {
		e.LogError("Set hostname failed", zap.Error(err), zap.String("output", string(output)))
	}
	
	passwdCmd := exec.CommandContext(ctx, "lxc", "exec", e.ContainerName, "--", "sh", "-c",
		fmt.Sprintf("echo 'root:%s' | chpasswd", password))
	output, err := passwdCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("set password failed: %v, output: %s", err, string(output))
	}
	
	e.LogInfo("Password configured", zap.String("output", string(output)))
	
	// 更新数据库中的配置
	e.updateDatabaseConfig()
	
	return nil
}

func (e *ReinstallContainerExecutor) updateDatabaseConfig() {
	// 更新数据库配置为新的重装配置
	updates := make(map[string]interface{})
	
	if e.ReinstallRequest.System != "" {
		updates["image"] = e.ReinstallRequest.System
	}
	if e.ReinstallRequest.Password != "" {
		updates["password"] = e.ReinstallRequest.Password
	}
	if e.ReinstallRequest.CPUs > 0 {
		updates["cpus"] = e.ReinstallRequest.CPUs
	}
	if e.ReinstallRequest.Memory != "" {
		updates["memory"] = e.ReinstallRequest.Memory
	}
	if e.ReinstallRequest.Disk != "" {
		updates["disk"] = e.ReinstallRequest.Disk
	}
	if e.ReinstallRequest.Ingress != "" {
		updates["ingress"] = e.ReinstallRequest.Ingress
	}
	if e.ReinstallRequest.Egress != "" {
		updates["egress"] = e.ReinstallRequest.Egress
	}
	if e.ReinstallRequest.TrafficLimit > 0 {
		updates["traffic_limit"] = e.ReinstallRequest.TrafficLimit
	}
	if e.ReinstallRequest.CPUAllowance != "" {
		updates["cpu_allowance"] = e.ReinstallRequest.CPUAllowance
	}
	if e.ReinstallRequest.MaxProcesses > 0 {
		updates["max_processes"] = e.ReinstallRequest.MaxProcesses
	}
	if e.ReinstallRequest.DiskIOLimit != "" {
		updates["disk_io_limit"] = e.ReinstallRequest.DiskIOLimit
	}
	
	// 布尔值需要特殊处理（即使是false也要更新）
	updates["allow_nesting"] = e.ReinstallRequest.AllowNesting
	updates["memory_swap"] = e.ReinstallRequest.MemorySwap
	updates["privileged"] = e.ReinstallRequest.Privileged
	
	if len(updates) > 0 {
		if err := e.DB.Model(&models.ContainerConfig{}).
			Where("container_name = ?", e.ContainerName).
			Updates(updates).Error; err != nil {
			e.LogError("Update container config in database failed", zap.Error(err))
		} else {
			e.LogInfo("Container config updated in database", zap.Any("updates", updates))
		}
	}
}

func (e *ReinstallContainerExecutor) getProxyRules() ([]models.ProxyRule, error) {
	var proxyRules []models.ProxyRule
	if err := e.DB.Where("container_name = ?", e.ContainerName).Find(&proxyRules).Error; err != nil {
		return nil, fmt.Errorf("query proxy rules failed: %v", err)
	}
	return proxyRules, nil
}

func (e *ReinstallContainerExecutor) recreateProxyRules(oldRules []models.ProxyRule) error {
	if len(oldRules) == 0 {
		e.LogInfo("No proxy rules to recreate")
		return nil
	}
	
	// 检查反向代理功能是否启用
	if !config.AppConfig.ProxyAPI.Enabled {
		e.LogInfo("Proxy feature disabled, skipping proxy rule recreation")
		return nil
	}
	
	// 获取新容器IP
	newIP, err := e.getContainerIP()
	if err != nil {
		e.LogError("Get new container IP failed", zap.Error(err))
		return err
	}
	
	e.LogInfo("开始重新生成反向代理配置", zap.String("new_ip", newIP))
	
	// 为每个代理规则重新生成配置
	for _, rule := range oldRules {
		e.LogInfo("Recreating proxy rule", 
			zap.String("domain", rule.Domain),
			zap.Int("container_port", rule.ContainerPort),
			zap.String("new_ip", newIP))
		
		// 调用services层的函数重新生成Nginx配置
		// 这里使用底层的配置生成逻辑，直接更新nginx配置文件
		if err := e.recreateSingleProxyConfig(rule, newIP); err != nil {
			e.LogError("Recreate proxy config failed", zap.Error(err), zap.String("domain", rule.Domain))
			continue
		}
		
		e.LogInfo("Proxy rule recreated", zap.String("domain", rule.Domain))
	}
	
	e.LogInfo("反向代理配置重新生成完成", zap.Int("count", len(oldRules)))
	return nil
}

func (e *ReinstallContainerExecutor) recreateSingleProxyConfig(rule models.ProxyRule, newIP string) error {
	// 获取工作目录
	baseDir, err := exec.Command("pwd").Output()
	if err != nil {
		return fmt.Errorf("获取工作目录失败: %v", err)
	}
	
	baseDirStr := strings.TrimSpace(string(baseDir))
	nginxDir := baseDirStr + "/nginx"
	configFile := fmt.Sprintf("%s/proxy-%s.conf", nginxDir, rule.Domain)
	templateFile := baseDirStr + "/nginx-default.tmpl"
	
	// 读取模板文件
	templateContent, err := exec.Command("cat", templateFile).Output()
	if err != nil {
		return fmt.Errorf("读取模板文件失败: %v", err)
	}
	
	// 使用模板生成配置
	tmplStr := string(templateContent)
	tmplStr = strings.ReplaceAll(tmplStr, "{{.GeneratedAt}}", time.Now().Format("2006-01-02 15:04:05"))
	tmplStr = strings.ReplaceAll(tmplStr, "{{.ContainerName}}", e.ContainerName)
	tmplStr = strings.ReplaceAll(tmplStr, "{{.Domain}}", rule.Domain)
	tmplStr = strings.ReplaceAll(tmplStr, "{{.ContainerIP}}", newIP)
	tmplStr = strings.ReplaceAll(tmplStr, "{{.ContainerPort}}", strconv.Itoa(rule.ContainerPort))
	
	// 写入配置文件
	writeCmd := exec.Command("sh", "-c", fmt.Sprintf("cat > %s", configFile))
	writeCmd.Stdin = strings.NewReader(tmplStr)
	if output, err := writeCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("写入配置文件失败: %v, 输出: %s", err, string(output))
	}
	
	e.LogInfo("配置文件已更新", zap.String("file", configFile), zap.String("new_ip", newIP))
	return nil
}

