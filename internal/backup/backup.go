// Package backup dumps all databases, encrypts them with age and uploads
// them to a storage backend with daily/weekly/monthly retention.
//
// Each run creates one folder named "<yyyy-MM-dd>_<tier>_<HHMMSS>" (tier:
// daily, weekly on Sundays, monthly on the 1st) inside the configured root.
// Retention is per tier, parsed from the folder name — no copies needed:
// a weekly folder simply lives longer than a daily one.
package backup

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"filippo.io/age"
	_ "github.com/lib/pq"
	"github.com/pecataToshev/dump-keep/internal/config"
	"github.com/pecataToshev/dump-keep/internal/storage"
)

// Summary describes what a successful run produced.
type Summary struct {
	Databases []string
	Skipped   []string
	Tier      string
	Folder    string
}

// tierFor returns the backup tier for a date: monthly on the 1st,
// weekly on Sundays, daily otherwise.
func tierFor(now time.Time) string {
	switch {
	case now.Day() == 1:
		return config.TierMonthly
	case now.Weekday() == time.Sunday:
		return config.TierWeekly
	default:
		return config.TierDaily
	}
}

// Run performs one full backup cycle: dump, encrypt, upload, prune.
func Run(ctx context.Context, cfg config.Config, store storage.Provider) (Summary, error) {
	var sum Summary

	recipient, err := age.ParseX25519Recipient(cfg.AgeRecipient)
	if err != nil {
		return sum, fmt.Errorf("parse AGE_RECIPIENT: %w", err)
	}

	dbs, err := listDatabases(cfg.PostgresURL)
	if err != nil {
		return sum, fmt.Errorf("list databases: %w", err)
	}

	skipSet := make(map[string]bool, len(cfg.SkipList))
	for _, name := range cfg.SkipList {
		skipSet[name] = true
	}

	var backed []string
	for _, db := range dbs {
		if skipSet[db] {
			sum.Skipped = append(sum.Skipped, db)
			continue
		}
		backed = append(backed, db)
	}
	sum.Databases = backed

	now := time.Now().UTC()
	stamp := now.Format("2006-01-02")
	sum.Tier = tierFor(now)
	folder := NewFolder(now, sum.Tier)
	sum.Folder = folder.Format()

	slog.Info("starting backup "+sum.Folder, "databases", backed)
	if len(sum.Skipped) > 0 {
		slog.Info("skipped databases (found on server, not backed up)", "skipped", sum.Skipped)
	}

	if err := store.EnsureFolder(ctx, sum.Folder); err != nil {
		return sum, fmt.Errorf("ensure run folder: %w", err)
	}

	// Globals: all roles + passwords.
	globalsName := stamp + "-globals.sql.age"
	globalsKey := sum.Folder + "/" + globalsName
	if err := dumpEncryptUpload(ctx, store, globalsKey, recipient,
		"pg_dumpall", "--globals-only", "-d", cfg.PostgresURL); err != nil {
		return sum, fmt.Errorf("backup globals: %w", err)
	}
	slog.Info("uploaded globals (roles)", "file", globalsName)

	for _, db := range backed {
		dbURL, err := withDBName(cfg.PostgresURL, db)
		if err != nil {
			return sum, err
		}
		name := fmt.Sprintf("%s-%s.dump.age", stamp, db)
		key := sum.Folder + "/" + name
		if err := dumpEncryptUpload(ctx, store, key, recipient,
			"pg_dump", "-Fc", "-d", dbURL); err != nil {
			return sum, fmt.Errorf("backup %s: %w", db, err)
		}
		slog.Info("uploaded database "+db, "file", name)
	}

	if cfg.RetentionMap != nil {
		deleted, err := pruneRunFolders(ctx, store, cfg.RetentionMap, now)
		if err != nil {
			return sum, fmt.Errorf("prune: %w", err)
		}
		if len(deleted) > 0 {
			slog.Info("pruned old backup folders", "count", len(deleted), "folders", deleted)
		}
	}

	return sum, nil
}

// pruneRunFolders queries the storage provider for all run folders and deletes
// the ones whose tier retention has expired based on the configured retention.
func pruneRunFolders(ctx context.Context, store storage.Provider, retention map[string]time.Duration, now time.Time) ([]string, error) {
	folders, err := store.ListFolders(ctx, "")
	if err != nil {
		return nil, err
	}

	var deleted []string
	for _, name := range folders {
		folder, err := ParseFolder(name)
		if err != nil {
			continue // Skip any folder that doesn't match our naming pattern
		}
		keep, ok := retention[folder.Tier]
		if !ok {
			continue // No retention configured for this tier — keep forever
		}
		if folder.Date.Before(now.Add(-keep)) {
			if err := store.DeleteFolder(ctx, name); err != nil {
				return deleted, fmt.Errorf("delete %s: %w", name, err)
			}
			deleted = append(deleted, name)
		}
	}
	return deleted, nil
}

// listDatabases returns every non-template database on the instance, so
// newly provisioned services are picked up automatically.
func listDatabases(superURL string) ([]string, error) {
	db, err := sql.Open("postgres", superURL)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.Query(
		`SELECT datname FROM pg_database WHERE NOT datistemplate ORDER BY datname`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func withDBName(rawURL, dbName string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("parse connection URL: %w", err)
	}
	u.Path = "/" + dbName
	return u.String(), nil
}
