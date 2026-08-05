package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"lxdapi/config"
	"lxdapi/models"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)

var (
	DB     *gorm.DB
	TaskWG sync.WaitGroup
	TaskMu sync.Mutex
)

type customLogger struct {
	LogLevel      logger.LogLevel
	SlowThreshold time.Duration
}

func (l *customLogger) LogMode(level logger.LogLevel) logger.Interface {
	newLogger := *l
	newLogger.LogLevel = level
	return &newLogger
}

func (l *customLogger) Info(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Info {
		log.Printf(msg, data...)
	}
}

func (l *customLogger) Warn(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Warn {
		log.Printf("[WARN] "+msg, data...)
	}
}

func (l *customLogger) Error(ctx context.Context, msg string, data ...interface{}) {
	if l.LogLevel >= logger.Error {
		log.Printf("[ERROR] "+msg, data...)
	}
}

func (l *customLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.LogLevel <= logger.Silent {
		return
	}

	elapsed := time.Since(begin)
	switch {
	case err != nil && l.LogLevel >= logger.Error && !errors.Is(err, gorm.ErrRecordNotFound):
		sql, rows := fc()
		log.Printf("[ERROR] [%.3fms] [rows:%d] %s | %v", float64(elapsed.Nanoseconds())/1e6, rows, sql, err)
	case elapsed > l.SlowThreshold && l.SlowThreshold != 0 && l.LogLevel >= logger.Warn:
		sql, rows := fc()
		log.Printf("[WARN] SLOW SQL >= %v [%.3fms] [rows:%d] %s", l.SlowThreshold, float64(elapsed.Nanoseconds())/1e6, rows, sql)
	case l.LogLevel == logger.Info:
		sql, rows := fc()
		log.Printf("[%.3fms] [rows:%d] %s", float64(elapsed.Nanoseconds())/1e6, rows, sql)
	}
}


func InitDB() {
	dbConfig := &config.AppConfig.System.Database
	
	if err := dbConfig.ValidateAndSetDefaults(); err != nil {
		log.Fatalf("\033[31m⚠️ 数据库配置错误: %v\033[0m", err)
	}

	if err := connectDB(dbConfig); err != nil {
		log.Fatalf("\033[31m⚠️ 数据库连接失败: %v\033[0m", err)
	}

	if err := migrateDB(); err != nil {
		log.Fatalf("\033[31m⚠️ 数据库迁移失败: %v\033[0m", err)
	}

	log.Printf("数据库初始化完成: %s", dbConfig.Type)
}

func connectDB(dbConfig *config.DatabaseConfig) error {
	var dialector gorm.Dialector
	var err error
	
	switch strings.ToLower(dbConfig.Type) {
	case "sqlite":
		dialector, err = createSQLiteDialector(dbConfig)
	default:
		return fmt.Errorf("不支持的数据库类型: %s (仅支持: sqlite)", dbConfig.Type)
	}
	
	if err != nil {
		return err
	}

	gormLogger := &customLogger{
		LogLevel:      logger.Error,
		SlowThreshold: 200 * time.Millisecond,
	}
	
	gormConfig := &gorm.Config{
		Logger: gormLogger,
	}

	DB, err = gorm.Open(dialector, gormConfig)
	if err != nil {
		return err
	}
	
	return nil
}

func createSQLiteDialector(dbConfig *config.DatabaseConfig) (gorm.Dialector, error) {
	dsn := dbConfig.SQLitePath + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	
	return sqlite.Dialector{Conn: sqlDB}, nil
}

func migrateDB() error {
	err := DB.AutoMigrate(
		&models.Task{},
		&models.ContainerConfig{},
		&models.TrafficSummary{},
		&models.NATRule{},
		&models.ConsoleToken{},
		&models.ProxyRule{},
		&models.ContainerInfoCache{},
		&models.IPv6BindingRule{},
	)
	
	return err
}

