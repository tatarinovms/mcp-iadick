package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestBodyLimit(t *testing.T) {
	for _, tc := range []struct {
		name   string
		length int64
		want   int
	}{
		{"small", 5, http.StatusOK},
		{"exact limit", maxRequestBodyBytes, http.StatusOK},
		{"known oversized", maxRequestBodyBytes + 1, http.StatusRequestEntityTooLarge},
		{"unknown oversized", -1, http.StatusRequestEntityTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true
				_, err := io.Copy(io.Discard, r.Body)
				var limitErr *http.MaxBytesError
				if errors.As(err, &limitErr) {
					w.WriteHeader(http.StatusRequestEntityTooLarge)
					return
				}
				if err != nil {
					t.Error(err)
				}
			})
			size := int(tc.length)
			if size < 0 {
				size = maxRequestBodyBytes + 1
			}
			req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader(strings.Repeat("x", size)))
			req.ContentLength = tc.length
			res := httptest.NewRecorder()
			limitRequestBody(next).ServeHTTP(res, req)
			if res.Code != tc.want {
				t.Fatalf("status = %d, want %d", res.Code, tc.want)
			}
			if tc.length > maxRequestBodyBytes && called {
				t.Fatal("oversized declared body reached the handler")
			}
		})
	}
}
