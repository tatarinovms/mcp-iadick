package server

import (
	"context"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tatarinovms/mcp-iadick/internal/rclone"
)

func TestServerToolsRegistered(t *testing.T) {
	client := &rclone.Client{Remote: "yandex"}
	srv := NewServer(client)

	tools := srv.ListTools()

	expected := []string{
		"yandex_list_directory",
		"yandex_get_file_info",
		"yandex_read_file",
		"yandex_write_file",
		"yandex_create_directory",
		"yandex_delete_file",
		"yandex_delete_directory",
		"yandex_copy_item",
		"yandex_move_item",
		"yandex_upload_file",
		"yandex_download_file",
		"yandex_search_files",
		"yandex_get_storage_info",
		"yandex_create_public_link",
		"yandex_remove_public_link",
		"yandex_upload_base64",
		"yandex_download_base64",
		"yandex_upload_from_url",
	}

	for _, exp := range expected {
		if _, ok := tools[exp]; !ok {
			t.Errorf("Expected tool %q to be registered, but not found", exp)
		}
	}
}

func TestServerMissingRequiredArg(t *testing.T) {
	client := &rclone.Client{Remote: "yandex"}
	srv := NewServer(client)

	tool := srv.GetTool("yandex_read_file")
	if tool == nil {
		t.Fatalf("Tool yandex_read_file not found")
	}

	// Call yandex_read_file without required 'path' parameter
	req := mcp.CallToolRequest{}
	req.Params.Name = "yandex_read_file"
	req.Params.Arguments = map[string]any{}

	res, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected execution error: %v", err)
	}
	if !res.IsError {
		t.Errorf("Expected error result for missing argument, got success: %v", res)
	}
}

func TestServerUploadBase64MissingArg(t *testing.T) {
	client := &rclone.Client{Remote: "yandex"}
	srv := NewServer(client)

	tool := srv.GetTool("yandex_upload_base64")
	if tool == nil {
		t.Fatalf("Tool yandex_upload_base64 not found")
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = "yandex_upload_base64"
	req.Params.Arguments = map[string]any{"remote_path": "test.png"}

	res, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected execution error: %v", err)
	}
	if !res.IsError {
		t.Errorf("Expected error result for missing content_base64, got success: %v", res)
	}
}

func TestServerUploadFromURLMissingArg(t *testing.T) {
	client := &rclone.Client{Remote: "yandex"}
	srv := NewServer(client)

	tool := srv.GetTool("yandex_upload_from_url")
	if tool == nil {
		t.Fatalf("Tool yandex_upload_from_url not found")
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = "yandex_upload_from_url"
	req.Params.Arguments = map[string]any{"url": "https://example.com/file.txt"}

	res, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("Unexpected execution error: %v", err)
	}
	if !res.IsError {
		t.Errorf("Expected error result for missing remote_path, got success: %v", res)
	}
}
