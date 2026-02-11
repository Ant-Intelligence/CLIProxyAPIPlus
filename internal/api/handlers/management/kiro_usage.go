package management

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	kiroauth "github.com/router-for-me/CLIProxyAPI/v6/internal/auth/kiro"
)

// kiroAccountUsage represents credit usage information for a single Kiro account.
type kiroAccountUsage struct {
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
	Breakdown         []kiroUsageBreakdownEntry `json:"breakdown,omitempty"`
	Error             string  `json:"error,omitempty"`
}

// kiroUsageBreakdownEntry represents one resource type's usage.
type kiroUsageBreakdownEntry struct {
	ResourceType string  `json:"resource_type"`
	DisplayName  string  `json:"display_name,omitempty"`
	TotalLimit   float64 `json:"total_limit"`
	CurrentUsage float64 `json:"current_usage"`
}

// GetKiroUsage queries subscription credit usage for all Kiro accounts.
func (h *Handler) GetKiroUsage(c *gin.Context) {
	if h == nil || h.authManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "auth manager unavailable"})
		return
	}

	auths := h.authManager.List()
	cwClient := kiroauth.NewCodeWhispererClient(h.cfg, "")

	var results []kiroAccountUsage
	for _, auth := range auths {
		if !strings.EqualFold(strings.TrimSpace(auth.Provider), "kiro") {
			continue
		}
		if auth.Disabled {
			continue
		}

		accessToken, _ := auth.Metadata["access_token"].(string)
		email, _ := auth.Metadata["email"].(string)

		name := strings.TrimSpace(auth.FileName)
		if name == "" {
			name = auth.ID
		}

		if accessToken == "" {
			results = append(results, kiroAccountUsage{
				Name:  name,
				Email: email,
				Error: "no access token available",
			})
			continue
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
		resp, err := cwClient.GetUsageLimits(ctx, accessToken)
		cancel()

		if err != nil {
			results = append(results, kiroAccountUsage{
				Name:  name,
				Email: email,
				Error: err.Error(),
			})
			continue
		}

		// Use email from API response if not in metadata
		if email == "" && resp.UserInfo != nil {
			email = resp.UserInfo.Email
		}

		entry := kiroAccountUsage{
			Name:           name,
			Email:          email,
			DaysUntilReset: resp.DaysUntilReset,
		}

		if resp.SubscriptionInfo != nil {
			entry.SubscriptionTitle = resp.SubscriptionInfo.SubscriptionTitle
			entry.SubscriptionType = resp.SubscriptionInfo.Type
		}

		if resp.NextDateReset != nil && *resp.NextDateReset > 0 {
			entry.NextReset = time.Unix(int64(*resp.NextDateReset/1000), 0).UTC().Format(time.RFC3339)
		}

		var totalLimit, totalUsage float64
		for _, b := range resp.UsageBreakdownList {
			var limit, usage float64
			if b.UsageLimitWithPrecision != nil {
				limit = *b.UsageLimitWithPrecision
			} else if b.UsageLimit != nil {
				limit = float64(*b.UsageLimit)
			}
			if b.CurrentUsageWithPrecision != nil {
				usage = *b.CurrentUsageWithPrecision
			} else if b.CurrentUsage != nil {
				usage = float64(*b.CurrentUsage)
			}
			totalLimit += limit
			totalUsage += usage

			entry.Breakdown = append(entry.Breakdown, kiroUsageBreakdownEntry{
				ResourceType: b.ResourceType,
				DisplayName:  b.DisplayName,
				TotalLimit:   limit,
				CurrentUsage: usage,
			})
		}

		entry.TotalLimit = totalLimit
		entry.CurrentUsage = totalUsage
		entry.RemainingQuota = totalLimit - totalUsage
		if entry.RemainingQuota < 0 {
			entry.RemainingQuota = 0
		}
		if totalLimit > 0 {
			entry.UsagePercent = (totalUsage / totalLimit) * 100
		}
		entry.IsExhausted = totalLimit > 0 && totalUsage >= totalLimit

		results = append(results, entry)
	}

	if results == nil {
		results = []kiroAccountUsage{}
	}

	c.JSON(http.StatusOK, gin.H{"accounts": results})
}
