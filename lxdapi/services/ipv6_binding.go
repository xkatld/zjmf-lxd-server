package services

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"lxdapi/config"
	"lxdapi/database"
	"lxdapi/models"
	"lxdapi/pkg/logger"

	"go.uber.org/zap"
)


type IPv6BindingManager struct {
	enabled       bool
	targetInterface string
	ipv6Pool      IPv6Pool
}


type IPv6Pool struct {
	StartAddr    *big.Int
	PrefixLength int
	PoolSize     int
	BasePrefix   string
}

var IPv6Manager *IPv6BindingManager


func InitIPv6BindingManager() error {
	// 检查IPv6绑定功能是否在配置中启用
	if !config.AppConfig.IPv6Binding.Enabled {
		IPv6Manager = &IPv6BindingManager{enabled: false}
		return nil
	}

	if !checkIPv6Support() {
		IPv6Manager = &IPv6BindingManager{enabled: false}
		return nil
	}


	targetInterface := config.AppConfig.IPv6Binding.Interface
	if targetInterface == "" {

	}


	if !interfaceExists(targetInterface) {
		IPv6Manager = &IPv6BindingManager{enabled: false}
		return nil
	}


	pool, err := initIPv6Pool()
	if err != nil {
		IPv6Manager = &IPv6BindingManager{enabled: false}
		return nil
	}

	IPv6Manager = &IPv6BindingManager{
		enabled:         true,
		targetInterface: targetInterface,
		ipv6Pool:       *pool,
	}


	if err := initIPv6BindingTable(); err != nil {
		return fmt.Errorf("初始化IPv6绑定数据表失败: %v", err)
	}

	ctx := context.Background()
	lc := &logger.Context{
		Action: "init_ipv6_binding_manager",
	}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Info(ctx, "IPv6绑定管理器初始化成功")
	logger.Global.Info(ctx, "目标网卡", zap.String("interface", targetInterface))
	logger.Global.Info(ctx, "IPv6地址池",
		zap.String("prefix", pool.BasePrefix),
		zap.Int("prefix_length", pool.PrefixLength),
		zap.Int("pool_size", pool.PoolSize))

	if err := IPv6Manager.restoreIPv6Addresses(); err != nil {
		logger.Global.Warn(ctx, "恢复IPv6地址失败", zap.Error(err))
	}

	return nil
}


func initIPv6Pool() (*IPv6Pool, error) {
	startAddr := config.AppConfig.IPv6Binding.IPv6Pool.Start
	if startAddr == "" {

	}

	prefixLength := config.AppConfig.IPv6Binding.IPv6Pool.PrefixLength
	if prefixLength == 0 {
		prefixLength = 64
	}

	poolSize := config.AppConfig.IPv6Binding.IPv6Pool.PoolSize
	if poolSize == 0 {
		poolSize = 1000
	}


	ip := net.ParseIP(startAddr)
	if ip == nil || ip.To4() != nil {
		return nil, fmt.Errorf("无效的IPv6起始地址: %s", startAddr)
	}


	ipInt := new(big.Int)
	ipInt.SetBytes(ip.To16())

	return &IPv6Pool{
		StartAddr:    ipInt,
		PrefixLength: prefixLength,
		PoolSize:     poolSize,
		BasePrefix:   startAddr,
	}, nil
}


func initIPv6BindingTable() error {
	return database.DB.AutoMigrate(&models.IPv6BindingRule{})
}


func (m *IPv6BindingManager) IsEnabled() bool {
	return m != nil && m.enabled
}


func (m *IPv6BindingManager) PreAllocateIPv6(containerName string) (string, error) {
	if !m.IsEnabled() {
		return "", fmt.Errorf("IPv6绑定管理器未启用")
	}

	publicIPv6, err := m.generateNextIPv6()
	if err != nil {
		return "", fmt.Errorf("生成IPv6地址失败: %v", err)
	}

	bindingRule := &models.IPv6BindingRule{
		ContainerName: containerName,
		PublicIPv6:    publicIPv6,
		ContainerIPv6: "::",
		Status:        "reserved",
		Description:   "Pre-allocated for container creation",
		Interface:     m.targetInterface,
	}

	if err := database.DB.Create(bindingRule).Error; err != nil {
		return "", fmt.Errorf("保存IPv6预分配记录失败: %v", err)
	}

	return publicIPv6, nil
}

func (m *IPv6BindingManager) ActivatePreAllocatedIPv6(containerName, containerIPv6 string) error {
	var rule models.IPv6BindingRule
	if err := database.DB.Where("container_name = ? AND status = ?", containerName, "reserved").
		First(&rule).Error; err != nil {
		return fmt.Errorf("未找到预分配的IPv6: %v", err)
	}

	ctx := context.Background()
	lc := &logger.Context{
		Container: containerName,
		Action:    "activate_ipv6",
	}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Debug(ctx, "激活预分配的IPv6地址",
		zap.String("public_ipv6", rule.PublicIPv6),
		zap.String("container_ipv6", containerIPv6))

	if err := m.addIPv6ToInterface(rule.PublicIPv6); err != nil {
		return fmt.Errorf("添加IPv6地址到网卡失败: %v", err)
	}

	if err := m.createIPv6ForwardingRules(rule.PublicIPv6, containerIPv6); err != nil {
		m.removeIPv6FromInterface(rule.PublicIPv6)
		return fmt.Errorf("创建IPv6转发规则失败: %v", err)
	}

	rule.ContainerIPv6 = containerIPv6
	rule.Status = "active"

	if err := database.DB.Save(&rule).Error; err != nil {
		return fmt.Errorf("更新IPv6绑定规则失败: %v", err)
	}

	return nil
}

func (m *IPv6BindingManager) AddIPv6Binding(containerName string, containerIPv6 string) (string, error) {
	if !m.IsEnabled() {
		return "", fmt.Errorf("IPv6绑定管理器未启用")
	}

	if !ContainerExists(containerName) {
		return "", fmt.Errorf("容器 %s 不存在", containerName)
	}

	publicIPv6, err := m.generateNextIPv6()
	if err != nil {
		return "", fmt.Errorf("生成IPv6地址失败: %v", err)
	}

	ctx := context.Background()
	lc := &logger.Context{
		Container: containerName,
		Action:    "add_ipv6_binding",
	}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Debug(ctx, "为容器分配IPv6地址",
		zap.String("public_ipv6", publicIPv6),
		zap.String("container_ipv6", containerIPv6))

	if err := m.addIPv6ToInterface(publicIPv6); err != nil {
		return "", fmt.Errorf("添加IPv6地址到网卡失败: %v", err)
	}

	if err := m.createIPv6ForwardingRules(publicIPv6, containerIPv6); err != nil {
		m.removeIPv6FromInterface(publicIPv6)
		return "", fmt.Errorf("创建IPv6转发规则失败: %v", err)
	}

	bindingRule := &models.IPv6BindingRule{
		ContainerName: containerName,
		PublicIPv6:    publicIPv6,
		ContainerIPv6: containerIPv6,
		Status:        "active",
		Description:   "Auto allocated",
		Interface:     m.targetInterface,
	}

	result := database.DB.Where("container_name = ? AND public_ipv6 = ?", 
		containerName, publicIPv6).FirstOrCreate(bindingRule)
	if result.Error != nil {
		m.deleteIPv6ForwardingRules(publicIPv6, containerIPv6)
		m.removeIPv6FromInterface(publicIPv6)
		return "", fmt.Errorf("保存IPv6绑定记录失败: %v", result.Error)
	}
	
	if result.RowsAffected == 0 {
		database.DB.Model(bindingRule).Updates(map[string]interface{}{
			"status":        "active",
			"container_ipv6": containerIPv6,
			"interface":     m.targetInterface,
		})
		logger.Global.Info(ctx, "IPv6绑定已存在，已更新状态",
			zap.String("public_ipv6", publicIPv6),
			zap.String("container_ipv6", containerIPv6))
	}

	logger.Global.Info(ctx, "IPv6绑定创建成功",
		zap.String("public_ipv6", publicIPv6),
		zap.String("container_ipv6", containerIPv6))
	
	if PersistentManager != nil && PersistentManager.IsEnabled() {
		PersistentManager.SaveRules()
	}
	
	return publicIPv6, nil
}

func (m *IPv6BindingManager) AllocateIPv6ForContainer(containerName string, description string) (*models.IPv6BindingRule, error) {
	if !m.IsEnabled() {
		return nil, fmt.Errorf("IPv6绑定管理器未启用")
	}

	if !ContainerExists(containerName) {
		return nil, fmt.Errorf("容器 %s 不存在", containerName)
	}

	containerIPv6, err := GetContainerIPv6(containerName)
	if err != nil {
		return nil, fmt.Errorf("获取容器IPv6地址失败: %v", err)
	}

	publicIPv6, err := m.AddIPv6Binding(containerName, containerIPv6)
	if err != nil {
		return nil, err
	}

	var bindingRule models.IPv6BindingRule
	if err := database.DB.Where("container_name = ? AND public_ipv6 = ?", containerName, publicIPv6).First(&bindingRule).Error; err != nil {
		return nil, fmt.Errorf("查询IPv6绑定记录失败: %v", err)
	}

	if description != "" {
		bindingRule.Description = description
		database.DB.Save(&bindingRule)
	}

	return &bindingRule, nil
}


func (m *IPv6BindingManager) RemoveIPv6Binding(publicIPv6 string) error {
	if !m.IsEnabled() {
		return fmt.Errorf("IPv6绑定管理器未启用")
	}

	var bindingRule models.IPv6BindingRule
	err := database.DB.Where("public_ipv6 = ?", publicIPv6).First(&bindingRule).Error
	if err != nil {
		return fmt.Errorf("IPv6绑定记录不存在")
	}

	ctx := context.Background()
	lc := &logger.Context{
		Container: bindingRule.ContainerName,
		Action:    "remove_ipv6_binding",
	}
	ctx = logger.NewContext(ctx, lc)

	if err := m.deleteIPv6ForwardingRules(bindingRule.PublicIPv6, bindingRule.ContainerIPv6); err != nil {
		logger.Global.Warn(ctx, "删除IPv6转发规则失败", zap.Error(err))
	}

	if err := m.removeIPv6FromInterface(bindingRule.PublicIPv6); err != nil {
		logger.Global.Warn(ctx, "从网卡移除IPv6地址失败", zap.Error(err))
	}

	if err := database.DB.Delete(&bindingRule).Error; err != nil {
		return fmt.Errorf("删除IPv6绑定记录失败: %v", err)
	}

	logger.Global.Info(ctx, "IPv6绑定删除成功", zap.String("public_ipv6", publicIPv6))
	
	if PersistentManager != nil && PersistentManager.IsEnabled() {
		PersistentManager.SaveRules()
	}
	
	return nil
}

func (m *IPv6BindingManager) DeleteIPv6Binding(containerName, publicIPv6 string) error {
	ctx := context.Background()
	lc := &logger.Context{
		Container: containerName,
		Action:    "delete_ipv6_binding",
	}
	ctx = logger.NewContext(ctx, lc)

	if !m.IsEnabled() {
		return fmt.Errorf("IPv6绑定管理器未启用")
	}

	var bindingRule models.IPv6BindingRule
	err := database.DB.Where("container_name = ? AND public_ipv6 = ?", containerName, publicIPv6).First(&bindingRule).Error
	if err != nil {
		return fmt.Errorf("IPv6绑定记录不存在")
	}

	if err := m.deleteIPv6ForwardingRules(bindingRule.PublicIPv6, bindingRule.ContainerIPv6); err != nil {
		logger.Global.Warn(ctx, "删除IPv6转发规则失败", zap.Error(err))
	}

	if err := m.removeIPv6FromInterface(bindingRule.PublicIPv6); err != nil {
		logger.Global.Warn(ctx, "从网卡移除IPv6地址失败", zap.Error(err))
	}

	if err := database.DB.Delete(&bindingRule).Error; err != nil {
		return fmt.Errorf("删除IPv6绑定记录失败: %v", err)
	}

	logger.Global.Info(ctx, "IPv6绑定删除成功", zap.String("public_ipv6", publicIPv6))
	
	if PersistentManager != nil && PersistentManager.IsEnabled() {
		PersistentManager.SaveRules()
	}
	
	return nil
}

func (m *IPv6BindingManager) GetIPv6BindingsByContainer(containerName string) ([]models.IPv6BindingRule, error) {
	if !m.IsEnabled() {
		return nil, fmt.Errorf("IPv6绑定管理器未启用")
	}

	var rules []models.IPv6BindingRule
	if err := database.DB.Where("container_name = ?", containerName).Find(&rules).Error; err != nil {
		return nil, err
	}

	return rules, nil
}


func (m *IPv6BindingManager) generateNextIPv6() (string, error) {

	var usedIPs []string
	database.DB.Model(&models.IPv6BindingRule{}).Pluck("public_ipv6", &usedIPs)

	usedMap := make(map[string]bool)
	for _, ip := range usedIPs {
		usedMap[ip] = true
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	maxAttempts := m.ipv6Pool.PoolSize * 2
	
	for attempt := 0; attempt < maxAttempts; attempt++ {
		randomOffset := rng.Int63n(int64(m.ipv6Pool.PoolSize))
		if randomOffset == 0 {
			randomOffset = 1
		}
		
		offset := new(big.Int).SetInt64(randomOffset)
		newIPInt := new(big.Int).Add(m.ipv6Pool.StartAddr, offset)

		ipBytes := make([]byte, 16)
		newIPInt.FillBytes(ipBytes)
		ip := net.IP(ipBytes)

		ipStr := ip.String()
		if !usedMap[ipStr] {
			ctx := context.Background()
			lc := &logger.Context{Action: "generate_ipv6"}
			ctx = logger.NewContext(ctx, lc)
			logger.Global.Debug(ctx, "随机分配IPv6地址",
				zap.String("ipv6", ipStr),
				zap.Int64("offset", randomOffset))
			return ipStr, nil
		}
	}

	ctx := context.Background()
	lc := &logger.Context{Action: "generate_ipv6"}
	ctx = logger.NewContext(ctx, lc)
	logger.Global.Debug(ctx, "随机分配失败，回退到顺序分配")
	for i := 1; i < m.ipv6Pool.PoolSize; i++ {
		offset := new(big.Int).SetInt64(int64(i))
		newIPInt := new(big.Int).Add(m.ipv6Pool.StartAddr, offset)

		ipBytes := make([]byte, 16)
		newIPInt.FillBytes(ipBytes)
		ip := net.IP(ipBytes)

		ipStr := ip.String()
		if !usedMap[ipStr] {
			logger.Global.Debug(ctx, "顺序分配IPv6地址",
				zap.String("ipv6", ipStr),
				zap.Int("offset", i))
			return ipStr, nil
		}
	}

	return "", fmt.Errorf("IPv6地址池已满，无可用地址")
}


func (m *IPv6BindingManager) addIPv6ToInterface(ipv6 string) error {
	cmd := exec.Command("ip", "-6", "addr", "add", fmt.Sprintf("%s/%d", ipv6, m.ipv6Pool.PrefixLength), "dev", m.targetInterface)

	ctx := context.Background()
	lc := &logger.Context{Action: "add_ipv6_to_interface"}
	ctx = logger.NewContext(ctx, lc)

	logger.Global.Debug(ctx, "执行命令", zap.Strings("args", cmd.Args))

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("添加IPv6地址失败: %v, 输出: %s", err, string(output))
	}

	logger.Global.Info(ctx, "IPv6地址已添加到网卡",
		zap.String("interface", m.targetInterface),
		zap.String("ipv6", ipv6))
	return nil
}


func (m *IPv6BindingManager) removeIPv6FromInterface(ipv6 string) error {
	cmd := exec.Command("ip", "-6", "addr", "del", fmt.Sprintf("%s/%d", ipv6, m.ipv6Pool.PrefixLength), "dev", m.targetInterface)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("移除IPv6地址失败: %v, 输出: %s", err, string(output))
	}

	ctx := context.Background()
	lc := &logger.Context{Action: "remove_ipv6_from_interface"}
	ctx = logger.NewContext(ctx, lc)
	logger.Global.Info(ctx, "IPv6地址已从网卡移除",
		zap.String("interface", m.targetInterface),
		zap.String("ipv6", ipv6))
	return nil
}

func (m *IPv6BindingManager) createIPv6ForwardingRules(publicIPv6, containerIPv6 string) error {
	ctx := context.Background()
	lc := &logger.Context{Action: "create_ipv6_forwarding_rules"}
	ctx = logger.NewContext(ctx, lc)

	dnatCmd := exec.Command("ip6tables", "-t", "nat", "-A", "PREROUTING",
		"-d", publicIPv6, "-j", "DNAT", "--to-destination", containerIPv6,
		"-m", "comment", "--comment", fmt.Sprintf("IPv6-BINDING-%s", publicIPv6))

	logger.Global.Debug(ctx, "执行DNAT命令", zap.Strings("args", dnatCmd.Args))

	if output, err := dnatCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("创建DNAT规则失败: %v, 输出: %s", err, string(output))
	}

	forwardCmd := exec.Command("ip6tables", "-A", "FORWARD",
		"-d", containerIPv6, "-j", "ACCEPT",
		"-m", "comment", "--comment", fmt.Sprintf("IPv6-BINDING-%s", publicIPv6))

	logger.Global.Debug(ctx, "执行FORWARD命令", zap.Strings("args", forwardCmd.Args))

	if output, err := forwardCmd.CombinedOutput(); err != nil {
		exec.Command("ip6tables", "-t", "nat", "-D", "PREROUTING",
			"-d", publicIPv6, "-j", "DNAT", "--to-destination", containerIPv6,
			"-m", "comment", "--comment", fmt.Sprintf("IPv6-BINDING-%s", publicIPv6)).Run()
		return fmt.Errorf("创建FORWARD规则失败: %v, 输出: %s", err, string(output))
	}

	logger.Global.Info(ctx, "IPv6转发规则创建成功",
		zap.String("public_ipv6", publicIPv6),
		zap.String("container_ipv6", containerIPv6))
	return nil
}


func (m *IPv6BindingManager) deleteIPv6ForwardingRules(publicIPv6, containerIPv6 string) error {
	ctx := context.Background()
	lc := &logger.Context{Action: "delete_ipv6_forwarding_rules"}
	ctx = logger.NewContext(ctx, lc)

	comment := fmt.Sprintf("IPv6-BINDING-%s", publicIPv6)

	logger.Global.Debug(ctx, "开始删除IPv6转发规则", zap.String("comment", comment))

	if err := m.cleanupIPv6RulesByComment("nat", "PREROUTING", comment); err != nil {
		logger.Global.Warn(ctx, "删除IPv6 DNAT规则失败", zap.Error(err))
	}

	if err := m.cleanupIPv6RulesByComment("filter", "FORWARD", comment); err != nil {
		logger.Global.Warn(ctx, "删除IPv6 FORWARD规则失败", zap.Error(err))
	}

	logger.Global.Info(ctx, "IPv6转发规则删除完成", zap.String("public_ipv6", publicIPv6))
	return nil
}

// cleanupIPv6RulesByComment 通过 comment 标记清理 IPv6 iptables 规则
func (m *IPv6BindingManager) cleanupIPv6RulesByComment(table, chain, comment string) error {
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

	ctx := context.Background()
	lc := &logger.Context{Action: "cleanup_ipv6_rules_by_comment"}
	ctx = logger.NewContext(ctx, lc)

	for i := len(lineNumbers) - 1; i >= 0; i-- {
		cmd := exec.Command("ip6tables", "-t", table, "-D", chain, strconv.Itoa(lineNumbers[i]))
		if err := cmd.Run(); err != nil {
			logger.Global.Warn(ctx, "删除ip6tables规则失败",
				zap.String("table", table),
				zap.String("chain", chain),
				zap.Int("line", lineNumbers[i]),
				zap.Error(err))
		} else {
			logger.Global.Debug(ctx, "删除ip6tables规则成功",
				zap.String("table", table),
				zap.String("chain", chain),
				zap.Int("line", lineNumbers[i]))
		}
	}

	return nil
}


func GetIPv6BindingList(containerName string) ([]models.IPv6BindingRule, error) {
	var bindings []models.IPv6BindingRule
	err := database.DB.Where("container_name = ?", containerName).Find(&bindings).Error
	return bindings, err
}

// CleanupAllIPv6Bindings 清理指定容器的所有IPv6绑定
func CleanupAllIPv6Bindings(containerName string) error {
	ctx := context.Background()
	lc := &logger.Context{
		Container: containerName,
		Action:    "cleanup_all_ipv6_bindings",
	}
	ctx = logger.NewContext(ctx, lc)

	if IPv6Manager == nil {
		logger.Global.Warn(ctx, "IPv6绑定管理器未初始化，跳过网络规则清理")
	} else {
		logger.Global.Info(ctx, "开始清理所有IPv6绑定")
	}

	bindings, err := GetIPv6BindingList(containerName)
	if err != nil {
		return fmt.Errorf("查询IPv6绑定失败: %v", err)
	}

	if len(bindings) == 0 {
		logger.Global.Info(ctx, "没有需要清理的IPv6绑定")
		return nil
	}

	logger.Global.Info(ctx, "找到IPv6绑定需要清理", zap.Int("count", len(bindings)))

	cleanedCount := 0
	for _, binding := range bindings {
		logger.Global.Debug(ctx, "清理IPv6绑定",
			zap.String("public_ipv6", binding.PublicIPv6),
			zap.String("container_ipv6", binding.ContainerIPv6))

		if IPv6Manager != nil && IPv6Manager.IsEnabled() {
			if err := IPv6Manager.deleteIPv6ForwardingRules(binding.PublicIPv6, binding.ContainerIPv6); err != nil {
				logger.Global.Warn(ctx, "清理IPv6转发规则失败", zap.Error(err))
			}

			if err := IPv6Manager.removeIPv6FromInterface(binding.PublicIPv6); err != nil {
				logger.Global.Warn(ctx, "从网卡移除IPv6地址失败", zap.Error(err))
			}
		}

		if err := database.DB.Unscoped().Delete(&binding).Error; err != nil {
			logger.Global.Warn(ctx, "从数据库删除IPv6绑定记录失败", zap.Error(err))
		} else {
			cleanedCount++
		}
	}

	logger.Global.Info(ctx, "IPv6绑定清理完成",
		zap.Int("cleaned", cleanedCount),
		zap.Int("total", len(bindings)))

	if PersistentManager != nil && PersistentManager.IsEnabled() {
		PersistentManager.SaveRules()
	}

	return nil
}

func interfaceExists(name string) bool {
	_, err := net.InterfaceByName(name)
	return err == nil
}

// restoreIPv6Addresses 从数据库恢复所有活动的IPv6地址到网卡（系统重启后自动恢复）
func (m *IPv6BindingManager) restoreIPv6Addresses() error {
	ctx := context.Background()
	lc := &logger.Context{
		Action: "restore_ipv6_addresses",
	}
	ctx = logger.NewContext(ctx, lc)

	if !m.IsEnabled() {
		return fmt.Errorf("IPv6绑定管理器未启用")
	}

	logger.Global.Info(ctx, "开始恢复IPv6地址")

	var bindings []models.IPv6BindingRule
	if err := database.DB.Where("status = ?", "active").Find(&bindings).Error; err != nil {
		return fmt.Errorf("查询IPv6绑定记录失败: %v", err)
	}

	if len(bindings) == 0 {
		logger.Global.Info(ctx, "没有需要恢复的IPv6地址")
		return nil
	}

	logger.Global.Info(ctx, "找到IPv6绑定记录", zap.Int("count", len(bindings)))

	successCount := 0
	skipCount := 0

	for _, binding := range bindings {
		cmd := exec.Command("ip", "-6", "addr", "show", "dev", m.targetInterface)
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			if bytes.Contains(output, []byte(binding.PublicIPv6)) {
				logger.Global.Debug(ctx, "IPv6地址已存在，跳过", zap.String("ipv6", binding.PublicIPv6))
				skipCount++
				continue
			}
		}

		if err := m.addIPv6ToInterface(binding.PublicIPv6); err != nil {
			logger.Global.Warn(ctx, "恢复IPv6地址失败",
				zap.String("ipv6", binding.PublicIPv6),
				zap.Error(err))
			continue
		}

		successCount++
		logger.Global.Debug(ctx, "IPv6地址恢复成功",
			zap.String("ipv6", binding.PublicIPv6),
			zap.String("container", binding.ContainerName))
	}

	logger.Global.Info(ctx, "IPv6地址恢复完成",
		zap.Int("success", successCount),
		zap.Int("skipped", skipCount),
		zap.Int("total", len(bindings)))

	return nil
}

// AddIPv6ToInterface 公开方法：将IPv6地址添加到网卡
func (m *IPv6BindingManager) AddIPv6ToInterface(ipv6 string) error {
	if !m.IsEnabled() {
		return fmt.Errorf("IPv6绑定管理器未启用")
	}
	return m.addIPv6ToInterface(ipv6)
}

// RemoveIPv6FromInterface 公开方法：从网卡移除IPv6地址
func (m *IPv6BindingManager) RemoveIPv6FromInterface(ipv6 string) error {
	if !m.IsEnabled() {
		return fmt.Errorf("IPv6绑定管理器未启用")
	}
	return m.removeIPv6FromInterface(ipv6)
}

// CreateIPv6ForwardingRules 公开方法：创建 IPv6 转发规则
func (m *IPv6BindingManager) CreateIPv6ForwardingRules(publicIPv6, containerIPv6 string) error {
	if !m.IsEnabled() {
		return fmt.Errorf("IPv6绑定管理器未启用")
	}
	return m.createIPv6ForwardingRules(publicIPv6, containerIPv6)
}

// DeleteIPv6ForwardingRules 公开方法：删除 IPv6 转发规则
func (m *IPv6BindingManager) DeleteIPv6ForwardingRules(publicIPv6, containerIPv6 string) error {
	if !m.IsEnabled() {
		return fmt.Errorf("IPv6绑定管理器未启用")
	}
	return m.deleteIPv6ForwardingRules(publicIPv6, containerIPv6)
}