package database
import (
	"database/sql"
	"log"
	"lxdweb/config"
	"lxdweb/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	_ "modernc.org/sqlite"
)
var DB *gorm.DB
func InitDB() {
	var err error
	dsn := config.AppConfig.Database.Path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		log.Fatalf("[ERROR] 数据库连接失败: %v", err)
	}
	DB, err = gorm.Open(sqlite.Dialector{Conn: sqlDB}, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("[ERROR] 数据库连接失败: %v", err)
	}
	err = DB.AutoMigrate(
		&models.Admin{},
		&models.Node{},
		&models.Container{},
		&models.ContainerCache{},
		&models.SyncTask{},
		&models.NodeInfoCache{},
		&models.OperationLog{},
		&models.Image{},
	)
	if err != nil {
		log.Fatalf("[ERROR] 数据库迁移失败: %v", err)
	}
	log.Printf("[DB] 数据库初始化完成")
}
func CheckAdminExists() {
	var count int64
	DB.Model(&models.Admin{}).Count(&count)
	if count == 0 {
		log.Printf("[WARN] 未检测到管理员账号，正在创建默认管理员...")
		
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(config.AppConfig.Admin.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Fatalf("[ERROR] 密码加密失败: %v", err)
		}
		
		admin := models.Admin{
			Username: config.AppConfig.Admin.Username,
			Password: string(hashedPassword),
			Email:    config.AppConfig.Admin.Email,
		}
		
		if err := DB.Create(&admin).Error; err != nil {
			log.Fatalf("[ERROR] 创建默认管理员失败: %v", err)
		}
		
		log.Printf("[SUCCESS] 默认管理员创建成功")
		log.Printf("  用户名: %s", config.AppConfig.Admin.Username)
		log.Printf("  密码: %s", config.AppConfig.Admin.Password)
		log.Printf("  请登录后及时修改密码！")
	}
}
