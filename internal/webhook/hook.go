package webhook

import (
	"context"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	coreauth "github.com/router-for-me/CLIProxyAPI/v6/sdk/cliproxy/auth"
)

// WebhookHook implements coreauth.Hook and dispatches webhook alerts
// for ban and rate-limiting events.
type WebhookHook struct {
	sender *Sender
}

// NewWebhookHook creates a hook backed by the given webhook configurations.
func NewWebhookHook(entries []config.WebhookEntry) *WebhookHook {
	return &WebhookHook{sender: NewSender(entries)}
}

// OnAuthRegistered is a no-op for webhook alerting.
func (h *WebhookHook) OnAuthRegistered(_ context.Context, _ *coreauth.Auth) {}

// OnAuthUpdated is a no-op for webhook alerting.
func (h *WebhookHook) OnAuthUpdated(_ context.Context, _ *coreauth.Auth) {}

// OnResult inspects the execution result and dispatches a webhook if
// the error indicates an account ban or rate-limiting event.
func (h *WebhookHook) OnResult(_ context.Context, result coreauth.Result) {
	event := classifyEvent(result)
	if event == "" {
		return
	}

	errMsg := ""
	httpStatus := 0
	if result.Error != nil {
		errMsg = result.Error.Message
		httpStatus = result.Error.HTTPStatus
	}
	h.sender.TrySend(event, result.AuthID, result.Provider, result.Model, httpStatus, errMsg)
}

// UpdateConfig replaces the webhook configuration for hot-reload.
func (h *WebhookHook) UpdateConfig(entries []config.WebhookEntry) {
	h.sender.UpdateConfig(entries)
}

// classifyEvent maps a Result to a webhook event constant.
// Returns "" if the result does not warrant an alert.
func classifyEvent(result coreauth.Result) string {
	if result.Success || result.Error == nil {
		return ""
	}
	status := result.Error.HTTPStatus

	if status == 403 && isAccountBanned(result.Error.Message) {
		return EventAccountBanned
	}
	if status == 429 {
		return EventRateLimited
	}
	return ""
}

// isAccountBanned mirrors the conductor's isAccountBannedError heuristic.
func isAccountBanned(msg string) bool {
	if msg == "" {
		return false
	}
	lower := strings.ToLower(msg)
	return strings.Contains(lower, "disabled") ||
		strings.Contains(lower, "violation") ||
		strings.Contains(lower, "suspended") ||
		strings.Contains(lower, "banned") ||
		strings.Contains(lower, "terminated")
}
