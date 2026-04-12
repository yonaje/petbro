package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/lib/pq"
	"github.com/yonaje/userservice/internal/models"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"gorm.io/gorm"
)

var tracer = otel.Tracer("userservice/repository")
var ErrUsernameAlreadyTaken = errors.New("username already taken")

type UserRepository interface {
	Users(ctx context.Context, page int, limit int, sort string) (users []models.User, err error)
	User(ctx context.Context, id int) (user *models.User, err error)
	Create(ctx context.Context, user *models.User) (err error)
	Update(ctx context.Context, id int, newUser map[string]any) (err error)
	Delete(ctx context.Context, id int) (err error)
	Exists(ctx context.Context, id int) (exists bool, err error)
}

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

func (r *userRepository) Users(ctx context.Context, page int, limit int, sort string) (users []models.User, err error) {
	ctx, span := tracer.Start(ctx, "UserRepository.Users")
	defer span.End()

	if err := r.db.WithContext(ctx).Order(sort).Limit(limit).Offset((page - 1) * limit).Find(&users).Error; err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get users")
		return nil, err
	}

	span.SetStatus(codes.Ok, "users retrieved successfully")
	return users, nil
}

func (r *userRepository) User(ctx context.Context, id int) (user *models.User, err error) {
	ctx, span := tracer.Start(ctx, "UserRepository.User")
	defer span.End()

	if err := r.db.WithContext(ctx).First(&user, id).Error; err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to get user")
		return nil, err
	}

	span.SetStatus(codes.Ok, "user retrieved successfully")
	return user, nil
}

func (r *userRepository) Create(ctx context.Context, user *models.User) (err error) {
	ctx, span := tracer.Start(ctx, "UserRepository.Create")
	defer span.End()

	if err := r.db.WithContext(ctx).Create(user).Error; err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create user")

		var pgErr *pq.Error
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%w: %v", ErrUsernameAlreadyTaken, err)
		}

		return err
	}

	span.SetStatus(codes.Ok, "user created successfully")
	return nil
}

func (r *userRepository) Update(ctx context.Context, id int, newUser map[string]any) (err error) {
	ctx, span := tracer.Start(ctx, "UserRepository.Update")
	defer span.End()

	result := r.db.WithContext(ctx).
		Model(&models.User{}).
		Where("id = ?", id).
		Updates(newUser)

	if result.Error != nil {
		span.RecordError(result.Error)
		span.SetStatus(codes.Error, "failed to update user")

		var pgErr *pq.Error
		if errors.As(result.Error, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("%w: %v", ErrUsernameAlreadyTaken, result.Error)
		}

		return result.Error
	}

	if result.RowsAffected == 0 {
		err := gorm.ErrRecordNotFound
		span.RecordError(err)
		span.SetStatus(codes.Error, "user not found")
		return err
	}

	span.SetStatus(codes.Ok, "user updated successfully")
	return nil
}

func (r *userRepository) Delete(ctx context.Context, id int) (err error) {
	ctx, span := tracer.Start(ctx, "UserRepository.Delete")
	defer span.End()

	result := r.db.WithContext(ctx).Unscoped().Delete(&models.User{}, id)
	if result.Error != nil {
		span.RecordError(result.Error)
		span.SetStatus(codes.Error, "failed to delete user")
		return result.Error
	}

	if result.RowsAffected == 0 {
		err := gorm.ErrRecordNotFound
		span.RecordError(err)
		span.SetStatus(codes.Error, "user not found")
		return err
	}

	span.SetStatus(codes.Ok, "user deleted successfully")
	return nil
}

func (r *userRepository) Exists(ctx context.Context, id int) (exists bool, err error) {
	ctx, span := tracer.Start(ctx, "UserRepository.Exists")
	defer span.End()

	var count int64
	if err := r.db.WithContext(ctx).Model(&models.User{}).Where("id = ?", id).Count(&count).Error; err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to check if user exists")
		return false, err
	}

	exists = count > 0

	if exists {
		span.SetStatus(codes.Ok, "user exists")
	} else {
		span.SetStatus(codes.Ok, "user does not exist")
	}

	return exists, nil
}
