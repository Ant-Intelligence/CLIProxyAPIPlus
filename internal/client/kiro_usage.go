package client

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"golang.org/x/term"
)

// KiroAccountUsage mirrors the server response structure.
type KiroAccountUsage struct {
	Name              string  `json:"name"`
	Email             string  `json:"email,omitempty"`
	TotalLimit        float64 `json:"total_limit"`
	CurrentUsage      float64 `json:"current_usage"`
	RemainingQuota    float64 `json:"remaining_quota"`
	UsagePercent      float64 `json:"usage_percent"`
	IsExhausted       bool    `json:"is_exhausted"`
	SubscriptionTitle string  `json:"subscription_title,omitempty"`
	SubscriptionType  string  `json:"subscription_type,omitempty"`
	DaysUntilReset    *int    `json:"days_until_reset,omitempty"`
	NextReset         string  `json:"next_reset,omitempty"`
	Error             string  `json:"error,omitempty"`
}

type kiroUsageResponse struct {
	Accounts []KiroAccountUsage `json:"accounts"`
	Error    string             `json:"error,omitempty"`
}

// ANSI color helpers
var colorEnabled = term.IsTerminal(int(os.Stdout.Fd()))

func green(s string) string {
	if !colorEnabled {
		return s
	}
	return "\033[32m" + s + "\033[0m"
}

func yellow(s string) string {
	if !colorEnabled {
		return s
	}
	return "\033[33m" + s + "\033[0m"
}

func red(s string) string {
	if !colorEnabled {
		return s
	}
	return "\033[31m" + s + "\033[0m"
}

func bold(s string) string {
	if !colorEnabled {
		return s
	}
	return "\033[1m" + s + "\033[0m"
}

// FetchKiroUsage calls the management API and returns account usage data.
func FetchKiroUsage(server, apiKey string) ([]KiroAccountUsage, error) {
	url := strings.TrimRight(server, "/") + "/v0/management/kiro-usage"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
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

	var result kiroUsageResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing response: %w", err)
	}
	if result.Error != "" {
		return nil, fmt.Errorf("server error: %s", result.Error)
	}

	return result.Accounts, nil
}

// PrintKiroUsageJSON prints accounts as JSON to stdout.
func PrintKiroUsageJSON(accounts []KiroAccountUsage) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(map[string]interface{}{"accounts": accounts})
}

// PrintKiroUsageTable prints a formatted table of account usage.
func PrintKiroUsageTable(accounts []KiroAccountUsage) {
	fmt.Println(bold("Kiro Account Credit Usage"))
	fmt.Println(strings.Repeat("=", 25))
	fmt.Println()

	if len(accounts) == 0 {
		fmt.Println("No Kiro accounts found.")
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Account\tSubscription\tUsed / Total\tRemaining\tUsage%\tReset In\tStatus")
	fmt.Fprintln(w, strings.Repeat("\u2500", 14)+"\t"+
		strings.Repeat("\u2500", 14)+"\t"+
		strings.Repeat("\u2500", 18)+"\t"+
		strings.Repeat("\u2500", 11)+"\t"+
		strings.Repeat("\u2500", 7)+"\t"+
		strings.Repeat("\u2500", 10)+"\t"+
		strings.Repeat("\u2500", 10))

	var healthy, exhausted, low int

	for _, a := range accounts {
		if a.Error != "" {
			name := displayName(a)
			fmt.Fprintf(w, "%s\t\t\t\t\t\t%s\n", name, red("ERR: "+a.Error))
			continue
		}

		name := displayName(a)
		sub := a.SubscriptionTitle
		if sub == "" {
			sub = a.SubscriptionType
		}

		used := formatNumber(a.CurrentUsage)
		total := formatNumber(a.TotalLimit)
		usedTotal := used + " / " + total
		remaining := formatNumber(a.RemainingQuota)
		pct := fmt.Sprintf("%.2f%%", a.UsagePercent)

		resetIn := "-"
		if a.DaysUntilReset != nil {
			d := *a.DaysUntilReset
			if d == 1 {
				resetIn = "1 day"
			} else {
				resetIn = fmt.Sprintf("%d days", d)
			}
		}

		var status string
		switch {
		case a.IsExhausted:
			status = red("EXHAUSTED")
			exhausted++
		case a.UsagePercent >= 80:
			status = yellow("LOW")
			low++
		default:
			status = green("OK")
			healthy++
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			name, sub, usedTotal, remaining, pct, resetIn, status)
	}
	w.Flush()

	fmt.Printf("\nTotal: %d accounts | %d healthy, %d exhausted, %d low\n",
		len(accounts), healthy, exhausted, low)
}

func displayName(a KiroAccountUsage) string {
	if a.Email != "" {
		return a.Email
	}
	return a.Name
}

func formatNumber(f float64) string {
	if f == math.Trunc(f) {
		return formatIntCommas(int64(f))
	}
	// Format with 2 decimal places then add commas to the integer part.
	s := fmt.Sprintf("%.2f", f)
	parts := strings.SplitN(s, ".", 2)
	intPart := parts[0]
	// Parse integer part for comma formatting.
	neg := ""
	if strings.HasPrefix(intPart, "-") {
		neg = "-"
		intPart = intPart[1:]
	}
	return neg + addCommas(intPart) + "." + parts[1]
}

func formatIntCommas(n int64) string {
	neg := ""
	if n < 0 {
		neg = "-"
		n = -n
	}
	return neg + addCommas(fmt.Sprintf("%d", n))
}

func addCommas(s string) string {
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	start := len(s) % 3
	if start > 0 {
		b.WriteString(s[:start])
	}
	for i := start; i < len(s); i += 3 {
		if b.Len() > 0 {
			b.WriteByte(',')
		}
		b.WriteString(s[i : i+3])
	}
	return b.String()
}
