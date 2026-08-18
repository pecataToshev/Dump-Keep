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
	for _, tier := range cfg.NotifyTiers {
		if sum.Tier == tier {
			msg := fmt.Sprintf("✅ **dump-keep backup `%s` done** — %d databases: %s",
				sum.Folder, len(sum.Databases), strings.Join(sum.Databases, ", "))
			if len(sum.Skipped) > 0 {
				msg += fmt.Sprintf("\n⊘ skipped: %s", strings.Join(sum.Skipped, ", "))
			}
			send(notifier, msg)
			break
		}
	}

	pinger.Success()
	slog.Info("backup complete: " + sum.Folder)
}

// buildNotifier creates a composite notifier from all configured channels.
func buildNotifier(cfg config.Config) notify.Notifier {
	var notifiers []notify.Notifier
	if cfg.DiscordWebhookURL != "" {
		notifiers = append(notifiers, notify.NewDiscord(cfg.DiscordWebhookURL))
	}
	if cfg.SlackWebhookURL != "" {
		notifiers = append(notifiers, notify.NewSlack(cfg.SlackWebhookURL))
	}
	if cfg.WebhookURL != "" {
		notifiers = append(notifiers, notify.NewWebhook(cfg.WebhookURL))
	}
	if len(notifiers) == 0 {
		return notify.Noop{}
	}
	return notify.NewMulti(notifiers...)
}

func send(n notify.Notifier, message string) {
	if err := n.Notify(message); err != nil {
		slog.Error("notification failed", "error", err)
	}
}
