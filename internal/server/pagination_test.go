package server

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tatarinovms/mcp-iadick/internal/rclone"
)

func TestMain(m *testing.M) {
	if os.Getenv("MCP_IADICK_SERVER_TEST_HELPER") == "1" {
		fmt.Fprint(os.Stdout, `[{"Path":"a.txt","Name":"a.txt","Size":1},{"Path":"b.txt","Name":"b.txt","Size":2},{"Path":"c.txt","Name":"c.txt","Size":3}]`)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestPaginatedListAndSearchHandlers(t *testing.T) {
	t.Setenv("MCP_IADICK_SERVER_TEST_HELPER", "1")
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	srv := NewServer(&rclone.Client{Remote: "yandex", RclonePath: exe})
	for _, name := range []string{"yandex_list_directory", "yandex_search_files"} {
		t.Run(name, func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Arguments = map[string]any{"path": "docs", "pattern": "*.txt", "limit": 1, "offset": 1, "fields": "Path, Size"}
			res, err := srv.GetTool(name).Handler(context.Background(), req)
			if err != nil || res.IsError {
				t.Fatalf("handler failed: %+v, %v", res, err)
			}
			content, ok := res.Content[0].(mcp.TextContent)
			if !ok {
				t.Fatalf("unexpected content: %+v", res.Content)
			}
			if strings.Contains(content.Text, "\n") {
				t.Fatal("JSON response is still pretty-printed")
			}
			var page struct {
				Count      int              `json:"count"`
				Offset     int              `json:"offset"`
				HasMore    bool             `json:"has_more"`
				NextOffset int              `json:"next_offset"`
				Items      []map[string]any `json:"items"`
			}
			if err := json.Unmarshal([]byte(content.Text), &page); err != nil {
				t.Fatal(err)
			}
			if page.Count != 1 || page.Offset != 1 || !page.HasMore || page.NextOffset != 2 || len(page.Items) != 1 {
				t.Fatalf("unexpected page: %+v", page)
			}
			if len(page.Items[0]) != 2 || page.Items[0]["Path"] != "b.txt" || page.Items[0]["Size"] != float64(2) {
				t.Fatalf("incorrect field projection: %+v", page.Items)
			}
			// Requesting the final page must not advertise another offset.
			req.Params.Arguments = map[string]any{"pattern": "*.txt", "offset": 3}
			res, err = srv.GetTool(name).Handler(context.Background(), req)
			if err != nil || res.IsError {
				t.Fatalf("last page failed: %+v, %v", res, err)
			}
			content = res.Content[0].(mcp.TextContent)
			if !strings.Contains(content.Text, `"items":[]`) || strings.Contains(content.Text, "next_offset") || !strings.Contains(content.Text, `"has_more":false`) {
				t.Fatalf("incorrect empty page: %s", content.Text)
			}
		})
	}
}

func TestReadAndListHandlersRejectInvalidParameters(t *testing.T) {
	srv := NewServer(&rclone.Client{RclonePath: "does-not-exist"})
	for _, tc := range []struct {
		name string
		args map[string]any
		want string
	}{
		{"yandex_read_file", map[string]any{"path": "x", "max_bytes": 0}, "max_bytes"},
		{"yandex_download_base64", map[string]any{"path": "x", "max_bytes": rclone.MaxReadBytes + 1}, "max_bytes"},
		{"yandex_read_file", map[string]any{"path": "x", "offset": -1}, "offset"},
		{"yandex_list_directory", map[string]any{"dirs_only": true, "files_only": true}, "dirs_only"},
		{"yandex_list_directory", map[string]any{"limit": rclone.MaxListLimit + 1}, "limit"},
		{"yandex_list_directory", map[string]any{"limit": 0}, "limit"},
		{"yandex_search_files", map[string]any{"pattern": "*", "limit": 0}, "limit"},
		{"yandex_search_files", map[string]any{"pattern": "*", "max_depth": -1}, "max_depth"},
		{"yandex_search_files", map[string]any{"pattern": "*", "fields": "unknown"}, "unknown item field"},
		{"yandex_list_directory", map[string]any{"fields": strings.Repeat("Path,", 100)}, "fields must not exceed"},
	} {
		t.Run(tc.name+tc.want, func(t *testing.T) {
			req := mcp.CallToolRequest{}
			req.Params.Arguments = tc.args
			res, err := srv.GetTool(tc.name).Handler(context.Background(), req)
			if err != nil || !res.IsError {
				t.Fatalf("expected parameter error: %+v, %v", res, err)
			}
			text := res.Content[0].(mcp.TextContent).Text
			if !strings.Contains(text, tc.want) {
				t.Fatalf("error = %q, want %q", text, tc.want)
			}
		})
	}
}
