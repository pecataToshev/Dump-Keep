package notify

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "No URL",
			input: "The process succeeded.",
			want:  "The process succeeded.",
		},
		{
			name:  "URL with password",
			input: "failed to connect: postgres://user:my-super-secret-password123@localhost:5432/mydb",
			want:  "failed to connect: postgres://user:***@localhost:5432/mydb",
		},
		{
			name:  "URL with password containing special chars",
			input: "postgres://app_user:p%40ss%23word@db.host.internal/prod",
			want:  "postgres://app_user:***@db.host.internal/prod",
		},
		{
			name:  "URL without password",
			input: "postgres://user@localhost/db",
			want:  "postgres://user@localhost/db",
		},
		{
			name:  "No username/password in URL",
			input: "postgres://localhost/db",
			want:  "postgres://localhost/db",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Redact(tt.input)
			if got != tt.want {
				t.Errorf("Redact() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDiscord_Notify(t *testing.T) {
	t.Run("successful notification with redaction", func(t *testing.T) {
		var receivedBody map[string]string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				t.Errorf("expected POST, got %s", r.Method)
			}
			if r.Header.Get("Content-Type") != "application/json" {
				t.Errorf("expected application/json content-type, got %s", r.Header.Get("Content-Type"))
			}

			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("failed to read request body: %v", err)
			}

			if err := json.Unmarshal(bodyBytes, &receivedBody); err != nil {
				t.Fatalf("failed to unmarshal request body: %v", err)
			}

			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		discord := NewDiscord(server.URL)
		err := discord.Notify("Alert: postgres://user:secret123@host:5432/db")
		if err != nil {
			t.Fatalf("Notify() failed: %v", err)
		}

		wantContent := "Alert: postgres://user:***@host:5432/db"
		if receivedBody["content"] != wantContent {
			t.Errorf("Notify() sent %q, want %q", receivedBody["content"], wantContent)
		}
	})

	t.Run("truncation of long messages", func(t *testing.T) {
		var receivedBody map[string]string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bodyBytes, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(bodyBytes, &receivedBody)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		discord := NewDiscord(server.URL)
		longMessage := strings.Repeat("A", 2000)
		err := discord.Notify(longMessage)
		if err != nil {
			t.Fatalf("Notify() failed: %v", err)
		}

		content := receivedBody["content"]
		if len(content) != 1903 { // 1900 ASCII bytes + 3-byte UTF-8 horizontal ellipsis ("…")
			t.Errorf("expected byte length 1903, got %d", len(content))
		}
		if !strings.HasSuffix(content, "…") {
			t.Errorf("expected content to end with ellipsis, got %q", content)
		}
	})

	t.Run("http error handler", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		discord := NewDiscord(server.URL)
		err := discord.Notify("hello")
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "discord webhook returned 400") {
			t.Errorf("unexpected error message: %v", err)
		}
	})
}
