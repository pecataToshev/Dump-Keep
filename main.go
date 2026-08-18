// dump-keep dumps every database on a PostgreSQL instance,
// encrypts the dumps with age (recipient/public key — the private key
// never leaves offline storage), uploads them to a storage backend
// (Google Drive or any S3-compatible storage) and prunes old backups.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/pecataToshev/dump-keep/internal/backup"
	"github.com/pecataToshev/dump-keep/internal/buildinfo"
	"github.com/pecataToshev/dump-keep/internal/config"
	"github.com/pecataToshev/dump-keep/internal/healthcheck"
	"github.com/pecataToshev/dump-keep/internal/notify"
	"github.com/pecataToshev/dump-keep/internal/storage"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				return slog.Attr{}
			}
			return a
		},
	})))

	slog.Info("dump-keep starting",
		"commit", buildinfo.Commit,
		"build_time", buildinfo.BuildTime,
		"source", buildinfo.Source)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration error", "error", err)
		os.Exit(1)
	}

	notifier := buildNotifier(cfg)
	slog.Info("configuration loaded",
		"storage_backend", cfg.StorageBackend,
		"retention", cfg.Retention,
		"notify_tiers", cfg.NotifyTiers,
		"notifiers", notifier.Types(),
		"healthcheck", cfg.HealthcheckURL != "",
		"skip_list", cfg.SkipList)

	pinger := healthcheck.New(cfg.HealthcheckURL)
	pinger.Start()

	store, err := storage.New(cfg)
	if err != nil {
		slog.Error("storage init error", "error", err)
		send(notifier, fmt.Sprintf("❌ **dump-keep failed to init storage**: %v", err))
		pinger.Fail()
		os.Exit(1)
	}

	sum, err := backup.Run(context.Background(), cfg, store)
	if err != nil {
		slog.Error("backup failed", "error", err)
		send(notifier, fmt.Sprintf("❌ **dump-keep backup failed**: %v", err))
		pinger.Fail()
		os.Exit(1)
	}

	// Success notification on configured tiers (default: weekly, monthly).
	if slices.Contains(cfg.NotifyTiers, sum.Tier) {
		msg := fmt.Sprintf("✅ **dump-keep backup `%s` done** — %d databases: %s",
			sum.Folder, len(sum.Databases), strings.Join(sum.Databases, ", "))
		if len(sum.Skipped) > 0 {
			msg += fmt.Sprintf("\n⊘ skipped: %s", strings.Join(sum.Skipped, ", "))
		}
		send(notifier, msg)
	}

	pinger.Success()
	slog.Info("backup complete: " + sum.Folder)
}

// buildNotifier creates a composite notifier from all configured channels.
// Always returns FanOut; with no channels configured it's a no-op.
func buildNotifier(cfg config.Config) notify.FanOut {
	fanout := notify.NewFanOut()
	if cfg.DiscordWebhookURL != "" {
		fanout.Add(notify.NewDiscord(cfg.DiscordWebhookURL))
	}
	if cfg.SlackWebhookURL != "" {
		fanout.Add(notify.NewSlack(cfg.SlackWebhookURL))
	}
	if cfg.WebhookURL != "" {
		fanout.Add(notify.NewWebhook(cfg.WebhookURL))
	}
	return fanout
}

func send(n notify.FanOut, message string) {
	if err := n.Notify(message); err != nil {
		slog.Error("notification failed", "error", err)
	}
}
