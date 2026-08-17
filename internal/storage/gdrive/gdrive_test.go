package gdrive

import (
	"testing"

	"github.com/pecataToshev/dump-keep/internal/config"
)

func TestSplitPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"single segment", "folder", []string{"folder"}},
		{"nested", "a/b/c", []string{"a", "b", "c"}},
		{"leading slash", "/a/b", []string{"a", "b"}},
		{"trailing slash", "a/b/", []string{"a", "b"}},
		{"both slashes", "/a/b/", []string{"a", "b"}},
		{"empty", "", nil},
		{"just slashes", "///", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitPath(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("splitPath(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("splitPath(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestSplitKey(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantFolder string
		wantFile   string
	}{
		{"file only", "dump.age", "", "dump.age"},
		{"one folder", "folder/dump.age", "folder", "dump.age"},
		{"nested folders", "2026-08-17_daily_031745/mydb.dump.age", "2026-08-17_daily_031745", "mydb.dump.age"},
		{"deeply nested", "a/b/c/file.age", "a/b/c", "file.age"},
		{"leading slash", "/folder/file.age", "folder", "file.age"},
		{"trailing slash", "folder/file.age/", "folder", "file.age"},
		{"empty", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFolder, gotFile := splitKey(tt.input)
			if gotFolder != tt.wantFolder {
				t.Errorf("splitKey(%q) folder = %q, want %q", tt.input, gotFolder, tt.wantFolder)
			}
			if gotFile != tt.wantFile {
				t.Errorf("splitKey(%q) file = %q, want %q", tt.input, gotFile, tt.wantFile)
			}
		})
	}
}

func TestNew_MissingVars(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.Config
	}{
		{"empty config", config.Config{}},
		{"missing SA JSON", config.Config{GDriveSharedDriveID: "drive-123"}},
		{"missing shared drive ID", config.Config{GDriveSAJSON: `{"type":"service_account"}`}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := New(tt.cfg)
			if err == nil {
				t.Error("expected error for missing config, got nil")
			}
		})
	}
}
