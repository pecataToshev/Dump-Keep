package backup

import (
	"fmt"
	"strings"
	"time"

	"github.com/pecataToshev/dump-keep/internal/config"
)

// Folder represents a structured backup folder.
type Folder struct {
	Date time.Time
	Tier string
	Time string
}

// Format returns the folder name string representation in the format YYYY-MM-DD_tier_HHMMSS.
func (f Folder) Format() string {
	return fmt.Sprintf("%s_%s_%s", f.Date.Format("2006-01-02"), f.Tier, f.Time)
}

// ParseFolder parses a folder name string in the format YYYY-MM-DD_tier_HHMMSS.
func ParseFolder(name string) (Folder, error) {
	parts := strings.Split(name, "_")
	if len(parts) < 2 {
		return Folder{}, fmt.Errorf("invalid backup folder name: %s", name)
	}
	date, err := time.Parse("2006-01-02", parts[0])
	if err != nil {
		return Folder{}, fmt.Errorf("invalid backup folder date %q: %w", parts[0], err)
	}
	tier := parts[1]
	if !config.ValidTier(tier) {
		return Folder{}, fmt.Errorf("unknown backup tier: %s", tier)
	}
	var folderTime string
	if len(parts) > 2 {
		folderTime = parts[2]
	}
	return Folder{
		Date: date,
		Tier: tier,
		Time: folderTime,
	}, nil
}

// NewFolder creates a new Folder instance for a given time and tier.
func NewFolder(t time.Time, tier string) Folder {
	return Folder{
		Date: t,
		Tier: tier,
		Time: t.Format("150405"),
	}
}
