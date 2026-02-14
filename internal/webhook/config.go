package webhook

import "time"

// Event types for webhook alerting.
const (
	EventAccountBanned = "account_banned"
	EventRateLimited   = "rate_limited"
)

// Webhook provider types.
const (
	TypeWecom = "wecom"
)

const (
	defaultThrottleMinutes = 10
)

// resolvedType returns the webhook type, defaulting to wecom.
func resolvedType(t string) string {
	if t == "" {
		return TypeWecom
	}
	return t
}

// resolvedEvents returns the event list, defaulting to account_banned only.
func resolvedEvents(events []string) []string {
	if len(events) == 0 {
		return []string{EventAccountBanned}
	}
	return events
}

// resolvedThrottle returns the throttle duration, defaulting to 10 minutes.
func resolvedThrottle(minutes int) time.Duration {
	if minutes <= 0 {
		return time.Duration(defaultThrottleMinutes) * time.Minute
	}
	return time.Duration(minutes) * time.Minute
}
