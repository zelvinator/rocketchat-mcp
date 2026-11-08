package mcpserver

import (
	"strings"
	"testing"
)

func TestListChannels(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_list_channels", map[string]any{
		"include_private": true,
	})
	if result.IsError {
		t.Fatalf("rocketchat_list_channels returned a tool error: %s", textOf(result))
	}

	var out listChannelsOutput
	decodeStructured(t, result, &out)
	if len(out.Channels) != 2 {
		t.Fatalf("channels = %d, want 2", len(out.Channels))
	}

	byName := make(map[string]channelInfo, len(out.Channels))
	for _, channel := range out.Channels {
		byName[channel.Name] = channel
	}

	general, ok := byName["general"]
	if !ok {
		t.Fatal("general channel is missing")
	}
	if general.ID != "C1" || general.Type != "channel" || general.Messages != 2 || general.Users != 3 {
		t.Errorf("general = %+v", general)
	}
	if general.CreatedAt != "2026-10-01T10:00:00Z" || general.UpdatedAt != "2026-11-08T15:00:00Z" {
		t.Errorf("general times = created %q, updated %q", general.CreatedAt, general.UpdatedAt)
	}
	if general.LastMessageAt != "2026-11-08T15:00:00Z" {
		t.Errorf("general last message = %q", general.LastMessageAt)
	}

	alerts, ok := byName["alerts"]
	if !ok {
		t.Fatal("alerts group is missing")
	}
	if alerts.ID != "G1" || alerts.Type != "group" || alerts.Messages != 1 || alerts.Users != 2 {
		t.Errorf("alerts = %+v", alerts)
	}
	if alerts.CreatedAt != "2026-10-02T10:00:00Z" || alerts.UpdatedAt != "2026-11-08T12:00:00Z" {
		t.Errorf("alerts times = created %q, updated %q", alerts.CreatedAt, alerts.UpdatedAt)
	}
	if alerts.LastMessageAt != "2026-11-08T12:00:00Z" {
		t.Errorf("alerts last message = %q", alerts.LastMessageAt)
	}

	text := textOf(result)
	for _, roomID := range []string{"id C1", "id G1"} {
		if !strings.Contains(text, roomID) {
			t.Errorf("content = %q, want it to contain %q", text, roomID)
		}
	}
}

func TestListChannelsPublicOnly(t *testing.T) {
	session := connect(t, newFakeRocketChat(t, false), "user-1", "token-1")

	result := callTool(t, session, "rocketchat_list_channels", nil)
	if result.IsError {
		t.Fatalf("rocketchat_list_channels returned a tool error: %s", textOf(result))
	}

	var out listChannelsOutput
	decodeStructured(t, result, &out)
	if len(out.Channels) != 1 || out.Channels[0].Name != "general" {
		t.Fatalf("channels = %+v, want only general", out.Channels)
	}
}

func TestClampLimit(t *testing.T) {
	tests := []struct {
		value    int
		fallback int
		max      int
		want     int
	}{
		{value: 0, fallback: 20, max: 100, want: 20},
		{value: -1, fallback: 20, max: 100, want: 20},
		{value: 5, fallback: 20, max: 100, want: 5},
		{value: 1000, fallback: 20, max: 100, want: 100},
	}
	for _, test := range tests {
		if got := clampLimit(test.value, test.fallback, test.max); got != test.want {
			t.Errorf("clampLimit(%d, %d, %d) = %d, want %d", test.value, test.fallback, test.max, got, test.want)
		}
	}
}
