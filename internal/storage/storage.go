// Package storage abstracts the destination where encrypted backups are
// uploaded. Backends implement the Provider interface and are constructed
// via New, which selects the backend based on config.StorageBackend.
//
// All paths are relative to the backend's configured root (a Drive folder
// or an S3 key prefix) and use "/" as the separator. Backends are
// responsible for prepending their root to the relative paths they receive.
package storage

import (
	"context"
	"fmt"
	"io"

	"github.com/pecataToshev/dump-keep/internal/config"
	"github.com/pecataToshev/dump-keep/internal/storage/gdrive"
	"github.com/pecataToshev/dump-keep/internal/storage/s3"
)

// Provider abstracts a backup storage destination.
type Provider interface {
	// EnsureFolder creates a folder at the given relative path if it doesn't
	// already exist. Intermediate segments are created as needed.
	EnsureFolder(ctx context.Context, path string) error

	// Put uploads content to the given key (full relative path including
	// filename, e.g. "2026-08-17_daily_031745/mydb.dump.age").
	Put(ctx context.Context, key string, content io.Reader) error

	// Delete removes a single object at the given key. A missing object is
	// treated as success (idempotent).
	Delete(ctx context.Context, key string) error

	// ListFolders returns the names of immediate subfolders under the given
	// prefix. An empty prefix lists the root. Names are relative to the
	// prefix and do not include a trailing "/".
	ListFolders(ctx context.Context, prefix string) ([]string, error)

	// DeleteFolder removes a folder and all of its contents.
	DeleteFolder(ctx context.Context, path string) error
}

// New creates the storage backend specified by cfg.StorageBackend.
// Backend-specific validation happens inside each constructor.
func New(cfg config.Config) (Provider, error) {
	switch cfg.StorageBackend {
	case config.BackendGDrive:
		return gdrive.New(cfg)
	case config.BackendS3:
		return s3.New(cfg)
	default:
		return nil, fmt.Errorf("unsupported STORAGE_BACKEND %q: must be %q or %q",
			cfg.StorageBackend, config.BackendGDrive, config.BackendS3)
	}
}
