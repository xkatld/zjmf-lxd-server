package models

import (
	"time"
)


type ConsoleToken struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Token     string    `gorm:"index:uidx_console_token,unique;size:64" json:"token"`
	Hostname  string    `gorm:"size:255" json:"hostname"`
	UserID    int       `json:"user_id"`
	ServiceID int       `json:"service_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `gorm:"default:false" json:"used"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}


type CreateTokenRequest struct {
	Hostname  string `json:"hostname" binding:"required"`
	UserID    int    `json:"user_id"`
	ServiceID int    `json:"service_id"`
	ServerIP  string `json:"server_ip"`
	ExpiresIn int    `json:"expires_in"`
}


type CreateTokenResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	WSUrl     string    `json:"ws_url"`
}


func (c *ConsoleToken) IsExpired() bool {
	return time.Now().After(c.ExpiresAt)
}


func (c *ConsoleToken) IsValid() bool {
	return !c.Used && !c.IsExpired()
}


func (c *ConsoleToken) MarkAsUsed() {
	c.Used = true
}
