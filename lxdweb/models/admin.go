package models
import (
	"time"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)
type Admin struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Username  string         `json:"username" gorm:"uniqueIndex;size:100;not null"`
	Password  string         `json:"-" gorm:"size:255;not null"`
	Email     string         `json:"email" gorm:"size:255"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
func (a *Admin) HashPassword() error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(a.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	a.Password = string(hashedPassword)
	return nil
}
func (a *Admin) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(a.Password), []byte(password))
	return err == nil
}
