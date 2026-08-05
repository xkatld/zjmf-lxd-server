package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	System          SystemConfig          `yaml:"system"`
	TaskStatus      TaskStatusConfig      `yaml:"task_status"`
	TaskQueue       TaskQueueConfig       `yaml:"task_queue"`
	Logging         LoggingConfig         `yaml:"logging"`
	ContainerAPI    ContainerAPIConfig    `yaml:"container_api"`
	TrafficAPI      TrafficAPIConfig      `yaml:"traffic_api"`
	NATAPI          NATAPIConfig          `yaml:"nat_api"`
	ConsoleAPI      ConsoleAPIConfig      `yaml:"console_api"`
	IPv6Binding     IPv6BindingConfig     `yaml:"ipv6_binding"`
	ProxyAPI        ProxyAPIConfig        `yaml:"proxy_api"`
}

type ServerConfig struct {
	Port int       `yaml:"port"`
	Mode string    `yaml:"mode"`
	TLS  TLSConfig `yaml:"tls"`
}

type TLSConfig struct {
	CertFile  string   `yaml:"cert_file"`
	KeyFile   string   `yaml:"key_file"`
	AutoGen   bool     `yaml:"auto_gen"`
	ServerIPs []string `yaml:"server_ips"`
}

type SystemConfig struct {
	Server   ServerConfig   `yaml:"server"`
	Security SecurityConfig `yaml:"security"`
	Database DatabaseConfig `yaml:"database"`
	CORS     CORSConfig     `yaml:"cors"`
}

type SecurityConfig struct {
	APIHash string `yaml:"api_hash"`
}

type DatabaseConfig struct {
	Type       string `yaml:"type"`
	SQLitePath string `yaml:"sqlite_path"`
	Path       string `yaml:"path"`
}

type TaskStatusConfig struct {
	RetentionHours int `yaml:"retention_hours"`
}

type TaskQueueConfig struct {
	Backend      string `yaml:"backend"`
	WorkerCount  int    `yaml:"worker_count"`
	PollInterval int    `yaml:"poll_interval"`
	MaxRetries   int    `yaml:"max_retries"`
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

type ContainerAPIConfig struct {
	StoragePools []string    `yaml:"storage_pools"`
	LXCFS        LXCFSConfig `yaml:"lxcfs"`
}

type LXCFSConfig struct {
	MountPath string `yaml:"mount_path"`
}

type TrafficAPIConfig struct {
	Interval   int               `yaml:"interval"`
	BatchSize  int               `yaml:"batch_size"`
	LimitCheck TrafficLimitCheck `yaml:"limit_check"`
	AutoReset  TrafficAutoReset  `yaml:"auto_reset"`
}

type TrafficLimitCheck struct {
	CheckInterval int `yaml:"check_interval"`
	BatchSize     int `yaml:"batch_size"`
}

type TrafficAutoReset struct {
	CheckInterval int `yaml:"check_interval"`
	BatchSize     int `yaml:"batch_size"`
}

type NATAPIConfig struct {
	Enabled     *bool                `yaml:"enabled"`
	Network     NATNetworkConfig     `yaml:"network"`
	ContainerIP NATContainerIPConfig `yaml:"container_ip"`
}

type NATNetworkConfig struct {
	ExternalInterface string   `yaml:"external_interface"`
	ExternalIPs       []string `yaml:"external_ips"`
	InternalInterface string   `yaml:"internal_interface"`
}

type NATContainerIPConfig struct {
	PreferredInterfaces []string `yaml:"preferred_interfaces"`
	IgnoredInterfaces   []string `yaml:"ignored_interfaces"`
}

type ConsoleAPIConfig struct {
	SessionTimeout int `yaml:"session_timeout"`
}

type IPv6BindingConfig struct {
	Enabled   bool           `yaml:"enabled"`
	Interface string         `yaml:"interface"`
	IPv6Pool  IPv6PoolConfig `yaml:"ipv6_pool"`
}

type IPv6PoolConfig struct {
	Start        string `yaml:"start"`
	PrefixLength int    `yaml:"prefix_length"`
	PoolSize     int    `yaml:"pool_size"`
}

type CORSConfig struct {
	AllowOrigins     []string `yaml:"allow_origins"`
	AllowCredentials bool     `yaml:"allow_credentials"`
}

type ProxyAPIConfig struct {
	Enabled bool `yaml:"enabled"`
}

var (
	AppConfig *Config
)

func LoadConfig() error {
	return LoadConfigFromFile("config.yaml")
}

func LoadConfigFromFile(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	AppConfig = &Config{}
	if err := yaml.Unmarshal(data, AppConfig); err != nil {
		return fmt.Errorf("解析配置文件失败: %v", err)
	}

	setDefaults()

	log.Printf("配置加载完成: 端口:%d 模式:%s 日志级别:%s 存储池:%v",
		AppConfig.System.Server.Port, AppConfig.System.Server.Mode, AppConfig.Logging.Level, AppConfig.ContainerAPI.StoragePools)
	return nil
}

func setDefaults() {
	if AppConfig.System.Server.Port == 0 {
		AppConfig.System.Server.Port = 8080
	}
	if AppConfig.System.Server.Mode == "" {
		AppConfig.System.Server.Mode = "debug"
	}
	if AppConfig.Logging.Level == "" {
		AppConfig.Logging.Level = "info"
	}
	if AppConfig.Logging.File == "" {
		AppConfig.Logging.File = "/var/log/lxdapi/app.log"
	}
	if AppConfig.Logging.MaxSize == 0 {
		AppConfig.Logging.MaxSize = 100
	}
	if AppConfig.Logging.MaxBackups == 0 {
		AppConfig.Logging.MaxBackups = 10
	}
	if AppConfig.Logging.MaxAge == 0 {
		AppConfig.Logging.MaxAge = 30
	}
	if AppConfig.TaskStatus.RetentionHours == 0 {
		AppConfig.TaskStatus.RetentionHours = 168
	}
	if AppConfig.TaskQueue.Backend == "" {
		AppConfig.TaskQueue.Backend = "database"
	}
	if AppConfig.TaskQueue.WorkerCount == 0 {
		AppConfig.TaskQueue.WorkerCount = 5
	}
	if AppConfig.TaskQueue.PollInterval == 0 {
		AppConfig.TaskQueue.PollInterval = 100
	}
	if AppConfig.TaskQueue.MaxRetries == 0 {
		AppConfig.TaskQueue.MaxRetries = 3
	}
	if AppConfig.TrafficAPI.Interval == 0 {
		AppConfig.TrafficAPI.Interval = 5
	}
	if AppConfig.TrafficAPI.BatchSize == 0 {
		AppConfig.TrafficAPI.BatchSize = 10
	}
	if AppConfig.TrafficAPI.LimitCheck.CheckInterval == 0 {
		AppConfig.TrafficAPI.LimitCheck.CheckInterval = 300
	}
	if AppConfig.TrafficAPI.LimitCheck.BatchSize == 0 {
		AppConfig.TrafficAPI.LimitCheck.BatchSize = 10
	}
	if AppConfig.TrafficAPI.AutoReset.CheckInterval == 0 {
		AppConfig.TrafficAPI.AutoReset.CheckInterval = 3600
	}
	if AppConfig.TrafficAPI.AutoReset.BatchSize == 0 {
		AppConfig.TrafficAPI.AutoReset.BatchSize = 10
	}
	if AppConfig.ConsoleAPI.SessionTimeout == 0 {
		AppConfig.ConsoleAPI.SessionTimeout = 1800
	}
	if len(AppConfig.System.CORS.AllowOrigins) == 0 {
		AppConfig.System.CORS.AllowOrigins = []string{"*"}
	}
	if len(AppConfig.ContainerAPI.StoragePools) == 0 {
		AppConfig.ContainerAPI.StoragePools = []string{"default"}
	}
	if AppConfig.NATAPI.Enabled == nil {
		defaultEnabled := true
		AppConfig.NATAPI.Enabled = &defaultEnabled
	}
}

func (d *DatabaseConfig) ValidateAndSetDefaults() error {
	if d.Type == "" {
		if d.Path != "" {
			d.Type = "sqlite"
			d.SQLitePath = d.Path
		} else {
			d.Type = "sqlite"
		}
	}
	
	switch strings.ToLower(d.Type) {
	case "sqlite":
		if d.SQLitePath == "" && d.Path != "" {
			d.SQLitePath = d.Path
		}
		if d.SQLitePath == "" {
			d.SQLitePath = "lxdapi.db"
		}
	default:
		return fmt.Errorf("不支持的数据库类型: %s (仅支持: sqlite)", d.Type)
	}
	
	return nil
}

func GetLXCTimeout() time.Duration {
	return 300 * time.Second
}

func GetLXCLongTimeout() time.Duration {
	return 300 * time.Second
}

func GetNATIptablesConfig() struct {
	Enabled           bool
	ChainPrefix       string
	CommentPrefix     string
	CleanupOnStart    bool
	StrictSourceCheck bool
	ConntrackOptim    bool
	BatchOperations   bool
} {
	return struct {
		Enabled           bool
		ChainPrefix       string
		CommentPrefix     string
		CleanupOnStart    bool
		StrictSourceCheck bool
		ConntrackOptim    bool
		BatchOperations   bool
	}{
		Enabled:           true,
		ChainPrefix:       "LXDAPINAT",
		CommentPrefix:     "lxdapinat",
		CleanupOnStart:    true,
		StrictSourceCheck: false,
		ConntrackOptim:    true,
		BatchOperations:   true,
	}
}

func IsIPv4NATEnabled() bool {
	if AppConfig != nil && AppConfig.NATAPI.Enabled != nil {
		return *AppConfig.NATAPI.Enabled
	}
	return true
}

func IsNATEnabled() bool {
	if AppConfig != nil && AppConfig.NATAPI.Enabled != nil {
		return *AppConfig.NATAPI.Enabled
	}
	return true
}

func GetStoragePools() []string {
	if AppConfig != nil && len(AppConfig.ContainerAPI.StoragePools) > 0 {
		return AppConfig.ContainerAPI.StoragePools
	}
	return []string{"default"}
}

func GetLXCFSMountPath() string {
	if AppConfig != nil && AppConfig.ContainerAPI.LXCFS.MountPath != "" {
		return AppConfig.ContainerAPI.LXCFS.MountPath
	}
	return "/var/lib/lxcfs"
}

func GetExternalIPv4() string {
	if AppConfig != nil && len(AppConfig.NATAPI.Network.ExternalIPs) > 0 {
		return AppConfig.NATAPI.Network.ExternalIPs[0]
	}
	return ""
}
