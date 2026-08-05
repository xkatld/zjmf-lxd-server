package models
import (
	"time"
	"gorm.io/gorm"
)
type Node struct {
	ID             uint           `json:"id" gorm:"primaryKey"`
	Name           string         `json:"name" gorm:"uniqueIndex;size:200;not null"`
	Description    string         `json:"description" gorm:"type:text"`
	Address        string         `json:"address" gorm:"size:500;not null"` 
	APIKey         string         `json:"api_key" gorm:"size:500"`          
	Status         string         `json:"status" gorm:"size:50;default:'inactive'"` 
	LastCheck      *time.Time     `json:"last_check"`
	AutoSync       bool           `json:"auto_sync" gorm:"default:false"`
	SyncInterval   int            `json:"sync_interval" gorm:"default:300"`
	BatchSize      int            `json:"batch_size" gorm:"default:5"`
	BatchInterval  int            `json:"batch_interval" gorm:"default:5"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}
type CreateNodeRequest struct {
	Name          string `json:"name" binding:"required"`
	Description   string `json:"description"`
	Address       string `json:"address" binding:"required"`
	APIKey        string `json:"api_key"`
	AutoSync      bool   `json:"auto_sync"`
	SyncInterval  int    `json:"sync_interval"`
	BatchSize     int    `json:"batch_size"`
	BatchInterval int    `json:"batch_interval"`
}
type UpdateNodeRequest struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	Address       string `json:"address"`
	APIKey        string `json:"api_key"`
	AutoSync      bool   `json:"auto_sync"`
	SyncInterval  int    `json:"sync_interval"`
	BatchSize     int    `json:"batch_size"`
	BatchInterval int    `json:"batch_interval"`
}
