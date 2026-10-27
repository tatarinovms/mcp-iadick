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
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Re-execute the test binary as a small, portable rclone stand-in. No cloud
// credentials or shell are needed, including on Windows.
func TestMain(m *testing.M) {
	if os.Getenv("MCP_IADICK_TEST_HELPER") == "1" {
		helperCommand()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func helperCommand() {
	if target := os.Getenv("MCP_IADICK_TEST_ARGS"); target != "" {
		data, _ := json.Marshal(os.Args[1:])
		if err := os.WriteFile(target, data, 0600); err != nil {
			panic(err)
		}
	}
	switch os.Args[1] {
	case "echo":
		_, _ = io.Copy(os.Stdout, os.Stdin)
	case "fail":
		fmt.Fprint(os.Stderr, "backend unavailable")
		os.Exit(7)
	case "stderr":
		_, _ = io.CopyN(os.Stderr, strings.NewReader(strings.Repeat("x", MaxStderrBytes+1024)), MaxStderrBytes+1024)
		os.Exit(7)
	case "flood":
		chunk := bytes.Repeat([]byte("x"), 32*1024)
		for {
			if _, err := os.Stdout.Write(chunk); err != nil {
				os.Exit(1)
			}
		}
	case "wait":
		if err := os.WriteFile(os.Args[2], []byte("ready"), 0600); err != nil {
			panic(err)
		}
		time.Sleep(30 * time.Second)
	case "cat":
		data := "0123456789"
		offset, count := 0, len(data)
		for i := 3; i+1 < len(os.Args); i += 2 {
			n, _ := strconv.Atoi(os.Args[i+1])
			switch os.Args[i] {
			case "--offset":
				offset = n
			case "--count":
				count = n
			}
		}
		if offset > len(data) {
			offset = len(data)
		}
		end := min(offset+count, len(data))
		fmt.Fprint(os.Stdout, data[offset:end])
	case "lsjson":
		data := os.Getenv("MCP_IADICK_TEST_LIST")
		if data == "stream" {
			fmt.Fprint(os.Stdout, `[{"Path":"a"},{"Path":"b"},{"Path":"c"},`)
			time.Sleep(30 * time.Second)
		} else {
			fmt.Fprint(os.Stdout, data)
		}
		if os.Getenv("MCP_IADICK_TEST_FAIL") == "1" {
			fmt.Fprint(os.Stderr, "listing failed")
			os.Exit(7)
		}
	case "rcat":
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			panic(err)
		}
		if target := os.Getenv("MCP_IADICK_TEST_CONTENT"); target != "" {
			if err := os.WriteFile(target, data, 0600); err != nil {
				panic(err)
			}
		}
	default:
		fmt.Fprint(os.Stderr, "unexpected test command: "+os.Args[1])
		os.Exit(9)
	}
}

func helperClient(t *testing.T) *Client {
	t.Helper()
	t.Setenv("MCP_IADICK_TEST_HELPER", "1")
	t.Setenv("GORACE", "atexit_sleep_ms=0")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return &Client{Remote: "yandex", RclonePath: exe, OperationTimeout: 5 * time.Second}
}

func TestRunStreamsInputAndReturnsErrors(t *testing.T) {
	c := helperClient(t)
	got, err := c.Run(context.Background(), []byte("hello"), "echo")
	if err != nil || string(got) != "hello" {
		t.Fatalf("echo = %q, %v", got, err)
	}
	_, err = c.Run(context.Background(), nil, "fail")
	if err == nil || !strings.Contains(err.Error(), "backend unavailable") {
		t.Fatalf("missing process diagnostic: %v", err)
	}
	_, err = c.Run(context.Background(), nil, "stderr")
	if err == nil || !strings.Contains(err.Error(), "[stderr truncated]") || len(err.Error()) > MaxStderrBytes+200 {
		t.Fatalf("stderr was not bounded: %v", err)
	}
}

func TestRunBoundsOutput(t *testing.T) {
	c := helperClient(t)
	c.MaxConcurrent = 1
	_, err := c.Run(context.Background(), nil, "flood")
	if err == nil || !strings.Contains(err.Error(), "output exceeds") {
		t.Fatalf("unbounded stdout: %v", err)
	}
	// An output-limit failure must release the process slot.
	if _, err := c.Run(context.Background(), nil, "echo"); err != nil {
		t.Fatal(err)
	}
}

func TestRunOperationTimeout(t *testing.T) {
	c := helperClient(t)
	c.OperationTimeout = 250 * time.Millisecond
	_, err := c.Run(context.Background(), nil, "wait", filepath.Join(t.TempDir(), "ready"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error = %v", err)
	}
}

func TestOperationLogsOmitPathsAndReportCompletedPages(t *testing.T) {
	c := helperClient(t)
	c.LogOperations = true
	var logs bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&logs)
	t.Cleanup(func() { log.SetOutput(previous) })
	if _, err := c.Run(context.Background(), []byte("secret-content"), "echo", "secret-path"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MCP_IADICK_TEST_LIST", "stream")
	if _, err := c.ListDirectory(context.Background(), "secret-path", ListOptions{Limit: 1}); err != nil {
		t.Fatal(err)
	}
	got := logs.String()
	if strings.Contains(got, "secret-") || !strings.Contains(got, "command=echo") || !strings.Contains(got, "result=page_complete") {
		t.Fatalf("unexpected operation logs: %q", got)
	}
}

func TestRunLimitsConcurrencyAndHonorsQueuedCancellation(t *testing.T) {
	c := helperClient(t)
	c.MaxConcurrent = 1
	firstReady := filepath.Join(t.TempDir(), "first")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := c.Run(ctx, nil, "wait", firstReady)
		done <- err
	}()
	defer func() {
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Errorf("running command cancellation = %v", err)
		}
	}()
	deadline := time.Now().Add(3 * time.Second)
	for {
		if _, err := os.Stat(firstReady); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("first process did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	secondReady := filepath.Join(t.TempDir(), "second")
	queuedCtx, stop := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer stop()
	_, err := c.Run(queuedCtx, nil, "wait", secondReady)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("queued cancellation = %v", err)
	}
	if _, err := os.Stat(secondReady); !os.IsNotExist(err) {
		t.Fatalf("queued process started despite concurrency limit: %v", err)
	}
}

func TestReadFileRangeAndTruncation(t *testing.T) {
	c := helperClient(t)
	for _, tc := range []struct {
		name          string
		limit, offset int
		want          string
		truncated     bool
	}{
		{"middle", 3, 2, "234", true},
		{"exact EOF", 3, 7, "789", false},
		{"past EOF", 3, 20, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := c.ReadFile(context.Background(), "file.txt", tc.limit, tc.offset)
			if err != nil {
				t.Fatal(err)
			}
			if got.Content != tc.want || got.IsTruncated != tc.truncated || got.BytesRead != len(tc.want) || got.Offset != tc.offset {
				t.Fatalf("unexpected read: %+v", got)
			}
		})
	}
	got, err := c.ReadFileBase64(context.Background(), "file.bin", 3, 2)
	if err != nil || got.ContentBase64 != base64.StdEncoding.EncodeToString([]byte("234")) || !got.IsTruncated {
		t.Fatalf("base64 read = %+v, %v", got, err)
	}
}

func TestReadRejectsInvalidLimitsBeforeStartingProcess(t *testing.T) {
	c := &Client{RclonePath: "does-not-exist"}
	for _, tc := range []struct{ count, offset int }{
		{0, 0}, {-1, 0}, {MaxReadBytes + 1, 0}, {1, -1},
	} {
		_, _, err := c.ReadFileRaw(context.Background(), "file", tc.count, tc.offset)
		if err == nil || (!strings.Contains(err.Error(), "max_bytes") && !strings.Contains(err.Error(), "offset")) {
			t.Errorf("ReadFileRaw(%d,%d) = %v", tc.count, tc.offset, err)
		}
	}
}

func TestListDirectoryPaginationAndDepth(t *testing.T) {
	c := helperClient(t)
	t.Setenv("MCP_IADICK_TEST_LIST", `[{"Path":"a","Size":1},{"Path":"b","IsDir":true},{"Path":"c","Size":1024},{"Path":"d"}]`)
	argsPath := filepath.Join(t.TempDir(), "args.json")
	t.Setenv("MCP_IADICK_TEST_ARGS", argsPath)
	page, err := c.ListDirectory(context.Background(), "docs", ListOptions{
		Recursive: true, MaxDepth: 2, FilesOnly: true, Pattern: "*.pdf", Limit: 2, Offset: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if page.Count != 2 || !page.HasMore || page.NextOffset == nil || *page.NextOffset != 3 || page.Items[0].Path != "b" || page.Items[1].SizeHuman != "1.00 KB" {
		t.Fatalf("unexpected page: %+v", page)
	}
	args, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	if err := json.Unmarshal(args, &got); err != nil {
		t.Fatal(err)
	}
	want := []string{"lsjson", "yandex:docs", "--recursive", "--max-depth", "2", "--files-only", "--include", "*.pdf"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("args = %v, want %v", got, want)
	}
	page, err = c.ListDirectory(context.Background(), "docs", ListOptions{Limit: 2, Offset: 3})
	if err != nil || page.HasMore || page.NextOffset != nil || page.Count != 1 || page.Items[0].Path != "d" {
		t.Fatalf("last page = %+v, %v", page, err)
	}
}

func TestListDirectoryStopsAfterPage(t *testing.T) {
	c := helperClient(t)
	t.Setenv("MCP_IADICK_TEST_LIST", "stream")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	page, err := c.ListDirectory(ctx, "", ListOptions{Limit: 2})
	if err != nil || page.Count != 2 || !page.HasMore {
		t.Fatalf("stream page = %+v, %v", page, err)
	}
}

func TestListDirectoryRejectsMalformedOutputAndProcessFailure(t *testing.T) {
	c := helperClient(t)
	for _, data := range []string{"", "null", "{}", "[", "[{}", "[{}] trailing", "[{}] {}"} {
		t.Setenv("MCP_IADICK_TEST_LIST", data)
		if _, err := c.ListDirectory(context.Background(), "", ListOptions{Limit: 2}); err == nil {
			t.Errorf("accepted malformed list %q", data)
		}
	}
	t.Setenv("MCP_IADICK_TEST_LIST", "[]")
	t.Setenv("MCP_IADICK_TEST_FAIL", "1")
	_, err := c.ListDirectory(context.Background(), "", ListOptions{Limit: 2})
	if err == nil || !strings.Contains(err.Error(), "listing failed") {
		t.Fatalf("lost process error after valid JSON: %v", err)
	}
}

func TestListDirectoryValidatesOptions(t *testing.T) {
	c := &Client{RclonePath: "does-not-exist"}
	for _, opts := range []ListOptions{
		{Limit: -1}, {Limit: MaxListLimit + 1}, {Offset: -1}, {MaxDepth: -1}, {DirsOnly: true, FilesOnly: true},
	} {
		if _, err := c.ListDirectory(context.Background(), "", opts); err == nil || strings.Contains(err.Error(), "executable") {
			t.Errorf("invalid options %+v: %v", opts, err)
		}
	}
}

func TestUploadsValidateContentBeforeWriting(t *testing.T) {
	c := helperClient(t)
	target := filepath.Join(t.TempDir(), "content")
	t.Setenv("MCP_IADICK_TEST_CONTENT", target)
	if err := c.WriteTextFile(context.Background(), "file", "hello"); err != nil {
		t.Fatal(err)
	}
	if n, err := c.UploadBase64(context.Background(), "file", "aGVsbG8"); err != nil || n != 5 {
		t.Fatalf("raw base64 upload = %d, %v", n, err)
	}
	if _, err := c.UploadBase64(context.Background(), "file", "aGVsbG8=!!!!"); err == nil {
		t.Fatal("accepted invalid base64")
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "hello" {
		t.Fatalf("invalid upload modified destination: %q, %v", data, err)
	}
	if err := c.WriteFile(context.Background(), "file", bytes.Repeat([]byte("x"), MaxUploadBytes+1)); err == nil {
		t.Fatal("accepted oversized upload")
	}
	if err := c.WriteTextFile(context.Background(), "file", ""); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(target)
	if err != nil || len(data) != 0 {
		t.Fatalf("empty upload = %q, %v", data, err)
	}
}

func TestBase64RejectsOversizedInputAndDecodedContent(t *testing.T) {
	if _, err := decodeBase64(strings.Repeat(" ", MaxBase64InputBytes+1)); err == nil {
		t.Fatal("accepted oversized input")
	}
	encoded := base64.StdEncoding.EncodeToString(make([]byte, MaxUploadBytes+1))
	if _, err := decodeBase64(encoded); err == nil {
		t.Fatal("accepted oversized decoded content")
	}
}

func BenchmarkDecodeListPage(b *testing.B) {
	var data strings.Builder
	data.WriteString("[")
	for i := 0; i < 10000; i++ {
		if i > 0 {
			data.WriteString(",")
		}
		fmt.Fprintf(&data, `{"Path":"file-%d","Size":1024}`, i)
	}
	data.WriteString("]")
	payload := data.String()
	b.Run("stream_page_100", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			if _, err := decodeListPage(strings.NewReader(payload), 100, 0); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("previous_full_list_10000", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			var items []Item
			if err := json.Unmarshal([]byte(payload), &items); err != nil {
				b.Fatal(err)
			}
			for j := range items {
				items[j].SizeHuman = FormatBytes(&items[j].Size)
			}
		}
	})
}

func TestLocalRcloneListing(t *testing.T) {
	bin, err := exec.LookPath("rclone")
	if err != nil {
		t.Skip("rclone is not installed; subprocess behavior is covered by the helper tests")
	}
	// The on-the-fly local backend does not read cloud credentials or access Disk.
	t.Setenv("RCLONE_CONFIG", os.DevNull)
	root := t.TempDir()
	for _, path := range []string{"a.txt", "nested/b.txt", "nested/deep/c.txt"} {
		target := filepath.Join(root, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte("hello"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	c := &Client{RclonePath: bin}
	target := ":local:" + filepath.ToSlash(root)
	page, err := c.ListDirectory(context.Background(), target, ListOptions{
		Recursive: true, MaxDepth: 2, FilesOnly: true, Limit: 100,
	})
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, item := range page.Items {
		paths = append(paths, item.Path)
	}
	slices.Sort(paths)
	if !slices.Equal(paths, []string{"a.txt", "nested/b.txt"}) {
		t.Fatalf("recursive depth did not restrict the listing: %v", paths)
	}
	page, err = c.ListDirectory(context.Background(), target, ListOptions{Recursive: true, Limit: 1})
	if err != nil || page.Count != 1 || !page.HasMore {
		t.Fatalf("real rclone page = %+v, %v", page, err)
	}
}
