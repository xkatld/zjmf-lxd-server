package models

import "time"

// ProxyRule 反向代理规则
type ProxyRule struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	ContainerName string    `json:"container_name" gorm:"index;size:500;not null"`
	Domain        string    `json:"domain" gorm:"uniqueIndex;size:500;not null"` // 唯一域名
	ContainerPort int       `json:"container_port" gorm:"not null;default:80"`    // 容器端口
	Status        string    `json:"status" gorm:"size:100;default:'active'"`      // active, inactive
	Description   string    `json:"description" gorm:"type:text"`
	SSLEnabled    bool      `json:"ssl_enabled" gorm:"default:false"`             // 是否启用SSL
	SSLType       string    `json:"ssl_type" gorm:"size:50;default:'none'"`       // none, self-signed, custom
	SSLCertPath   string    `json:"ssl_cert_path" gorm:"size:500"`                // 证书文件路径
	SSLKeyPath    string    `json:"ssl_key_path" gorm:"size:500"`                 // 私钥文件路径
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// AddProxyRequest 添加反向代理请求
type AddProxyRequest struct {
	Hostname      string `json:"hostname" form:"hostname" binding:"required"`
	Domain        string `json:"domain" form:"domain" binding:"required"`
	ContainerPort int    `json:"container_port" form:"container_port" binding:"omitempty,min=1,max=65535"`
	Description   string `json:"description" form:"description" binding:"omitempty,max=255"`
	SSLEnabled    bool   `json:"ssl_enabled" form:"ssl_enabled"`                      // 是否启用SSL
	SSLType       string `json:"ssl_type" form:"ssl_type"`                            // self-signed 或 custom
	SSLCert       string `json:"ssl_cert" form:"ssl_cert" binding:"omitempty"`        // 证书内容（用户填写）
	SSLKey        string `json:"ssl_key" form:"ssl_key" binding:"omitempty"`          // 私钥内容（用户填写）
}

// DeleteProxyRequest 删除反向代理请求
type DeleteProxyRequest struct {
	Hostname string `json:"hostname" form:"hostname" binding:"required"`
	Domain   string `json:"domain" form:"domain" binding:"required"`
}

// ProxyResponse 通用响应
type ProxyResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data interface{} `json:"data,omitempty"`
}

// ProxyListResponse 列表响应
type ProxyListResponse struct {
	Code int         `json:"code"`
	Msg  string      `json:"msg"`
	Data []ProxyRule `json:"data"`
}

// ProxyCheckResponse 域名检查响应
type ProxyCheckResponse struct {
	Code int              `json:"code"`
	Msg  string           `json:"msg"`
	Data ProxyCheckData   `json:"data"`
}

type ProxyCheckData struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// IsActive 检查规则是否激活
func (p *ProxyRule) IsActive() bool {
	return p.Status == "active"
}

