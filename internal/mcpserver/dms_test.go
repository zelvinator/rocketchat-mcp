package mcpserver

import (
	"strings"
	"testing"
)

func TestListDMs(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_list_dms", nil)
	if result.IsError {
		t.Fatalf("rocketchat_list_dms returned a tool error: %s", textOf(result))
	}

	var out listDMsOutput
	decodeStructured(t, result, &out)
	if len(out.DMs) != 1 {
		t.Fatalf("dms = %d, want 1", len(out.DMs))
	}

	dm := out.DMs[0]
	if dm.ID != "D1" || dm.Name != "alice, bob" {
		t.Errorf("dm = %+v", dm)
	}
	if len(dm.Usernames) != 2 || dm.Usernames[0] != "alice" || dm.Usernames[1] != "bob" {
		t.Errorf("usernames = %+v", dm.Usernames)
	}
	if dm.Messages != 1 || dm.Users != 2 {
		t.Errorf("counts = messages %d, users %d", dm.Messages, dm.Users)
	}
	if dm.CreatedAt != "2026-10-03T10:00:00Z" || dm.UpdatedAt != "2026-11-08T16:00:00Z" {
		t.Errorf("times = created %q, updated %q", dm.CreatedAt, dm.UpdatedAt)
	}
	if dm.LastMessageAt != "2026-11-08T16:00:00Z" {
		t.Errorf("last message = %q", dm.LastMessageAt)
	}

	if text := textOf(result); !strings.Contains(text, "alice, bob") {
		t.Errorf("content = %q, want the participant names", text)
	}
	if text := textOf(result); !strings.Contains(text, "id D1") {
		t.Errorf("content = %q, want the room ID", text)
	}
}

func TestListDMsWithoutCredentials(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "", "")

	result := callTool(t, session, "rocketchat_list_dms", nil)
	if !result.IsError {
		t.Fatalf("rocketchat_list_dms returned no tool error: %s", textOf(result))
	}
	if text := textOf(result); !strings.Contains(text, "ROCKETCHAT_USER_ID") {
		t.Errorf("content = %q, want it to name the missing variables", text)
	}
}
