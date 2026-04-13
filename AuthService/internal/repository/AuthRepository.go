package repository

import (
	"context"

	"github.com/yonaje/authservice/internal/jwt"
	"github.com/yonaje/authservice/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"gorm.io/gorm"
)

var tracer = otel.Tracer("authservice/repository")

type AuthRepository interface {
	Create(ctx context.Context, auth *models.AuthAccount) error
	GetByEmail(ctx context.Context, email string) (*models.AuthAccount, error)
	GetByUserID(ctx context.Context, userID string) (*models.AuthAccount, error)

	SaveSession(ctx context.Context, session *models.Session) error
	UpdateSession(ctx context.Context, session *models.Session) error
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
	ctx, span := tracer.Start(ctx, "AuthRepository.Create")
	defer span.End()
	span.SetAttributes(
		attribute.Int("auth.user_id", int(auth.UserID)),
		attribute.Bool("auth.status", auth.Status),
	)

	if err := r.db.WithContext(ctx).Create(auth).Error; err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create auth account")
		return err
	}

	span.SetStatus(codes.Ok, "auth account created successfully")
	return nil
}

func (r *authRepository) GetByEmail(ctx context.Context, email string) (*models.AuthAccount, error) {
	ctx, span := tracer.Start(ctx, "AuthRepository.GetByEmail")
	defer span.End()
	span.SetAttributes(attribute.Bool("auth.email_present", email != ""))

	var auth models.AuthAccount
	err := r.db.WithContext(ctx).Where("email = ?", email).First(&auth).Error
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get auth account by email")
		return nil, err
	}

	span.SetAttributes(
		attribute.Int("auth.user_id", int(auth.UserID)),
		attribute.Bool("auth.status", auth.Status),
	)
	span.SetStatus(codes.Ok, "auth account retrieved by email successfully")
	return &auth, nil
}

func (r *authRepository) GetByUserID(ctx context.Context, userID string) (*models.AuthAccount, error) {
	ctx, span := tracer.Start(ctx, "AuthRepository.GetByUserID")
	defer span.End()
	span.SetAttributes(attribute.String("auth.user_id", userID))

	var auth models.AuthAccount
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&auth).Error
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get auth account by user id")
		return nil, err
	}

	span.SetAttributes(
		attribute.Bool("auth.email_present", auth.Email != ""),
		attribute.Bool("auth.status", auth.Status),
	)
	span.SetStatus(codes.Ok, "auth account retrieved by user id successfully")
	return &auth, nil
}

func (r *authRepository) SaveSession(ctx context.Context, session *models.Session) error {
	ctx, span := tracer.Start(ctx, "AuthRepository.SaveSession")
	defer span.End()
	span.SetAttributes(
		attribute.String("session.id", session.ID),
		attribute.String("session.user_id", session.UserID),
	)

	if err := r.db.WithContext(ctx).Create(session).Error; err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to save session")
		return err
	}

	span.SetStatus(codes.Ok, "session saved successfully")
	return nil
}

func (r *authRepository) UpdateSession(ctx context.Context, session *models.Session) error {
	ctx, span := tracer.Start(ctx, "AuthRepository.UpdateSession")
	defer span.End()
	span.SetAttributes(
		attribute.String("session.id", session.ID),
		attribute.String("session.user_id", session.UserID),
	)

	if err := r.db.WithContext(ctx).Model(&models.Session{}).
		Where("id = ?", session.ID).
		Updates(map[string]any{
			"refresh_token": session.RefreshToken,
			"expires_at":    session.ExpiresAt,
			"user_agent":    session.UserAgent,
			"ip":            session.IP,
		}).Error; err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to update session")
		return err
	}

	span.SetStatus(codes.Ok, "session updated successfully")
	return nil
}

func (r *authRepository) GetSessionByRefreshToken(ctx context.Context, refreshToken string) (*models.Session, error) {
	ctx, span := tracer.Start(ctx, "AuthRepository.GetSessionByRefreshToken")
	defer span.End()
	span.SetAttributes(attribute.Bool("session.refresh_token_present", refreshToken != ""))

	var session models.Session
	err := r.db.WithContext(ctx).Where("refresh_token = ?", jwt.HashRefreshToken(refreshToken)).First(&session).Error
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get session by refresh token")
		return nil, err
	}

	span.SetAttributes(
		attribute.String("session.id", session.ID),
		attribute.String("session.user_id", session.UserID),
	)
	span.SetStatus(codes.Ok, "session retrieved by refresh token successfully")
	return &session, nil
}

func (r *authRepository) DeleteSessionByRefreshToken(ctx context.Context, refreshToken string) error {
	ctx, span := tracer.Start(ctx, "AuthRepository.DeleteSessionByRefreshToken")
	defer span.End()
	span.SetAttributes(attribute.Bool("session.refresh_token_present", refreshToken != ""))

	if err := r.db.WithContext(ctx).Where("refresh_token = ?", jwt.HashRefreshToken(refreshToken)).Delete(&models.Session{}).Error; err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete session by refresh token")
		return err
	}

	span.SetStatus(codes.Ok, "session deleted by refresh token successfully")
	return nil
}

func (r *authRepository) DeleteSessionsByUserID(ctx context.Context, userID string) error {
	ctx, span := tracer.Start(ctx, "AuthRepository.DeleteSessionsByUserID")
	defer span.End()
	span.SetAttributes(attribute.String("session.user_id", userID))

	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&models.Session{}).Error; err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to delete sessions by user id")
		return err
	}

	span.SetStatus(codes.Ok, "sessions deleted by user id successfully")
	return nil
}
