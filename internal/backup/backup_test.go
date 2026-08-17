package backup

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/pecataToshev/dump-keep/internal/config"
)

func TestTierFor(t *testing.T) {
	tests := []struct {
		name string
		date time.Time
		want string
	}{
		{
			name: "First of the month - Monthly",
			date: time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC), // Wednesday
			want: config.TierMonthly,
		},
		{
			name: "Sunday but not first of the month - Weekly",
			date: time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC), // Sunday
			want: config.TierWeekly,
		},
		{
			name: "Normal weekday - Daily",
			date: time.Date(2026, 7, 6, 12, 0, 0, 0, time.UTC), // Monday
			want: config.TierDaily,
		},
		{
			name: "First of the month on a Sunday - Monthly takes precedence",
			date: time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC), // Sunday
			want: config.TierMonthly,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tierFor(tt.date)
			if got != tt.want {
				t.Errorf("tierFor(%s) = %q, want %q", tt.date.Format("2006-01-02"), got, tt.want)
			}
		})
	}
}

func TestWithDBName(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		dbName  string
		want    string
		wantErr bool
	}{
		{
			name:   "standard connection string",
			rawURL: "postgres://user:pass@localhost:5432/postgres",
			dbName: "my_app_db",
			want:   "postgres://user:pass@localhost:5432/my_app_db",
		},
		{
			name:   "connection string with query parameters",
			rawURL: "postgres://user:pass@localhost:5432/postgres?sslmode=disable&timezone=UTC",
			dbName: "another_db",
			want:   "postgres://user:pass@localhost:5432/another_db?sslmode=disable&timezone=UTC",
		},
		{
			name:    "invalid URL",
			rawURL:  "://invalid-url",
			dbName:  "db",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := withDBName(tt.rawURL, tt.dbName)
			if (err != nil) != tt.wantErr {
				t.Fatalf("withDBName() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("withDBName() = %q, want %q", got, tt.want)
			}
		})
	}
}

type mockStorage struct {
	folders []string
	deleted []string
}

func (m *mockStorage) EnsureFolder(_ context.Context, _ string) error { return nil }
func (m *mockStorage) Put(_ context.Context, _ string, _ io.Reader) error {
	return nil
}
func (m *mockStorage) Delete(_ context.Context, key string) error {
	m.deleted = append(m.deleted, key)
	return nil
}
func (m *mockStorage) ListFolders(_ context.Context, _ string) ([]string, error) {
	return m.folders, nil
}
func (m *mockStorage) DeleteFolder(_ context.Context, path string) error {
	m.deleted = append(m.deleted, path)
	return nil
}

func TestPruneRunFolders(t *testing.T) {
	now := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	storage := &mockStorage{
		folders: []string{
			"2026-07-01_daily_120000", // Daily expired (retention is 7 days)
			"2026-07-18_daily_120000", // Daily retained
			"random-nonmatching",      // Ignored
		},
	}

	deleted, err := pruneRunFolders(context.Background(), storage, map[string]time.Duration{
		config.TierDaily:   7 * 24 * time.Hour,
		config.TierWeekly:  4 * 7 * 24 * time.Hour,
		config.TierMonthly: 24 * 31 * 24 * time.Hour,
	}, now)
	if err != nil {
		t.Fatalf("pruneRunFolders failed: %v", err)
	}

	if len(deleted) != 1 || deleted[0] != "2026-07-01_daily_120000" {
		t.Errorf("expected deleted list to be ['2026-07-01_daily_120000'], got %v", deleted)
	}

	if len(storage.deleted) != 1 || storage.deleted[0] != "2026-07-01_daily_120000" {
		t.Errorf("expected folder '2026-07-01_daily_120000' to be deleted, got %v", storage.deleted)
	}
}
