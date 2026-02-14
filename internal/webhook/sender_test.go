package webhook

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

func TestClassifyEvent_AccountBanned(t *testing.T) {
	tests := []struct {
		msg  string
		want string
	}{
		{"This service has been disabled in this account for violation of Terms of Service", EventAccountBanned},
		{"Account suspended due to policy violation", EventAccountBanned},
		{"Your account has been banned", EventAccountBanned},
		{"Account terminated for abuse", EventAccountBanned},
	}
	for _, tt := range tests {
		result := coreauth.Result{
			AuthID:   "test.json",
			Provider: "antigravity",
			Model:    "gemini-2.5-pro",
			Success:  false,
			Error:    &coreauth.Error{HTTPStatus: 403, Message: tt.msg},
		}
		got := classifyEvent(result)
		if got != tt.want {
			t.Errorf("classifyEvent(%q) = %q, want %q", tt.msg, got, tt.want)
		}
	}
}

func TestClassifyEvent_RateLimited(t *testing.T) {
	result := coreauth.Result{
		AuthID:   "test.json",
		Provider: "gemini-cli",
		Model:    "gemini-2.5-pro",
		Success:  false,
		Error:    &coreauth.Error{HTTPStatus: 429, Message: "quota exceeded"},
	}
	got := classifyEvent(result)
	if got != EventRateLimited {
		t.Errorf("classifyEvent(429) = %q, want %q", got, EventRateLimited)
	}
}

func TestClassifyEvent_Generic403NotAlerted(t *testing.T) {
	result := coreauth.Result{
		AuthID:   "test.json",
		Provider: "claude",
		Model:    "claude-sonnet-4",
		Success:  false,
		Error:    &coreauth.Error{HTTPStatus: 403, Message: "Forbidden"},
	}
	got := classifyEvent(result)
	if got != "" {
		t.Errorf("classifyEvent(generic 403) = %q, want empty", got)
	}
}

func TestClassifyEvent_SuccessIgnored(t *testing.T) {
	result := coreauth.Result{
		AuthID:   "test.json",
		Provider: "claude",
		Model:    "claude-sonnet-4",
		Success:  true,
	}
	got := classifyEvent(result)
	if got != "" {
		t.Errorf("classifyEvent(success) = %q, want empty", got)
	}
}

func TestThrottle_SuppressesDuplicate(t *testing.T) {
	entries := []config.WebhookEntry{
		{
			URL:             "https://example.com/webhook",
			Type:            "wecom",
			Events:          []string{EventAccountBanned},
			ThrottleMinutes: 60,
		},
	}
	sender := NewSender(entries)

	sender.TrySend(EventAccountBanned, "user@gmail.com.json", "antigravity", "gemini-2.5-pro", 403, "banned")
	if _, ok := sender.ThrottledAt("user@gmail.com.json", EventAccountBanned, "https://example.com/webhook"); !ok {
		t.Fatal("expected throttle entry after first TrySend")
	}

	first, _ := sender.ThrottledAt("user@gmail.com.json", EventAccountBanned, "https://example.com/webhook")
	sender.TrySend(EventAccountBanned, "user@gmail.com.json", "antigravity", "gemini-2.5-pro", 403, "banned")
	second, _ := sender.ThrottledAt("user@gmail.com.json", EventAccountBanned, "https://example.com/webhook")
	if !first.Equal(second) {
		t.Fatalf("throttle timestamp changed: first=%v second=%v", first, second)
	}
}

func TestFormatWecomMessage_ValidJSON(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2026-02-14T10:30:00+08:00")
	data, err := formatWecomMessage(
		EventAccountBanned,
		"/home/user/.cli-proxy-api/user@gmail.com.json",
		"antigravity",
		"gemini-2.5-pro",
		403,
		"This service has been disabled",
		ts,
	)
	if err != nil {
		t.Fatalf("formatWecomMessage failed: %v", err)
	}

	var payload wecomPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if payload.MsgType != "markdown" {
		t.Errorf("msgtype = %q, want markdown", payload.MsgType)
	}
	if !strings.Contains(payload.Markdown.Content, "Account Banned") {
		t.Error("content missing 'Account Banned'")
	}
	if !strings.Contains(payload.Markdown.Content, "user@gmail.com") {
		t.Error("content missing stripped account name")
	}
	if !strings.Contains(payload.Markdown.Content, "antigravity") {
		t.Error("content missing provider")
	}
	if !strings.Contains(payload.Markdown.Content, "gemini-2.5-pro") {
		t.Error("content missing model")
	}
}

func TestFormatWecomMessage_TruncatesLongError(t *testing.T) {
	longErr := strings.Repeat("x", 300)
	ts, _ := time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")
	data, err := formatWecomMessage(EventRateLimited, "test.json", "gemini-cli", "gemini-2.5-pro", 429, longErr, ts)
	if err != nil {
		t.Fatalf("formatWecomMessage failed: %v", err)
	}
	var payload wecomPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if !strings.Contains(payload.Markdown.Content, "...") {
		t.Error("expected truncation marker '...' in content")
	}
}

func TestResolvedDefaults(t *testing.T) {
	if got := resolvedType(""); got != TypeWecom {
		t.Errorf("resolvedType('') = %q, want %q", got, TypeWecom)
	}
	if got := resolvedType("wecom"); got != "wecom" {
		t.Errorf("resolvedType('wecom') = %q, want wecom", got)
	}
	events := resolvedEvents(nil)
	if len(events) != 1 || events[0] != EventAccountBanned {
		t.Errorf("resolvedEvents(nil) = %v, want [account_banned]", events)
	}
	d := resolvedThrottle(0)
	if d != 10*time.Minute {
		t.Errorf("resolvedThrottle(0) = %v, want 10m", d)
	}
}
