package mcpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchMessagesByText(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_search_messages", map[string]any{
		"room_id": "C1",
		"query":   "attachment",
	})
	if result.IsError {
		t.Fatalf("rocketchat_search_messages returned a tool error: %s", textOf(result))
	}

	var out searchMessagesOutput
	decodeStructured(t, result, &out)
	if len(out.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(out.Messages))
	}
	if out.Messages[0].ID != "M3" || out.Messages[0].Author != "bob" {
		t.Errorf("message = %+v, want M3 by bob", out.Messages[0])
	}
	if out.HasMore {
		t.Error("HasMore = true, want false")
	}
	if out.NextOffset != 0 {
		t.Errorf("NextOffset = %d, want 0", out.NextOffset)
	}
	if text := textOf(result); !strings.Contains(text, "three with attachment") {
		t.Errorf("content = %q, want the matching message", text)
	}
}

func TestSearchMessagesByAuthorPages(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	first := callTool(t, session, "rocketchat_search_messages", map[string]any{
		"room_id": "C1",
		"author":  "bob",
		"limit":   2,
	})
	if first.IsError {
		t.Fatalf("rocketchat_search_messages returned a tool error: %s", textOf(first))
	}

	var firstPage searchMessagesOutput
	decodeStructured(t, first, &firstPage)
	if len(firstPage.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(firstPage.Messages))
	}
	if firstPage.Messages[0].ID != "M5" || firstPage.Messages[1].ID != "M4" {
		t.Errorf("message order = %s, %s", firstPage.Messages[0].ID, firstPage.Messages[1].ID)
	}
	if !firstPage.HasMore {
		t.Error("HasMore = false, want true")
	}
	if firstPage.NextOffset != 2 {
		t.Errorf("NextOffset = %d, want 2", firstPage.NextOffset)
	}
	if text := textOf(first); !strings.Contains(text, "Next page: use offset=2") {
		t.Errorf("content = %q, want an offset hint", text)
	}

	second := callTool(t, session, "rocketchat_search_messages", map[string]any{
		"room_id": "C1",
		"author":  "bob",
		"limit":   2,
		"offset":  firstPage.NextOffset,
	})
	if second.IsError {
		t.Fatalf("second page returned a tool error: %s", textOf(second))
	}

	var secondPage searchMessagesOutput
	decodeStructured(t, second, &secondPage)
	if len(secondPage.Messages) != 1 || secondPage.Messages[0].ID != "M3" {
		t.Fatalf("second page messages = %+v, want [M3]", secondPage.Messages)
	}
	if secondPage.HasMore {
		t.Error("HasMore = true, want false")
	}
}

func TestSearchMessagesAuthorWithoutQuery(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_search_messages", map[string]any{
		"room_id": "C1",
		"author":  "carol",
	})
	if result.IsError {
		t.Fatalf("rocketchat_search_messages returned a tool error: %s", textOf(result))
	}

	var out searchMessagesOutput
	decodeStructured(t, result, &out)
	if len(out.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(out.Messages))
	}
	for _, message := range out.Messages {
		if message.Author != "carol" {
			t.Errorf("message %s is by %q, want carol", message.ID, message.Author)
		}
	}
}

func TestSearchMessagesTextAndAuthor(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_search_messages", map[string]any{
		"room_id": "C1",
		"query":   "four",
		"author":  "bob",
	})
	if result.IsError {
		t.Fatalf("rocketchat_search_messages returned a tool error: %s", textOf(result))
	}

	var out searchMessagesOutput
	decodeStructured(t, result, &out)
	if len(out.Messages) != 1 || out.Messages[0].ID != "M4" {
		t.Fatalf("messages = %+v, want [M4]", out.Messages)
	}
}

func TestSearchMessagesRequiresRoomID(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_search_messages", map[string]any{
		"query": "attachment",
	})
	if !result.IsError {
		t.Fatalf("rocketchat_search_messages returned no tool error: %s", textOf(result))
	}
}

func TestSearchMessagesRequiresQueryOrAuthor(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_search_messages", map[string]any{
		"room_id": "C1",
	})
	if !result.IsError {
		t.Fatalf("rocketchat_search_messages returned no tool error: %s", textOf(result))
	}
	if text := textOf(result); !strings.Contains(text, "query or author is required") {
		t.Errorf("content = %q, want a query/author error", text)
	}
}

func TestSearchMessagesRejectsInvalidAuthor(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_search_messages", map[string]any{
		"room_id": "C1",
		"author":  "carol smith",
	})
	if !result.IsError {
		t.Fatalf("rocketchat_search_messages returned no tool error: %s", textOf(result))
	}
	if text := textOf(result); !strings.Contains(text, "not a valid Rocket.Chat username") {
		t.Errorf("content = %q, want a username error", text)
	}
}

func TestSearchMessagesRejectsDuplicateFromFilter(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_search_messages", map[string]any{
		"room_id": "C1",
		"query":   "from:bob deploy",
		"author":  "carol",
	})
	if !result.IsError {
		t.Fatalf("rocketchat_search_messages returned no tool error: %s", textOf(result))
	}
	if text := textOf(result); !strings.Contains(text, "not both") {
		t.Errorf("content = %q, want a duplicate from: error", text)
	}
}

func TestSearchMessagesRejectsNegativeOffset(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_search_messages", map[string]any{
		"room_id": "C1",
		"query":   "attachment",
		"offset":  -1,
	})
	if !result.IsError {
		t.Fatalf("rocketchat_search_messages returned no tool error: %s", textOf(result))
	}
	if text := textOf(result); !strings.Contains(text, "offset must not be negative") {
		t.Errorf("content = %q, want an offset error", text)
	}
}

func TestSearchMessagesWithoutCredentials(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "", "")

	result := callTool(t, session, "rocketchat_search_messages", map[string]any{
		"room_id": "C1",
		"query":   "attachment",
	})
	if !result.IsError {
		t.Fatalf("rocketchat_search_messages returned no tool error: %s", textOf(result))
	}
	if text := textOf(result); !strings.Contains(text, "ROCKETCHAT_USER_ID") {
		t.Errorf("content = %q, want it to name the missing variables", text)
	}
}

func TestSearchMessagesRequestFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"success": false})
	}))
	t.Cleanup(server.Close)

	session := connect(t, server.URL, "user-1", "token-1")
	result := callTool(t, session, "rocketchat_search_messages", map[string]any{
		"room_id": "C1",
		"query":   "attachment",
	})
	if !result.IsError {
		t.Fatalf("rocketchat_search_messages returned no tool error: %s", textOf(result))
	}
	if text := textOf(result); !strings.Contains(text, "success=false") {
		t.Errorf("content = %q, want it to report the failed request", text)
	}
}
