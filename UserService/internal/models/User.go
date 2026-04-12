package models

type User struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	Username    string `json:"username" gorm:"uniqueIndex"`
	Description string `json:"description"`
	Avatar      string `json:"avatar"`
}
