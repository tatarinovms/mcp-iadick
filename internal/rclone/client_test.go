package rclone

import (
	"bytes"
	"testing"
)

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		val  *int64
		want string
	}{
		{nil, "N/A"},
		{int64Ptr(-1), "N/A"},
		{int64Ptr(0), "0.00 B"},
		{int64Ptr(1024), "1.00 KB"},
		{int64Ptr(1024 * 1024), "1.00 MB"},
		{int64Ptr(1024 * 1024 * 1024 * 2), "2.00 GB"},
	}

	for _, tt := range tests {
		got := FormatBytes(tt.val)
		if got != tt.want {
			t.Errorf("FormatBytes(%v) = %q; want %q", tt.val, got, tt.want)
		}
	}
}

func TestResolvePath(t *testing.T) {
	client := &Client{Remote: "yandex"}

	if got := client.ResolvePath(""); got != "yandex:" {
		t.Errorf("ResolvePath('') = %q, want 'yandex:'", got)
	}
	if got := client.ResolvePath("/"); got != "yandex:" {
		t.Errorf("ResolvePath('/') = %q, want 'yandex:'", got)
	}
	if got := client.ResolvePath("Documents"); got != "yandex:Documents" {
		t.Errorf("ResolvePath('Documents') = %q, want 'yandex:Documents'", got)
	}
	if got := client.ResolvePath("/Documents/file.txt"); got != "yandex:Documents/file.txt" {
		t.Errorf("ResolvePath('/Documents/file.txt') = %q, want 'yandex:Documents/file.txt'", got)
	}
	if got := client.ResolvePath("yandex:Photos"); got != "yandex:Photos" {
		t.Errorf("ResolvePath('yandex:Photos') = %q, want 'yandex:Photos'", got)
	}
	if got := client.ResolvePath("other:backup"); got != "other:backup" {
		t.Errorf("ResolvePath('other:backup') = %q, want 'other:backup'", got)
	}
}

func TestDecodeBase64(t *testing.T) {
	// Standard base64
	data, err := decodeBase64("SGVsbG8gV29ybGQ=")
	if err != nil || string(data) != "Hello World" {
		t.Errorf("Failed standard base64: %v, %q", err, string(data))
	}

	// Data URI prefix
	data, err = decodeBase64("data:image/png;base64,SGVsbG8gV29ybGQ=")
	if err != nil || string(data) != "Hello World" {
		t.Errorf("Failed data uri base64: %v, %q", err, string(data))
	}

	// With newlines and spaces
	data, err = decodeBase64("  SGVsbG8\n  gV29ybGQ=\n ")
	if err != nil || string(data) != "Hello World" {
		t.Errorf("Failed whitespace base64: %v, %q", err, string(data))
	}

	// Binary payload (PNG signature)
	pngSig := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	b64Png := "iVBORw0KGgo="
	data, err = decodeBase64(b64Png)
	if err != nil || !bytes.Equal(data, pngSig) {
		t.Errorf("Failed binary base64: %v, got %x, want %x", err, data, pngSig)
	}

	// Invalid base64
	_, err = decodeBase64("not a base64 string!!!")
	if err == nil {
		t.Errorf("Expected error for invalid base64, got nil")
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
