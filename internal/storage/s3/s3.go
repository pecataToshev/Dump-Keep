// Package s3 implements the storage.Provider interface for any S3-compatible
// storage (AWS S3, MinIO, Backblaze B2, Cloudflare R2, etc.) using the
// MinIO Go client. Folders are represented as key prefixes with "/" delimiters.
package s3

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"github.com/pecataToshev/dump-keep/internal/config"
)

// Client wraps a MinIO S3 client scoped to one bucket and an optional key prefix.
type Client struct {
	mc     *minio.Client
	bucket string
	prefix string // root prefix, no leading/trailing slash
}

// New creates an S3 storage provider from config. It validates that the
// required S3 env vars are set and establishes a connection to the endpoint.
func New(cfg config.Config) (*Client, error) {
	var missing []string
	if cfg.S3Endpoint == "" {
		missing = append(missing, "S3_ENDPOINT")
	}
	if cfg.S3Bucket == "" {
		missing = append(missing, "S3_BUCKET")
	}
	if cfg.S3AccessKey == "" {
		missing = append(missing, "S3_ACCESS_KEY")
	}
	if cfg.S3SecretKey == "" {
		missing = append(missing, "S3_SECRET_KEY")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("s3: missing environment variables: %v", missing)
	}

	mc, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure:       isHTTPS(cfg.S3Endpoint),
		Region:       cfg.S3Region,
		BucketLookup: bucketLookupType(cfg.S3PathStyle),
	})
	if err != nil {
		return nil, fmt.Errorf("s3: create client: %w", err)
	}

	return &Client{
		mc:     mc,
		bucket: cfg.S3Bucket,
		prefix: strings.Trim(cfg.S3Prefix, "/"),
	}, nil
}

// EnsureFolder is a no-op for S3 — folders are implicit (created by key
// prefixes when objects are uploaded).
func (c *Client) EnsureFolder(_ context.Context, _ string) error {
	return nil
}

// Put uploads content to the given key. Uses multipart upload for large
// objects via MinIO's PutObject with -1 size (streamed, unknown length).
func (c *Client) Put(ctx context.Context, key string, content io.Reader) error {
	fullKey := c.fullKey(key)
	_, err := c.mc.PutObject(ctx, c.bucket, fullKey, content, -1,
		minio.PutObjectOptions{ContentType: "application/octet-stream"})
	if err != nil {
		return fmt.Errorf("s3: upload %q: %w", fullKey, err)
	}
	return nil
}

// Delete removes a single object at the given key. Missing objects are
// treated as success (S3 DeleteObject is already idempotent).
func (c *Client) Delete(ctx context.Context, key string) error {
	fullKey := c.fullKey(key)
	if err := c.mc.RemoveObject(ctx, c.bucket, fullKey, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("s3: delete %q: %w", fullKey, err)
	}
	return nil
}

// ListFolders returns the names of immediate subfolders (distinct top-level
// prefixes) under the given prefix. An empty prefix lists the root.
func (c *Client) ListFolders(ctx context.Context, prefix string) ([]string, error) {
	searchPrefix := c.fullKey(prefix)
	if searchPrefix != "" {
		searchPrefix += "/"
	}

	var names []string
	objCh := c.mc.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{
		Prefix:    searchPrefix,
		Recursive: false,
	})
	for obj := range objCh {
		if obj.Err != nil {
			return names, fmt.Errorf("s3: list under %q: %w", searchPrefix, obj.Err)
		}
		// In non-recursive mode, "folders" appear as objects ending with "/".
		// Strip the search prefix and the trailing "/" to get the folder name.
		name := strings.TrimPrefix(obj.Key, searchPrefix)
		name = strings.TrimSuffix(name, "/")
		if name == "" || strings.Contains(name, "/") {
			continue // skip nested keys and empty segments
		}
		names = append(names, name)
	}
	return names, nil
}

// DeleteFolder removes all objects under the given path prefix.
func (c *Client) DeleteFolder(ctx context.Context, path string) error {
	searchPrefix := c.fullKey(path)
	if searchPrefix != "" {
		searchPrefix += "/"
	}

	objCh := c.mc.ListObjects(ctx, c.bucket, minio.ListObjectsOptions{
		Prefix:    searchPrefix,
		Recursive: true,
	})
	for obj := range objCh {
		if obj.Err != nil {
			return fmt.Errorf("s3: list for delete under %q: %w", searchPrefix, obj.Err)
		}
		if err := c.mc.RemoveObject(ctx, c.bucket, obj.Key, minio.RemoveObjectOptions{}); err != nil {
			return fmt.Errorf("s3: delete %q: %w", obj.Key, err)
		}
	}
	return nil
}

// fullKey prepends the configured root prefix to the relative key.
func (c *Client) fullKey(key string) string {
	key = strings.Trim(key, "/")
	if c.prefix == "" {
		return key
	}
	if key == "" {
		return c.prefix
	}
	return c.prefix + "/" + key
}

func isHTTPS(endpoint string) bool {
	return strings.HasPrefix(endpoint, "https://") || (!strings.HasPrefix(endpoint, "http://") && !strings.Contains(endpoint, "localhost") && !strings.Contains(endpoint, "127.0.0.1"))
}

func bucketLookupType(pathStyle bool) minio.BucketLookupType {
	if pathStyle {
		return minio.BucketLookupPath
	}
	return minio.BucketLookupAuto
}
