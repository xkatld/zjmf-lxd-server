package models

import (
	"time"
	"gorm.io/gorm"
)

type NodeInfoCache struct {
	ID             uint           `json:"id" gorm:"primaryKey"`
	NodeID         uint           `json:"node_id" gorm:"uniqueIndex;not null"`

	SystemInfo     string         `json:"system_info" gorm:"type:text"`

	LastSync       time.Time      `json:"last_sync"`
	
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `json:"-" gorm:"index"`
}

func (NodeInfoCache) TableName() string {
	return "node_info_cache"
}

