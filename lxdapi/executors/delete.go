package executors

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"lxdapi/config"
	"lxdapi/errors"
	"lxdapi/models"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type DeleteContainerExecutor struct {
	*BaseExecutor
}

func NewDeleteContainerExecutor(db *gorm.DB, logger *zap.Logger, task *models.Task) *DeleteContainerExecutor {
	return &DeleteContainerExecutor{
		BaseExecutor: NewBaseExecutor(db, logger, task),
	}
}

func (e *DeleteContainerExecutor) Execute() error {
	ctx := context.Background()
	
	e.StartStep("STEP_1", "Check container status")
	status, err := e.getContainerStatus(ctx)
	if err != nil {
		e.CompleteStep(false, errors.ERR_LXC_QUERY_FAIL, fmt.Sprintf("Get container status failed: %v", err))
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, fmt.Sprintf("Container status: %s", status))
	}
	
	e.StartStep("STEP_2", "Get container IP")
	containerIP, err := e.getContainerIP()
	if err != nil {
		e.LogInfo("Get container IP failed (container may be stopped or deleted), will skip rule cleanup", zap.Error(err))
		containerIP = ""
		e.CompleteStep(true, errors.ERR_SUCCESS, "Container IP not available")
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, fmt.Sprintf("Container IP: %s", containerIP))
	}
	
	if status == "Running" {
		e.StartStep("STEP_3", "Stop container")
		if err := e.stopContainer(ctx); err != nil {
			e.CompleteStep(false, errors.ERR_LXC_STOP_FAIL, fmt.Sprintf("Stop container failed: %v", err))
		} else {
			e.CompleteStep(true, errors.ERR_SUCCESS, "Container stopped successfully")
		}
	} else {
		e.StartStep("STEP_3", "Container already stopped")
		e.CompleteStep(true, errors.ERR_SUCCESS, "Container is already stopped")
	}
	
	e.StartStep("STEP_4", "Query NAT rules")
	natRules, err := e.getNATRules()
	if err != nil {
		e.CompleteStep(false, errors.ERR_NAT_QUERY_FAIL, fmt.Sprintf("Query NAT rules failed: %v", err))
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, fmt.Sprintf("Found %d NAT rules", len(natRules)))
	}
	
	e.StartStep("STEP_5", "Delete iptables NAT rules")
	if err := e.deleteIptablesRules(natRules, containerIP); err != nil {
		e.CompleteStep(false, errors.ERR_NAT_CLEANUP_FAIL, fmt.Sprintf("Delete iptables rules failed: %v", err))
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, fmt.Sprintf("Deleted %d iptables rules", len(natRules)))
	}
	
	e.StartStep("STEP_6", "Query IPv6 bindings")
	ipv6Bindings, err := e.getIPv6Bindings()
	if err != nil {
		e.CompleteStep(false, errors.ERR_IPV6_QUERY_FAIL, fmt.Sprintf("Query IPv6 bindings failed: %v", err))
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, fmt.Sprintf("Found %d IPv6 bindings", len(ipv6Bindings)))
	}
	
	e.StartStep("STEP_7", "Delete IPv6 routes")
	if err := e.deleteIPv6Routes(ipv6Bindings); err != nil {
		e.CompleteStep(false, errors.ERR_IPV6_CLEANUP_FAIL, fmt.Sprintf("Delete IPv6 routes failed: %v", err))
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, fmt.Sprintf("Deleted %d IPv6 routes", len(ipv6Bindings)))
	}
	
	e.StartStep("STEP_8", "Query proxy rules")
	proxyRules, err := e.getProxyRules()
	if err != nil {
		e.CompleteStep(false, errors.ERR_PROXY_RULE_NOT_FOUND, fmt.Sprintf("Query proxy rules failed: %v", err))
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, fmt.Sprintf("Found %d proxy rules", len(proxyRules)))
	}
	
	e.StartStep("STEP_9", "Delete proxy rules")
	if err := e.deleteProxyRules(proxyRules); err != nil {
		e.CompleteStep(false, errors.ERR_PROXY_CONFIG_GENERATE, fmt.Sprintf("Delete proxy rules failed: %v", err))
	} else {
		e.CompleteStep(true, errors.ERR_SUCCESS, fmt.Sprintf("Deleted %d proxy rules", len(proxyRules)))
	}
	
	e.StartStep("STEP_10", "Delete LXD container")
	if status == "NotFound" {
		e.CompleteStep(true, errors.ERR_SUCCESS, "Container already deleted (not found)")
	} else {
		if err := e.deleteLXDContainer(ctx); err != nil {
			e.CompleteStep(false, errors.ERR_LXC_DELETE_FAIL, fmt.Sprintf("Delete container failed: %v", err))
			return err
		}
		e.CompleteStep(true, errors.ERR_SUCCESS, "Container deleted successfully")
	}
	
	e.StartStep("STEP_11", "Clean up database records")
	if err := e.cleanupDatabase(); err != nil {
		e.CompleteStep(false, errors.ERR_DB_DELETE_FAIL, fmt.Sprintf("Cleanup database failed: %v", err))
		return err
	}
	e.CompleteStep(true, errors.ERR_SUCCESS, "Database records cleaned up")
	
	return nil
}

func (e *DeleteContainerExecutor) getContainerStatus(ctx context.Context) (string, error) {
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

func (e *DeleteContainerExecutor) stopContainer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "lxc", "stop", e.ContainerName, "--force")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("lxc stop failed: %v, output: %s", err, string(output))
	}
	
	e.LogInfo("Container stopped", zap.String("output", string(output)))
	return nil
}

func (e *DeleteContainerExecutor) getNATRules() ([]models.NATRule, error) {
	var natRules []models.NATRule
	if err := e.DB.Where("container_name = ?", e.ContainerName).Find(&natRules).Error; err != nil {
		return nil, fmt.Errorf("query NAT rules failed: %v", err)
	}
	return natRules, nil
}

func (e *DeleteContainerExecutor) deleteIptablesRules(natRules []models.NATRule, containerIP string) error {
	for _, rule := range natRules {
		isPortRange := rule.ExternalPortEnd > 0 && rule.InternalPortEnd > 0
		
		if isPortRange {
			e.LogInfo("Deleting NAT port range rule by comment", 
				zap.Int("external_port_start", rule.ExternalPort),
				zap.Int("external_port_end", rule.ExternalPortEnd),
				zap.Int("internal_port_start", rule.InternalPort),
				zap.Int("internal_port_end", rule.InternalPortEnd),
				zap.String("protocol", rule.Protocol))
			
			comment := fmt.Sprintf("lxdapinat_%s_%d-%d_%d-%d_%s", e.ContainerName, rule.ExternalPort, rule.ExternalPortEnd, rule.InternalPort, rule.InternalPortEnd, rule.Protocol)
			
			if err := e.cleanupRulesByComment("nat", "PREROUTING", comment); err != nil {
				e.LogInfo("Cleanup IPv4 DNAT range rules (ignore if not found)", zap.Error(err))
			}
			
			if err := e.cleanupRulesByComment("filter", "FORWARD", comment); err != nil {
				e.LogInfo("Cleanup IPv4 FORWARD range rules (ignore if not found)", zap.Error(err))
			}
			
			if rule.IPVersion == "ipv6" || rule.IPVersion == "dual" {
				comment6 := fmt.Sprintf("lxdapinat_ipv6_%s_%d-%d_%d-%d_%s", e.ContainerName, rule.ExternalPort, rule.ExternalPortEnd, rule.InternalPort, rule.InternalPortEnd, rule.Protocol)
				
				if err := e.cleanupRulesByCommentIPv6("nat", "PREROUTING", comment6); err != nil {
					e.LogInfo("Cleanup IPv6 DNAT range rules (ignore if not found)", zap.Error(err))
				}
				
				if err := e.cleanupRulesByCommentIPv6("filter", "FORWARD", comment6); err != nil {
					e.LogInfo("Cleanup IPv6 FORWARD range rules (ignore if not found)", zap.Error(err))
				}
			}
		} else {
			e.LogInfo("Deleting NAT rule by comment", 
				zap.Int("external_port", rule.ExternalPort),
				zap.Int("internal_port", rule.InternalPort),
				zap.String("protocol", rule.Protocol))
			
			comment := fmt.Sprintf("lxdapinat_%s_%d_%d_%s", e.ContainerName, rule.ExternalPort, rule.InternalPort, rule.Protocol)
			
			if err := e.cleanupRulesByComment("nat", "PREROUTING", comment); err != nil {
				e.LogInfo("Cleanup IPv4 DNAT rules (ignore if not found)", zap.Error(err))
			}
			
			if err := e.cleanupRulesByComment("filter", "FORWARD", comment); err != nil {
				e.LogInfo("Cleanup IPv4 FORWARD rules (ignore if not found)", zap.Error(err))
			}
			
			if rule.IPVersion == "ipv6" || rule.IPVersion == "dual" {
				comment6 := fmt.Sprintf("lxdapinat_ipv6_%s_%d_%d_%s", e.ContainerName, rule.ExternalPort, rule.InternalPort, rule.Protocol)
				
				if err := e.cleanupRulesByCommentIPv6("nat", "PREROUTING", comment6); err != nil {
					e.LogInfo("Cleanup IPv6 DNAT rules (ignore if not found)", zap.Error(err))
				}
				
				if err := e.cleanupRulesByCommentIPv6("filter", "FORWARD", comment6); err != nil {
					e.LogInfo("Cleanup IPv6 FORWARD rules (ignore if not found)", zap.Error(err))
				}
			}
		}
	}
	
	e.LogInfo("All NAT iptables rules deleted", zap.Int("count", len(natRules)))
	return nil
}

func (e *DeleteContainerExecutor) getIPv6Bindings() ([]models.IPv6BindingRule, error) {
	var bindings []models.IPv6BindingRule
	if err := e.DB.Where("container_name = ?", e.ContainerName).Find(&bindings).Error; err != nil {
		return nil, fmt.Errorf("query IPv6 bindings failed: %v", err)
	}
	return bindings, nil
}

func (e *DeleteContainerExecutor) deleteIPv6Routes(bindings []models.IPv6BindingRule) error {
	ipv6Config := config.AppConfig.IPv6Binding
	prefixLength := ipv6Config.IPv6Pool.PrefixLength
	
	for _, binding := range bindings {
		e.LogInfo("Deleting IPv6 binding", 
			zap.String("public_ipv6", binding.PublicIPv6),
			zap.String("container_ipv6", binding.ContainerIPv6))
		
		// 使用comment标记删除IPv6 NAT规则
		comment := fmt.Sprintf("IPv6-BINDING-%s", binding.PublicIPv6)
		
		// 删除 IPv6 DNAT 规则（PREROUTING链在nat表）
		if err := e.cleanupRulesByCommentIPv6("nat", "PREROUTING", comment); err != nil {
			e.LogInfo("Cleanup IPv6 binding DNAT rules (ignore if not found)", zap.Error(err))
		}
		
		// 删除 IPv6 FORWARD 规则（FORWARD链在filter表）
		if err := e.cleanupRulesByCommentIPv6("filter", "FORWARD", comment); err != nil {
			e.LogInfo("Cleanup IPv6 binding FORWARD rules (ignore if not found)", zap.Error(err))
		}
		
		// 从网卡移除IPv6地址
		ipv6WithPrefix := fmt.Sprintf("%s/%d", binding.PublicIPv6, prefixLength)
		cmdAddr := exec.Command("ip", "-6", "addr", "del", ipv6WithPrefix, "dev", binding.Interface)
		if output, err := cmdAddr.CombinedOutput(); err != nil {
			outputStr := string(output)
			if !strings.Contains(outputStr, "not found") && !strings.Contains(outputStr, "Cannot assign") {
				e.LogInfo("Delete IPv6 address (ignore if not found)", zap.String("output", outputStr))
			}
		}
	}
	
	e.LogInfo("All IPv6 routes deleted", zap.Int("count", len(bindings)))
	return nil
}

func (e *DeleteContainerExecutor) deleteLXDContainer(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "lxc", "delete", e.ContainerName, "--force")
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		if strings.Contains(outputStr, "Instance not found") || strings.Contains(outputStr, "not found") {
			e.LogInfo("Container already deleted", zap.String("output", outputStr))
			return nil
		}
		return fmt.Errorf("lxc delete failed: %v, output: %s", err, outputStr)
	}
	
	e.LogInfo("Container deleted", zap.String("output", string(output)))
	return nil
}

func (e *DeleteContainerExecutor) getContainerIP() (string, error) {
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

func (e *DeleteContainerExecutor) getContainerIPv6() (string, error) {
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

func (e *DeleteContainerExecutor) getProxyRules() ([]models.ProxyRule, error) {
	var proxyRules []models.ProxyRule
	if err := e.DB.Where("container_name = ?", e.ContainerName).Find(&proxyRules).Error; err != nil {
		return nil, fmt.Errorf("query proxy rules failed: %v", err)
	}
	return proxyRules, nil
}

func (e *DeleteContainerExecutor) deleteProxyRules(proxyRules []models.ProxyRule) error {
	if len(proxyRules) == 0 {
		e.LogInfo("No proxy rules to delete")
		return nil
	}
	
	// 检查反向代理功能是否启用
	if !config.AppConfig.ProxyAPI.Enabled {
		e.LogInfo("Proxy feature disabled, skipping proxy rule cleanup")
		return nil
	}
	
	e.LogInfo("清理容器的代理规则")
	
	// 获取程序根目录
	baseDir, err := os.Getwd()
	if err != nil {
		e.LogError("Get working directory failed", zap.Error(err))
		return fmt.Errorf("获取工作目录失败: %v", err)
	}
	
	// 删除配置文件、软链接和SSL证书
	for _, rule := range proxyRules {
		// 删除配置文件
		configFile := filepath.Join(baseDir, "nginx", fmt.Sprintf("proxy-%s.conf", rule.Domain))
		if err := os.Remove(configFile); err != nil && !os.IsNotExist(err) {
			e.LogInfo("Delete proxy config file (ignore if not found)", 
				zap.String("file", configFile),
				zap.String("error", err.Error()))
		}
		
		// 删除软链接
		symlinkPath := filepath.Join("/etc/nginx/sites-enabled", fmt.Sprintf("proxy-%s.conf", rule.Domain))
		if err := os.Remove(symlinkPath); err != nil && !os.IsNotExist(err) {
			e.LogInfo("Delete proxy symlink (ignore if not found)", 
				zap.String("symlink", symlinkPath),
				zap.String("error", err.Error()))
		}
		
		// 删除SSL证书（如果启用了SSL）
		if rule.SSLEnabled {
			certFile := filepath.Join(baseDir, "nginx", "certs", rule.Domain+".crt")
			keyFile := filepath.Join(baseDir, "nginx", "certs", rule.Domain+".key")
			
			if err := os.Remove(certFile); err != nil && !os.IsNotExist(err) {
				e.LogInfo("Delete SSL certificate (ignore if not found)", 
					zap.String("file", certFile),
					zap.String("error", err.Error()))
			}
			
			if err := os.Remove(keyFile); err != nil && !os.IsNotExist(err) {
				e.LogInfo("Delete SSL key (ignore if not found)", 
					zap.String("file", keyFile),
					zap.String("error", err.Error()))
			}
			
			e.LogInfo("SSL certificate deleted", zap.String("domain", rule.Domain))
		}
	}
	
	// 测试并重载 Nginx
	testCmd := exec.Command("nginx", "-t")
	if output, err := testCmd.CombinedOutput(); err != nil {
		e.LogInfo("Nginx config test failed (non-critical)", 
			zap.String("output", string(output)))
	} else {
		// 重载 Nginx
		reloadCmd := exec.Command("systemctl", "reload", "nginx")
		if output, err := reloadCmd.CombinedOutput(); err != nil {
			e.LogInfo("Nginx reload failed (non-critical)", 
				zap.String("output", string(output)))
		}
	}
	
	e.LogInfo("All proxy rules deleted", zap.Int("count", len(proxyRules)))
	return nil
}

func (e *DeleteContainerExecutor) cleanupDatabase() error {
	if err := e.DB.Where("container_name = ?", e.ContainerName).Delete(&models.NATRule{}).Error; err != nil {
		e.LogError("Delete NAT rules from database failed", zap.Error(err))
	}
	
	if err := e.DB.Where("container_name = ?", e.ContainerName).Delete(&models.IPv6BindingRule{}).Error; err != nil {
		e.LogError("Delete IPv6 bindings from database failed", zap.Error(err))
	}
	
	if err := e.DB.Where("container_name = ?", e.ContainerName).Delete(&models.ProxyRule{}).Error; err != nil {
		e.LogError("Delete proxy rules from database failed", zap.Error(err))
	}
	
	if err := e.DB.Where("container_name = ?", e.ContainerName).Delete(&models.ContainerConfig{}).Error; err != nil {
		e.LogError("Delete container config from database failed", zap.Error(err))
	}
	
	if err := e.DB.Where("container_name = ?", e.ContainerName).Delete(&models.TrafficSummary{}).Error; err != nil {
		e.LogError("Delete traffic summary from database failed", zap.Error(err))
	}
	
	if err := e.DB.Where("hostname = ?", e.ContainerName).Delete(&models.ContainerInfoCache{}).Error; err != nil {
		e.LogError("Delete container info cache from database failed", zap.Error(err))
	}
	
	
	e.LogInfo("Database records cleaned up")
	return nil
}

// cleanupRulesByComment 通过comment标记清理IPv4 iptables规则
func (e *DeleteContainerExecutor) cleanupRulesByComment(table, chain, comment string) error {
	cmd := exec.Command("iptables", "-t", table, "-L", chain, "--line-numbers", "-v")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("列出iptables规则失败: %v", err)
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
		cmd := exec.Command("iptables", "-t", table, "-D", chain, strconv.Itoa(lineNumbers[i]))
		if err := cmd.Run(); err != nil {
			e.LogInfo("Delete iptables rule failed (ignore)", 
				zap.String("table", table),
				zap.String("chain", chain),
				zap.Int("line", lineNumbers[i]),
				zap.Error(err))
		}
	}

	return nil
}

// cleanupRulesByCommentIPv6 通过comment标记清理IPv6 ip6tables规则
func (e *DeleteContainerExecutor) cleanupRulesByCommentIPv6(table, chain, comment string) error {
	cmd := exec.Command("ip6tables", "-t", table, "-L", chain, "--line-numbers", "-v")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("列出ip6tables规则失败: %v", err)
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
		cmd := exec.Command("ip6tables", "-t", table, "-D", chain, strconv.Itoa(lineNumbers[i]))
		if err := cmd.Run(); err != nil {
			e.LogInfo("Delete ip6tables rule failed (ignore)", 
				zap.String("table", table),
				zap.String("chain", chain),
				zap.Int("line", lineNumbers[i]),
				zap.Error(err))
		}
	}

	return nil
}