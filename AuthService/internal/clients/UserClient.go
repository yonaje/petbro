package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var tracer = otel.Tracer("authservice/clients/user")

type UserClient interface {
	CreateUser(ctx context.Context, username string, description string, avatar string) (int, error)
	DeleteUser(ctx context.Context, userID int) error
}

type userClient struct {
	baseURL string
	client  *http.Client
}

func NewUserClient(baseURL string) UserClient {
	return &userClient{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *userClient) CreateUser(ctx context.Context, username string, description string, avatar string) (int, error) {
	ctx, span := tracer.Start(ctx, "UserClient.CreateUser")
	defer span.End()
	span.SetAttributes(
		attribute.String("http.method", http.MethodPost),
		attribute.String("http.route", "/internal/user"),
		attribute.Bool("user.description_present", description != ""),
		attribute.Bool("user.avatar_present", avatar != ""),
		attribute.Bool("user.username_present", username != ""),
	)

	body, err := json.Marshal(map[string]any{
		"username":    username,
		"description": description,
		"avatar":      avatar,
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to marshal request body")
		return 0, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.baseURL+"/internal/user",
		bytes.NewBuffer(body),
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create request")
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to call user service")
		return 0, err
	}
	defer resp.Body.Close()
	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	if resp.StatusCode != http.StatusCreated {
		err = fmt.Errorf("failed to create user: %d", resp.StatusCode)
		span.RecordError(err)
		span.SetStatus(codes.Error, "user service returned unexpected status")
		return 0, err
	}

	var createdUser struct {
		ID int `json:"id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&createdUser); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to decode user service response")
		return 0, err
	}

	span.SetAttributes(attribute.Int("user.id", createdUser.ID))
	span.SetStatus(codes.Ok, "user created successfully")
	return createdUser.ID, nil
}

func (c *userClient) DeleteUser(ctx context.Context, userID int) error {
	ctx, span := tracer.Start(ctx, "UserClient.DeleteUser")
	defer span.End()

	internalRoute := c.baseURL + "/internal/user/" + strconv.Itoa(userID)
	span.SetAttributes(
		attribute.String("http.method", http.MethodDelete),
		attribute.String("http.route", "/internal/user/{id}"),
		attribute.Int("user.id", userID),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, internalRoute, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create request")
		return err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to call user service")
		return err
	}
	defer resp.Body.Close()

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		err = fmt.Errorf("failed to delete user: %d", resp.StatusCode)
		span.RecordError(err)
		span.SetStatus(codes.Error, "user service returned unexpected status")
		return err
	}

	span.SetStatus(codes.Ok, "user deleted successfully")
	return nil
}
