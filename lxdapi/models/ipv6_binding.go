package models

import "time"


type IPv6BindingRule struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	ContainerName   string    `json:"container_name" gorm:"index;not null"`
	PublicIPv6      string    `json:"public_ipv6" gorm:"index:uidx_public_ipv6,unique;not null"`    // 公网IPv6地址
	ContainerIPv6   string    `json:"container_ipv6" gorm:"not null"`             // 容器内网IPv6地址
	Status          string    `json:"status" gorm:"default:'active'"`             // active, inactive, error
	Description     string    `json:"description" gorm:"type:text"`
	Interface       string    `json:"interface" gorm:"default:'wg0';not null"`    // 绑定的网卡接口
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}


type IPv6BindingRequest struct {
	Hostname    string `json:"hostname" form:"hostname" binding:"required"`
	Description string `json:"description" form:"description" binding:"omitempty"`
}


type IPv6BindingStatusResponse struct {
    Code int                  `json:"code"`
    Msg  string               `json:"msg"`
    Data IPv6BindingStatusData `json:"data"`
}

type IPv6BindingStatusData struct {
    Enabled   bool           `json:"enabled"`
    Interface string         `json:"interface"`
    PoolInfo  IPv6PoolStatus `json:"pool_info"`
}

type IPv6PoolStatus struct {
    Start        string `json:"start"`
    PrefixLength int    `json:"prefix_length"`
    PoolSize     int    `json:"pool_size"`
}


type IPv6BindingResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}


type IPv6BindingListResponse struct {
	Code int               `json:"code"`
	Msg  string            `json:"msg"`
	Data []IPv6BindingRule `json:"data"`
}


type DeleteIPv6BindingRequest struct {
	Hostname   string `json:"hostname" form:"hostname" binding:"required"`
	PublicIPv6 string `json:"public_ipv6" form:"public_ipv6" binding:"required"`
}


func (i *IPv6BindingRule) IsActive() bool {
	return i.Status == "active"
}


func (i *IPv6BindingRule) GetDisplayIPv6() string {
	if len(i.PublicIPv6) > 20 {
		return i.PublicIPv6[:15] + "..."
	}
	return i.PublicIPv6
}