package rclone

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Item represents a file or directory metadata from rclone lsjson.
type Item struct {
	Path      string    `json:"Path"`
	Name      string    `json:"Name"`
	Size      int64     `json:"Size"`
	SizeHuman string    `json:"SizeHuman"`
	MimeType  string    `json:"MimeType,omitempty"`
	ModTime   time.Time `json:"ModTime"`
	IsDir     bool      `json:"IsDir"`
}

// StorageInfo represents disk space quota from rclone about.
type StorageInfo struct {
	Remote         string   `json:"remote"`
	TotalBytes     *int64   `json:"total_bytes,omitempty"`
	UsedBytes      *int64   `json:"used_bytes,omitempty"`
	FreeBytes      *int64   `json:"free_bytes,omitempty"`
	TrashedBytes   *int64   `json:"trashed_bytes,omitempty"`
	TotalHuman     string   `json:"total_human"`
	UsedHuman      string   `json:"used_human"`
	FreeHuman      string   `json:"free_human"`
	TrashedHuman   string   `json:"trashed_human,omitempty"`
	UsedPercentage *float64 `json:"used_percentage,omitempty"`
}

// ReadResult represents the content read from a file.
type ReadResult struct {
	Path        string `json:"path"`
	Content     string `json:"content"`
	BytesRead   int    `json:"bytes_read"`
	Offset      int    `json:"offset"`
	IsTruncated bool   `json:"is_truncated"`
}

// Client wraps the rclone CLI.
type Client struct {
	Remote     string
	RclonePath string
}

// FormatBytes formats byte count into a human-readable string.
func FormatBytes(size *int64) string {
	if size == nil || *size < 0 {
		return "N/A"
	}
	s := float64(*size)
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	for _, unit := range units {
		if s < 1024.0 || unit == units[len(units)-1] {
			return fmt.Sprintf("%.2f %s", s, unit)
		}
		s /= 1024.0
	}
	return fmt.Sprintf("%d B", *size)
}

// NewClient initializes a new rclone Client.
func NewClient(remote, customPath string) (*Client, error) {
	if remote == "" {
		remote = os.Getenv("RCLONE_REMOTE")
	}
	if remote == "" {
		remote = "yandex"
	}
	remote = strings.TrimSuffix(strings.TrimSpace(remote), ":")

	binPath, err := findRclone(customPath)
	if err != nil {
		return nil, err
	}

	return &Client{
		Remote:     remote,
		RclonePath: binPath,
	}, nil
}

func findRclone(customPath string) (string, error) {
	if customPath == "" {
		customPath = os.Getenv("RCLONE_PATH")
	}
	if customPath != "" {
		if fi, err := os.Stat(customPath); err == nil && !fi.IsDir() {
			return customPath, nil
		}
		if p, err := exec.LookPath(customPath); err == nil {
			return p, nil
		}
	}

	if p, err := exec.LookPath("rclone"); err == nil {
		return p, nil
	}

	candidates := []string{
		"/opt/homebrew/bin/rclone",
		"/usr/local/bin/rclone",
		"/usr/bin/rclone",
	}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, ".local/bin/rclone"))
	}

	for _, cand := range candidates {
		if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
			return cand, nil
		}
	}

	return "", fmt.Errorf("rclone executable not found. Please install rclone or set RCLONE_PATH")
}

// ResolvePath converts a path into a full remote:path string.
func (c *Client) ResolvePath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || trimmed == "/" || trimmed == "." {
		return c.Remote + ":"
	}

	if strings.Contains(trimmed, ":") && !strings.HasPrefix(trimmed, "/") && !strings.HasPrefix(trimmed, "\\") {
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) > 0 && !strings.ContainsAny(parts[0], "/\\") {
			return trimmed
		}
	}

	clean := strings.TrimPrefix(trimmed, "/")
	return c.Remote + ":" + clean
}

// Run executes an rclone command with context.
func (c *Client) Run(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.RclonePath, args...)
	if len(stdin) > 0 {
		cmd.Stdin = bytes.NewReader(stdin)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errOutput := strings.TrimSpace(stderr.String())
		if errOutput != "" {
			return nil, fmt.Errorf("rclone error: %s (%w)", errOutput, err)
		}
		return nil, fmt.Errorf("rclone command failed: %w", err)
	}

	return stdout.Bytes(), nil
}

// GetStorageInfo returns disk usage data via rclone about.
func (c *Client) GetStorageInfo(ctx context.Context) (*StorageInfo, error) {
	target := c.Remote + ":"
	out, err := c.Run(ctx, nil, "about", target, "--json")
	if err != nil {
		return nil, err
	}

	var raw struct {
		Total   *int64 `json:"total"`
		Used    *int64 `json:"used"`
		Free    *int64 `json:"free"`
		Trashed *int64 `json:"trashed"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, fmt.Errorf("failed to parse rclone about json: %w", err)
	}

	info := &StorageInfo{
		Remote:       target,
		TotalBytes:   raw.Total,
		UsedBytes:    raw.Used,
		FreeBytes:    raw.Free,
		TrashedBytes: raw.Trashed,
		TotalHuman:   FormatBytes(raw.Total),
		UsedHuman:    FormatBytes(raw.Used),
		FreeHuman:    FormatBytes(raw.Free),
	}
	if raw.Trashed != nil {
		info.TrashedHuman = FormatBytes(raw.Trashed)
	}
	if raw.Total != nil && *raw.Total > 0 && raw.Used != nil {
		pct := (float64(*raw.Used) / float64(*raw.Total)) * 100.0
		info.UsedPercentage = &pct
	}

	return info, nil
}

// ListDirectory lists files and folders using rclone lsjson.
func (c *Client) ListDirectory(
	ctx context.Context,
	path string,
	recursive bool,
	maxDepth int,
	dirsOnly, filesOnly bool,
	pattern string,
) ([]Item, error) {
	resolved := c.ResolvePath(path)
	args := []string{"lsjson", resolved}

	if recursive {
		args = append(args, "--recursive")
	} else if maxDepth > 0 {
		args = append(args, "--max-depth", fmt.Sprintf("%d", maxDepth))
	}

	if dirsOnly {
		args = append(args, "--dirs-only")
	} else if filesOnly {
		args = append(args, "--files-only")
	}

	if pattern != "" {
		args = append(args, "--include", pattern)
	}

	out, err := c.Run(ctx, nil, args...)
	if err != nil {
		return nil, err
	}

	var items []Item
	if err := json.Unmarshal(out, &items); err != nil {
		return nil, fmt.Errorf("failed to parse lsjson output: %w", err)
	}

	for i := range items {
		if items[i].IsDir {
			items[i].SizeHuman = "DIR"
		} else {
			items[i].SizeHuman = FormatBytes(&items[i].Size)
		}
	}

	return items, nil
}

// GetFileInfo fetches metadata for a specific path using rclone lsjson --stat.
func (c *Client) GetFileInfo(ctx context.Context, path string) (*Item, error) {
	resolved := c.ResolvePath(path)
	out, err := c.Run(ctx, nil, "lsjson", resolved, "--stat")
	if err != nil {
		return nil, err
	}

	var item Item
	if err := json.Unmarshal(out, &item); err != nil {
		return nil, fmt.Errorf("failed to parse stat output: %w", err)
	}

	if item.IsDir {
		item.SizeHuman = "DIR"
	} else {
		item.SizeHuman = FormatBytes(&item.Size)
	}

	return &item, nil
}

// ReadFile reads contents of a file with byte count and offset support.
func (c *Client) ReadFile(ctx context.Context, path string, maxBytes, offset int) (*ReadResult, error) {
	resolved := c.ResolvePath(path)
	args := []string{"cat", resolved}

	if offset > 0 {
		args = append(args, "--offset", fmt.Sprintf("%d", offset))
	}
	if maxBytes > 0 {
		args = append(args, "--count", fmt.Sprintf("%d", maxBytes+1))
	}

	out, err := c.Run(ctx, nil, args...)
	if err != nil {
		return nil, err
	}

	truncated := false
	if maxBytes > 0 && len(out) > maxBytes {
		truncated = true
		out = out[:maxBytes]
	}

	return &ReadResult{
		Path:        resolved,
		Content:     string(out),
		BytesRead:   len(out),
		Offset:      offset,
		IsTruncated: truncated,
	}, nil
}

// WriteFile writes content to a file via rclone rcat.
func (c *Client) WriteFile(ctx context.Context, path string, content []byte) error {
	resolved := c.ResolvePath(path)
	_, err := c.Run(ctx, content, "rcat", resolved)
	return err
}

// CreateDirectory creates a folder via rclone mkdir.
func (c *Client) CreateDirectory(ctx context.Context, path string) error {
	resolved := c.ResolvePath(path)
	_, err := c.Run(ctx, nil, "mkdir", resolved)
	return err
}

// DeleteFile deletes a single file via rclone deletefile.
func (c *Client) DeleteFile(ctx context.Context, path string) error {
	resolved := c.ResolvePath(path)
	_, err := c.Run(ctx, nil, "deletefile", resolved)
	return err
}

// DeleteDirectory deletes a directory (rmdir if not recursive, purge if recursive).
func (c *Client) DeleteDirectory(ctx context.Context, path string, recursive bool) error {
	resolved := c.ResolvePath(path)
	cmd := "rmdir"
	if recursive {
		cmd = "purge"
	}
	_, err := c.Run(ctx, nil, cmd, resolved)
	return err
}

// CopyItem copies a file or directory via rclone copyto.
func (c *Client) CopyItem(ctx context.Context, src, dst string) error {
	srcResolved := c.ResolvePath(src)
	dstResolved := c.ResolvePath(dst)
	_, err := c.Run(ctx, nil, "copyto", srcResolved, dstResolved)
	return err
}

// MoveItem moves or renames a file or directory via rclone moveto.
func (c *Client) MoveItem(ctx context.Context, src, dst string) error {
	srcResolved := c.ResolvePath(src)
	dstResolved := c.ResolvePath(dst)
	_, err := c.Run(ctx, nil, "moveto", srcResolved, dstResolved)
	return err
}

// UploadFile copies local file to remote via rclone copyto.
func (c *Client) UploadFile(ctx context.Context, localPath, remotePath string) error {
	absLocal, err := filepath.Abs(localPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(absLocal); err != nil {
		return fmt.Errorf("local file not found: %s", absLocal)
	}

	dstResolved := c.ResolvePath(remotePath)
	_, err = c.Run(ctx, nil, "copyto", absLocal, dstResolved)
	return err
}

// DownloadFile copies remote file to local filesystem via rclone copyto.
func (c *Client) DownloadFile(ctx context.Context, remotePath, localPath string) error {
	absLocal, err := filepath.Abs(localPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absLocal), 0755); err != nil {
		return err
	}

	srcResolved := c.ResolvePath(remotePath)
	_, err = c.Run(ctx, nil, "copyto", srcResolved, absLocal)
	return err
}

// CreatePublicLink generates a shareable link via rclone link.
func (c *Client) CreatePublicLink(ctx context.Context, path, expire string) (string, error) {
	resolved := c.ResolvePath(path)
	args := []string{"link", resolved}
	if expire != "" {
		args = append(args, "--expire", expire)
	}
	out, err := c.Run(ctx, nil, args...)
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[len(lines)-1] == "" {
		return "", fmt.Errorf("no public link returned")
	}
	return strings.TrimSpace(lines[len(lines)-1]), nil
}

// RemovePublicLink removes a public link via rclone link --unlink.
func (c *Client) RemovePublicLink(ctx context.Context, path string) error {
	resolved := c.ResolvePath(path)
	_, err := c.Run(ctx, nil, "link", "--unlink", resolved)
	return err
}
