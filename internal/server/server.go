package server

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/tatarinovms/mcp-iadick/internal/rclone"
)

// NewServer builds and configures the MCP server with all Yandex Disk tools.
func NewServer(client *rclone.Client) *mcpserver.MCPServer {
	s := mcpserver.NewMCPServer(
		"mcp-iadick",
		"0.1.0",
		mcpserver.WithToolCapabilities(true),
		mcpserver.WithPromptCapabilities(true),
		mcpserver.WithResourceCapabilities(false, false),
	)

	registerTools(s, client)
	return s
}

func jsonResult(data any) (*mcp.CallToolResult, error) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("JSON marshal error: %v", err)), nil
	}
	return mcp.NewToolResultText(string(b)), nil
}

func registerTools(s *mcpserver.MCPServer, client *rclone.Client) {
	// 1. yandex_list_directory
	s.AddTool(
		mcp.NewTool("yandex_list_directory",
			mcp.WithDescription("List contents of a directory on Yandex Disk with file sizes and modification dates."),
			mcp.WithString("path", mcp.Description("Directory path on Yandex Disk (empty for root, e.g. '', 'Documents')")),
			mcp.WithBoolean("recursive", mcp.Description("If true, list recursively")),
			mcp.WithInteger("max_depth", mcp.Description("Maximum depth for traversal (default: 1)")),
			mcp.WithBoolean("dirs_only", mcp.Description("If true, list only folders")),
			mcp.WithBoolean("files_only", mcp.Description("If true, list only files")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			path := req.GetString("path", "")
			recursive := req.GetBool("recursive", false)
			maxDepth := req.GetInt("max_depth", 1)
			dirsOnly := req.GetBool("dirs_only", false)
			filesOnly := req.GetBool("files_only", false)

			items, err := client.ListDirectory(ctx, path, recursive, maxDepth, dirsOnly, filesOnly, "")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			return jsonResult(map[string]any{
				"path":  client.ResolvePath(path),
				"count": len(items),
				"items": items,
			})
		},
	)

	// 2. yandex_get_file_info
	s.AddTool(
		mcp.NewTool("yandex_get_file_info",
			mcp.WithDescription("Get detailed metadata and stats for a file or directory on Yandex Disk."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Remote path (e.g. 'Documents/report.pdf')")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			path, err := req.RequireString("path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			info, err := client.GetFileInfo(ctx, path)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(info)
		},
	)

	// 3. yandex_read_file
	s.AddTool(
		mcp.NewTool("yandex_read_file",
			mcp.WithDescription("Read text contents of a file on Yandex Disk (with max_bytes protection)."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Remote file path")),
			mcp.WithInteger("max_bytes", mcp.Description("Maximum bytes to read (default: 100000)")),
			mcp.WithInteger("offset", mcp.Description("Starting byte offset (default: 0)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			path, err := req.RequireString("path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			maxBytes := req.GetInt("max_bytes", 100000)
			offset := req.GetInt("offset", 0)

			res, err := client.ReadFile(ctx, path, maxBytes, offset)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(res)
		},
	)

	// 4. yandex_write_file
	s.AddTool(
		mcp.NewTool("yandex_write_file",
			mcp.WithDescription("Write text content to a file on Yandex Disk (creates or overwrites)."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Remote file path")),
			mcp.WithString("content", mcp.Required(), mcp.Description("File content string")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			path, err := req.RequireString("path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			content, err := req.RequireString("content")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if err := client.WriteFile(ctx, path, []byte(content)); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(map[string]any{
				"path":          client.ResolvePath(path),
				"status":        "success",
				"bytes_written": len(content),
			})
		},
	)

	// 5. yandex_create_directory
	s.AddTool(
		mcp.NewTool("yandex_create_directory",
			mcp.WithDescription("Create a new directory on Yandex Disk."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Remote directory path")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			path, err := req.RequireString("path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if err := client.CreateDirectory(ctx, path); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(map[string]any{
				"path":   client.ResolvePath(path),
				"status": "directory_created",
			})
		},
	)

	// 6. yandex_delete_file
	s.AddTool(
		mcp.NewTool("yandex_delete_file",
			mcp.WithDescription("Delete a single file on Yandex Disk."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Remote file path to delete")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			path, err := req.RequireString("path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if err := client.DeleteFile(ctx, path); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(map[string]any{
				"path":   client.ResolvePath(path),
				"status": "file_deleted",
			})
		},
	)

	// 7. yandex_delete_directory
	s.AddTool(
		mcp.NewTool("yandex_delete_directory",
			mcp.WithDescription("Delete a directory on Yandex Disk (rmdir if not recursive, purge if recursive)."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Remote directory path")),
			mcp.WithBoolean("recursive", mcp.Description("If true, purges directory and all its contents")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			path, err := req.RequireString("path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			recursive := req.GetBool("recursive", false)

			if err := client.DeleteDirectory(ctx, path, recursive); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(map[string]any{
				"path":      client.ResolvePath(path),
				"recursive": recursive,
				"status":    "directory_deleted",
			})
		},
	)

	// 8. yandex_copy_item
	s.AddTool(
		mcp.NewTool("yandex_copy_item",
			mcp.WithDescription("Copy a file or directory within Yandex Disk."),
			mcp.WithString("source_path", mcp.Required(), mcp.Description("Source path")),
			mcp.WithString("destination_path", mcp.Required(), mcp.Description("Destination path")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			src, err := req.RequireString("source_path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			dst, err := req.RequireString("destination_path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if err := client.CopyItem(ctx, src, dst); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(map[string]any{
				"source":      client.ResolvePath(src),
				"destination": client.ResolvePath(dst),
				"status":      "copied",
			})
		},
	)

	// 9. yandex_move_item
	s.AddTool(
		mcp.NewTool("yandex_move_item",
			mcp.WithDescription("Move or rename a file or directory within Yandex Disk."),
			mcp.WithString("source_path", mcp.Required(), mcp.Description("Source path")),
			mcp.WithString("destination_path", mcp.Required(), mcp.Description("Destination path")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			src, err := req.RequireString("source_path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			dst, err := req.RequireString("destination_path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if err := client.MoveItem(ctx, src, dst); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(map[string]any{
				"source":      client.ResolvePath(src),
				"destination": client.ResolvePath(dst),
				"status":      "moved",
			})
		},
	)

	// 10. yandex_upload_file
	s.AddTool(
		mcp.NewTool("yandex_upload_file",
			mcp.WithDescription("Upload a file from local computer filesystem to Yandex Disk."),
			mcp.WithString("local_path", mcp.Required(), mcp.Description("Path to local file")),
			mcp.WithString("remote_path", mcp.Required(), mcp.Description("Target path on Yandex Disk")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			localPath, err := req.RequireString("local_path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			remotePath, err := req.RequireString("remote_path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if err := client.UploadFile(ctx, localPath, remotePath); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(map[string]any{
				"local_path":  localPath,
				"remote_path": client.ResolvePath(remotePath),
				"status":      "uploaded",
			})
		},
	)

	// 11. yandex_download_file
	s.AddTool(
		mcp.NewTool("yandex_download_file",
			mcp.WithDescription("Download a file from Yandex Disk to local computer filesystem."),
			mcp.WithString("remote_path", mcp.Required(), mcp.Description("Path to file on Yandex Disk")),
			mcp.WithString("local_path", mcp.Required(), mcp.Description("Destination path on local filesystem")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			remotePath, err := req.RequireString("remote_path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			localPath, err := req.RequireString("local_path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if err := client.DownloadFile(ctx, remotePath, localPath); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(map[string]any{
				"remote_path": client.ResolvePath(remotePath),
				"local_path":  localPath,
				"status":      "downloaded",
			})
		},
	)

	// 12. yandex_search_files
	s.AddTool(
		mcp.NewTool("yandex_search_files",
			mcp.WithDescription("Search for files matching a wildcard pattern (e.g. '*.pdf', '*report*') on Yandex Disk."),
			mcp.WithString("pattern", mcp.Required(), mcp.Description("Wildcard pattern to match")),
			mcp.WithString("path", mcp.Description("Subdirectory to search inside (empty for root)")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			pattern, err := req.RequireString("pattern")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			path := req.GetString("path", "")

			items, err := client.ListDirectory(ctx, path, true, 0, false, false, pattern)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(map[string]any{
				"search_pattern": pattern,
				"search_path":    client.ResolvePath(path),
				"count":          len(items),
				"items":          items,
			})
		},
	)

	// 13. yandex_get_storage_info
	s.AddTool(
		mcp.NewTool("yandex_get_storage_info",
			mcp.WithDescription("Get Yandex Disk storage quota: total, used, free space in bytes and human-readable units."),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			info, err := client.GetStorageInfo(ctx)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(info)
		},
	)

	// 14. yandex_create_public_link
	s.AddTool(
		mcp.NewTool("yandex_create_public_link",
			mcp.WithDescription("Create a public sharing link (yadi.sk) for a file or folder on Yandex Disk."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Remote path to file or folder")),
			mcp.WithString("expire", mcp.Description("Optional link expiration (e.g. '1d', '7d')")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			path, err := req.RequireString("path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			expire := req.GetString("expire", "")

			link, err := client.CreatePublicLink(ctx, path, expire)
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(map[string]any{
				"path":       client.ResolvePath(path),
				"public_url": link,
				"expire":     expire,
				"status":     "link_created",
			})
		},
	)

	// 15. yandex_remove_public_link
	s.AddTool(
		mcp.NewTool("yandex_remove_public_link",
			mcp.WithDescription("Revoke/remove public sharing link for a file or folder on Yandex Disk."),
			mcp.WithString("path", mcp.Required(), mcp.Description("Remote path")),
		),
		func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			path, err := req.RequireString("path")
			if err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}

			if err := client.RemovePublicLink(ctx, path); err != nil {
				return mcp.NewToolResultError(err.Error()), nil
			}
			return jsonResult(map[string]any{
				"path":   client.ResolvePath(path),
				"status": "link_removed",
			})
		},
	)
}
