package database

import (
	"context"
	"fmt"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"go.uber.org/zap"
)

func Connect(log *zap.Logger, uri, username, password string) (neo4j.DriverWithContext, error) {
	driver, err := neo4j.NewDriverWithContext(
		uri,
		neo4j.BasicAuth(username, password, ""),
	)
	if err != nil {
		log.Error("Failed to create neo4j driver",
			zap.String("operation", "database.Connect"),
			zap.String("step", "create_driver"),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to create neo4j driver: %w", err)
	}

	if err := driver.VerifyConnectivity(context.Background()); err != nil {
		log.Error("Failed to verify neo4j connectivity",
			zap.String("operation", "database.Connect"),
			zap.String("step", "verify_connectivity"),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to connect to neo4j: %w", err)
	}

	session := driver.NewSession(context.Background(), neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(context.Background())

	if _, err := session.Run(
		context.Background(),
		"CREATE CONSTRAINT user_id_unique IF NOT EXISTS FOR (u:User) REQUIRE u.id IS UNIQUE",
		nil,
	); err != nil {
		log.Error("Failed to create neo4j constraints",
			zap.String("operation", "database.Connect"),
			zap.String("step", "create_constraints"),
			zap.Error(err),
		)
		return nil, fmt.Errorf("failed to create neo4j constraints: %w", err)
	}

	log.Info("Successfully connected to database",
		zap.String("operation", "database.Connect"),
		zap.String("step", "connection_successful"),
	)

	return driver, nil
}
