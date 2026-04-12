package database

import (
	"fmt"

	_ "github.com/lib/pq"
	"github.com/yonaje/userservice/internal/models"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(log *zap.Logger,
	host, user, password, dbName string,
) (db *gorm.DB, err error) {
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s sslmode=disable",
		host, user, password, dbName)

	db, err = gorm.Open(postgres.Open(dsn))
	if err != nil {
		log.Error("Failed to connect to database",
			zap.String("operation", "database.Connect"),
			zap.String("step", "open_connection"),
			zap.Error(err),
		)
		return nil, err
	}

	if err := db.AutoMigrate(&models.User{}); err != nil {
		log.Error("Failed to migrate database",
			zap.String("operation", "database.Connect"),
			zap.String("step", "auto_migrate"),
			zap.Error(err),
		)
		return nil, err
	}

	log.Info("Successfully connected to database",
		zap.String("operation", "database.Connect"),
		zap.String("step", "connection_successful"),
	)

	return db, nil
}
