package mcpserver

import (
	"strings"
	"testing"
)

func TestListMessagesChannelFirstPage(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_list_messages", map[string]any{
		"room_id": "C1",
		"limit":   3,
	})
	if result.IsError {
		t.Fatalf("rocketchat_list_messages returned a tool error: %s", textOf(result))
	}

	var out listMessagesOutput
	decodeStructured(t, result, &out)
	if len(out.Messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(out.Messages))
	}
	if out.Messages[0].ID != "M5" || out.Messages[1].ID != "M4" || out.Messages[2].ID != "M3" {
		t.Errorf("message order = %s, %s, %s", out.Messages[0].ID, out.Messages[1].ID, out.Messages[2].ID)
	}
	if !out.HasMore {
		t.Error("HasMore = false, want true")
	}
	if out.NextBefore != "2026-11-08T13:00:00Z" {
		t.Errorf("NextBefore = %q, want %q", out.NextBefore, "2026-11-08T13:00:00Z")
	}

	attachmentMessage := out.Messages[2]
	if len(attachmentMessage.Attachments) != 1 || attachmentMessage.Attachments[0].Title != "Loki error logs" {
		t.Errorf("attachments = %+v", attachmentMessage.Attachments)
	}
	if len(attachmentMessage.Attachments) == 1 && len(attachmentMessage.Attachments[0].Fields) != 1 {
		t.Errorf("attachment fields = %+v", attachmentMessage.Attachments[0].Fields)
	}

	if text := textOf(result); !strings.Contains(text, "Next page: use before=2026-11-08T13:00:00Z") {
		t.Errorf("content = %q, want a next-page hint", text)
	}
}

func TestListMessagesChannelNextPage(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	first := callTool(t, session, "rocketchat_list_messages", map[string]any{
		"room_id": "C1",
		"limit":   3,
	})
	var firstPage listMessagesOutput
	decodeStructured(t, first, &firstPage)
	if firstPage.NextBefore == "" {
		t.Fatal("first page has no NextBefore")
	}

	second := callTool(t, session, "rocketchat_list_messages", map[string]any{
		"room_id": "C1",
		"limit":   3,
		"before":  firstPage.NextBefore,
	})
	if second.IsError {
		t.Fatalf("second page returned a tool error: %s", textOf(second))
	}

	var secondPage listMessagesOutput
	decodeStructured(t, second, &secondPage)
	if len(secondPage.Messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(secondPage.Messages))
	}
	if secondPage.Messages[0].ID != "M2" || secondPage.Messages[1].ID != "M1" {
		t.Errorf("message order = %s, %s", secondPage.Messages[0].ID, secondPage.Messages[1].ID)
	}
	if secondPage.HasMore {
		t.Error("HasMore = true, want false")
	}
	if secondPage.NextBefore != "" {
		t.Errorf("NextBefore = %q, want empty", secondPage.NextBefore)
	}
}

func TestListMessagesAfterBound(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_list_messages", map[string]any{
		"room_id": "C1",
		"limit":   10,
		"after":   "2026-11-08T12:00:00Z",
	})
	if result.IsError {
		t.Fatalf("rocketchat_list_messages returned a tool error: %s", textOf(result))
	}

	var out listMessagesOutput
	decodeStructured(t, result, &out)
	if len(out.Messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(out.Messages))
	}
	for _, message := range out.Messages {
		if message.ID == "M2" {
			t.Error("M2 must be excluded by the after bound")
		}
	}
}

func TestListMessagesGroup(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_list_messages", map[string]any{
		"room_id": "G1",
	})
	if result.IsError {
		t.Fatalf("rocketchat_list_messages returned a tool error: %s", textOf(result))
	}

	var out listMessagesOutput
	decodeStructured(t, result, &out)
	if len(out.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(out.Messages))
	}
	if out.Messages[0].Author != "alice" || out.Messages[0].Text != "group message" {
		t.Errorf("message = %+v", out.Messages[0])
	}
}

func TestListMessagesDirectMessage(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_list_messages", map[string]any{
		"room_id": "D1",
	})
	if result.IsError {
		t.Fatalf("rocketchat_list_messages returned a tool error: %s", textOf(result))
	}

	var out listMessagesOutput
	decodeStructured(t, result, &out)
	if len(out.Messages) != 1 {
		t.Fatalf("messages = %d, want 1", len(out.Messages))
	}
	if out.Messages[0].Author != "bob" || out.Messages[0].Text != "direct message" {
		t.Errorf("message = %+v", out.Messages[0])
	}
}

func TestListMessagesRejectsUnsupportedRoom(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_list_messages", map[string]any{
		"room_id": "L1",
	})
	if !result.IsError {
		t.Fatalf("rocketchat_list_messages returned no tool error: %s", textOf(result))
	}
	if text := textOf(result); !strings.Contains(text, "only channels, private groups, and direct messages") {
		t.Errorf("content = %q, want an unsupported room error", text)
	}
}

func TestListMessagesRequiresRoomID(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_list_messages", nil)
	if !result.IsError {
		t.Fatalf("rocketchat_list_messages returned no tool error: %s", textOf(result))
	}
}

func TestListMessagesRejectsInvalidTime(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_list_messages", map[string]any{
		"room_id": "C1",
		"before":  "yesterday",
	})
	if !result.IsError {
		t.Fatalf("rocketchat_list_messages returned no tool error: %s", textOf(result))
	}
	if text := textOf(result); !strings.Contains(text, "invalid time") {
		t.Errorf("content = %q, want an invalid time error", text)
	}
}

func TestListMessagesRejectsInvalidRange(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_list_messages", map[string]any{
		"room_id": "C1",
		"after":   "2026-11-08T15:00:00Z",
		"before":  "2026-11-08T14:00:00Z",
	})
	if !result.IsError {
		t.Fatalf("rocketchat_list_messages returned no tool error: %s", textOf(result))
	}
	if text := textOf(result); !strings.Contains(text, "after must be earlier than before") {
		t.Errorf("content = %q, want a range error", text)
	}
}
