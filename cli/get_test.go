package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
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

// A 2xx response whose body cannot be read or parsed surfaces from the shared
// SDK executor as a bare error with no HTTP status and no net.Error type.
// Those are backend/transport conditions and must not be reported as CLI
// defects, so classifyGetResponseError marks them operational (expected).
func TestClassifyGetResponseErrorMarksTransportFailuresExpected(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{
			name: "parse failure",
			err:  errors.New("error parsing response json: invalid character '<' looking for beginning of value"),
		},
		{
			name: "parse failure wrapped",
			err:  fmt.Errorf("error parsing response json: %w", io.ErrUnexpectedEOF),
		},
		{
			name: "body read failure",
			err:  fmt.Errorf("error reading response body: %w", io.ErrUnexpectedEOF),
		},
		{
			name: "non-json destination mismatch",
			err:  errors.New("expected destination type of 'string' or '[]byte' for responses with content-type 'text/html' that is not 'application/json'"),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			classified := classifyGetResponseError(tc.err)
			if !core.IsExpectedCLIError(classified) {
				t.Fatalf("expected %q to be classified as an expected CLI error", tc.name)
			}
			if classified.Error() != tc.err.Error() {
				t.Fatalf("expected user-facing message to be unchanged, got %q want %q", classified.Error(), tc.err.Error())
			}
		})
	}
}

// Errors that already carry a specific classification keep their category and
// are returned unchanged (no operational downgrade).
func TestClassifyGetResponseErrorPreservesTypedClassification(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{name: "not found", err: &blaxel.Error{StatusCode: 404}},
		{name: "unauthorized", err: &blaxel.Error{StatusCode: 401}},
		{name: "context canceled", err: context.Canceled},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			classified := classifyGetResponseError(tc.err)
			if classified != tc.err {
				t.Fatalf("expected already-classified error to be returned unchanged")
			}
			if !core.IsExpectedCLIError(classified) {
				t.Fatalf("expected %q to remain an expected CLI error", tc.name)
			}
		})
	}
}

func TestClassifyGetResponseErrorNilIsPassthrough(t *testing.T) {
	if got := classifyGetResponseError(nil); got != nil {
		t.Fatalf("expected nil error to pass through, got %v", got)
	}
}

// Guard: without classifyGetResponseError, a bare transport/parse failure is
// unclassified and would be reported to Sentry as an unexpected CLI defect.
// This proves the classification call is load-bearing.
func TestGetResponseParseErrorUnclassifiedIsReported(t *testing.T) {
	raw := fmt.Errorf("error parsing response json: %w", io.ErrUnexpectedEOF)
	if core.IsExpectedCLIError(raw) {
		t.Fatalf("expected a bare parse error to be unclassified (reportable) before classification")
	}
}
