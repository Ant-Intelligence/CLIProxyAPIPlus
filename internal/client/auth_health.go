package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

// AuthFileEntry represents a single credential entry from the management API.
type AuthFileEntry struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Provider      string  `json:"provider"`
	Email         string  `json:"email,omitempty"`
	Status        string  `json:"status"`
	StatusMessage string  `json:"status_message,omitempty"`
	Disabled      bool    `json:"disabled"`
	Unavailable   bool    `json:"unavailable"`
	LastRefresh   *string `json:"last_refresh,omitempty"`
	UpdatedAt     *string `json:"updated_at,omitempty"`
}

type authFilesResponse struct {
	Files []AuthFileEntry `json:"files"`
	Error string          `json:"error,omitempty"`
}

// FetchAuthFiles calls GET /v0/management/auth-files and returns all entries.
func FetchAuthFiles(server, apiKey string) ([]AuthFileEntry, error) {
	url := strings.TrimRight(server, "/") + "/v0/management/auth-files"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	c := &http.Client{Timeout: 30 * time.Second}
	resp, err := c.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var result authFilesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("server error: %s", result.Error)
	}

	return result.Files, nil
}

// FilterByProvider returns entries whose provider starts with the given name (case-insensitive).
// This allows "gemini" to match "gemini", "gemini-cli", "gemini-api-key", etc.
// If provider is empty, returns all entries.
func FilterByProvider(entries []AuthFileEntry, provider string) []AuthFileEntry {
	if provider == "" {
		return entries
	}
	p := strings.ToLower(strings.TrimSpace(provider))
	var filtered []AuthFileEntry
	for _, e := range entries {
		ep := strings.ToLower(strings.TrimSpace(e.Provider))
		if ep == p || strings.HasPrefix(ep, p+"-") || strings.HasPrefix(ep, p+"_") {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// PrintAuthHealthJSON prints entries as JSON to stdout.
func PrintAuthHealthJSON(entries []AuthFileEntry) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]interface{}{"accounts": entries})
}

// PrintAuthHealthTable prints a formatted table of account health.
func PrintAuthHealthTable(entries []AuthFileEntry, provider string) {
	title := "Auth Account Health"
	if provider != "" {
		title += " (" + provider + ")"
	}
	fmt.Println(bold(title))
	fmt.Println(strings.Repeat("=", len(title)))
	fmt.Println()

	if len(entries) == 0 {
		fmt.Println("No accounts found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Account\tProvider\tStatus\tMessage\tLast Refresh")
	fmt.Fprintln(w,
		strings.Repeat("\u2500", 30)+"\t"+
			strings.Repeat("\u2500", 14)+"\t"+
			strings.Repeat("\u2500", 10)+"\t"+
			strings.Repeat("\u2500", 24)+"\t"+
			strings.Repeat("\u2500", 20))

	var healthy, errored, disabled int

	for _, e := range entries {
		name := e.Email
		if name == "" {
			name = e.Name
		}

		status, msg := resolveHealthStatus(e)

		var statusColored string
		switch status {
		case "active":
			statusColored = green("ACTIVE")
			healthy++
		case "disabled":
			statusColored = yellow("DISABLED")
			disabled++
		case "error":
			statusColored = red("ERROR")
			errored++
		case "unavailable":
			statusColored = red("UNAVAIL")
			errored++
		default:
			statusColored = yellow(strings.ToUpper(status))
			errored++
		}

		lastRefresh := "-"
		if e.LastRefresh != nil && *e.LastRefresh != "" {
			lastRefresh = formatTimeAgo(*e.LastRefresh)
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			name, e.Provider, statusColored, msg, lastRefresh)
	}
	w.Flush()

	fmt.Printf("\nTotal: %d accounts | %s, %s, %s\n",
		len(entries),
		green(fmt.Sprintf("%d healthy", healthy)),
		red(fmt.Sprintf("%d error", errored)),
		yellow(fmt.Sprintf("%d disabled", disabled)))
}

func resolveHealthStatus(e AuthFileEntry) (status, msg string) {
	if e.Disabled {
		return "disabled", e.StatusMessage
	}
	if e.Unavailable {
		return "unavailable", e.StatusMessage
	}
	s := strings.ToLower(strings.TrimSpace(e.Status))
	if s == "" {
		s = "unknown"
	}
	return s, e.StatusMessage
}

func formatTimeAgo(raw string) string {
	// Try common time formats
	for _, layout := range []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
	} {
		if t, err := time.Parse(layout, raw); err == nil {
			ago := time.Since(t)
			switch {
			case ago < time.Minute:
				return "just now"
			case ago < time.Hour:
				return fmt.Sprintf("%dm ago", int(ago.Minutes()))
			case ago < 24*time.Hour:
				return fmt.Sprintf("%dh ago", int(ago.Hours()))
			default:
				return fmt.Sprintf("%dd ago", int(ago.Hours()/24))
			}
		}
	}
	return raw
}
