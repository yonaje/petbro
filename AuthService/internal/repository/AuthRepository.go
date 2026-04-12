package repository

import (
	"context"

	"github.com/yonaje/authservice/internal/models"
	"gorm.io/gorm"
)

type AuthRepository interface {
	Create(ctx context.Context, auth *models.AuthAccount) error
	GetByEmail(ctx context.Context, email string) (*models.AuthAccount, error)
	GetByUserID(ctx context.Context, userID string) (*models.AuthAccount, error)

	SaveSession(ctx context.Context, session *models.Session) error
	GetSessionByRefreshToken(ctx context.Context, refreshToken string) (*models.Session, error)
	DeleteSessionByRefreshToken(ctx context.Context, refreshToken string) error
	DeleteSessionsByUserID(ctx context.Context, userID string) error
}

type authRepository struct {
	db *gorm.DB
}

func NewAuthRepository(db *gorm.DB) AuthRepository {
	return &authRepository{db: db}
}

func (r *authRepository) Create(ctx context.Context, auth *models.AuthAccount) error {
	return r.db.WithContext(ctx).Create(auth).Error
}

func (r *authRepository) GetByEmail(ctx context.Context, email string) (*models.AuthAccount, error) {
	var auth models.AuthAccount
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&auth).Error
	if err != nil {
		return nil, err
	}
	return &auth, nil
}

func (r *authRepository) GetByUserID(ctx context.Context, userID string) (*models.AuthAccount, error) {
	var auth models.AuthAccount
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&auth).Error
	if err != nil {
		return nil, err
	}
	return &auth, nil
}

func (r *authRepository) SaveSession(ctx context.Context, session *models.Session) error {
	return r.db.WithContext(ctx).Create(session).Error
}

func (r *authRepository) GetSessionByRefreshToken(ctx context.Context, refreshToken string) (*models.Session, error) {
	var session models.Session
	err := r.db.WithContext(ctx).Where("refresh_token = ?", refreshToken).First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil

}

func (r *authRepository) DeleteSessionByRefreshToken(ctx context.Context, refreshToken string) error {
	return r.db.WithContext(ctx).Where("refresh_token = ?", refreshToken).Delete(&models.Session{}).Error
}

func (r *authRepository) DeleteSessionsByUserID(ctx context.Context, userID string) error {
	return r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&models.Session{}).Error
}
