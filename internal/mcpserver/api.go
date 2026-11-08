package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const maxResponseBytes = 8 << 20

// rocketAPI calls the Rocket.Chat endpoints that gorocket does not implement.
type rocketAPI struct {
	baseURL   string
	userID    string
	authToken string
	client    *http.Client
}

func (a *rocketAPI) get(ctx context.Context, path string, query url.Values, out any) error {
	if a.userID == "" || a.authToken == "" {
		return ErrNoCredentials
	}

	endpoint := strings.TrimRight(a.baseURL, "/") + "/api/v1/" + path
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("rocketchat: create %s request: %w", path, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-User-Id", a.userID)
	req.Header.Set("X-Auth-Token", a.authToken)

	resp, err := a.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return fmt.Errorf("rocketchat: read %s response: %w", path, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return newAPIError(resp.StatusCode, body)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("rocketchat: decode %s response: %w", path, err)
	}
	return nil
}

type apiError struct {
	StatusCode int
	Message    string
}

func (e *apiError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("rocketchat: HTTP %d", e.StatusCode)
	}
	return fmt.Sprintf("rocketchat: HTTP %d: %s", e.StatusCode, e.Message)
}

func newAPIError(statusCode int, body []byte) error {
	message := strings.TrimSpace(string(body))
	var payload struct {
		Error string `json:"error"`
	}
	if json.Unmarshal(body, &payload) == nil && payload.Error != "" {
		message = payload.Error
	}
	if len(message) > 512 {
		message = message[:512] + "..."
	}

	return &apiError{
		StatusCode: statusCode,
		Message:    message,
	}
}
