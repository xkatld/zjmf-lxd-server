package config
import (
	"crypto/rand"
	"fmt"
	"log"
	"os"
	"gopkg.in/yaml.v3"
)
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Admin    AdminConfig    `yaml:"admin"`
	Database DatabaseConfig `yaml:"database"`
	Sync     SyncConfig     `yaml:"sync"`
	Logging  LoggingConfig  `yaml:"logging"`
}

type AdminConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Email    string `yaml:"email"`
}
type ServerConfig struct {
	Address       string `yaml:"address"`
	Mode          string `yaml:"mode"`
	SessionSecret string `yaml:"session_secret"`
	EnableHTTPS   bool   `yaml:"enable_https"`
	CertFile      string `yaml:"cert_file"`
	KeyFile       string `yaml:"key_file"`
}
type DatabaseConfig struct {
	Path string `yaml:"path"`
}
type SyncConfig struct {
	Interval      int `yaml:"interval"`
	BatchSize     int `yaml:"batch_size"`
	BatchInterval int `yaml:"batch_interval"`
}
type LoggingConfig struct {
	Level      string `yaml:"level"`
	File       string `yaml:"file"`
	MaxSize    int    `yaml:"max_size"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAge     int    `yaml:"max_age"`
	Compress   bool   `yaml:"compress"`
	DevMode    bool   `yaml:"dev_mode"`
}
var AppConfig *Config
func LoadConfig() error {
	configFile := "config.yaml"
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		if err := createDefaultConfig(configFile); err != nil {
			return err
		}
	}
	data, err := os.ReadFile(configFile)
	if err != nil {
		return err
	}
	AppConfig = &Config{}
	if err := yaml.Unmarshal(data, AppConfig); err != nil {
		return err
	}
	if AppConfig.Server.Address == "" {
		AppConfig.Server.Address = "0.0.0.0:3000"
	}
	if AppConfig.Server.Mode == "" {
		AppConfig.Server.Mode = "release"
	}
	if AppConfig.Server.SessionSecret == "" {
		AppConfig.Server.SessionSecret = "lxdweb-secret-key-change-me"
	}
	if AppConfig.Server.CertFile == "" {
		AppConfig.Server.CertFile = "cert.pem"
	}
	if AppConfig.Server.KeyFile == "" {
		AppConfig.Server.KeyFile = "key.pem"
	}
	
	if AppConfig.Admin.Username == "" {
		AppConfig.Admin.Username = "admin"
	}
	if AppConfig.Admin.Password == "" {
		AppConfig.Admin.Password = "admin123"
	}
	
	if AppConfig.Database.Path == "" {
		AppConfig.Database.Path = "lxdweb.db"
	}
	if AppConfig.Sync.Interval <= 0 {
		AppConfig.Sync.Interval = 300  
	}
	if AppConfig.Sync.BatchSize <= 0 {
		AppConfig.Sync.BatchSize = 5
	}
	if AppConfig.Sync.BatchInterval <= 0 {
		AppConfig.Sync.BatchInterval = 2
	}
	if AppConfig.Logging.Level == "" {
		AppConfig.Logging.Level = "info"
	}
	if AppConfig.Logging.File == "" {
		AppConfig.Logging.File = "lxdweb.log"
	}
	if AppConfig.Logging.MaxSize <= 0 {
		AppConfig.Logging.MaxSize = 100
	}
	if AppConfig.Logging.MaxBackups <= 0 {
		AppConfig.Logging.MaxBackups = 10
	}
	if AppConfig.Logging.MaxAge <= 0 {
		AppConfig.Logging.MaxAge = 30
	}
	log.Printf("[CONFIG] 配置加载完成: %s", AppConfig.Server.Address)
	return nil
}
func createDefaultConfig(filename string) error {
	sessionSecret := generateRandomString(64)
	defaultConfig := fmt.Sprintf(`server:
  # 服务器监听地址
  address: "0.0.0.0:3000"
  # 运行模式: debug | release
  mode: "release"
  # 会话密钥
  session_secret: "%s"
  # 启用 HTTPS
  enable_https: true
  # 证书文件路径
  cert_file: "cert.pem"
  # 密钥文件路径
  key_file: "key.pem"

# 默认管理员账号（首次启动自动创建）
admin:
  username: "admin"
  password: "admin123"
  email: ""

database:
  # 数据库文件路径
  path: "lxdweb.db"

sync:
  # 同步间隔（秒）
  interval: 300
  # 每批同步数量
  batch_size: 5
  # 批次间隔（秒）
  batch_interval: 2

logging:
  # 日志级别: debug | info | warn | error
  level: "info"
  # 日志文件保存路径
  file: "lxdweb.log"
  # 单个日志文件最大大小（MB）
  max_size: 100
  # 保留的旧日志文件数量
  max_backups: 10
  # 保留的旧日志文件天数
  max_age: 30
  # 是否压缩旧日志文件
  compress: true
  # 开发模式（控制台输出格式）
  dev_mode: false
`, sessionSecret)
	return os.WriteFile(filename, []byte(defaultConfig), 0600)
}
func generateRandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!@#$%^&*"
	result := make([]byte, length)
	for i := range result {
		result[i] = charset[randomInt(len(charset))]
	}
	return string(result)
}
func randomInt(max int) int {
	b := make([]byte, 1)
	_, err := rand.Read(b)
	if err != nil {
		return 0
	}
	return int(b[0]) % max
}
