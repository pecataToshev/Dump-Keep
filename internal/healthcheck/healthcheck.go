// Package healthcheck pings a healthchecks.io-style endpoint to signal
// start, success and failure of a backup run. No-op when the URL is unset.
package healthcheck

import (
	"log/slog"
	"net/http"
	"time"
)

// Pinger sends start/success/fail signals to a healthchecks-style endpoint.
// A zero-value Pinger (empty URL) is a no-op.
type Pinger struct {
	url    string
	client *http.Client
}

// New returns a Pinger for the given base URL. Empty URL → no-op.
func New(url string) *Pinger {
	return &Pinger{
		url:    url,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

// Start signals the beginning of a run (healthchecks.io measures elapsed time).
func (p *Pinger) Start() {
	p.ping("/start")
}

// Success signals a successful run.
func (p *Pinger) Success() {
	p.ping("")
}

// Fail signals a failed run.
func (p *Pinger) Fail() {
	p.ping("/fail")
}

func (p *Pinger) ping(suffix string) {
	if p.url == "" {
		return
	}
	if _, err := p.client.Get(p.url + suffix); err != nil {
		slog.Error("healthcheck ping failed", "error", err)
	}
}
