package services

import (
	"context"
	"fmt"
	"math/rand/v2"
	"net"
	"strings"

	"lxdapi/database"
	"lxdapi/models"
	"lxdapi/pkg/logger"

	"go.uber.org/zap"
)

func AddNATRule(containerName string, externalPort, internalPort int, protocol string) error {
	return AddNATRuleWithIPVersion(containerName, externalPort, internalPort, protocol, "ipv4", "")
}

func AddNATRuleForContainer(containerName string, externalPort, internalPort int, protocol, description string) error {
	// 只支持 IPv4 NAT
	ipVersion := "ipv4"
	return AddNATRuleWithIPVersion(containerName, externalPort, internalPort, protocol, ipVersion, description)
}

// AddNATRuleForContainerV2 新版NAT规则添加，支持端口段和多协议
func AddNATRuleForContainerV2(req models.AddNATRequest) error {
	if !ContainerExists(req.Hostname) {
		return fmt.Errorf("容器 %s 不存在", req.Hostname)
	}

	// 根据容器的网络模式决定IP版本
	var containerConfig models.ContainerConfig
	if err := database.DB.Where("container_name = ?", req.Hostname).First(&containerConfig).Error; err != nil {
		return fmt.Errorf("查询容器配置失败: %v", err)
	}

	networkMode := containerConfig.NetworkMode
	if networkMode == "" {
		networkMode = "mode1"
	}

	// 根据网络模式决定IP版本
	var ipVersion string
	switch networkMode {
	case "mode1", "mode2":
		ipVersion = "ipv4"
	default:
		ipVersion = "ipv4"
	}

	var protocols []string
	if req.Protocol == "both" {
		protocols = []string{"tcp", "udp"}
	} else {
		protocols = []string{req.Protocol}
	}

	// 判断是单端口还是端口段
	isPortRange := req.ExternalPortEnd > 0 && req.InternalPortEnd > 0

	if isPortRange {
		// 端口段模式
		return addNATRuleRange(req, protocols, ipVersion)
	} else {
		// 单端口模式
		return addNATRuleSingle(req, protocols, ipVersion)
	}
}

// addNATRuleSingle 添加单端口NAT规则（支持多协议）
func addNATRuleSingle(req models.AddNATRequest, protocols []string, ipVersion string) error {
	externalPort := req.ExternalPort

	// 如果没有指定外部端口，自动分配
	if externalPort == 0 {
		var err error
		externalPort, err = NextAvailableExternalPort(protocols[0])
		if err != nil {
			return fmt.Errorf("自动分配外网端口失败: %v", err)
		}
	}

	// 为每个协议添加规则
	for _, protocol := range protocols {
		// 检查端口可用性
		if err := ValidateExternalPortAvailability(externalPort, protocol); err != nil {
			return fmt.Errorf("端口 %d/%s %v", externalPort, protocol, err)
		}

		// 添加NAT规则
		if err := addNATRuleWithProtocol(req.Hostname, externalPort, req.InternalPort, 0, 0, protocol, ipVersion, req.Description); err != nil {
			return err
		}
	}

	return nil
}

// addNATRuleRange 添加端口段NAT规则（支持多协议）
func addNATRuleRange(req models.AddNATRequest, protocols []string, ipVersion string) error {
	rangeSize := req.ExternalPortEnd - req.ExternalPort + 1

	// 检查所有端口是否可用
	for i := 0; i < rangeSize; i++ {
		extPort := req.ExternalPort + i
		for _, protocol := range protocols {
			if err := ValidateExternalPortAvailability(extPort, protocol); err != nil {
				return fmt.Errorf("端口 %d/%s 不可用: %v", extPort, protocol, err)
			}
		}
	}

	// 为每个协议添加端口段规则
	for _, protocol := range protocols {
		if err := addNATRuleWithProtocol(
			req.Hostname,
			req.ExternalPort,
			req.InternalPort,
			req.ExternalPortEnd,
			req.InternalPortEnd,
			protocol,
			ipVersion,
			req.Description,
		); err != nil {
			return err
		}
	}

	return nil
}

// addNATRuleWithProtocol 添加单个协议的NAT规则（支持端口段）
func addNATRuleWithProtocol(containerName string, externalPort, internalPort, externalPortEnd, internalPortEnd int, protocol, ipVersion, description string) error {
	if IptablesManager == nil {
		return fmt.Errorf("iptables NAT管理器未初始化")
	}

	// 根据IP版本检查对应的NAT功能是否启用
	switch ipVersion {
	case "ipv4":
		if !IptablesManager.IsEnabled() {
			return fmt.Errorf("IPv4 NAT功能未启用")
		}
	case "ipv6":
		if !IptablesManager.ipv6Supported {
			return fmt.Errorf("IPv6 NAT功能未启用")
		}
	case "dual":
		if !IptablesManager.IsEnabled() && !IptablesManager.ipv6Supported {
			return fmt.Errorf("IPv4和IPv6 NAT功能均未启用")
		}
	}

	// 判断是否为端口段
	isPortRange := externalPortEnd > 0 && internalPortEnd > 0

	// 创建iptables规则
	actualIPVersion := ipVersion

	if isPortRange {
		// 端口段规则
		switch ipVersion {
		case "ipv4":
			if err := IptablesManager.AddNATRuleRangeIPv4(containerName, externalPort, externalPortEnd, internalPort, internalPortEnd, protocol); err != nil {
				return fmt.Errorf("添加IPv4端口段NAT规则失败: %v", err)
			}
		case "ipv6":
			if !IptablesManager.ipv6Supported {
				return fmt.Errorf("系统IPv6支持未启用")
			}
			if err := IptablesManager.AddNATRuleRangeIPv6(containerName, externalPort, externalPortEnd, internalPort, internalPortEnd, protocol); err != nil {
				return fmt.Errorf("添加IPv6端口段NAT规则失败: %v", err)
			}
		case "dual":
			if err := IptablesManager.AddNATRuleRangeIPv4(containerName, externalPort, externalPortEnd, internalPort, internalPortEnd, protocol); err != nil {
				return fmt.Errorf("添加IPv4端口段NAT规则失败: %v", err)
			}
			actualIPVersion = "ipv4"
			if IptablesManager.ipv6Supported {
				if err := IptablesManager.AddNATRuleRangeIPv6(containerName, externalPort, externalPortEnd, internalPort, internalPortEnd, protocol); err != nil {
					ctx := context.Background()
					lc := &logger.Context{Container: containerName, Action: "add_nat_range_ipv6"}
					ctx = logger.NewContext(ctx, lc)
					logger.Global.Warn(ctx, "添加IPv6端口段NAT规则失败", zap.Error(err))
				} else {
					actualIPVersion = "dual"
				}
			}
		}
	} else {
		// 单端口规则
		switch ipVersion {
		case "ipv4":
			if err := IptablesManager.AddNATRuleIPv4(containerName, externalPort, internalPort, protocol); err != nil {
				return fmt.Errorf("添加IPv4 NAT规则失败: %v", err)
			}
		case "ipv6":
			if !IptablesManager.ipv6Supported {
				return fmt.Errorf("系统IPv6支持未启用")
			}
			if err := IptablesManager.AddNATRuleIPv6(containerName, externalPort, internalPort, protocol); err != nil {
				return fmt.Errorf("添加IPv6 NAT规则失败: %v", err)
			}
		case "dual":
			if err := IptablesManager.AddNATRuleIPv4(containerName, externalPort, internalPort, protocol); err != nil {
				return fmt.Errorf("添加IPv4 NAT规则失败: %v", err)
			}
			actualIPVersion = "ipv4"
			if IptablesManager.ipv6Supported {
				if err := IptablesManager.AddNATRuleIPv6(containerName, externalPort, internalPort, protocol); err != nil {
					ctx := context.Background()
					lc := &logger.Context{Container: containerName, Action: "add_nat_ipv6"}
					ctx = logger.NewContext(ctx, lc)
					logger.Global.Warn(ctx, "添加IPv6 NAT规则失败", zap.Error(err))
				} else {
					actualIPVersion = "dual"
				}
			}
		}
	}

	// 保存到数据库
	natRule := models.NATRule{
		ContainerName:   containerName,
		ExternalPort:    externalPort,
		InternalPort:    internalPort,
		ExternalPortEnd: externalPortEnd,
		InternalPortEnd: internalPortEnd,
		Protocol:        protocol,
		IPVersion:       actualIPVersion,
		Status:          "active",
		Description:     description,
		NATMethod:       "iptables",
	}

	// 自动生成描述
	if natRule.Description == "" {
		if isPortRange {
			natRule.Description = fmt.Sprintf("端口段转发(%s): %s:%d-%d -> %d-%d (%s)",
				actualIPVersion, containerName, externalPort, externalPortEnd, internalPort, internalPortEnd, protocol)
		} else {
			natRule.Description = fmt.Sprintf("端口转发(%s): %s:%d -> %d (%s)",
				actualIPVersion, containerName, externalPort, internalPort, protocol)
		}
	}

	result := database.DB.Where("container_name = ? AND external_port = ? AND protocol = ?",
		containerName, externalPort, protocol).FirstOrCreate(&natRule)
	if result.Error != nil {
		ctx := context.Background()
		lc := &logger.Context{Container: containerName, Action: "save_nat_rule"}
		ctx = logger.NewContext(ctx, lc)
		logger.Global.Error(ctx, "保存NAT规则到数据库失败", zap.Error(result.Error))
		return fmt.Errorf("保存NAT规则到数据库失败: %v", result.Error)
	}

	if result.RowsAffected == 0 {
		database.DB.Model(&natRule).Updates(map[string]interface{}{
			"status":            "active",
			"internal_port":     internalPort,
			"external_port_end": externalPortEnd,
			"internal_port_end": internalPortEnd,
			"description":       natRule.Description,
			"nat_method":        "iptables",
		})
	}

	ctx := context.Background()
	lc := &logger.Context{Container: containerName, Action: "add_nat_rule"}
	ctx = logger.NewContext(ctx, lc)

	if isPortRange {
		logger.Global.Info(ctx, fmt.Sprintf("NAT端口段规则添加成功(%s)", actualIPVersion),
			zap.Int("external_port_start", externalPort),
			zap.Int("external_port_end", externalPortEnd),
			zap.Int("internal_port_start", internalPort),
			zap.Int("internal_port_end", internalPortEnd),
			zap.String("protocol", protocol))
	} else {
		logger.Global.Info(ctx, fmt.Sprintf("NAT规则添加成功(%s)", actualIPVersion),
			zap.Int("external_port", externalPort),
			zap.Int("internal_port", internalPort),
			zap.String("protocol", protocol))
	}

	return nil
}

func ValidateExternalPortAvailability(port int, protocol string) error {
	available, reason := CheckExternalPortAvailability(port, protocol, "")
	if !available {
		if reason == "" {
			reason = fmt.Sprintf("端口 %d/%s 不可用", port, protocol)
		}
		return fmt.Errorf(reason)
	}
	return nil
}

func CheckExternalPortAvailability(port int, protocol string, excludeContainer string) (bool, string) {
	if port < 10000 || port > 65535 {
		return false, "端口号必须在10000-65535之间"
	}
	if protocol == "" {
		protocol = "tcp"
	}
	if protocol != "tcp" && protocol != "udp" && protocol != "both" {
		return false, "仅支持TCP、UDP或BOTH协议"
	}
	if IptablesManager == nil {
		return false, "iptables NAT管理器未初始化"
	}

	// both协议需要检查TCP和UDP
	protocols := []string{protocol}
	if protocol == "both" {
		protocols = []string{"tcp", "udp"}
	}

	for _, proto := range protocols {
		if IsExternalPortUsed(port, proto) {
			return false, fmt.Sprintf("端口 %d/%s 已被占用", port, strings.ToUpper(proto))
		}
	}

	for _, proto := range protocols {
		if isLocalPortBound(port, proto) {
			return false, fmt.Sprintf("端口 %d/%s 被服务器本地进程占用", port, strings.ToUpper(proto))
		}
	}

	return true, ""
}

func AddNATRuleForService(containerName string, externalPort, internalPort int, protocol, serviceName string) error {
	description := fmt.Sprintf("%s服务端口: %s:%d -> %d (%s)", serviceName, containerName, externalPort, internalPort, protocol)
	return AddNATRuleWithIPVersion(containerName, externalPort, internalPort, protocol, "ipv4", description)
}

// ListNATRules 获取容器的所有NAT规则
func ListNATRules(containerName string) ([]models.NATRule, error) {
	var rules []models.NATRule
	err := database.DB.Where("container_name = ?", containerName).Find(&rules).Error
	if err != nil {
		return nil, fmt.Errorf("查询NAT规则失败: %v", err)
	}

	return rules, nil
}

// ListAllNATRules 获取所有NAT规则（不限容器）
func ListAllNATRules() ([]models.NATRule, error) {
	var rules []models.NATRule
	err := database.DB.Order("external_port ASC").Find(&rules).Error
	if err != nil {
		return nil, fmt.Errorf("查询所有NAT规则失败: %v", err)
	}

	return rules, nil
}

func CleanupAllNATRules(containerName string) error {
	ctx := context.Background()
	lc := &logger.Context{
		Container: containerName,
		Action:    "cleanup_all_nat_rules",
	}
	ctx = logger.NewContext(ctx, lc)

	if IptablesManager == nil {
		logger.Global.Warn(ctx, "iptables NAT管理器未初始化，跳过iptables规则清理")
	} else {
		logger.Global.Info(ctx, "开始清理所有用户NAT规则")
	}

	var natRules []models.NATRule
	err := database.DB.Where("container_name = ?", containerName).Find(&natRules).Error
	if err != nil {
		return fmt.Errorf("查询用户NAT规则失败: %v", err)
	}

	if len(natRules) == 0 {
		logger.Global.Info(ctx, "没有需要清理的用户NAT规则")
		return nil
	}

	logger.Global.Info(ctx, "找到用户NAT规则", zap.Int("count", len(natRules)))

	cleanedCount := 0
	for _, rule := range natRules {
		logger.Global.Debug(ctx, "清理NAT规则",
			zap.Int("external_port", rule.ExternalPort),
			zap.Int("internal_port", rule.InternalPort),
			zap.String("protocol", rule.Protocol))

		if IptablesManager != nil && IptablesManager.IsEnabled() {
			if err := IptablesManager.RemoveNATRule(containerName, rule.ExternalPort, rule.InternalPort, rule.Protocol); err != nil {
				logger.Global.Warn(ctx, "清理iptables NAT规则失败", zap.Error(err))
			}
		}

		if err := database.DB.Unscoped().Delete(&rule).Error; err != nil {
			logger.Global.Warn(ctx, "从数据库删除NAT规则失败", zap.Error(err))
		} else {
			cleanedCount++
		}
	}

	logger.Global.Info(ctx, "用户NAT规则清理完成",
		zap.Int("cleaned", cleanedCount),
		zap.Int("total", len(natRules)))
	return nil
}

func DeleteNATRule(containerName string, externalPort, internalPort int, protocol string) error {
	ctx := context.Background()
	lc := &logger.Context{
		Container: containerName,
		Action:    "delete_nat_rule",
	}
	ctx = logger.NewContext(ctx, lc)

	if IptablesManager == nil {
		return fmt.Errorf("iptables NAT管理器未初始化")
	}
	// 删除操作不需要检查具体功能是否启用，只要管理器初始化即可

	logger.Global.Info(ctx, "开始删除NAT规则",
		zap.Int("external_port", externalPort),
		zap.Int("internal_port", internalPort),
		zap.String("protocol", protocol))

	var rule models.NATRule
	err := database.DB.Where("container_name = ? AND external_port = ? AND internal_port = ? AND protocol = ?",
		containerName, externalPort, internalPort, protocol).First(&rule).Error
	if err != nil {
		if err.Error() == "record not found" || err.Error() == "not found" {
			return fmt.Errorf("NAT规则不存在: %s:%d -> %d (%s)", containerName, externalPort, internalPort, protocol)
		}
		return fmt.Errorf("查询NAT规则失败: %v", err)
	}

	if err := IptablesManager.RemoveNATRule(containerName, externalPort, internalPort, protocol); err != nil {
		logger.Global.Error(ctx, "删除iptables NAT规则失败", zap.Error(err))
		return fmt.Errorf("删除iptables NAT规则失败: %v", err)
	}

	deleteErr := database.DB.Unscoped().Delete(&rule).Error
	if deleteErr != nil {
		logger.Global.Error(ctx, "从数据库删除NAT规则失败", zap.Error(deleteErr))
		return fmt.Errorf("从数据库删除NAT规则失败: %v", deleteErr)
	}

	logger.Global.Info(ctx, "NAT规则删除成功",
		zap.Int("external_port", externalPort),
		zap.Int("internal_port", internalPort),
		zap.String("protocol", protocol))

	return nil
}

func DeleteNATRuleRange(containerName string, externalPortStart, externalPortEnd, internalPortStart, internalPortEnd int, protocol string) error {
	ctx := context.Background()
	lc := &logger.Context{
		Container: containerName,
		Action:    "delete_nat_rule_range",
	}
	ctx = logger.NewContext(ctx, lc)

	if IptablesManager == nil {
		return fmt.Errorf("iptables NAT管理器未初始化")
	}

	logger.Global.Info(ctx, "开始删除NAT端口段规则",
		zap.Int("external_port_start", externalPortStart),
		zap.Int("external_port_end", externalPortEnd),
		zap.Int("internal_port_start", internalPortStart),
		zap.Int("internal_port_end", internalPortEnd),
		zap.String("protocol", protocol))

	var rule models.NATRule
	err := database.DB.Where("container_name = ? AND external_port = ? AND internal_port = ? AND external_port_end = ? AND internal_port_end = ? AND protocol = ?",
		containerName, externalPortStart, internalPortStart, externalPortEnd, internalPortEnd, protocol).First(&rule).Error
	if err != nil {
		if err.Error() == "record not found" || err.Error() == "not found" {
			return fmt.Errorf("NAT端口段规则不存在: %s:%d-%d -> %d-%d (%s)", containerName, externalPortStart, externalPortEnd, internalPortStart, internalPortEnd, protocol)
		}
		return fmt.Errorf("查询NAT端口段规则失败: %v", err)
	}

	if err := IptablesManager.RemoveNATRuleRange(containerName, externalPortStart, externalPortEnd, internalPortStart, internalPortEnd, protocol); err != nil {
		logger.Global.Error(ctx, "删除iptables NAT端口段规则失败", zap.Error(err))
		return fmt.Errorf("删除iptables NAT端口段规则失败: %v", err)
	}

	deleteErr := database.DB.Unscoped().Delete(&rule).Error
	if deleteErr != nil {
		logger.Global.Error(ctx, "从数据库删除NAT端口段规则失败", zap.Error(deleteErr))
		return fmt.Errorf("从数据库删除NAT端口段规则失败: %v", deleteErr)
	}

	logger.Global.Info(ctx, "NAT端口段规则删除成功",
		zap.Int("external_port_start", externalPortStart),
		zap.Int("external_port_end", externalPortEnd),
		zap.Int("internal_port_start", internalPortStart),
		zap.Int("internal_port_end", internalPortEnd),
		zap.String("protocol", protocol))

	return nil
}

func NextAvailableExternalPort(protocol string) (int, error) {
	const minPort = 10000
	const maxPort = 65535

	start := rand.IntN(maxPort-minPort+1) + minPort

	port := start
	for tried := 0; tried <= (maxPort - minPort); tried++ {
		if !IsExternalPortUsed(port, protocol) && !isLocalPortBound(port, protocol) {
			return port, nil
		}
		port++
		if port > maxPort {
			port = minPort
		}
	}
	return 0, fmt.Errorf("没有可用端口")
}

// isLocalPortBound 检测端口是否被本机服务占用
func isLocalPortBound(port int, protocol string) bool {
	addr := fmt.Sprintf(":%d", port)
	if protocol == "udp" {
		conn, err := net.ListenPacket("udp", addr)
		if err != nil {
			return true
		}
		_ = conn.Close()
		return false
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return true
	}
	_ = ln.Close()
	return false
}

// IsExternalPortUsed 检查端口是否被NAT规则或SSH占用（支持端口段检测）
func IsExternalPortUsed(port int, protocol string) bool {
	var natCount int64
	database.DB.Model(&models.NATRule{}).Where(
		"protocol = ? AND ((external_port = ? AND (external_port_end IS NULL OR external_port_end = 0)) OR (external_port <= ? AND external_port_end >= ?))",
		protocol, port, port, port).Count(&natCount)

	ctx := context.Background()
	lc := &logger.Context{Action: "check_port_used"}
	ctx = logger.NewContext(ctx, lc)
	logger.Global.Debug(ctx, "端口检查",
		zap.Int("port", port),
		zap.String("protocol", protocol),
		zap.Int64("nat_count", natCount))
	return natCount > 0
}

// IsExternalPortUsedExcluding 检查端口是否被NAT规则占用（排除指定容器，支持端口段检测）
func IsExternalPortUsedExcluding(port int, protocol, excludeContainer string) bool {
	var natCount int64
	database.DB.Model(&models.NATRule{}).Where(
		"protocol = ? AND container_name != ? AND ((external_port = ? AND (external_port_end IS NULL OR external_port_end = 0)) OR (external_port <= ? AND external_port_end >= ?))",
		protocol, excludeContainer, port, port, port).Count(&natCount)

	ctx := context.Background()
	lc := &logger.Context{
		Container: excludeContainer,
		Action:    "check_port_excluding",
	}
	ctx = logger.NewContext(ctx, lc)
	logger.Global.Debug(ctx, "端口检查(排除容器)",
		zap.Int("port", port),
		zap.String("protocol", protocol),
		zap.Int64("nat_count", natCount))
	return natCount > 0
}

// AddNATRuleWithIPVersion 添加指定IP版本的NAT规则（ipv4/ipv6/dual）
func AddNATRuleWithIPVersion(containerName string, externalPort, internalPort int, protocol, ipVersion, description string) error {
	if !ContainerExists(containerName) {
		return fmt.Errorf("容器 %s 不存在", containerName)
	}

	if externalPort == 0 {
		var err error
		externalPort, err = NextAvailableExternalPort(protocol)
		if err != nil {
			return fmt.Errorf("自动分配外网端口失败: %v", err)
		}
	}

	if err := ValidateExternalPortAvailability(externalPort, protocol); err != nil {
		// 如果同一容器已有该端口规则，返回更具体的提示
		if IsExternalPortUsed(externalPort, protocol) {
			var existingCount int64
			database.DB.Model(&models.NATRule{}).Where("container_name = ? AND external_port = ? AND protocol = ?", containerName, externalPort, protocol).
				Count(&existingCount)
			if existingCount > 0 {
				return fmt.Errorf("该容器已存在外网端口 %d/%s 的转发规则", externalPort, protocol)
			}
		}
		return err
	}

	if IsExternalPortUsedExcluding(externalPort, protocol, containerName) {
		return fmt.Errorf("外网端口 %d/%s 已被占用", externalPort, protocol)
	}

	if IptablesManager == nil {
		return fmt.Errorf("iptables NAT管理器未初始化")
	}

	// 根据IP版本检查对应的NAT功能是否启用
	switch ipVersion {
	case "ipv4":
		if !IptablesManager.IsEnabled() {
			return fmt.Errorf("IPv4 NAT功能未启用")
		}
	case "ipv6":
		if !IptablesManager.ipv6Supported {
			return fmt.Errorf("IPv6 NAT功能未启用")
		}
	case "dual":
		if !IptablesManager.IsEnabled() && !IptablesManager.ipv6Supported {
			return fmt.Errorf("IPv4和IPv6 NAT功能均未启用")
		}
	}

	// 记录实际创建的IP版本
	actualIPVersion := ipVersion
	ipv6Created := false

	switch ipVersion {
	case "ipv4":
		if err := IptablesManager.AddNATRuleIPv4(containerName, externalPort, internalPort, protocol); err != nil {
			return fmt.Errorf("添加IPv4 NAT规则失败: %v", err)
		}
		actualIPVersion = "ipv4"
	case "ipv6":
		if !IptablesManager.ipv6Supported {
			return fmt.Errorf("系统IPv6支持未启用")
		}
		if err := IptablesManager.AddNATRuleIPv6(containerName, externalPort, internalPort, protocol); err != nil {
			return fmt.Errorf("添加IPv6 NAT规则失败: %v", err)
		}
		actualIPVersion = "ipv6"
	case "dual":
		if err := IptablesManager.AddNATRuleIPv4(containerName, externalPort, internalPort, protocol); err != nil {
			return fmt.Errorf("添加IPv4 NAT规则失败: %v", err)
		}
		actualIPVersion = "ipv4"
		if IptablesManager.ipv6Supported {
			if err := IptablesManager.AddNATRuleIPv6(containerName, externalPort, internalPort, protocol); err != nil {
				ctx := context.Background()
				lc := &logger.Context{Container: containerName, Action: "add_nat_ipv6"}
				ctx = logger.NewContext(ctx, lc)
				logger.Global.Warn(ctx, "添加IPv6 NAT规则失败", zap.Error(err))
			} else {
				ipv6Created = true
				actualIPVersion = "dual"
			}
		}
	default:
		return fmt.Errorf("不支持的IP版本: %s", ipVersion)
	}

	natRule := models.NATRule{
		ContainerName: containerName,
		ExternalPort:  externalPort,
		InternalPort:  internalPort,
		Protocol:      protocol,
		IPVersion:     actualIPVersion, // 使用实际创建的IP版本
		Status:        "active",
		Description:   description,
		NATMethod:     "iptables",
	}

	// 如果没有提供描述，使用默认描述
	if natRule.Description == "" {
		natRule.Description = fmt.Sprintf("端口转发(%s): %s:%d -> %d (%s)", actualIPVersion, containerName, externalPort, internalPort, protocol)
	}

	result := database.DB.Where("container_name = ? AND external_port = ? AND protocol = ?",
		containerName, externalPort, protocol).FirstOrCreate(&natRule)
	if result.Error != nil {
		ctx := context.Background()
		lc := &logger.Context{Container: containerName, Action: "save_nat_rule"}
		ctx = logger.NewContext(ctx, lc)
		logger.Global.Error(ctx, "保存NAT规则到数据库失败", zap.Error(result.Error))
		return fmt.Errorf("保存NAT规则到数据库失败: %v", result.Error)
	}

	if result.RowsAffected == 0 {
		database.DB.Model(&natRule).Updates(map[string]interface{}{
			"status":        "active",
			"internal_port": internalPort,
			"description":   natRule.Description,
			"nat_method":    "iptables",
		})
		ctx := context.Background()
		lc := &logger.Context{Container: containerName, Action: "update_nat_rule"}
		ctx = logger.NewContext(ctx, lc)
		logger.Global.Info(ctx, "NAT规则已存在，已更新状态",
			zap.Int("external_port", externalPort),
			zap.Int("internal_port", internalPort),
			zap.String("protocol", protocol))
	}

	ctx := context.Background()
	lc := &logger.Context{Container: containerName, Action: "add_nat_rule"}
	ctx = logger.NewContext(ctx, lc)

	logMsg := fmt.Sprintf("NAT规则添加成功(%s)", actualIPVersion)
	if ipVersion == "dual" && !ipv6Created {
		logMsg += " [仅IPv4，IPv6未启用或创建失败]"
	}
	logger.Global.Info(ctx, logMsg,
		zap.Int("external_port", externalPort),
		zap.Int("internal_port", internalPort),
		zap.String("protocol", protocol))

	return nil
}
