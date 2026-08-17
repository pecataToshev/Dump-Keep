package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Slack posts messages to a Slack incoming webhook.
type Slack struct {
	webhookURL string
}

func NewSlack(webhookURL string) *Slack {
	return &Slack{webhookURL: webhookURL}
}

// Notify posts the message to the Slack channel. Content is redacted and
// truncated to Slack's text limit.
func (s *Slack) Notify(message string) error {
	message = Redact(message)
	if len(message) > 2900 {
		message = message[:2900] + "…"
	}

	body, err := json.Marshal(map[string]string{"text": message})
	if err != nil {
		return err
	}

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(s.webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("slack webhook returned %s", resp.Status)
	}
	return nil
}
