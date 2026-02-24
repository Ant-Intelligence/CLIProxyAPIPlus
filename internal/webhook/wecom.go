package webhook

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"
)

const maxErrorLen = 200

// wecomPayload is the JSON body sent to the WeChat Work webhook API.
type wecomPayload struct {
	MsgType  string         `json:"msgtype"`
	Markdown wecomMarkdown  `json:"markdown"`
}

type wecomMarkdown struct {
	Content string `json:"content"`
}

// formatWecomMessage builds a WeChat Work markdown webhook payload.
func formatWecomMessage(event string, authID, provider, model string, httpStatus int, errMsg string, ts time.Time) ([]byte, error) {
	var title string
	switch event {
	case EventAccountBanned:
		title = `<font color="warning">Account Banned</font>`
	case EventRateLimited:
		title = `<font color="comment">Rate Limited</font>`
	default:
		title = event
	}

	// Strip directory and .json suffix from auth ID for readability.
	account := filepath.Base(authID)
	account = strings.TrimSuffix(account, ".json")

	if len(errMsg) > maxErrorLen {
		errMsg = errMsg[:maxErrorLen] + "..."
	}

	content := fmt.Sprintf("## %s\n> Account: %s\n> Provider: %s\n> Model: %s\n> HTTP Status: %d\n> Error: %s\n> Time: %s",
		title,
		account,
		provider,
		model,
		httpStatus,
		errMsg,
		ts.Format("2006-01-02 15:04:05 MST"),
	)

	payload := wecomPayload{
		MsgType:  "markdown",
		Markdown: wecomMarkdown{Content: content},
	}
	return json.Marshal(payload)
}
