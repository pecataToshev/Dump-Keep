package notify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Discord posts messages to a Discord channel webhook.
type Discord struct {
	webhookURL string
}

func NewDiscord(webhookURL string) *Discord {
	return &Discord{webhookURL: webhookURL}
}

func (*Discord) Type() string { return "discord" }

// Notify posts the message to the channel. Content is redacted and
// truncated to Discord's message limit.
func (d *Discord) Notify(message string) error {
	message = Redact(message)
	if len(message) > 1900 {
		message = message[:1900] + "…"
	}

	body, err := json.Marshal(map[string]string{"content": message})
	if err != nil {
		return err
	}

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(d.webhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook returned %s", resp.Status)
	}
	return nil
}
