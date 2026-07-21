package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/blaxel-ai/toolkit/cli/core"
)

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	orig := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	os.Stderr = orig
	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	return string(buf[:n])
}

func TestPrintCursorHint(t *testing.T) {
	resource := &core.Resource{Kind: "Agent", Plural: "agents"}
	result := core.PaginatedResult{
		Items: []any{1, 2},
		Meta:  core.PaginationMeta{HasMore: true, NextCursor: "abc", Total: 10},
	}

	for _, format := range []string{"json", "yaml"} {
		if out := captureStderr(t, func() { printCursorHint(resource, result, format) }); out != "" {
			t.Errorf("format %q: expected no hint, got %q", format, out)
		}
	}

	out := captureStderr(t, func() { printCursorHint(resource, result, "pretty") })
	if !strings.Contains(out, "--cursor abc") {
		t.Errorf("expected cursor hint, got %q", out)
	}

	noMore := core.PaginatedResult{Items: []any{1}, Meta: core.PaginationMeta{HasMore: false}}
	if out := captureStderr(t, func() { printCursorHint(resource, noMore, "pretty") }); out != "" {
		t.Errorf("expected no hint when HasMore is false, got %q", out)
	}
}
