package config

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    time.Duration
		wantErr bool
	}{
		{"hours", "12h", 12 * time.Hour, false},
		{"days", "7d", 7 * 24 * time.Hour, false},
		{"weeks", "4w", 4 * 7 * 24 * time.Hour, false},
		{"months", "24m", 24 * 31 * 24 * time.Hour, false},
		{"single hour", "1h", 1 * time.Hour, false},
		{"empty", "", 0, true},
		{"missing number", "d", 0, true},
		{"unknown unit", "5x", 0, true},
		{"with spaces", "  7d  ", 7 * 24 * time.Hour, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDuration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseRetention(t *testing.T) {
	t.Run("valid default format", func(t *testing.T) {
		got, err := ParseRetention("7d,4w,24m")
		if err != nil {
			t.Fatalf("ParseRetention failed: %v", err)
		}
		if got[TierDaily] != 7*24*time.Hour {
			t.Errorf("daily = %v, want %v", got[TierDaily], 7*24*time.Hour)
		}
		if got[TierWeekly] != 4*7*24*time.Hour {
			t.Errorf("weekly = %v, want %v", got[TierWeekly], 4*7*24*time.Hour)
		}
		if got[TierMonthly] != 24*31*24*time.Hour {
			t.Errorf("monthly = %v, want %v", got[TierMonthly], 24*31*24*time.Hour)
		}
	})

	t.Run("custom values", func(t *testing.T) {
		got, err := ParseRetention("14d,8w,12m")
		if err != nil {
			t.Fatalf("ParseRetention failed: %v", err)
		}
		if got[TierDaily] != 14*24*time.Hour {
			t.Errorf("daily = %v, want %v", got[TierDaily], 14*24*time.Hour)
		}
		if got[TierWeekly] != 8*7*24*time.Hour {
			t.Errorf("weekly = %v, want %v", got[TierWeekly], 8*7*24*time.Hour)
		}
		if got[TierMonthly] != 12*31*24*time.Hour {
			t.Errorf("monthly = %v, want %v", got[TierMonthly], 12*31*24*time.Hour)
		}
	})

	t.Run("with spaces", func(t *testing.T) {
		got, err := ParseRetention("7d, 4w, 24m")
		if err != nil {
			t.Fatalf("ParseRetention failed: %v", err)
		}
		if got[TierDaily] != 7*24*time.Hour {
			t.Errorf("daily = %v, want %v", got[TierDaily], 7*24*time.Hour)
		}
	})

	t.Run("too few values", func(t *testing.T) {
		_, err := ParseRetention("7d,4w")
		if err == nil {
			t.Error("expected error for 2 values, got nil")
		}
	})

	t.Run("too many values", func(t *testing.T) {
		_, err := ParseRetention("7d,4w,24m,1h")
		if err == nil {
			t.Error("expected error for 4 values, got nil")
		}
	})

	t.Run("invalid duration", func(t *testing.T) {
		_, err := ParseRetention("7d,4w,bad")
		if err == nil {
			t.Error("expected error for invalid duration, got nil")
		}
	})
}

func TestValidTier(t *testing.T) {
	if !ValidTier(TierDaily) {
		t.Error("daily should be valid")
	}
	if !ValidTier(TierWeekly) {
		t.Error("weekly should be valid")
	}
	if !ValidTier(TierMonthly) {
		t.Error("monthly should be valid")
	}
	if ValidTier("hourly") {
		t.Error("hourly should not be valid")
	}
	if ValidTier("") {
		t.Error("empty string should not be valid")
	}
}
