package s3

import (
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/pecataToshev/dump-keep/internal/config"
)

func newTestClient(prefix string) *Client {
	return &Client{
		bucket: "test-bucket",
		prefix: prefix,
	}
}

func TestFullKey(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		key    string
		want   string
	}{
		{"no prefix", "", "folder/file.age", "folder/file.age"},
		{"with prefix", "backups/pg", "folder/file.age", "backups/pg/folder/file.age"},
		{"empty key no prefix", "", "", ""},
		{"empty key with prefix", "backups", "", "backups"},
		{"key with leading slash", "backups", "/folder/file.age", "backups/folder/file.age"},
		{"key with trailing slash", "backups", "folder/", "backups/folder"},
		{"prefix with trailing slash (trimmed by config)", "backups/pg", "run/dump.age", "backups/pg/run/dump.age"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(tt.prefix)
			got := c.fullKey(tt.key)
			if got != tt.want {
				t.Errorf("fullKey(%q) = %q, want %q", tt.key, got, tt.want)
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
		{"missing endpoint", config.Config{
			S3Bucket: "b", S3AccessKey: "a", S3SecretKey: "s",
		}},
		{"missing bucket", config.Config{
			S3Endpoint: "https://s3.amazonaws.com", S3AccessKey: "a", S3SecretKey: "s",
		}},
		{"missing access key", config.Config{
			S3Endpoint: "https://s3.amazonaws.com", S3Bucket: "b", S3SecretKey: "s",
		}},
		{"missing secret key", config.Config{
			S3Endpoint: "https://s3.amazonaws.com", S3Bucket: "b", S3AccessKey: "a",
		}},
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

func TestIsHTTPS(t *testing.T) {
	tests := []struct {
		endpoint string
		want     bool
	}{
		{"https://s3.amazonaws.com", true},
		{"http://localhost:9000", false},
		{"http://127.0.0.1:9000", false},
		{"s3.amazonaws.com", true}, // no scheme, not localhost → assume HTTPS
		{"localhost:9000", false},
	}

	for _, tt := range tests {
		t.Run(tt.endpoint, func(t *testing.T) {
			got := isHTTPS(tt.endpoint)
			if got != tt.want {
				t.Errorf("isHTTPS(%q) = %v, want %v", tt.endpoint, got, tt.want)
			}
		})
	}
}

func TestBucketLookupType(t *testing.T) {
	if bucketLookupType(true) != minio.BucketLookupPath {
		t.Error("pathStyle=true should return BucketLookupPath")
	}
	if bucketLookupType(false) != minio.BucketLookupAuto {
		t.Error("pathStyle=false should return BucketLookupAuto")
	}
}
