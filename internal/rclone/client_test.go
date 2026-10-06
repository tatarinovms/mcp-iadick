package rclone

import (
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

func int64Ptr(v int64) *int64 {
	return &v
}
