package backup

import (
	"testing"
	"time"

	"github.com/pecataToshev/dump-keep/internal/config"
)

func TestFolder(t *testing.T) {
	t.Run("NewFolder and Format", func(t *testing.T) {
		date := time.Date(2026, 7, 20, 15, 30, 45, 0, time.UTC)
		folder := NewFolder(date, config.TierDaily)

		wantName := "2026-07-20_daily_153045"
		if got := folder.Format(); got != wantName {
			t.Errorf("folder.Format() = %q, want %q", got, wantName)
		}
	})

	t.Run("ParseFolder - valid", func(t *testing.T) {
		tests := []struct {
			name     string
			input    string
			wantDate time.Time
			wantTier string
			wantTime string
		}{
			{
				name:     "daily tier with time suffix",
				input:    "2026-07-20_daily_153045",
				wantDate: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
				wantTier: config.TierDaily,
				wantTime: "153045",
			},
			{
				name:     "weekly tier with time suffix",
				input:    "2026-07-20_weekly_112233",
				wantDate: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
				wantTier: config.TierWeekly,
				wantTime: "112233",
			},
			{
				name:     "monthly tier with time suffix",
				input:    "2026-07-20_monthly_235959",
				wantDate: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
				wantTier: config.TierMonthly,
				wantTime: "235959",
			},
			{
				name:     "missing time suffix (legacy/renamed folder)",
				input:    "2026-07-20_daily",
				wantDate: time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC),
				wantTier: config.TierDaily,
				wantTime: "",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				folder, err := ParseFolder(tt.input)
				if err != nil {
					t.Fatalf("ParseFolder(%q) failed: %v", tt.input, err)
				}
				if !folder.Date.Equal(tt.wantDate) {
					t.Errorf("folder.Date = %v, want %v", folder.Date, tt.wantDate)
				}
				if folder.Tier != tt.wantTier {
					t.Errorf("folder.Tier = %q, want %q", folder.Tier, tt.wantTier)
				}
				if folder.Time != tt.wantTime {
					t.Errorf("folder.Time = %q, want %q", folder.Time, tt.wantTime)
				}
			})
		}
	})

	t.Run("ParseFolder - invalid name", func(t *testing.T) {
		if _, err := ParseFolder("invalid-format"); err == nil {
			t.Error("expected error for invalid format")
		}
		if _, err := ParseFolder("2026-07-20_unknown-tier_153045"); err == nil {
			t.Error("expected error for unknown tier")
		}
		if _, err := ParseFolder("invalid-date_daily_153045"); err == nil {
			t.Error("expected error for invalid date")
		}
	})
}
