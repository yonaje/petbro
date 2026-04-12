package models

import "time"

type Session struct {
	ID           string `json:"id" gorm:"primaryKey"`
	UserID       string `json:"user_id"`
	RefreshToken string `json:"refresh_token"`

	UserAgent string `json:"user_agent"`
	IP        string `json:"ip"`

	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}
