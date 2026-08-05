package services

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"text/template"
	"time"

	"lxdapi/config"
	"lxdapi/database"
	"lxdapi/models"
	"lxdapi/pkg/logger"

	"go.uber.org/zap"
)

// 常量定义
const (
	DEFAULT_OUTPUT_DIR       = "nginx"                      // 输出目录
	DEFAULT_FILE_PREFIX      = "proxy-"                     // 文件前缀
	DEFAULT_FILE_SUFFIX      = ".conf"                      // 文件后缀
	DEFAULT_TEMPLATE_FILE    = "nginx-default.tmpl"         // 模板文件名
	DEFAULT_SYSTEM_NGINX_DIR = "/etc/nginx/sites-enabled"  // 系统nginx目录
	DEFAULT_RELOAD_COMMAND   = "systemctl reload nginx"    // reload命令
)

// NginxProxyManager Nginx反向代理管理器
type NginxProxyManager struct {
	enabled         bool
	baseDir         string
	outputDir       string
	filePrefix      string
	fileSuffix      string
	templateFile    string
	template        *template.Template
	createSymlink   bool
	systemNginxDir  string
	autoTest        bool
	autoReload      bool
	reloadCommand   string
}

// ProxyTemplateData 模板数据
type ProxyTemplateData struct {
	Domain        string
	ContainerName string
	ContainerIP   string
	ContainerPort int
	GeneratedAt   string
	SSLEnabled    bool
	SSLCertPath   string
	SSLKeyPath    string
}

var ProxyManager *NginxProxyManager

// InitNginxProxyManager 初始化Nginx反向代理管理器
func InitNginxProxyManager() error {
	ctx := context.Background()
	lc := &logger.Context{
		Action: "init_nginx_proxy_manager",
	}
	ctx = logger.NewContext(ctx, lc)

	if !config.AppConfig.ProxyAPI.Enabled {
		ProxyManager = &NginxProxyManager{enabled: false}
		logger.Global.Info(ctx, "反向代理功能未启用")
		return nil
	}

	baseDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("获取工作目录失败: %v", err)
	}

	ProxyManager = &NginxProxyManager{
		enabled:        true,
		baseDir:        baseDir,
		outputDir:      DEFAULT_OUTPUT_DIR,
		filePrefix:     DEFAULT_FILE_PREFIX,
		fileSuffix:     DEFAULT_FILE_SUFFIX,
		templateFile:   DEFAULT_TEMPLATE_FILE,
		createSymlink:  true,
		systemNginxDir: DEFAULT_SYSTEM_NGINX_DIR,
		autoTest:       true,
		autoReload:     true,
		reloadCommand:  DEFAULT_RELOAD_COMMAND,
	}

	nginxDir := filepath.Join(baseDir, ProxyManager.outputDir)
	if err := os.MkdirAll(nginxDir, 0755); err != nil {
		return fmt.Errorf("创建nginx目录失败: %v", err)
	}

	if err := ProxyManager.loadTemplate(); err != nil {
		return fmt.Errorf("加载模板失败: %v", err)
	}

	logger.Global.Info(ctx, "Nginx反向代理管理器初始化成功")
	logger.Global.Info(ctx, "配置文件目录", zap.String("dir", nginxDir))
	logger.Global.Info(ctx, "模板文件", zap.String("template", ProxyManager.templateFile))

	if err := ProxyManager.RestoreAllProxyRules(); err != nil {
		logger.Global.Warn(ctx, "恢复代理规则失败", zap.Error(err))
	}

	return nil
}

// loadTemplate 加载Nginx配置模板
func (m *NginxProxyManager) loadTemplate() error {
	templatePath := filepath.Join(m.baseDir, m.templateFile)
	
	// 检查模板文件是否存在
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		return fmt.Errorf("模板文件不存在: %s (请创建 %s)", templatePath, m.templateFile)
	}

	// 读取模板文件
	content, err := os.ReadFile(templatePath)
	if err != nil {
		return fmt.Errorf("读取模板文件失败: %v", err)
	}

	// 解析模板
	tmpl, err := template.New("nginx").Parse(string(content))
	if err != nil {
		return fmt.Errorf("解析模板失败: %v", err)
	}

	m.template = tmpl
	
	ctx := context.Background()
	lc := &logger.Context{Action: "load_proxy_template"}
	ctx = logger.NewContext(ctx, lc)
	logger.Global.Info(ctx, "模板加载成功", zap.String("path", templatePath))
	return nil
}

// IsEnabled 检查功能是否启用
func (m *NginxProxyManager) IsEnabled() bool {
	return m != nil && m.enabled
}

// AddProxyRule 添加反向代理规则
func (m *NginxProxyManager) AddProxyRule(rule *models.ProxyRule) error {
	ctx := context.Background()
	lc := &logger.Context{
		Container: rule.ContainerName,
		Action:    "add_proxy_rule",
	}
	ctx = logger.NewContext(ctx, lc)

	if !m.IsEnabled() {
		return fmt.Errorf("反向代理功能未启用")
	}

	logger.Global.Info(ctx, "添加反向代理规则",
		zap.String("domain", rule.Domain),
		zap.Int("port", rule.ContainerPort),
		zap.Bool("ssl", rule.SSLEnabled))

	if !ContainerExists(rule.ContainerName) {
		return fmt.Errorf("容器不存在: %s", rule.ContainerName)
	}

	containerIP, err := m.getContainerIP(rule.ContainerName)
	if err != nil {
		return fmt.Errorf("获取容器IP失败: %v", err)
	}

	configContent, err := m.generateConfig(rule, containerIP)
	if err != nil {
		return fmt.Errorf("生成配置失败: %v", err)
	}

	configFile := m.getConfigFilePath(rule.Domain)
	if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %v", err)
	}
	logger.Global.Info(ctx, "配置文件已生成", zap.String("file", configFile))

	if m.createSymlink {
		if err := m.createSymlinkToSystem(rule.Domain); err != nil {
			logger.Global.Warn(ctx, "创建软链接失败", zap.Error(err))
		}
	}

	if m.autoTest {
		if err := m.testNginxConfig(); err != nil {
			os.Remove(configFile)
			m.removeSymlink(rule.Domain)
			return fmt.Errorf("Nginx配置测试失败: %v", err)
		}
		logger.Global.Info(ctx, "Nginx配置测试通过")
	}

	if m.autoReload {
		if err := m.reloadNginx(); err != nil {
			logger.Global.Warn(ctx, "Nginx重载失败", zap.Error(err))
		} else {
			logger.Global.Info(ctx, "Nginx重载成功")
		}
	}

	logger.Global.Info(ctx, "反向代理规则添加成功", zap.String("domain", rule.Domain))
	return nil
}

// DeleteProxyRule 删除反向代理规则
func (m *NginxProxyManager) DeleteProxyRule(rule *models.ProxyRule) error {
	ctx := context.Background()
	lc := &logger.Context{
		Container: rule.ContainerName,
		Action:    "delete_proxy_rule",
	}
	ctx = logger.NewContext(ctx, lc)

	if !m.IsEnabled() {
		return fmt.Errorf("反向代理功能未启用")
	}

	logger.Global.Info(ctx, "删除反向代理规则", zap.String("domain", rule.Domain))

	configFile := m.getConfigFilePath(rule.Domain)
	if err := os.Remove(configFile); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除配置文件失败: %v", err)
	}
	logger.Global.Info(ctx, "配置文件已删除", zap.String("file", configFile))

	if m.createSymlink {
		if err := m.removeSymlink(rule.Domain); err != nil {
			logger.Global.Warn(ctx, "删除软链接失败", zap.Error(err))
		}
	}

	if rule.SSLEnabled && CertManager != nil {
		if err := CertManager.DeleteCert(rule.Domain); err != nil {
			logger.Global.Warn(ctx, "删除SSL证书失败", zap.Error(err))
		} else {
			logger.Global.Info(ctx, "SSL证书已删除")
		}
	}

	if m.autoReload {
		if err := m.reloadNginx(); err != nil {
			logger.Global.Warn(ctx, "Nginx重载失败", zap.Error(err))
		} else {
			logger.Global.Info(ctx, "Nginx重载成功")
		}
	}

	logger.Global.Info(ctx, "反向代理规则删除成功", zap.String("domain", rule.Domain))
	return nil
}

// generateConfig 生成Nginx配置内容
func (m *NginxProxyManager) generateConfig(rule *models.ProxyRule, containerIP string) (string, error) {
	data := ProxyTemplateData{
		Domain:        rule.Domain,
		ContainerName: rule.ContainerName,
		ContainerIP:   containerIP,
		ContainerPort: rule.ContainerPort,
		GeneratedAt:   time.Now().Format("2006-01-02 15:04:05"),
		SSLEnabled:    rule.SSLEnabled,
		SSLCertPath:   rule.SSLCertPath,
		SSLKeyPath:    rule.SSLKeyPath,
	}

	var buf bytes.Buffer
	if err := m.template.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("模板执行失败: %v", err)
	}

	return buf.String(), nil
}

// getContainerIP 获取容器IP地址
func (m *NginxProxyManager) getContainerIP(containerName string) (string, error) {
	// 使用现有的服务函数获取容器IP
	containerIP, err := GetContainerIP(containerName)
	if err != nil {
		return "", fmt.Errorf("获取容器IP失败: %v", err)
	}
	if containerIP == "" {
		return "", fmt.Errorf("容器没有IPv4地址")
	}
	return containerIP, nil
}

// getConfigFilePath 获取配置文件路径
func (m *NginxProxyManager) getConfigFilePath(domain string) string {
	filename := m.filePrefix + domain + m.fileSuffix
	return filepath.Join(m.baseDir, m.outputDir, filename)
}

// getSymlinkPath 获取软链接路径
func (m *NginxProxyManager) getSymlinkPath(domain string) string {
	filename := m.filePrefix + domain + m.fileSuffix
	return filepath.Join(m.systemNginxDir, filename)
}

// createSymlinkToSystem 创建软链接到系统nginx目录
func (m *NginxProxyManager) createSymlinkToSystem(domain string) error {
	source := m.getConfigFilePath(domain)
	target := m.getSymlinkPath(domain)

	// 检查目标目录是否存在
	if _, err := os.Stat(m.systemNginxDir); os.IsNotExist(err) {
		return fmt.Errorf("系统nginx目录不存在: %s", m.systemNginxDir)
	}

	// 如果软链接已存在，先删除
	if _, err := os.Lstat(target); err == nil {
		os.Remove(target)
	}

	if err := os.Symlink(source, target); err != nil {
		return fmt.Errorf("创建软链接失败: %v", err)
	}

	ctx := context.Background()
	lc := &logger.Context{Action: "create_proxy_symlink"}
	ctx = logger.NewContext(ctx, lc)
	logger.Global.Info(ctx, "软链接已创建", zap.String("target", target), zap.String("source", source))
	return nil
}

func (m *NginxProxyManager) removeSymlink(domain string) error {
	target := m.getSymlinkPath(domain)
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("删除软链接失败: %v", err)
	}
	
	ctx := context.Background()
	lc := &logger.Context{Action: "remove_proxy_symlink"}
	ctx = logger.NewContext(ctx, lc)
	logger.Global.Info(ctx, "软链接已删除", zap.String("target", target))
	return nil
}

// testNginxConfig 测试Nginx配置
func (m *NginxProxyManager) testNginxConfig() error {
	cmd := exec.Command("nginx", "-t")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, string(output))
	}
	return nil
}

// reloadNginx 重载Nginx
func (m *NginxProxyManager) reloadNginx() error {
	cmd := exec.Command("sh", "-c", m.reloadCommand)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, string(output))
	}
	return nil
}

// RestoreAllProxyRules 恢复所有代理规则（启动时调用）
func (m *NginxProxyManager) RestoreAllProxyRules() error {
	ctx := context.Background()
	lc := &logger.Context{
		Action: "restore_all_proxy_rules",
	}
	ctx = logger.NewContext(ctx, lc)

	if !m.IsEnabled() {
		return fmt.Errorf("反向代理功能未启用")
	}

	logger.Global.Info(ctx, "开始恢复反向代理规则")

	var rules []models.ProxyRule
	if err := database.DB.Where("status = ?", "active").Find(&rules).Error; err != nil {
		return fmt.Errorf("查询代理规则失败: %v", err)
	}

	if len(rules) == 0 {
		logger.Global.Info(ctx, "没有需要恢复的代理规则")
		return nil
	}

	logger.Global.Info(ctx, "找到代理规则", zap.Int("count", len(rules)))

	successCount := 0
	failCount := 0

	for _, rule := range rules {
		logger.Global.Debug(ctx, "恢复代理规则",
			zap.String("domain", rule.Domain),
			zap.String("container", rule.ContainerName),
			zap.Int("port", rule.ContainerPort))

		if !ContainerExists(rule.ContainerName) {
			logger.Global.Warn(ctx, "容器不存在，跳过", zap.String("container", rule.ContainerName))
			failCount++
			continue
		}

		containerIP, err := m.getContainerIP(rule.ContainerName)
		if err != nil {
			logger.Global.Warn(ctx, "获取容器IP失败", zap.Error(err))
			failCount++
			continue
		}

		configContent, err := m.generateConfig(&rule, containerIP)
		if err != nil {
			logger.Global.Warn(ctx, "生成配置失败", zap.Error(err))
			failCount++
			continue
		}

		configFile := m.getConfigFilePath(rule.Domain)
		if err := os.WriteFile(configFile, []byte(configContent), 0644); err != nil {
			logger.Global.Warn(ctx, "写入配置文件失败", zap.Error(err))
			failCount++
			continue
		}

		if m.createSymlink {
			if err := m.createSymlinkToSystem(rule.Domain); err != nil {
				logger.Global.Warn(ctx, "创建软链接失败", zap.Error(err))
			}
		}

		successCount++
		logger.Global.Debug(ctx, "代理规则恢复成功", zap.String("domain", rule.Domain))
	}

	logger.Global.Info(ctx, "代理规则恢复完成",
		zap.Int("success", successCount),
		zap.Int("failed", failCount),
		zap.Int("total", len(rules)))

	logger.Global.Info(ctx, "提示: 请手动执行 'nginx -t && systemctl reload nginx' 使配置生效")

	return nil
}

// ValidateDomain 验证域名格式
func ValidateDomain(domain string) bool {
	// 简单的域名格式验证
	pattern := `^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`
	matched, _ := regexp.MatchString(pattern, domain)
	return matched
}

// CleanupProxyRules 清理容器的所有代理规则（容器删除时调用）
func CleanupProxyRules(containerName string) error {
	ctx := context.Background()
	lc := &logger.Context{
		Container: containerName,
		Action:    "cleanup_proxy_rules",
	}
	ctx = logger.NewContext(ctx, lc)

	if ProxyManager == nil || !ProxyManager.IsEnabled() {
		return nil
	}

	logger.Global.Info(ctx, "清理容器的代理规则")

	var rules []models.ProxyRule
	if err := database.DB.Where("container_name = ?", containerName).Find(&rules).Error; err != nil {
		return fmt.Errorf("查询代理规则失败: %v", err)
	}

	if len(rules) == 0 {
		logger.Global.Info(ctx, "容器没有代理规则")
		return nil
	}

	logger.Global.Info(ctx, "找到代理规则需要清理", zap.Int("count", len(rules)))

	for _, rule := range rules {
		configFile := ProxyManager.getConfigFilePath(rule.Domain)
		if err := os.Remove(configFile); err != nil && !os.IsNotExist(err) {
			logger.Global.Warn(ctx, "删除配置文件失败", zap.Error(err))
		}

		if ProxyManager.createSymlink {
			ProxyManager.removeSymlink(rule.Domain)
		}

		// 删除SSL证书（如果启用了SSL）
		if rule.SSLEnabled && CertManager != nil {
			if err := CertManager.DeleteCert(rule.Domain); err != nil {
				logger.Global.Warn(ctx, "删除SSL证书失败", zap.Error(err))
			} else {
				logger.Global.Info(ctx, "SSL证书已删除", zap.String("domain", rule.Domain))
			}
		}

		database.DB.Delete(&rule)
		logger.Global.Debug(ctx, "代理规则已清理", zap.String("domain", rule.Domain))
	}

	if ProxyManager.autoReload {
		if err := ProxyManager.reloadNginx(); err != nil {
			logger.Global.Warn(ctx, "Nginx重载失败", zap.Error(err))
		}
	}

	logger.Global.Info(ctx, "容器的代理规则清理完成")
	return nil
}
