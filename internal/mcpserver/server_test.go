package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPing(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "ping", nil)
	if result.IsError {
		t.Fatalf("ping returned a tool error: %s", textOf(result))
	}
	if text := textOf(result); !strings.Contains(text, "pong") {
		t.Errorf("ping content = %q, want it to contain %q", text, "pong")
	}
}

func TestRocketChatMe(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_me", nil)
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

func TestRocketChatMeWithoutCredentials(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "", "")

	result := callTool(t, session, "rocketchat_me", nil)
	if !result.IsError {
		t.Fatalf("rocketchat_me returned no tool error: %s", textOf(result))
	}
	if text := textOf(result); !strings.Contains(text, "ROCKETCHAT_USER_ID") {
		t.Errorf("rocketchat_me content = %q, want it to name the missing variables", text)
	}
}

func TestRocketChatMeRequestFailed(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, true), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_me", nil)
	if !result.IsError {
		t.Fatalf("rocketchat_me returned no tool error: %s", textOf(result))
	}
	if text := textOf(result); !strings.Contains(text, "success=false") {
		t.Errorf("rocketchat_me content = %q, want it to report the failed request", text)
	}
}

func TestRocketChatMeRetriesRateLimit(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/me" {
			http.NotFound(w, r)
			return
		}
		if attempts.Add(1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"_id":      "user-1",
			"username": "alice",
			"name":     "Alice",
			"status":   "online",
			"success":  true,
		})
	}))
	t.Cleanup(server.Close)

	session := connect(t, server.URL, "user-1", "token-1")
	result := callTool(t, session, "rocketchat_me", nil)
	if result.IsError {
		t.Fatalf("rocketchat_me returned a tool error: %s", textOf(result))
	}
	if attempts.Load() != 2 {
		t.Errorf("Me attempts = %d, want 2", attempts.Load())
	}
}

func TestListTools(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result, err := session.ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	want := map[string]bool{
		"ping":                       false,
		"rocketchat_me":              false,
		"rocketchat_list_channels":   false,
		"rocketchat_list_dms":        false,
		"rocketchat_list_messages":   false,
		"rocketchat_search_messages": false,
	}
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

func connect(t *testing.T, url, userID, authToken string) *mcp.ClientSession {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	t.Cleanup(cancel)

	server := New(Options{
		RocketChatURL: url,
		UserID:        userID,
		AuthToken:     authToken,
		Version:       "test",
	})
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

func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(t.Context(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool %s: %v", name, err)
	}
	return result
}

func decodeStructured(t *testing.T, result *mcp.CallToolResult, out any) {
	t.Helper()
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if err := json.Unmarshal(data, out); err != nil {
		t.Fatalf("unmarshal structured content: %v", err)
	}
}

func newFakeRocketChat(t *testing.T, failMe bool) string {
	t.Helper()

	channelMessages := []map[string]any{
		{
			"_id": "M5",
			"rid": "C1",
			"msg": "five",
			"ts":  "2026-11-08T15:00:00.000Z",
			"u":   map[string]any{"_id": "user-2", "username": "bob", "name": "Bob"},
		},
		{
			"_id": "M4",
			"rid": "C1",
			"msg": "four",
			"ts":  "2026-11-08T14:00:00.000Z",
			"u":   map[string]any{"_id": "user-2", "username": "bob", "name": "Bob"},
		},
		{
			"_id": "M3",
			"rid": "C1",
			"msg": "three with attachment",
			"ts":  "2026-11-08T13:00:00.000Z",
			"u":   map[string]any{"_id": "user-2", "username": "bob", "name": "Bob"},
			"attachments": []any{
				map[string]any{
					"title": "Loki error logs",
					"fields": []any{
						map[string]any{"title": "Status", "value": "RESOLVED"},
					},
				},
			},
		},
		{
			"_id": "M2",
			"rid": "C1",
			"msg": "two",
			"ts":  "2026-11-08T12:00:00.000Z",
			"u":   map[string]any{"_id": "user-3", "username": "carol", "name": "Carol"},
		},
		{
			"_id": "M1",
			"rid": "C1",
			"msg": "one",
			"ts":  "2026-11-08T11:00:00.000Z",
			"u":   map[string]any{"_id": "user-3", "username": "carol", "name": "Carol"},
		},
	}
	groupMessages := []map[string]any{
		{
			"_id": "G-M1",
			"rid": "G1",
			"msg": "group message",
			"ts":  "2026-11-08T12:00:00.000Z",
			"u":   map[string]any{"_id": "user-1", "username": "alice", "name": "Alice"},
		},
	}
	dmMessages := []map[string]any{
		{
			"_id": "D-M1",
			"rid": "D1",
			"msg": "direct message",
			"ts":  "2026-11-08T16:00:00.000Z",
			"u":   map[string]any{"_id": "user-2", "username": "bob", "name": "Bob"},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-User-Id") != "user-1" || r.Header.Get("X-Auth-Token") != "token-1" {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"success": false, "error": "unauthorized"})
			return
		}

		switch r.URL.Path {
		case "/api/v1/me":
			if failMe {
				writeJSON(w, http.StatusUnauthorized, map[string]any{"success": false, "error": "unauthorized"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{
				"_id":      "user-1",
				"username": "alice",
				"name":     "Alice",
				"status":   "online",
				"success":  true,
			})
		case "/api/v1/channels.list":
			writeJSON(w, http.StatusOK, map[string]any{
				"channels": []any{
					map[string]any{
						"_id":        "C1",
						"name":       "general",
						"t":          "c",
						"msgs":       2,
						"usersCount": 3,
						"ro":         false,
						"ts":         "2026-10-01T10:00:00.000Z",
						"_updatedAt": "2026-11-08T15:00:00.000Z",
						"lastMessage": map[string]any{
							"ts": "2026-11-08T15:00:00.000Z",
						},
						"lm": "2026-11-08T15:00:00.000Z",
					},
				},
				"success": true,
			})
		case "/api/v1/groups.list":
			writeJSON(w, http.StatusOK, map[string]any{
				"groups": []any{
					map[string]any{
						"_id":        "G1",
						"name":       "alerts",
						"t":          "p",
						"msgs":       1,
						"usersCount": 2,
						"ro":         false,
						"ts":         "2026-10-02T10:00:00.000Z",
						"_updatedAt": "2026-11-08T12:00:00.000Z",
						"lastMessage": map[string]any{
							"ts": "2026-11-08T12:00:00.000Z",
						},
						"lm": "2026-11-08T12:00:00.000Z",
					},
				},
				"success": true,
			})
		case "/api/v1/rooms.info":
			roomID := r.URL.Query().Get("roomId")
			var room map[string]any
			switch roomID {
			case "C1":
				room = map[string]any{"_id": "C1", "name": "general", "t": "c"}
			case "G1":
				room = map[string]any{"_id": "G1", "name": "alerts", "t": "p"}
			case "D1":
				room = map[string]any{"_id": "D1", "name": "dm", "t": "d"}
			case "L1":
				room = map[string]any{"_id": "L1", "name": "livechat", "t": "l"}
			default:
				writeJSON(w, http.StatusNotFound, map[string]any{"success": false, "error": "room not found"})
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"room": room, "success": true})
		case "/api/v1/im.list":
			writeJSON(w, http.StatusOK, map[string]any{
				"ims": []any{
					map[string]any{
						"_id":        "D1",
						"t":          "d",
						"usernames":  []any{"alice", "bob"},
						"usersCount": 2,
						"msgs":       1,
						"ts":         "2026-10-03T10:00:00.000Z",
						"_updatedAt": "2026-11-08T16:00:00.000Z",
						"lastMessage": map[string]any{
							"ts": "2026-11-08T16:00:00.000Z",
						},
					},
				},
				"success": true,
			})
		case "/api/v1/channels.history":
			writeHistory(w, r, channelMessages)
		case "/api/v1/groups.history":
			writeHistory(w, r, groupMessages)
		case "/api/v1/im.history":
			writeHistory(w, r, dmMessages)
		case "/api/v1/chat.search":
			writeSearch(w, r, map[string][]map[string]any{
				"C1": channelMessages,
				"G1": groupMessages,
				"D1": dmMessages,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	return server.URL
}

// writeSearch reproduces the parts of Rocket.Chat 7's chat.search route that
// the client has to respect: the schema allows only roomId, searchText, count
// and offset (additionalProperties is false), searchText is required, and a
// from:<username> token narrows the results to that author.
func writeSearch(w http.ResponseWriter, r *http.Request, rooms map[string][]map[string]any) {
	allowed := map[string]bool{"roomId": true, "searchText": true, "count": true, "offset": true}
	for key := range r.URL.Query() {
		if !allowed[key] {
			writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "unknown query param " + key})
			return
		}
	}

	roomID := r.URL.Query().Get("roomId")
	searchText := strings.TrimSpace(r.URL.Query().Get("searchText"))
	if roomID == "" || searchText == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "roomId and searchText are required"})
		return
	}

	authors := map[string]bool{}
	text := searchText
	for {
		match := fromPattern.FindStringSubmatch(text)
		if match == nil {
			break
		}
		authors[strings.ToLower(match[0][len("from:"):])] = true
		text = strings.Replace(text, match[0], "", 1)
	}
	text = strings.TrimSpace(text)

	matches := make([]map[string]any, 0, len(rooms[roomID]))
	for _, message := range rooms[roomID] {
		author := strings.ToLower(message["u"].(map[string]any)["username"].(string))
		if len(authors) > 0 && !authors[author] {
			continue
		}
		if text != "" && !strings.Contains(strings.ToLower(message["msg"].(string)), strings.ToLower(text)) {
			continue
		}
		matches = append(matches, message)
	}

	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	count, _ := strconv.Atoi(r.URL.Query().Get("count"))
	if count <= 0 {
		count = 50
	}
	if offset >= len(matches) {
		matches = matches[:0]
	} else {
		matches = matches[offset:]
		if len(matches) > count {
			matches = matches[:count]
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"messages": matches, "success": true})
}

func writeHistory(w http.ResponseWriter, r *http.Request, messages []map[string]any) {
	latest := r.URL.Query().Get("latest")
	oldest := r.URL.Query().Get("oldest")
	count, _ := strconv.Atoi(r.URL.Query().Get("count"))
	if count <= 0 {
		count = 20
	}

	filtered := make([]map[string]any, 0, len(messages))
	for _, message := range messages {
		timestamp, err := time.Parse(time.RFC3339Nano, message["ts"].(string))
		if err != nil {
			continue
		}
		if latest != "" {
			latestTime, err := time.Parse(time.RFC3339Nano, latest)
			if err == nil && !timestamp.Before(latestTime) {
				continue
			}
		}
		if oldest != "" {
			oldestTime, err := time.Parse(time.RFC3339Nano, oldest)
			if err == nil && !timestamp.After(oldestTime) {
				continue
			}
		}
		filtered = append(filtered, message)
	}

	if len(filtered) > count {
		filtered = filtered[:count]
	}
	writeJSON(w, http.StatusOK, map[string]any{"messages": filtered, "success": true})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
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
