package clients

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

var tracer = otel.Tracer("friendservice/clients/user")

type UserClient interface {
	UserExists(ctx context.Context, userID int) (bool, error)
}

type userClient struct {
	baseURL string
	client  *http.Client
}

func NewUserClient(baseURL string) UserClient {
	return &userClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *userClient) UserExists(ctx context.Context, userID int) (bool, error) {
	ctx, span := tracer.Start(ctx, "UserClient.UserExists")
	defer span.End()

	internalRoute := c.baseURL + "/internal/user/" + strconv.Itoa(userID)
	span.SetAttributes(
		attribute.String("http.method", http.MethodHead),
		attribute.String("http.route", "/internal/user/{id}"),
		attribute.Int("user.id", userID),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, internalRoute, nil)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to create request")
		return false, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to call user service")
		return false, err
	}
	defer resp.Body.Close()

	span.SetAttributes(attribute.Int("http.status_code", resp.StatusCode))

	switch resp.StatusCode {
	case http.StatusOK:
		span.SetStatus(codes.Ok, "user exists")
		return true, nil
	case http.StatusNotFound:
		span.SetStatus(codes.Ok, "user does not exist")
		return false, nil
	default:
		err = fmt.Errorf("user service returned unexpected status: %d", resp.StatusCode)
		span.RecordError(err)
		span.SetStatus(codes.Error, "user service returned unexpected status")
		return false, err
	}
}
