package services

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"lxdapi/config"
	"lxdapi/pkg/logger"

	"go.uber.org/zap"
)

const (
	IPTABLES_CHAIN_PREFIX = "LXDAPINAT"
	COMMENT_PREFIX        = "lxdapinat"
)

type IptablesNATManager struct {
	enabled           bool
	externalInterface string
	externalIPs       []string
	externalIPv6s     []string
	internalInterface string
	ipv6Supported     bool
}

var IptablesManager *IptablesNATManager

func InitIptablesNAT() error {
	ctx := context.Background()
	lc := &logger.Context{
		Action: "init_iptables_nat",
	}
	ctx = logger.NewContext(ctx, lc)

	// 只支持 IPv4 NAT
	ipv4NATEnabled := config.IsNATEnabled()
	
	if !ipv4NATEnabled {
		logger.Global.Info(ctx, "NAT功能已禁用，跳过初始化")
		return nil
	}

	iptablesConfig := config.GetNATIptablesConfig()

	IptablesManager = &IptablesNATManager{enabled: iptablesConfig.Enabled}

	if err := checkIptablesAvailable(); err != nil {
		logger.Global.Warn(ctx, "iptables不可用", zap.Error(err))
		IptablesManager.enabled = false
		return nil
	}

	IptablesManager.ipv6Supported = false

	if err := IptablesManager.detectNetworkConfig(); err != nil {
		logger.Global.Warn(ctx, "网络配置检测失败", zap.Error(err))
	}

	if err := IptablesManager.createCustomChains(); err != nil {
		return fmt.Errorf("创建 iptables 自定义链失败: %v", err)
	}

	logger.Global.Info(ctx, "iptables NAT管理器初始化成功")
	logger.Global.Info(ctx, "外网网卡配置",
		zap.String("interface", IptablesManager.externalInterface),
		zap.Strings("ipv4", IptablesManager.externalIPs))
	logger.Global.Info(ctx, "内网网卡", zap.String("interface", IptablesManager.internalInterface))
	return nil
}

func (m *IptablesNATManager) detectNetworkConfig() error {
	ctx := context.Background()
	lc := &logger.Context{Action: "load_network_config"}
	ctx = logger.NewContext(ctx, lc)

	natConfig := config.AppConfig.NATAPI.Network
	
	// 直接从配置文件加载网络配置
	m.externalInterface = natConfig.ExternalInterface
	m.externalIPs = natConfig.ExternalIPs
	m.internalInterface = "lxdbr0"

	logger.Global.Info(ctx, "从配置文件加载网络配置",
		zap.String("external_interface", m.externalInterface),
		zap.Strings("external_ips", m.externalIPs),
		zap.Strings("external_ipv6s", m.externalIPv6s))

	return nil
}

func (m *IptablesNATManager) detectExternalInterface() (string, error) {
	cmd := exec.Command("ip", "route", "show", "default")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取默认路由失败: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "default") {
			fields := strings.Fields(line)
			for i, field := range fields {
				if field == "dev" && i+1 < len(fields) {
					return fields[i+1], nil
				}
			}
		}
	}

	return "", fmt.Errorf("未找到默认网卡")
}

func (m *IptablesNATManager) guessExternalInterface() string {
	commonNames := []string{"eth0", "ens3", "ens33", "enp0s3", "eno1"}
	
	for _, name := range commonNames {
		if m.interfaceExists(name) {
			return name
		}
	}
	
	return "eth0"
}

func (m *IptablesNATManager) interfaceExists(name string) bool {
	_, err := net.InterfaceByName(name)
	return err == nil
}

func (m *IptablesNATManager) detectExternalIPs(interfaceName string) ([]string, error) {
	if interfaceName == "" {
		return []string{}, fmt.Errorf("网卡名为空")
	}

	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return []string{}, fmt.Errorf("网卡 %s 不存在: %v", interfaceName, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return []string{}, fmt.Errorf("获取网卡 %s 地址失败: %v", interfaceName, err)
	}

	var ips []string
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipv4 := ipnet.IP.To4(); ipv4 != nil {
				ips = append(ips, ipv4.String())
			}
		}
	}

	if len(ips) == 0 {
		return []string{}, fmt.Errorf("网卡 %s 没有可用的IPv4地址", interfaceName)
	}

	return ips, nil
}

func (m *IptablesNATManager) IsEnabled() bool {
	return m.enabled
}

func (m *IptablesNATManager) IsIPv6Supported() bool {
	return m.ipv6Supported
}


func (m *IptablesNATManager) AddNATRule(containerName string, externalPort, internalPort int, protocol string) error {
	return m.AddNATRuleIPv4(containerName, externalPort, internalPort, protocol)
}

func (m *IptablesNATManager) AddNATRuleIPv4(containerName string, externalPort, internalPort int, protocol string) error {
	if !m.enabled {
		return fmt.Errorf("iptables模式未启用")
	}

	containerIP, err := GetContainerIP(containerName)
	if err != nil {
		return fmt.Errorf("获取容器IP失败: %v", err)
	}

	comment := fmt.Sprintf("%s_%s_%d_%d_%s", COMMENT_PREFIX, containerName, externalPort, internalPort, protocol)

	dnatRule := m.buildDNATRule(externalPort, internalPort, protocol, containerIP)

	if err := m.addIptablesRule("nat", "PREROUTING", dnatRule, comment); err != nil {
		return fmt.Errorf("添加DNAT规则失败: %v", err)
	}

	forwardRule := m.buildForwardRule(internalPort, protocol, containerIP)
	if err := m.addIptablesRule("filter", "FORWARD", forwardRule, comment); err != nil {
		m.removeIptablesRule("nat", "PREROUTING", dnatRule, comment)
		return fmt.Errorf("添加FORWARD规则失败: %v", err)
	}

	ctx := context.Background()
	lc := &logger.Context{Action: "add_nat_rule_ipv4"}
	ctx = logger.NewContext(ctx, lc)
	logger.Global.Info(ctx, "iptables NAT规则添加成功",
		zap.String("external_ip", m.getExternalIPString()),
		zap.Int("external_port", externalPort),
		zap.String("container_ip", containerIP),
		zap.Int("internal_port", internalPort),
		zap.String("protocol", protocol),
		zap.String("interface", m.externalInterface))
	
	// 保存规则
	if PersistentManager != nil && PersistentManager.IsEnabled() {
		PersistentManager.SaveRules()
	}
	
	return nil
}

// RemoveNATRule 删除NAT规则
func (m *IptablesNATManager) RemoveNATRule(containerName string, externalPort, internalPort int, protocol string) error {
	if !m.enabled {
		return fmt.Errorf("iptables模式未启用")
	}

	ctx := context.Background()
	lc := &logger.Context{
		Container: containerName,
		Action:    "remove_nat_rule",
	}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Info(ctx, "开始删除iptables NAT规则",
		zap.Int("external_port", externalPort),
		zap.Int("internal_port", internalPort),
		zap.String("protocol", protocol))

	comment := fmt.Sprintf("%s_%s_%d_%d_%s", COMMENT_PREFIX, containerName, externalPort, internalPort, protocol)
	logger.Global.Debug(ctx, "清理IPv4规则", zap.String("comment", comment))

	if err := m.cleanupRulesByComment("nat", "PREROUTING", comment); err != nil {
		logger.Global.Warn(ctx, "清理IPv4 DNAT规则失败", zap.Error(err))
	}
	if err := m.cleanupRulesByComment("filter", "FORWARD", comment); err != nil {
		logger.Global.Warn(ctx, "清理IPv4 FORWARD规则失败", zap.Error(err))
	}

	if m.ipv6Supported {
		ipv6Comment := fmt.Sprintf("%s_ipv6_%s_%d_%d_%s", COMMENT_PREFIX, containerName, externalPort, internalPort, protocol)
		logger.Global.Debug(ctx, "清理IPv6规则", zap.String("comment", ipv6Comment))

		if err := m.cleanupRulesByCommentIPv6("nat", "PREROUTING", ipv6Comment); err != nil {
			logger.Global.Warn(ctx, "清理IPv6 DNAT规则失败", zap.Error(err))
		}
		if err := m.cleanupRulesByCommentIPv6("filter", "FORWARD", ipv6Comment); err != nil {
			logger.Global.Warn(ctx, "清理IPv6 FORWARD规则失败", zap.Error(err))
		}
	}

	logger.Global.Info(ctx, "iptables NAT规则删除完成",
		zap.Int("external_port", externalPort),
		zap.Int("internal_port", internalPort),
		zap.String("protocol", protocol))
	
	// 保存规则
	if PersistentManager != nil && PersistentManager.IsEnabled() {
		PersistentManager.SaveRules()
	}
	
	return nil
}

func (m *IptablesNATManager) RemoveNATRuleRange(containerName string, externalPortStart, externalPortEnd, internalPortStart, internalPortEnd int, protocol string) error {
	if !m.enabled {
		return fmt.Errorf("iptables模式未启用")
	}

	ctx := context.Background()
	lc := &logger.Context{
		Container: containerName,
		Action:    "remove_nat_rule_range",
	}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Info(ctx, "开始删除iptables NAT端口段规则",
		zap.Int("external_port_start", externalPortStart),
		zap.Int("external_port_end", externalPortEnd),
		zap.Int("internal_port_start", internalPortStart),
		zap.Int("internal_port_end", internalPortEnd),
		zap.String("protocol", protocol))

	comment := fmt.Sprintf("%s_%s_%d-%d_%d-%d_%s", COMMENT_PREFIX, containerName, externalPortStart, externalPortEnd, internalPortStart, internalPortEnd, protocol)
	logger.Global.Debug(ctx, "清理IPv4端口段规则", zap.String("comment", comment))

	if err := m.cleanupRulesByComment("nat", "PREROUTING", comment); err != nil {
		logger.Global.Warn(ctx, "清理IPv4 DNAT端口段规则失败", zap.Error(err))
	}
	if err := m.cleanupRulesByComment("filter", "FORWARD", comment); err != nil {
		logger.Global.Warn(ctx, "清理IPv4 FORWARD端口段规则失败", zap.Error(err))
	}

	if m.ipv6Supported {
		ipv6Comment := fmt.Sprintf("%s_ipv6_%s_%d-%d_%d-%d_%s", COMMENT_PREFIX, containerName, externalPortStart, externalPortEnd, internalPortStart, internalPortEnd, protocol)
		logger.Global.Debug(ctx, "清理IPv6端口段规则", zap.String("comment", ipv6Comment))

		if err := m.cleanupRulesByCommentIPv6("nat", "PREROUTING", ipv6Comment); err != nil {
			logger.Global.Warn(ctx, "清理IPv6 DNAT端口段规则失败", zap.Error(err))
		}
		if err := m.cleanupRulesByCommentIPv6("filter", "FORWARD", ipv6Comment); err != nil {
			logger.Global.Warn(ctx, "清理IPv6 FORWARD端口段规则失败", zap.Error(err))
		}
	}

	logger.Global.Info(ctx, "iptables NAT端口段规则删除完成",
		zap.Int("external_port_start", externalPortStart),
		zap.Int("external_port_end", externalPortEnd),
		zap.Int("internal_port_start", internalPortStart),
		zap.Int("internal_port_end", internalPortEnd),
		zap.String("protocol", protocol))
	
	// 保存规则
	if PersistentManager != nil && PersistentManager.IsEnabled() {
		PersistentManager.SaveRules()
	}
	
	return nil
}

func (m *IptablesNATManager) buildDNATRule(externalPort, internalPort int, protocol, containerIP string) string {
	var rule strings.Builder

	if m.externalInterface != "" {
		rule.WriteString(fmt.Sprintf("-i %s ", m.externalInterface))
	}

	if len(m.externalIPs) > 0 {
		rule.WriteString(fmt.Sprintf("-d %s ", m.externalIPs[0]))
	}

	rule.WriteString(fmt.Sprintf("-p %s --dport %d ", protocol, externalPort))
	rule.WriteString(fmt.Sprintf("-j DNAT --to-destination %s:%d", containerIP, internalPort))

	return rule.String()
}

func (m *IptablesNATManager) buildForwardRule(internalPort int, protocol, containerIP string) string {
	var rule strings.Builder

	if m.externalInterface != "" {
		rule.WriteString(fmt.Sprintf("-i %s ", m.externalInterface))
	}

	if m.internalInterface != "" {
		rule.WriteString(fmt.Sprintf("-o %s ", m.internalInterface))
	}

	rule.WriteString(fmt.Sprintf("-d %s -p %s --dport %d ", containerIP, protocol, internalPort))
	rule.WriteString("-j ACCEPT")

	return rule.String()
}

func (m *IptablesNATManager) getExternalIPString() string {
	if len(m.externalIPs) == 0 {
		return "0.0.0.0"
	}
	return m.externalIPs[0]
}



func GetContainerIP(containerName string) (string, error) {
	cmd := exec.Command("lxc", "list", containerName, "--format", "csv", "-c", "4")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取容器列表失败: %v", err)
	}

	lines := strings.TrimSpace(string(output))
	if lines == "" {
		return "", fmt.Errorf("容器%s不存在或未运行", containerName)
	}

	lines = strings.Trim(lines, "\"")
	networkLines := strings.Split(lines, "\n")
	
	preferredInterfaces := []string{"eth0", "ens3", "ens33", "enp0s3", "eno1"}
	
	var foundIPs []string
	var fallbackIPs []string
	
	for _, line := range networkLines {
		line = strings.TrimSpace(line)
		if line == "" || line == "-" {
			continue
		}
		
		// 解析 "IP (interface)" 格式
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			ip := parts[0]
			interfaceName := strings.Trim(parts[1], "()")
			
			if isDockerInterface(interfaceName) {
				ctx := context.Background()
				lc := &logger.Context{Action: "detect_external_ips"}
				ctx = logger.NewContext(ctx, lc)
				logger.Global.Debug(ctx, "跳过Docker接口", zap.String("ip", ip), zap.String("interface", interfaceName))
				continue
			}

			if !isValidIPv4(ip) {
				continue
			}

			isPreferred := false
			for _, preferred := range preferredInterfaces {
				if interfaceName == preferred {
					foundIPs = append(foundIPs, ip)
					isPreferred = true
					ctx := context.Background()
					lc := &logger.Context{Action: "detect_external_ips"}
					ctx = logger.NewContext(ctx, lc)
					logger.Global.Debug(ctx, "找到优先接口IP", zap.String("ip", ip), zap.String("interface", interfaceName))
					break
				}
			}

			if !isPreferred {
				fallbackIPs = append(fallbackIPs, ip)
				ctx := context.Background()
				lc := &logger.Context{Action: "detect_external_ips"}
				ctx = logger.NewContext(ctx, lc)
				logger.Global.Debug(ctx, "找到备选接口IP", zap.String("ip", ip), zap.String("interface", interfaceName))
			}
		} else {
			// 兼容旧格式，只有IP没有接口名
			ip := strings.Split(line, " ")[0]
			if isValidIPv4(ip) {
				fallbackIPs = append(fallbackIPs, ip)
			}
		}
	}
	
	ctx := context.Background()
	lc := &logger.Context{Action: "detect_external_ips"}
	ctx = logger.NewContext(ctx, lc)

	if len(foundIPs) > 0 {
		logger.Global.Debug(ctx, "选择优先IP", zap.String("ip", foundIPs[0]))
		return foundIPs[0], nil
	}

	if len(fallbackIPs) > 0 {
		logger.Global.Debug(ctx, "选择备选IP", zap.String("ip", fallbackIPs[0]))
		return fallbackIPs[0], nil
	}

	return "", fmt.Errorf("容器%s没有可用的IPv4地址", containerName)
}

func isDockerInterface(interfaceName string) bool {
	dockerInterfaces := []string{"docker0", "br-"}
	for _, dockerIf := range dockerInterfaces {
		if strings.HasPrefix(interfaceName, dockerIf) {
			return true
		}
	}
	return false
}

func isValidIPv4(ip string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}
	return parsedIP.To4() != nil && !parsedIP.IsLoopback()
}

func (m *IptablesNATManager) createCustomChains() error {
	return nil
}

func (m *IptablesNATManager) addIptablesRule(table, chain, rule, comment string) error {
	cmd := exec.Command("iptables", "-t", table, "-A", chain)
	
	ruleArgs := strings.Fields(rule)
	cmd.Args = append(cmd.Args, ruleArgs...)
	
	if comment != "" {
		cmd.Args = append(cmd.Args, "-m", "comment", "--comment", comment)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("执行iptables命令失败: %v, 输出: %s", err, string(output))
	}

	return nil
}

func (m *IptablesNATManager) removeIptablesRule(table, chain, rule, comment string) error {
	cmd := exec.Command("iptables", "-t", table, "-D", chain)
	
	ruleArgs := strings.Fields(rule)
	cmd.Args = append(cmd.Args, ruleArgs...)
	
	if comment != "" {
		cmd.Args = append(cmd.Args, "-m", "comment", "--comment", comment)
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("执行iptables删除命令失败: %v, 输出: %s", err, string(output))
	}

	return nil
}

func (m *IptablesNATManager) cleanupRulesByComment(table, chain, comment string) error {
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

	ctx := context.Background()
	lc := &logger.Context{Action: "cleanup_rules_by_comment"}
	ctx = logger.NewContext(ctx, lc)

	for i := len(lineNumbers) - 1; i >= 0; i-- {
		cmd := exec.Command("iptables", "-t", table, "-D", chain, strconv.Itoa(lineNumbers[i]))
		if err := cmd.Run(); err != nil {
			logger.Global.Warn(ctx, "删除iptables规则失败", zap.Error(err))
		}
	}

	return nil
}

func (m *IptablesNATManager) ruleExists(containerName string, externalPort, internalPort int, protocol string) (bool, error) {
	comment := fmt.Sprintf("%s_%s_%d_%d_%s", COMMENT_PREFIX, containerName, externalPort, internalPort, protocol)
	
	cmd := exec.Command("iptables", "-t", "nat", "-L", "PREROUTING", "-v")
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}

	return strings.Contains(string(output), comment), nil
}

func checkIptablesAvailable() error {
	cmd := exec.Command("iptables", "--version")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("iptables命令不可用: %v", err)
	}

	cmd = exec.Command("iptables", "-t", "nat", "-L", "-n")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("无权限操作iptables: %v", err)
	}

	return nil
}

func checkIPv6Support() bool {
	cmd := exec.Command("ip6tables", "--version")
	if err := cmd.Run(); err != nil {
		return false
	}

	cmd = exec.Command("ip6tables", "-t", "nat", "-L", "-n")
	if err := cmd.Run(); err != nil {
		return false
	}

	if _, err := exec.Command("cat", "/proc/net/if_inet6").Output(); err != nil {
		return false
	}

	if err := enableIPv6Forwarding(); err != nil {
		return false
	}

	ctx := context.Background()
	lc := &logger.Context{Action: "check_ipv6_support"}
	ctx = logger.NewContext(ctx, lc)
	logger.Global.Info(ctx, "IPv6和ip6tables支持可用")
	return true
}

// enableIPv6Forwarding 启用IPv6转发功能
func enableIPv6Forwarding() error {
	// 检查当前IPv6转发状态
	cmd := exec.Command("sysctl", "-n", "net.ipv6.conf.all.forwarding")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("检查IPv6转发状态失败: %v", err)
	}

	ctx := context.Background()
	lc := &logger.Context{Action: "enable_ipv6_forwarding"}
	ctx = logger.NewContext(ctx, lc)

	currentValue := strings.TrimSpace(string(output))
	if currentValue != "1" {
		cmd = exec.Command("sysctl", "-w", "net.ipv6.conf.all.forwarding=1")
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("启用IPv6转发失败: %v", err)
		}
		logger.Global.Info(ctx, "IPv6转发已启用")
	} else {
		logger.Global.Debug(ctx, "IPv6转发已经启用")
	}

	return nil
}

func (m *IptablesNATManager) detectExternalIPv6s(interfaceName string) ([]string, error) {
	if interfaceName == "" {
		return []string{}, fmt.Errorf("网卡名为空")
	}

	iface, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return []string{}, fmt.Errorf("网卡 %s 不存在: %v", interfaceName, err)
	}

	addrs, err := iface.Addrs()
	if err != nil {
		return []string{}, fmt.Errorf("获取网卡 %s 地址失败: %v", interfaceName, err)
	}

	var ipv6s []string
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipv6 := ipnet.IP.To16(); ipv6 != nil && ipnet.IP.To4() == nil && !ipnet.IP.IsLinkLocalUnicast() {
				ipv6s = append(ipv6s, ipv6.String())
			}
		}
	}

	return ipv6s, nil
}

func GetContainerIPv6(containerName string) (string, error) {
	cmd := exec.Command("lxc", "list", containerName, "--format", "csv", "-c", "6")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取容器列表失败: %v", err)
	}

	ctx := context.Background()
	lc := &logger.Context{
		Container: containerName,
		Action:    "get_container_ipv6",
	}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Debug(ctx, "获取容器IPv6地址", zap.String("output", strings.TrimSpace(string(output))))

	line := strings.TrimSpace(string(output))
	if line == "" || line == "-" {
		return "", fmt.Errorf("容器 %s 没有IPv6地址", containerName)
	}

	logger.Global.Debug(ctx, "处理行", zap.String("line", line))
	parts := strings.Fields(line)
	logger.Global.Debug(ctx, "分割后的部分", zap.Strings("parts", parts))

	if len(parts) >= 1 {
		ip := parts[0]
		logger.Global.Debug(ctx, "检查IP", zap.String("ip", ip))

		if parsedIP := net.ParseIP(ip); parsedIP != nil && parsedIP.To4() == nil {
			if parsedIP.IsLoopback() {
				logger.Global.Debug(ctx, "跳过回环地址", zap.String("ip", ip))
				return "", fmt.Errorf("容器 %s 的IPv6地址是回环地址(::1)，需要等待容器完全启动", containerName)
			}
			if parsedIP.IsLinkLocalUnicast() {
				logger.Global.Debug(ctx, "跳过链路本地地址", zap.String("ip", ip))
				return "", fmt.Errorf("容器 %s 的IPv6地址是链路本地地址，需要等待容器完全启动", containerName)
			}
			logger.Global.Debug(ctx, "找到有效IPv6地址", zap.String("ipv6", ip))
			return ip, nil
		} else {
			logger.Global.Debug(ctx, "IP无效或非IPv6", zap.String("ip", ip))
		}
	}

	return "", fmt.Errorf("未找到容器 %s 的有效IPv6地址", containerName)
}

func (m *IptablesNATManager) AddNATRuleIPv6(containerName string, externalPort, internalPort int, protocol string) error {
	if !m.enabled {
		return fmt.Errorf("iptables模式未启用")
	}

	if !m.ipv6Supported {
		return fmt.Errorf("IPv6支持未启用")
	}

	ctx := context.Background()
	lc := &logger.Context{
		Container: containerName,
		Action:    "add_nat_rule_ipv6",
	}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Debug(ctx, "开始添加IPv6 NAT规则",
		zap.Int("external_port", externalPort),
		zap.Int("internal_port", internalPort),
		zap.String("protocol", protocol))

	containerIPv6, err := GetContainerIPv6(containerName)
	if err != nil {
		return fmt.Errorf("获取容器IPv6地址失败: %v", err)
	}

	logger.Global.Debug(ctx, "获取到容器IPv6地址", zap.String("ipv6", containerIPv6))
	logger.Global.Debug(ctx, "网络配置",
		zap.String("external_interface", m.externalInterface),
		zap.Strings("external_ipv6s", m.externalIPv6s),
		zap.String("internal_interface", m.internalInterface))

	comment := fmt.Sprintf("%s_ipv6_%s_%d_%d_%s", COMMENT_PREFIX, containerName, externalPort, internalPort, protocol)

	dnatRule := m.buildDNATRuleIPv6(externalPort, internalPort, protocol, containerIPv6)
	logger.Global.Debug(ctx, "构建的DNAT规则", zap.String("rule", dnatRule))

	if err := m.addIP6tablesRule("nat", "PREROUTING", dnatRule, comment); err != nil {
		return fmt.Errorf("添加IPv6 DNAT规则失败: %v", err)
	}
	logger.Global.Debug(ctx, "IPv6 DNAT规则添加成功")

	forwardRule := m.buildForwardRuleIPv6(internalPort, protocol, containerIPv6)
	logger.Global.Debug(ctx, "构建的FORWARD规则", zap.String("rule", forwardRule))

	if err := m.addIP6tablesRule("filter", "FORWARD", forwardRule, comment); err != nil {
		m.removeIP6tablesRule("nat", "PREROUTING", dnatRule, comment)
		return fmt.Errorf("添加IPv6 FORWARD规则失败: %v", err)
	}
	logger.Global.Debug(ctx, "IPv6 FORWARD规则添加成功")

	logger.Global.Info(ctx, "IPv6 iptables NAT规则添加成功",
		zap.String("external_ipv6", m.getExternalIPv6String()),
		zap.Int("external_port", externalPort),
		zap.String("container_ipv6", containerIPv6),
		zap.Int("internal_port", internalPort),
		zap.String("protocol", protocol),
		zap.String("interface", m.externalInterface))
	
	// 保存规则
	if PersistentManager != nil && PersistentManager.IsEnabled() {
		PersistentManager.SaveRules()
	}
	
	return nil
}

func (m *IptablesNATManager) buildDNATRuleIPv6(externalPort, internalPort int, protocol, containerIPv6 string) string {
	var rule strings.Builder

	if m.externalInterface != "" {
		rule.WriteString(fmt.Sprintf("-i %s ", m.externalInterface))
	}

	if len(m.externalIPv6s) > 0 {
		rule.WriteString(fmt.Sprintf("-d %s ", m.externalIPv6s[0]))
	}

	rule.WriteString(fmt.Sprintf("-p %s --dport %d ", protocol, externalPort))
	rule.WriteString(fmt.Sprintf("-j DNAT --to-destination [%s]:%d", containerIPv6, internalPort))

	return rule.String()
}

func (m *IptablesNATManager) buildForwardRuleIPv6(internalPort int, protocol, containerIPv6 string) string {
	var rule strings.Builder

	if m.externalInterface != "" {
		rule.WriteString(fmt.Sprintf("-i %s ", m.externalInterface))
	}

	if m.internalInterface != "" {
		rule.WriteString(fmt.Sprintf("-o %s ", m.internalInterface))
	}

	rule.WriteString(fmt.Sprintf("-d %s -p %s --dport %d ", containerIPv6, protocol, internalPort))
	rule.WriteString("-j ACCEPT")

	return rule.String()
}

func (m *IptablesNATManager) getExternalIPv6String() string {
	if len(m.externalIPv6s) == 0 {
		return "::"
	}
	return m.externalIPv6s[0]
}

func (m *IptablesNATManager) addIP6tablesRule(table, chain, rule, comment string) error {
	ctx := context.Background()
	lc := &logger.Context{Action: "add_ip6tables_rule"}
	ctx = logger.NewContext(ctx, lc)

	cmd := exec.Command("ip6tables", "-t", table, "-A", chain, "-m", "comment", "--comment", comment)
	cmd.Args = append(cmd.Args, strings.Fields(rule)...)

	logger.Global.Debug(ctx, "执行ip6tables命令", zap.Strings("args", cmd.Args))

	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Global.Debug(ctx, "ip6tables命令失败", zap.String("output", string(output)))
		return fmt.Errorf("ip6tables命令失败: %v, 输出: %s", err, string(output))
	} else {
		logger.Global.Debug(ctx, "ip6tables命令执行成功")
		if len(output) > 0 {
			logger.Global.Debug(ctx, "ip6tables命令输出", zap.String("output", string(output)))
		}
	}
	return nil
}

func (m *IptablesNATManager) removeIP6tablesRule(table, chain, rule, comment string) error {
	cmd := exec.Command("ip6tables", "-t", table, "-D", chain, "-m", "comment", "--comment", comment)
	cmd.Args = append(cmd.Args, strings.Fields(rule)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ip6tables删除命令失败: %v, 输出: %s", err, string(output))
	}
	return nil
}

func (m *IptablesNATManager) cleanupRulesByCommentIPv6(table, chain, commentPattern string) error {
	cmd := exec.Command("ip6tables", "-t", table, "-L", chain, "--line-numbers", "-v")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("获取ip6tables规则列表失败: %v", err)
	}

	lines := strings.Split(string(output), "\n")
	var linesToDelete []string
	for _, line := range lines {
		if strings.Contains(line, commentPattern) {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				linesToDelete = append(linesToDelete, fields[0])
			}
		}
	}

	ctx := context.Background()
	lc := &logger.Context{Action: "cleanup_ipv6_rules_by_comment"}
	ctx = logger.NewContext(ctx, lc)

	for i := len(linesToDelete) - 1; i >= 0; i-- {
		lineNum := linesToDelete[i]
		cmd := exec.Command("ip6tables", "-t", table, "-D", chain, lineNum)
		if output, err := cmd.CombinedOutput(); err != nil {
			logger.Global.Warn(ctx, "删除IPv6规则失败",
				zap.String("line", lineNum),
				zap.Error(err),
				zap.String("output", string(output)))
		}
	}

	return nil
}

// AddNATRuleRangeIPv4 添加IPv4端口段NAT规则
func (m *IptablesNATManager) AddNATRuleRangeIPv4(containerName string, externalPortStart, externalPortEnd, internalPortStart, internalPortEnd int, protocol string) error {
	if !m.enabled {
		return fmt.Errorf("iptables模式未启用")
	}

	containerIP, err := GetContainerIP(containerName)
	if err != nil {
		return fmt.Errorf("获取容器IP失败: %v", err)
	}

	comment := fmt.Sprintf("%s_%s_%d-%d_%d-%d_%s", COMMENT_PREFIX, containerName, externalPortStart, externalPortEnd, internalPortStart, internalPortEnd, protocol)

	// 构建端口段DNAT规则
	dnatRule := m.buildDNATRuleRange(externalPortStart, externalPortEnd, internalPortStart, internalPortEnd, protocol, containerIP)

	if err := m.addIptablesRule("nat", "PREROUTING", dnatRule, comment); err != nil {
		return fmt.Errorf("添加端口段DNAT规则失败: %v", err)
	}

	forwardRule := m.buildForwardRuleRange(internalPortStart, internalPortEnd, protocol, containerIP)
	if err := m.addIptablesRule("filter", "FORWARD", forwardRule, comment); err != nil {
		m.removeIptablesRule("nat", "PREROUTING", dnatRule, comment)
		return fmt.Errorf("添加端口段FORWARD规则失败: %v", err)
	}

	ctx := context.Background()
	lc := &logger.Context{Action: "add_nat_range_ipv4"}
	ctx = logger.NewContext(ctx, lc)
	logger.Global.Info(ctx, "iptables端口段NAT规则添加成功",
		zap.String("external_ip", m.getExternalIPString()),
		zap.Int("external_port_start", externalPortStart),
		zap.Int("external_port_end", externalPortEnd),
		zap.String("container_ip", containerIP),
		zap.Int("internal_port_start", internalPortStart),
		zap.Int("internal_port_end", internalPortEnd),
		zap.String("protocol", protocol))

	return nil
}

// AddNATRuleRangeIPv6 添加IPv6端口段NAT规则
func (m *IptablesNATManager) AddNATRuleRangeIPv6(containerName string, externalPortStart, externalPortEnd, internalPortStart, internalPortEnd int, protocol string) error {
	if !m.enabled {
		return fmt.Errorf("iptables模式未启用")
	}

	if !m.ipv6Supported {
		return fmt.Errorf("IPv6支持未启用")
	}

	containerIPv6, err := GetContainerIPv6(containerName)
	if err != nil {
		return fmt.Errorf("获取容器IPv6地址失败: %v", err)
	}

	comment := fmt.Sprintf("%s_ipv6_%s_%d-%d_%d-%d_%s", COMMENT_PREFIX, containerName, externalPortStart, externalPortEnd, internalPortStart, internalPortEnd, protocol)

	dnatRule := m.buildDNATRuleRangeIPv6(externalPortStart, externalPortEnd, internalPortStart, internalPortEnd, protocol, containerIPv6)

	if err := m.addIP6tablesRule("nat", "PREROUTING", dnatRule, comment); err != nil {
		return fmt.Errorf("添加IPv6端口段DNAT规则失败: %v", err)
	}

	forwardRule := m.buildForwardRuleRangeIPv6(internalPortStart, internalPortEnd, protocol, containerIPv6)
	if err := m.addIP6tablesRule("filter", "FORWARD", forwardRule, comment); err != nil {
		m.removeIP6tablesRule("nat", "PREROUTING", dnatRule, comment)
		return fmt.Errorf("添加IPv6端口段FORWARD规则失败: %v", err)
	}

	ctx := context.Background()
	lc := &logger.Context{Action: "add_nat_range_ipv6"}
	ctx = logger.NewContext(ctx, lc)
	logger.Global.Info(ctx, "IPv6端口段NAT规则添加成功",
		zap.String("external_ipv6", m.getExternalIPv6String()),
		zap.Int("external_port_start", externalPortStart),
		zap.Int("external_port_end", externalPortEnd),
		zap.String("container_ipv6", containerIPv6),
		zap.Int("internal_port_start", internalPortStart),
		zap.Int("internal_port_end", internalPortEnd),
		zap.String("protocol", protocol))

	return nil
}

// buildDNATRuleRange 构建端口段DNAT规则
func (m *IptablesNATManager) buildDNATRuleRange(externalPortStart, externalPortEnd, internalPortStart, internalPortEnd int, protocol, containerIP string) string {
	var rule strings.Builder
	rule.WriteString(fmt.Sprintf("-p %s ", protocol))
	rule.WriteString(fmt.Sprintf("-d %s ", m.getExternalIPString()))
	rule.WriteString(fmt.Sprintf("--dport %d:%d ", externalPortStart, externalPortEnd))
	rule.WriteString(fmt.Sprintf("-j DNAT --to-destination %s:%d-%d", containerIP, internalPortStart, internalPortEnd))
	return rule.String()
}

// buildDNATRuleRangeIPv6 构建IPv6端口段DNAT规则
func (m *IptablesNATManager) buildDNATRuleRangeIPv6(externalPortStart, externalPortEnd, internalPortStart, internalPortEnd int, protocol, containerIPv6 string) string {
	var rule strings.Builder
	rule.WriteString(fmt.Sprintf("-p %s ", protocol))
	rule.WriteString(fmt.Sprintf("-d %s ", m.getExternalIPv6String()))
	rule.WriteString(fmt.Sprintf("--dport %d:%d ", externalPortStart, externalPortEnd))
	rule.WriteString(fmt.Sprintf("-j DNAT --to-destination [%s]:%d-%d", containerIPv6, internalPortStart, internalPortEnd))
	return rule.String()
}

// buildForwardRuleRange 构建端口段FORWARD规则
func (m *IptablesNATManager) buildForwardRuleRange(internalPortStart, internalPortEnd int, protocol, containerIP string) string {
	var rule strings.Builder
	rule.WriteString(fmt.Sprintf("-p %s ", protocol))
	rule.WriteString(fmt.Sprintf("-d %s ", containerIP))
	rule.WriteString(fmt.Sprintf("--dport %d:%d ", internalPortStart, internalPortEnd))
	rule.WriteString(fmt.Sprintf("-i %s -j ACCEPT", m.externalInterface))
	return rule.String()
}

// buildForwardRuleRangeIPv6 构建IPv6端口段FORWARD规则
func (m *IptablesNATManager) buildForwardRuleRangeIPv6(internalPortStart, internalPortEnd int, protocol, containerIPv6 string) string {
	var rule strings.Builder
	rule.WriteString(fmt.Sprintf("-p %s ", protocol))
	rule.WriteString(fmt.Sprintf("-d %s ", containerIPv6))
	rule.WriteString(fmt.Sprintf("--dport %d:%d ", internalPortStart, internalPortEnd))
	rule.WriteString(fmt.Sprintf("-i %s -j ACCEPT", m.externalInterface))
	return rule.String()
}