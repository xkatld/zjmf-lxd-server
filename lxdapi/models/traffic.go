package models

import (
	"time"

	"gorm.io/gorm"
)


type TrafficSummary struct {
	gorm.Model
	ContainerName   string `gorm:"index:uidx_traffic_container_name,unique;size:500"`
	TotalReceived   uint64
	TotalSent       uint64
	TotalBytes      uint64
	LastRecordTime  time.Time
	LastReceived    uint64
	LastSent        uint64
	DailyReceived   uint64
	DailySent       uint64
	MonthlyReceived uint64
	MonthlySent     uint64
}


type TrafficRecordResponse struct {
	ID               uint      `json:"id" example:"1"`
	CreatedAt        time.Time `json:"created_at"`
	ContainerName    string    `json:"container_name" example:"test-container"`
	RecordTime       time.Time `json:"record_time"`
	BytesReceived    uint64    `json:"bytes_received" example:"1048576"`
	BytesSent        uint64    `json:"bytes_sent" example:"524288"`
	PacketsReceived  uint64    `json:"packets_received" example:"1024"`
	PacketsSent      uint64    `json:"packets_sent" example:"512"`
	BytesReceivedInc uint64    `json:"bytes_received_inc" example:"1024"`
	BytesSentInc     uint64    `json:"bytes_sent_inc" example:"512"`
	TotalBytes       uint64    `json:"total_bytes" example:"1572864"`
	TotalBytesInc    uint64    `json:"total_bytes_inc" example:"1536"`
}


func (t *TrafficSummary) GetTotalGB() float64 {
	return float64(t.TotalBytes) / 1024 / 1024 / 1024
}


func (t *TrafficSummary) GetDailyGB() float64 {
	return float64(t.DailyReceived+t.DailySent) / 1024 / 1024 / 1024
}


func (t *TrafficSummary) GetMonthlyGB() float64 {
	return float64(t.MonthlyReceived+t.MonthlySent) / 1024 / 1024 / 1024
}
