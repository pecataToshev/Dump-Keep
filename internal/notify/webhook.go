package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Webhook posts messages to a generic HTTP webhook endpoint.
// The message is sent as a JSON body: {"text": "<message>"}.
// This works with any service that accepts a simple JSON payload
// (custom endpoints, ntfy.sh, Gotify, etc.).
type Webhook struct {
	url string
}

func NewWebhook(url string) *Webhook {
	return &Webhook{url: url}
}

func (*Webhook) Type() string { return "webhook" }

// Notify posts the message to the webhook. Content is redacted and
// truncated to a reasonable limit.
func (w *Webhook) Notify(message string) error {
	message = Redact(message)
	if len(message) > 1900 {
		message = message[:1900] + "…"
	}

	body, err := json.Marshal(map[string]string{"text": message})
	if err != nil {
		return err
	}

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(w.url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned %s", resp.Status)
	}
	return nil
}
