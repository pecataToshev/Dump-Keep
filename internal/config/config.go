package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Backup tier constants.
const (
	TierDaily   = "daily"
	TierWeekly  = "weekly"
	TierMonthly = "monthly"
)

// validTiers is the set of recognised tier names, used for folder name
// validation. This is independent of retention configuration — tiers are
// always daily/weekly/monthly regardless of whether pruning is enabled.
var validTiers = map[string]bool{
	TierDaily:   true,
	TierWeekly:  true,
	TierMonthly: true,
}

// ValidTier returns true if the given tier name is a recognised backup tier.
func ValidTier(tier string) bool {
	return validTiers[tier]
}

// Storage backend constants.
const (
	BackendGDrive = "gdrive"
	BackendS3     = "s3"
)

// defaultRetention is used when RETENTION is unset.
const defaultRetention = "7d,4w,24m"

// Config holds all runtime settings, read from the environment.
type Config struct {
	PostgresURL    string
	AgeRecipient   string
	SkipList       []string
	SkipDatabases  string
	SkipFilePath   string
	StorageBackend string

	// Retention is the raw RETENTION env var value.
	// RetentionMap is the parsed result: nil when pruning is disabled
	// (RETENTION=none), otherwise maps tier → duration.
	Retention    string
	RetentionMap map[string]time.Duration

	// Google Drive backend
	GDriveSAJSON        string
	GDriveSharedDriveID string
	GDriveFolderID      string

	// S3 backend
	S3Endpoint  string
	S3Bucket    string
	S3Region    string
	S3AccessKey string
	S3SecretKey string
	S3PathStyle bool
	S3Prefix    string

	// Notifications
	HealthcheckURL    string
	DiscordWebhookURL string
	SlackWebhookURL   string
	WebhookURL        string
	NotifyTiers       []string
}

// Load reads all environment variables, loads the skip list (if configured),
// and returns a fully validated Config.
func Load() (Config, error) {
	cfg := Config{
		PostgresURL:    os.Getenv("POSTGRES_URL"),
		AgeRecipient:   os.Getenv("AGE_RECIPIENT"),
		SkipDatabases:  os.Getenv("SKIP_DATABASES"),
		SkipFilePath:   os.Getenv("SKIP_DATABASES_FILE_PATH"),
		StorageBackend: os.Getenv("STORAGE_BACKEND"),
		Retention:      os.Getenv("RETENTION"),

		GDriveSAJSON:        os.Getenv("GDRIVE_SA_JSON"),
		GDriveSharedDriveID: os.Getenv("GDRIVE_SHARED_DRIVE_ID"),
		GDriveFolderID:      os.Getenv("GDRIVE_FOLDER_ID"),

		S3Endpoint:  os.Getenv("S3_ENDPOINT"),
		S3Bucket:    os.Getenv("S3_BUCKET"),
		S3Region:    os.Getenv("S3_REGION"),
		S3AccessKey: os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey: os.Getenv("S3_SECRET_KEY"),
		S3PathStyle: envBool("S3_PATH_STYLE", false),
		S3Prefix:    strings.Trim(os.Getenv("S3_PREFIX"), "/"),

		HealthcheckURL:    os.Getenv("HEALTHCHECK_URL"),
		DiscordWebhookURL: os.Getenv("DISCORD_WEBHOOK_URL"),
		SlackWebhookURL:   os.Getenv("SLACK_WEBHOOK_URL"),
		WebhookURL:        os.Getenv("WEBHOOK_URL"),
		NotifyTiers:       parseCSV(os.Getenv("NOTIFY_TIERS")),
	}

	if cfg.GDriveFolderID == "" {
		cfg.GDriveFolderID = cfg.GDriveSharedDriveID
	}

	// Parse retention: "none" disables pruning, unset uses defaults,
	// otherwise parse positional "daily,weekly,monthly" durations.
	if cfg.Retention == "" {
		cfg.Retention = defaultRetention
	}
	if cfg.Retention != "none" {
		retentionMap, err := ParseRetention(cfg.Retention)
		if err != nil {
			return cfg, fmt.Errorf("parse RETENTION: %w", err)
		}
		cfg.RetentionMap = retentionMap
	}

	// Notify tiers: which backup tiers trigger a success notification.
	// Default: weekly,monthly. Set to "daily,weekly,monthly" to notify on every run.
	// Set to "none" to disable success notifications entirely.
	if len(cfg.NotifyTiers) == 0 {
		cfg.NotifyTiers = []string{TierWeekly, TierMonthly}
	} else if len(cfg.NotifyTiers) == 1 && cfg.NotifyTiers[0] == "none" {
		cfg.NotifyTiers = nil
	} else {
		for _, tier := range cfg.NotifyTiers {
			if !ValidTier(tier) {
				return cfg, fmt.Errorf("NOTIFY_TIERS: unknown tier %q (use daily, weekly, monthly, or none)", tier)
			}
		}
	}

	// Build skip list from SKIP_DATABASES env var and/or SKIP_DATABASES_FILE_PATH.
	// Both sources are merged; either or both may be unset (no skip list).
	if cfg.SkipDatabases != "" {
		cfg.SkipList = append(cfg.SkipList, parseCSV(cfg.SkipDatabases)...)
	}
	if cfg.SkipFilePath != "" {
		raw, err := os.ReadFile(cfg.SkipFilePath)
		if err != nil {
			return cfg, fmt.Errorf("read skip file %s: %w", cfg.SkipFilePath, err)
		}
		cfg.SkipList = append(cfg.SkipList, ParseSkipList(string(raw))...)
	}

	var missing []string
	for name, value := range map[string]string{
		"POSTGRES_URL":    cfg.PostgresURL,
		"AGE_RECIPIENT":   cfg.AgeRecipient,
		"STORAGE_BACKEND": cfg.StorageBackend,
	} {
		if value == "" {
			missing = append(missing, name)
		}
	}

	// Validate backend-specific required vars.
	switch cfg.StorageBackend {
	case BackendGDrive:
		for name, value := range map[string]string{
			"GDRIVE_SA_JSON":         cfg.GDriveSAJSON,
			"GDRIVE_SHARED_DRIVE_ID": cfg.GDriveSharedDriveID,
		} {
			if value == "" {
				missing = append(missing, name)
			}
		}
	case BackendS3:
		for name, value := range map[string]string{
			"S3_ENDPOINT":   cfg.S3Endpoint,
			"S3_BUCKET":     cfg.S3Bucket,
			"S3_ACCESS_KEY": cfg.S3AccessKey,
			"S3_SECRET_KEY": cfg.S3SecretKey,
		} {
			if value == "" {
				missing = append(missing, name)
			}
		}
	case "":
		missing = append(missing, "STORAGE_BACKEND (must be 'gdrive' or 's3')")
	default:
		return cfg, fmt.Errorf("unknown STORAGE_BACKEND %q: must be 'gdrive' or 's3'", cfg.StorageBackend)
	}

	if len(missing) > 0 {
		return cfg, fmt.Errorf("missing environment variables: %v", missing)
	}
	return cfg, nil
}

// ParseRetention parses a positional retention string like "7d,4w,24m" into
// a map of tier → duration. The order is fixed: daily, weekly, monthly.
// All three values must be provided.
func ParseRetention(s string) (map[string]time.Duration, error) {
	parts := parseCSV(s)
	if len(parts) != 3 {
		return nil, fmt.Errorf("expected 3 values (daily,weekly,monthly), got %d", len(parts))
	}
	tiers := []string{TierDaily, TierWeekly, TierMonthly}
	retention := make(map[string]time.Duration, 3)
	for i, tier := range tiers {
		d, err := ParseDuration(parts[i])
		if err != nil {
			return nil, fmt.Errorf("%s retention %q: %w", tier, parts[i], err)
		}
		retention[tier] = d
	}
	return retention, nil
}

// ParseDuration parses a human-friendly duration string with suffixes:
// h (hours), d (days), w (weeks), m (months, 31 days each).
func ParseDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty duration")
	}
	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	n, err := atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q: %w", numStr, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("duration must be positive, got %d", n)
	}
	var hours time.Duration
	switch unit {
	case 'h':
		hours = time.Duration(n)
	case 'd':
		hours = time.Duration(n) * 24
	case 'w':
		hours = time.Duration(n) * 24 * 7
	case 'm':
		hours = time.Duration(n) * 24 * 31
	default:
		return 0, fmt.Errorf("unknown unit %q (use h, d, w, or m)", string(unit))
	}
	return hours * time.Hour, nil
}

// ParseSkipList reads raw string lines and returns trimmed, non-comment,
// non-blank lines.
func ParseSkipList(raw string) []string {
	var list []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		list = append(list, line)
	}
	return list
}

// parseCSV splits a comma-separated string into trimmed, non-empty values.
func parseCSV(s string) []string {
	var list []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			list = append(list, part)
		}
	}
	return list
}

// atoi parses a positive integer string. Returns an error if the string
// is empty or contains non-digit characters.
func atoi(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty number")
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("non-digit character %q", string(c))
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

// envBool reads an env var as a boolean, returning the default if unset.
func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v == "true" || v == "1" || v == "yes"
}
