package rocketchat

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/me" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("X-User-Id") != "user-1" || r.Header.Get("X-Auth-Token") != "token-1" {
			http.Error(w, `{"success":false,"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"_id":"user-1","username":"alice","name":"Alice","status":"online"}`))
	}))
	t.Cleanup(server.Close)

	client, err := New(server.URL, "user-1", "token-1")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	user, err := client.Me(context.Background())
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if user.ID != "user-1" || user.Username != "alice" || user.Name != "Alice" || user.Status != "online" {
		t.Errorf("Me returned %+v", user)
	}
}

func TestMeRequiresCredentials(t *testing.T) {
	client, err := New("http://localhost:3000", "", "")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = client.Me(context.Background())
	if !errors.Is(err, ErrNoCredentials) {
		t.Fatalf("Me error = %v, want ErrNoCredentials", err)
	}
}

func TestAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"success":false,"error":"You must be logged in to do this."}`))
	}))
	t.Cleanup(server.Close)

	client, err := New(server.URL, "user-1", "token-1")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	_, err = client.Me(context.Background())
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("Me error = %v, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusUnauthorized {
		t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, http.StatusUnauthorized)
	}
	if !strings.Contains(apiErr.Error(), "logged in") {
		t.Errorf("Error() = %q, want the API message", apiErr.Error())
	}
}

func TestNewRejectsInvalidURL(t *testing.T) {
	for _, rawURL := range []string{"", "localhost:3000", "ftp://chat.example.com"} {
		if _, err := New(rawURL, "", ""); err == nil {
			t.Errorf("New(%q) returned no error", rawURL)
		}
	}
}
