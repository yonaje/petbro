package database

import (
	"context"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

func Connect(uri, username, password string) neo4j.DriverWithContext {
	driver, err := neo4j.NewDriverWithContext(
		uri,
		neo4j.BasicAuth(username, password, ""),
	)
	if err != nil {
		panic("failed to create neo4j driver: " + err.Error())
	}

	if err := driver.VerifyConnectivity(context.Background()); err != nil {
		panic("failed to connect to neo4j: " + err.Error())
	}

	session := driver.NewSession(context.Background(), neo4j.SessionConfig{AccessMode: neo4j.AccessModeWrite})
	defer session.Close(context.Background())

	if _, err := session.Run(
		context.Background(),
		"CREATE CONSTRAINT user_id_unique IF NOT EXISTS FOR (u:User) REQUIRE u.id IS UNIQUE",
		nil,
	); err != nil {
		panic("failed to create neo4j constraints: " + err.Error())
	}

	return driver
}
