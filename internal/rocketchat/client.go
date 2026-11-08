// Package rocketchat provides a minimal client for the Rocket.Chat REST API.
package rocketchat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ErrNoCredentials reports that the client has no user ID or auth token.
var ErrNoCredentials = errors.New("rocketchat: ROCKETCHAT_USER_ID and ROCKETCHAT_AUTH_TOKEN are required")

// Client calls the Rocket.Chat REST API.
type Client struct {
	baseURL    string
	userID     string
	authToken  string
	httpClient *http.Client
}

// New returns a client for the Rocket.Chat workspace at baseURL.
func New(baseURL, userID, authToken string) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("rocketchat: parse base URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("rocketchat: base URL %q needs an http or https scheme", baseURL)
	}
	if u.Host == "" {
		return nil, fmt.Errorf("rocketchat: base URL %q needs a host", baseURL)
	}

	return &Client{
		baseURL:    u.String(),
		userID:     userID,
		authToken:  authToken,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// User is the subset of the Rocket.Chat user object that this server uses.
type User struct {
	ID       string `json:"_id"`
	Username string `json:"username"`
	Name     string `json:"name"`
	Status   string `json:"status"`
}

// Me returns the user that owns the configured credentials.
func (c *Client) Me(ctx context.Context) (*User, error) {
	var user User
	if err := c.get(ctx, "/api/v1/me", &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	if c.userID == "" || c.authToken == "" {
		return ErrNoCredentials
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("rocketchat: create GET %s request: %w", path, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-User-Id", c.userID)
	req.Header.Set("X-Auth-Token", c.authToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("rocketchat: GET %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode > http.StatusMultipleChoices-1 {
		return newAPIError(http.MethodGet, path, resp)
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("rocketchat: decode GET %s response: %w", path, err)
	}
	return nil
}

// APIError is a non-2xx response from the Rocket.Chat API.
type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("rocketchat: %s %s: %s", e.Method, e.Path, http.StatusText(e.StatusCode))
	}
	return fmt.Sprintf("rocketchat: %s %s: HTTP %d: %s", e.Method, e.Path, e.StatusCode, e.Message)
}

func newAPIError(method, path string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))

	message := strings.TrimSpace(string(body))
	var payload struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &payload) == nil && payload.Error != "" {
		message = payload.Error
	}

	return &APIError{
		Method:     method,
		Path:       path,
		StatusCode: resp.StatusCode,
		Message:    message,
	}
}
