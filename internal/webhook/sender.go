package webhook

import (
	"bytes"
	"net/http"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
)

// throttleKey uniquely identifies a (auth, event, webhook URL) combination.
type throttleKey struct {
	AuthID string
	Event  string
	URL    string
}

// Sender dispatches webhook notifications with per-key throttling.
type Sender struct {
	mu       sync.Mutex
	entries  []config.WebhookEntry
	lastSent map[throttleKey]time.Time
	client   *http.Client
}

// NewSender creates a Sender for the given webhook configurations.
func NewSender(entries []config.WebhookEntry) *Sender {
	return &Sender{
		entries:  entries,
		lastSent: make(map[throttleKey]time.Time),
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

// TrySend attempts to dispatch a webhook for the given event. It checks the
// throttle window and, if not throttled, sends asynchronously.
func (s *Sender) TrySend(event string, authID, provider, model string, httpStatus int, errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, entry := range s.entries {
		if !eventMatches(entry, event) {
			continue
		}
		key := throttleKey{AuthID: authID, Event: event, URL: entry.URL}
		throttle := resolvedThrottle(entry.ThrottleMinutes)
		if last, ok := s.lastSent[key]; ok && now.Sub(last) < throttle {
			continue
		}
		s.lastSent[key] = now

		typ := resolvedType(entry.Type)
		go s.send(typ, entry.URL, event, authID, provider, model, httpStatus, errMsg, now)
	}
}

// UpdateConfig replaces the webhook entries and prunes stale throttle keys.
func (s *Sender) UpdateConfig(entries []config.WebhookEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = entries

	// Build set of current URLs for pruning.
	urls := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		urls[e.URL] = struct{}{}
	}
	for key := range s.lastSent {
		if _, ok := urls[key.URL]; !ok {
			delete(s.lastSent, key)
		}
	}
}

func (s *Sender) send(typ, url, event, authID, provider, model string, httpStatus int, errMsg string, ts time.Time) {
	var body []byte
	var err error

	switch typ {
	case TypeWecom:
		body, err = formatWecomMessage(event, authID, provider, model, httpStatus, errMsg, ts)
	default:
		log.Warnf("webhook: unsupported type %q", typ)
		return
	}
	if err != nil {
		log.Warnf("webhook: failed to format %s message: %v", typ, err)
		return
	}

	resp, err := s.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Warnf("webhook: POST to %s failed: %v", url, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Warnf("webhook: POST to %s returned status %d", url, resp.StatusCode)
		return
	}
	log.Debugf("webhook: sent %s alert for %s to %s", event, authID, url)
}

// eventMatches checks if the webhook entry is subscribed to the given event.
func eventMatches(entry config.WebhookEntry, event string) bool {
	for _, e := range resolvedEvents(entry.Events) {
		if e == event {
			return true
		}
	}
	return false
}

// ThrottledAt returns the last-sent time for a throttle key. Exported for testing.
func (s *Sender) ThrottledAt(authID, event, url string) (time.Time, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := throttleKey{AuthID: authID, Event: event, URL: url}
	t, ok := s.lastSent[key]
	return t, ok
}

// SetNow is intentionally NOT provided; tests should use short throttle windows.
// The sender uses real time.Now() to keep the production path simple.
