// Package gdrive implements the storage.Provider interface for Google
// Shared Drives using a service account. Folders are resolved by walking
// path segments — each segment is looked up by name under its parent, and
// created if missing. Folder ID lookups are cached for the lifetime of the
// client to avoid repeated API calls during a single backup run.
package gdrive

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"google.golang.org/api/drive/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"github.com/pecataToshev/dump-keep/internal/config"
)

const folderMimeType = "application/vnd.google-apps.folder"

// Client wraps the Google Drive API scoped to one Shared Drive.
// A Shared Drive is required: files uploaded by a service account into a
// regular shared folder would count against the account's own 15 GB quota.
type Client struct {
	svc     *drive.Service
	driveID string
	rootID  string

	mu    sync.Mutex
	cache map[string]string // path → folder ID
}

// New creates a GDrive storage provider from config. It validates that the
// required GDrive env vars are set and establishes a Drive API connection.
func New(cfg config.Config) (*Client, error) {
	var missing []string
	if cfg.GDriveSAJSON == "" {
		missing = append(missing, "GDRIVE_SA_JSON")
	}
	if cfg.GDriveSharedDriveID == "" {
		missing = append(missing, "GDRIVE_SHARED_DRIVE_ID")
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("gdrive: missing environment variables: %v", missing)
	}

	svc, err := drive.NewService(context.Background(),
		option.WithCredentialsJSON([]byte(cfg.GDriveSAJSON)),
		option.WithScopes(drive.DriveScope),
	)
	if err != nil {
		return nil, fmt.Errorf("gdrive: create service: %w", err)
	}

	rootID := cfg.GDriveFolderID
	if rootID == "" {
		rootID = cfg.GDriveSharedDriveID
	}

	return &Client{
		svc:     svc,
		driveID: cfg.GDriveSharedDriveID,
		rootID:  rootID,
		cache:   make(map[string]string),
	}, nil
}

// EnsureFolder creates a folder at the given relative path if it doesn't
// already exist. Intermediate segments are created as needed.
func (c *Client) EnsureFolder(ctx context.Context, path string) error {
	if path == "" {
		return nil
	}
	_, err := c.resolveFolder(ctx, path)
	return err
}

// resolveFolder walks the path segments, looking up or creating each folder.
// Returns the folder ID. Results are cached for the client's lifetime.
func (c *Client) resolveFolder(ctx context.Context, path string) (string, error) {
	c.mu.Lock()
	if id, ok := c.cache[path]; ok {
		c.mu.Unlock()
		return id, nil
	}
	c.mu.Unlock()

	segments := splitPath(path)
	parentID := c.rootID

	for _, seg := range segments {
		c.mu.Lock()
		if id, ok := c.cache[seg]; ok {
			parentID = id
			c.mu.Unlock()
			continue
		}
		c.mu.Unlock()

		id, err := c.lookupOrCreate(ctx, parentID, seg)
		if err != nil {
			return "", fmt.Errorf("resolve folder %q: %w", path, err)
		}

		c.mu.Lock()
		c.cache[seg] = id
		c.mu.Unlock()
		parentID = id
	}

	c.mu.Lock()
	c.cache[path] = parentID
	c.mu.Unlock()
	return parentID, nil
}

// lookupOrCreate finds a child folder by name under parentID, or creates it.
func (c *Client) lookupOrCreate(ctx context.Context, parentID, name string) (string, error) {
	query := fmt.Sprintf(
		"name = '%s' and mimeType = '%s' and '%s' in parents and trashed = false",
		strings.ReplaceAll(name, "'", `\'`), folderMimeType, parentID,
	)
	list, err := c.svc.Files.List().
		Q(query).
		Corpora("drive").
		DriveId(c.driveID).
		IncludeItemsFromAllDrives(true).
		SupportsAllDrives(true).
		Fields("files(id)").
		Context(ctx).
		Do()
	if err != nil {
		return "", fmt.Errorf("list folder %q: %w", name, err)
	}
	if len(list.Files) > 0 {
		return list.Files[0].Id, nil
	}

	folder, err := c.svc.Files.Create(&drive.File{
		Name:     name,
		MimeType: folderMimeType,
		Parents:  []string{parentID},
	}).SupportsAllDrives(true).Fields("id").Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("create folder %q: %w", name, err)
	}
	return folder.Id, nil
}

// Put uploads content to the given key (relative path including filename).
func (c *Client) Put(ctx context.Context, key string, content io.Reader) error {
	folderPath, name := splitKey(key)
	folderID, err := c.resolveFolder(ctx, folderPath)
	if err != nil {
		return err
	}

	_, err = c.svc.Files.Create(&drive.File{
		Name:    name,
		Parents: []string{folderID},
	}).
		Media(content, googleapi.ChunkSize(8*1024*1024)).
		SupportsAllDrives(true).
		Fields("id").
		Context(ctx).
		Do()
	if err != nil {
		return fmt.Errorf("upload %q: %w", key, err)
	}
	return nil
}

// Delete removes a single object at the given key. Missing objects are
// treated as success.
func (c *Client) Delete(ctx context.Context, key string) error {
	fileID, err := c.findFile(ctx, key)
	if err != nil {
		return err
	}
	if fileID == "" {
		return nil // not found — idempotent
	}
	return c.trashFile(ctx, fileID)
}

// DeleteFolder removes a folder and all of its contents by trashing the
// folder (Drive cascades trash to children).
func (c *Client) DeleteFolder(ctx context.Context, path string) error {
	folderID, err := c.resolveFolder(ctx, path)
	if err != nil {
		return err
	}
	return c.trashFile(ctx, folderID)
}

// ListFolders returns the names of immediate subfolders under the given
// prefix. An empty prefix lists the root.
func (c *Client) ListFolders(ctx context.Context, prefix string) ([]string, error) {
	parentID := c.rootID
	if prefix != "" {
		var err error
		parentID, err = c.resolveFolder(ctx, prefix)
		if err != nil {
			return nil, err
		}
	}

	query := fmt.Sprintf(
		"'%s' in parents and mimeType = '%s' and trashed = false",
		parentID, folderMimeType,
	)
	var names []string
	pageToken := ""
	for {
		call := c.svc.Files.List().
			Q(query).
			Corpora("drive").
			DriveId(c.driveID).
			IncludeItemsFromAllDrives(true).
			SupportsAllDrives(true).
			Fields("nextPageToken, files(name)").
			PageSize(200).
			Context(ctx)
		if pageToken != "" {
			call = call.PageToken(pageToken)
		}
		list, err := call.Do()
		if err != nil {
			return names, fmt.Errorf("list folders under %q: %w", prefix, err)
		}
		for _, f := range list.Files {
			names = append(names, f.Name)
		}
		pageToken = list.NextPageToken
		if pageToken == "" {
			return names, nil
		}
	}
}

// findFile resolves a full key path to a file ID.
func (c *Client) findFile(ctx context.Context, key string) (string, error) {
	folderPath, name := splitKey(key)
	folderID, err := c.resolveFolder(ctx, folderPath)
	if err != nil {
		return "", err
	}
	query := fmt.Sprintf(
		"name = '%s' and '%s' in parents and trashed = false",
		strings.ReplaceAll(name, "'", `\'`), folderID,
	)
	list, err := c.svc.Files.List().
		Q(query).
		Corpora("drive").
		DriveId(c.driveID).
		IncludeItemsFromAllDrives(true).
		SupportsAllDrives(true).
		Fields("files(id)").
		Context(ctx).
		Do()
	if err != nil {
		return "", fmt.Errorf("find file %q: %w", key, err)
	}
	if len(list.Files) == 0 {
		return "", nil
	}
	return list.Files[0].Id, nil
}

func (c *Client) trashFile(ctx context.Context, fileID string) error {
	_, err := c.svc.Files.Update(fileID, &drive.File{Trashed: true}).
		SupportsAllDrives(true).
		Context(ctx).
		Do()
	if err != nil {
		if gErr, ok := err.(*googleapi.Error); ok && gErr.Code == 404 {
			return nil // already gone — idempotent
		}
		return fmt.Errorf("trash file %q: %w", fileID, err)
	}
	return nil
}

// splitPath splits "a/b/c" into ["a", "b", "c"].
func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	return strings.Split(path, "/")
}

// splitKey splits "folder/path/file.ext" into ("folder/path", "file.ext").
func splitKey(key string) (string, string) {
	key = strings.Trim(key, "/")
	idx := strings.LastIndex(key, "/")
	if idx == -1 {
		return "", key
	}
	return key[:idx], key[idx+1:]
}
