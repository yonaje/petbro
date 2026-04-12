package clients

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

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
		client:  &http.Client{},
	}
}

func (c *userClient) UserExists(ctx context.Context, userID int) (bool, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodHead,
		c.baseURL+"/internal/user/"+strconv.Itoa(userID),
		nil,
	)
	if err != nil {
		return false, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("user service returned unexpected status: %d", resp.StatusCode)
	}
}
