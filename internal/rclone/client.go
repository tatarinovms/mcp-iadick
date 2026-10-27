package rclone

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	MaxReadBytes            = 10 * 1024 * 1024
	MaxUploadBytes          = 10 * 1024 * 1024
	MaxBase64InputBytes     = (MaxUploadBytes+2)/3*4 + 4096
	MaxOutputBytes          = 16 * 1024 * 1024
	MaxStderrBytes          = 64 * 1024
	DefaultListLimit        = 1000
	MaxListLimit            = 5000
	DefaultOperationTimeout = 5 * time.Minute
	DefaultMaxConcurrent    = 4
)

var errPageComplete = errors.New("page complete")

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

// Base64ReadResult represents binary content read from a file and base64-encoded.
type Base64ReadResult struct {
	Path          string `json:"path"`
	ContentBase64 string `json:"content_base64"`
	BytesRead     int    `json:"bytes_read"`
	Offset        int    `json:"offset"`
	IsTruncated   bool   `json:"is_truncated"`
}

// Client wraps the rclone CLI.
type Client struct {
	Remote     string
	RclonePath string

	// Configure these before the first operation. Zero values use the defaults.
	OperationTimeout time.Duration
	MaxConcurrent    int
	LogOperations    bool
	initOnce         sync.Once
	slots            chan struct{}
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
	if len(stdin) > MaxUploadBytes {
		return nil, fmt.Errorf("upload exceeds %d bytes; use yandex_upload_file or yandex_upload_from_url", MaxUploadBytes)
	}
	var stdout bytes.Buffer
	if err := c.run(ctx, bytes.NewReader(stdin), &stdout, args...); err != nil {
		return nil, err
	}
	return stdout.Bytes(), nil
}

// run streams stdout and bounds both the process lifetime and its output.
// The timeout includes time spent waiting for a free process slot.
func (c *Client) run(ctx context.Context, stdin io.Reader, stdout io.Writer, args ...string) (runErr error) {
	started := time.Now()
	defer func() {
		if c.LogOperations && len(args) > 0 {
			result := "success"
			if runErr != nil {
				result = "error"
			}
			if errors.Is(context.Cause(ctx), errPageComplete) {
				result = "page_complete"
			}
			log.Printf("rclone command=%s duration=%s result=%s", args[0], time.Since(started), result)
		}
	}()
	timeout := c.OperationTimeout
	if timeout <= 0 {
		timeout = DefaultOperationTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	c.initOnce.Do(func() {
		n := c.MaxConcurrent
		if n <= 0 {
			n = DefaultMaxConcurrent
		}
		c.slots = make(chan struct{}, n)
	})
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-ctx.Done():
		return ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, c.RclonePath, args...)
	cmd.Stdin = stdin
	cmd.WaitDelay = time.Second
	output := &outputWriter{dst: stdout, remaining: MaxOutputBytes, cancel: cancel}
	stderr := stderrBuffer{}
	cmd.Stdout = output
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if output.err != nil {
			return output.err
		}
		if ctx.Err() != nil {
			return fmt.Errorf("rclone command canceled: %w", ctx.Err())
		}
		errOutput := strings.TrimSpace(stderr.buf.String())
		if stderr.truncated {
			errOutput += " [stderr truncated]"
		}
		if errOutput != "" {
			return fmt.Errorf("rclone error: %s (%w)", errOutput, err)
		}
		return fmt.Errorf("rclone command failed: %w", err)
	}

	return nil
}

type outputWriter struct {
	dst       io.Writer
	remaining int
	cancel    context.CancelFunc
	err       error
}

func (w *outputWriter) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		w.err = fmt.Errorf("rclone output exceeds %d bytes; narrow the path, depth, or search pattern", MaxOutputBytes)
		w.cancel()
		return 0, w.err
	}
	n, err := w.dst.Write(p)
	w.remaining -= n
	if err != nil {
		w.err = err
		w.cancel()
	}
	return n, err
}

// Keep a bounded diagnostic prefix while draining the rest of stderr.
type stderrBuffer struct {
	buf       bytes.Buffer
	truncated bool
}

func (b *stderrBuffer) Write(p []byte) (int, error) {
	n := len(p)
	remaining := MaxStderrBytes - b.buf.Len()
	if len(p) > remaining {
		b.truncated = true
		p = p[:remaining]
	}
	_, _ = b.buf.Write(p)
	return n, nil
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

type ListOptions struct {
	Recursive bool
	MaxDepth  int
	DirsOnly  bool
	FilesOnly bool
	Pattern   string
	Limit     int
	Offset    int
}

type ListResult struct {
	Items      []Item `json:"items"`
	Count      int    `json:"count"`
	Offset     int    `json:"offset"`
	HasMore    bool   `json:"has_more"`
	NextOffset *int   `json:"next_offset,omitempty"`
}

// ListDirectory decodes one page from the lsjson stream, then stops rclone.
// Offset pages repeat the traversal; they are not a snapshot of the directory.
func (c *Client) ListDirectory(ctx context.Context, path string, opts ListOptions) (*ListResult, error) {
	if opts.Limit == 0 {
		opts.Limit = DefaultListLimit
	}
	if opts.Limit < 1 || opts.Limit > MaxListLimit {
		return nil, fmt.Errorf("limit must be between 1 and %d", MaxListLimit)
	}
	if opts.Offset < 0 || opts.Offset > int(^uint(0)>>1)-opts.Limit {
		return nil, fmt.Errorf("offset must be non-negative and leave room for limit")
	}
	if opts.MaxDepth < 0 {
		return nil, fmt.Errorf("max_depth must be non-negative (0 means unlimited for recursive listings)")
	}
	if opts.DirsOnly && opts.FilesOnly {
		return nil, fmt.Errorf("dirs_only and files_only cannot both be true")
	}
	resolved := c.ResolvePath(path)
	args := []string{"lsjson", resolved}

	if opts.Recursive {
		args = append(args, "--recursive")
	}
	if opts.MaxDepth > 0 {
		args = append(args, "--max-depth", fmt.Sprintf("%d", opts.MaxDepth))
	}

	if opts.DirsOnly {
		args = append(args, "--dirs-only")
	} else if opts.FilesOnly {
		args = append(args, "--files-only")
	}

	if opts.Pattern != "" {
		args = append(args, "--include", opts.Pattern)
	}

	streamCtx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	reader, writer := io.Pipe()
	done := make(chan error, 1)
	go func() {
		err := c.run(streamCtx, nil, writer, args...)
		_ = writer.CloseWithError(err)
		done <- err
	}()
	defer reader.Close()

	page, parseErr := decodeListPage(reader, opts.Limit, opts.Offset)
	if parseErr != nil || page.HasMore {
		cause := parseErr
		if cause == nil {
			cause = errPageComplete
		}
		cancel(cause)
		_ = reader.Close()
	}
	runErr := <-done
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if parseErr != nil {
		// Keep process failures (including deadline exceeded) instead of a
		// secondary JSON error, unless we stopped the process after bad JSON.
		if runErr != nil && !errors.Is(runErr, context.Canceled) && !errors.Is(runErr, io.ErrClosedPipe) {
			return nil, runErr
		}
		return nil, fmt.Errorf("failed to parse lsjson output: %w", parseErr)
	}
	if runErr != nil && (!page.HasMore || (!errors.Is(runErr, context.Canceled) && !errors.Is(runErr, io.ErrClosedPipe))) {
		return nil, runErr
	}
	return page, nil
}

func decodeListPage(reader io.Reader, limit, offset int) (*ListResult, error) {
	dec := json.NewDecoder(reader)
	token, err := dec.Token()
	if err != nil {
		return nil, err
	}
	if token != json.Delim('[') {
		return nil, fmt.Errorf("expected a JSON array")
	}
	page := &ListResult{Items: make([]Item, 0, limit), Offset: offset}
	for index := 0; dec.More(); index++ {
		var item Item
		if err := dec.Decode(&item); err != nil {
			return nil, err
		}
		if index < offset {
			continue
		}
		if len(page.Items) == limit {
			page.HasMore = true
			next := offset + limit
			page.NextOffset = &next
			page.Count = len(page.Items)
			return page, nil
		}
		if item.IsDir {
			item.SizeHuman = "DIR"
		} else {
			item.SizeHuman = FormatBytes(&item.Size)
		}
		page.Items = append(page.Items, item)
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	// Consume EOF so a trailing process error or malformed JSON is not hidden.
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("unexpected data after JSON array")
		}
		return nil, err
	}
	page.Count = len(page.Items)
	return page, nil
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

// ReadFileRaw reads raw bytes of a file with byte count and offset support.
func (c *Client) ReadFileRaw(ctx context.Context, path string, maxBytes, offset int) ([]byte, bool, error) {
	if maxBytes < 1 || maxBytes > MaxReadBytes {
		return nil, false, fmt.Errorf("max_bytes must be between 1 and %d", MaxReadBytes)
	}
	if offset < 0 {
		return nil, false, fmt.Errorf("offset must be non-negative")
	}
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
		return nil, false, err
	}

	truncated := false
	if maxBytes > 0 && len(out) > maxBytes {
		truncated = true
		out = out[:maxBytes]
	}

	return out, truncated, nil
}

// ReadFile reads contents of a file as a string.
func (c *Client) ReadFile(ctx context.Context, path string, maxBytes, offset int) (*ReadResult, error) {
	out, truncated, err := c.ReadFileRaw(ctx, path, maxBytes, offset)
	if err != nil {
		return nil, err
	}

	return &ReadResult{
		Path:        c.ResolvePath(path),
		Content:     string(out),
		BytesRead:   len(out),
		Offset:      offset,
		IsTruncated: truncated,
	}, nil
}

// ReadFileBase64 reads contents of a file and encodes as base64 string.
func (c *Client) ReadFileBase64(ctx context.Context, path string, maxBytes, offset int) (*Base64ReadResult, error) {
	out, truncated, err := c.ReadFileRaw(ctx, path, maxBytes, offset)
	if err != nil {
		return nil, err
	}

	return &Base64ReadResult{
		Path:          c.ResolvePath(path),
		ContentBase64: base64.StdEncoding.EncodeToString(out),
		BytesRead:     len(out),
		Offset:        offset,
		IsTruncated:   truncated,
	}, nil
}

// WriteFile writes content to a file via rclone rcat.
func (c *Client) WriteFile(ctx context.Context, path string, content []byte) error {
	return c.writeFile(ctx, path, bytes.NewReader(content), len(content))
}

// WriteTextFile avoids copying the content string into another byte slice.
func (c *Client) WriteTextFile(ctx context.Context, path, content string) error {
	return c.writeFile(ctx, path, strings.NewReader(content), len(content))
}

func (c *Client) writeFile(ctx context.Context, path string, content io.Reader, size int) error {
	if size > MaxUploadBytes {
		return fmt.Errorf("upload exceeds %d bytes; use yandex_upload_file or yandex_upload_from_url", MaxUploadBytes)
	}
	resolved := c.ResolvePath(path)
	return c.run(ctx, content, io.Discard, "rcat", resolved)
}

// decodeBase64 strips potential data URL prefixes, whitespaces and decodes standard or raw base64.
func decodeBase64(contentBase64 string) ([]byte, error) {
	if len(contentBase64) > MaxBase64InputBytes {
		return nil, fmt.Errorf("base64 input exceeds %d bytes; use yandex_upload_file or yandex_upload_from_url", MaxBase64InputBytes)
	}
	clean := strings.TrimSpace(contentBase64)
	if idx := strings.Index(clean, ";base64,"); idx != -1 {
		clean = clean[idx+8:]
	}
	clean = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == ' ' || r == '\t' {
			return -1
		}
		return r
	}, clean)
	encoding := base64.RawStdEncoding
	if strings.HasSuffix(clean, "=") {
		encoding = base64.StdEncoding
	}
	// Check before allocating the decoded payload.
	decodedLen := encoding.DecodedLen(len(clean))
	if encoding == base64.StdEncoding {
		decodedLen -= len(clean) - len(strings.TrimRight(clean, "="))
	}
	if decodedLen > MaxUploadBytes {
		return nil, fmt.Errorf("decoded upload exceeds %d bytes", MaxUploadBytes)
	}
	data, err := encoding.DecodeString(clean)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64: %w", err)
	}
	return data, nil
}

// UploadBase64 decodes base64 content and writes it to remote path.
func (c *Client) UploadBase64(ctx context.Context, path, contentBase64 string) (int, error) {
	data, err := decodeBase64(contentBase64)
	if err != nil {
		return 0, err
	}

	if err := c.WriteFile(ctx, path, data); err != nil {
		return 0, err
	}
	return len(data), nil
}

// UploadFromURL downloads a file from URL directly to Yandex Disk via rclone copyurl.
func (c *Client) UploadFromURL(ctx context.Context, url, remotePath string, autoFilename bool) (string, error) {
	resolved := c.ResolvePath(remotePath)
	args := []string{"copyurl", url, resolved, "--print-filename"}
	if autoFilename || strings.HasSuffix(remotePath, "/") || remotePath == "" {
		args = append(args, "--auto-filename")
	}

	out, err := c.Run(ctx, nil, args...)
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	filename := ""
	if len(lines) > 0 {
		filename = strings.TrimSpace(lines[len(lines)-1])
	}
	return filename, nil
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
