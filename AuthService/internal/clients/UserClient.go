package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type UserClient interface {
	CreateUser(ctx context.Context, username string, description string, avatar string) (int, error)
}

type userClient struct {
	baseURL string
	client  *http.Client
}

func NewUserClient(baseURL string) UserClient {
	return &userClient{
		baseURL: baseURL,
		client:  &http.Client{},
	}
}

func (c *userClient) CreateUser(ctx context.Context, username string, description string, avatar string) (int, error) {
	body, err := json.Marshal(map[string]any{
		"username":    username,
		"description": description,
		"avatar":      avatar,
	})
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		c.baseURL+"/internal/user",
		bytes.NewBuffer(body),
	)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return 0, fmt.Errorf("failed to create user: %d", resp.StatusCode)
	}

	var createdUser struct {
		ID int `json:"id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&createdUser); err != nil {
		return 0, err
	}

	return createdUser.ID, nil
}
