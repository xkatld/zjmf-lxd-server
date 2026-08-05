package models

import (
	"fmt"
	"time"
)


type NATRule struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	ContainerName   string    `json:"container_name" gorm:"index;size:500;not null"`
	ExternalPort    int       `json:"external_port" gorm:"not null"`
	InternalPort    int       `json:"internal_port" gorm:"not null"`
	ExternalPortEnd int       `json:"external_port_end" gorm:"default:0"`        // 端口段结束，0表示单端口
	InternalPortEnd int       `json:"internal_port_end" gorm:"default:0"`        // 端口段结束，0表示单端口
	Protocol        string    `json:"protocol" gorm:"size:50;not null"`          // tcp, udp, both
	IPVersion       string    `json:"ip_version" gorm:"size:50;default:'ipv4';not null"` // ipv4, ipv6, dual
	Status          string    `json:"status" gorm:"size:100;default:'active'"`
	Description     string    `json:"description" gorm:"type:text"`
	NATMethod       string    `json:"nat_method" gorm:"size:100;default:'iptables'"` // 固定为iptables
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}


type AddNATRequest struct {
	Hostname        string `json:"hostname" form:"hostname" binding:"required"`
	ExternalPort    int    `json:"dport" form:"dport" binding:"omitempty,min=10000,max=65535"`
	InternalPort    int    `json:"sport" form:"sport" binding:"required,min=1,max=65535"`
	ExternalPortEnd int    `json:"dport_end" form:"dport_end" binding:"omitempty,min=10000,max=65535"` // 端口段结束，可选
	InternalPortEnd int    `json:"sport_end" form:"sport_end" binding:"omitempty,min=1,max=65535"`     // 端口段结束，可选
	Protocol        string `json:"dtype" form:"dtype" binding:"omitempty,oneof=tcp udp both"`          // tcp, udp, both (默认both)
	IPVersion       string `json:"ip_version" form:"ip_version" binding:"omitempty,oneof=ipv4 ipv6 dual" default:"dual"`
	Description     string `json:"description" form:"description" binding:"omitempty,max=255"`
}


type AddNATRequestStandard struct {
	ExternalPort int    `json:"dport" form:"dport" binding:"omitempty,min=10000,max=65535"`
	InternalPort int    `json:"sport" form:"sport" binding:"required,min=1,max=65535"`
	Protocol     string `json:"dtype" form:"dtype" binding:"required,oneof=tcp udp"`
	IPVersion    string `json:"ip_version" form:"ip_version" binding:"omitempty,oneof=ipv4 ipv6 dual" default:"dual"`
}


type DeleteNATRequest struct {
	Hostname        string `json:"hostname" form:"hostname" binding:"required"`
	ExternalPort    int    `json:"dport" form:"dport" binding:"required,min=10000,max=65535"`
	InternalPort    int    `json:"sport" form:"sport" binding:"required,min=1,max=65535"`
	ExternalPortEnd int    `json:"dport_end" form:"dport_end" binding:"omitempty,min=10000,max=65535"`
	InternalPortEnd int    `json:"sport_end" form:"sport_end" binding:"omitempty,min=1,max=65535"`
	Protocol        string `json:"dtype" form:"dtype" binding:"required,oneof=tcp udp"`
}


type NATListResponse struct {
	Code    int       `json:"code"`
	Msg     string    `json:"msg"`
	TraceID string    `json:"trace_id,omitempty"`
	Data    []NATRule `json:"data"`
}


type NATOperationResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

type NATPortCheckResponse struct {
	Code int             `json:"code"`
	Msg  string          `json:"msg"`
	Data NATPortCheckData `json:"data"`
}

type NATPortCheckData struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}


func (n *NATRule) IsActive() bool {
	return n.Status == "active"
}

func (n *NATRule) IsPortRange() bool {
	return n.ExternalPortEnd > 0 && n.InternalPortEnd > 0
}

func (n *NATRule) GetPortRangeSize() int {
	if !n.IsPortRange() {
		return 1
	}
	return (n.ExternalPortEnd - n.ExternalPort + 1)
}

func (n *NATRule) GetPortRangeString() string {
	if n.IsPortRange() {
		return fmt.Sprintf("%d-%d:%d-%d", n.ExternalPort, n.ExternalPortEnd, n.InternalPort, n.InternalPortEnd)
	}
	return fmt.Sprintf("%d:%d", n.ExternalPort, n.InternalPort)
}

func (n *NATRule) GetProtocolList() []string {
	if n.Protocol == "both" {
		return []string{"tcp", "udp"}
	}
	return []string{n.Protocol}
}
