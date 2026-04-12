package models

import "time"

type AuthAccount struct {
	ID uint `gorm:"primaryKey"`

	UserID       uint   `json:"user_id"`
	Email        string `json:"email"`
	PasswordHash string `json:"-"`

	Status bool `json:"status"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
