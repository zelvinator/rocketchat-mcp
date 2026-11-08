package mcpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"forgejo.srv.hnatekmar.dev/hnatekmar/rocketchat-mcp/internal/rocketchat"
)

func TestPing(t *testing.T) {
	session := connect(t, newFakeRocketChat(t))

	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "ping"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("ping returned a tool error: %s", textOf(result))
	}
	if text := textOf(result); !strings.Contains(text, "pong") {
		t.Errorf("ping content = %q, want it to contain %q", text, "pong")
	}
}

func TestRocketChatMe(t *testing.T) {
	session := connect(t, newFakeRocketChat(t))

	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: "rocketchat_me"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if result.IsError {
		t.Fatalf("rocketchat_me returned a tool error: %s", textOf(result))
	}
	if text := textOf(result); !strings.Contains(text, "alice") {
		t.Errorf("rocketchat_me content = %q, want it to contain %q", text, "alice")
	}
	if result.StructuredContent == nil {
		t.Error("rocketchat_me returned no structured content")
	}
}

func TestListTools(t *testing.T) {
	session := connect(t, newFakeRocketChat(t))

	result, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	want := map[string]bool{"ping": false, "rocketchat_me": false}
	for _, tool := range result.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("tool %q is not registered", name)
		}
	}
}

func connect(t *testing.T, rc *rocketchat.Client) *mcp.ClientSession {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	server := New(rc, Options{Version: "test"})
	clientTransport, serverTransport := mcp.NewInMemoryTransports()

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("connect server: %v", err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("connect client: %v", err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })

	return clientSession
}

func newFakeRocketChat(t *testing.T) *rocketchat.Client {
	t.Helper()

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

	client, err := rocketchat.New(server.URL, "user-1", "token-1")
	if err != nil {
		t.Fatalf("rocketchat.New: %v", err)
	}
	return client
}

func textOf(result *mcp.CallToolResult) string {
	var text strings.Builder
	for _, content := range result.Content {
		if textContent, ok := content.(*mcp.TextContent); ok {
			text.WriteString(textContent.Text)
		}
	}
	return text.String()
}
