package deploy

import (
	"encoding/json"
	"testing"
)

func TestBuildProgress(t *testing.T) {
	tests := []struct {
		name   string
		events string
		want   string
		ok     bool
	}{
		{
			name: "newest progress event",
			events: `[
				{"type":"ai.blaxel.controlplane.buildimage.created","message":"Build queued: waiting for a builder","time":"2026-11-18T20:00:00Z"},
				{"type":"ai.blaxel.controlplane.buildimage.created","message":"Build started on a builder with 8192 MB of memory","time":"2026-11-18T20:00:05Z"}
			]`,
			want: "Build started on a builder with 8192 MB of memory",
			ok:   true,
		},
		{
			name: "ordered by time, not position",
			events: `[
				{"type":"ai.blaxel.controlplane.buildimage.created","message":"Build waiting for capacity: builds in progress reserve 32768 MB of 32768 MB.","time":"2026-11-18T20:00:05Z"},
				{"type":"ai.blaxel.controlplane.buildimage.created","message":"Build queued: waiting for a builder","time":"2026-11-18T20:00:00Z"}
			]`,
			want: "Build waiting for capacity: builds in progress reserve 32768 MB of 32768 MB.",
			ok:   true,
		},
		{
			name:   "the build's end is not progress",
			events: `[{"type":"ai.blaxel.controlplane.buildimage.failed","message":"the build failed","time":"2026-11-18T20:00:00Z"}]`,
		},
		{
			name:   "initiated only",
			events: `[{"type":"ai.blaxel.controlplane.buildimage.initiated","message":"Image build initiated","time":"2026-11-18T20:00:00Z"}]`,
		},
		{
			name:   "first line only",
			events: `[{"type":"ai.blaxel.controlplane.buildimage.created","message":"Build queued: waiting for a builder\nmore","time":"2026-11-18T20:00:00Z"}]`,
			want:   "Build queued: waiting for a builder",
			ok:     true,
		},
		{
			name:   "infrastructure detail is never shown",
			events: `[{"type":"ai.blaxel.controlplane.buildimage.created","message":"operation error SQS: SendMessage","time":"2026-11-18T20:00:00Z"}]`,
		},
		{
			name:   "control characters are never shown",
			events: `[{"type":"ai.blaxel.controlplane.buildimage.created","message":"Build queued\u001b[2J","time":"2026-11-18T20:00:00Z"}]`,
		},
		{name: "no events", events: `[]`},
		{name: "malformed", events: `{}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := BuildProgress(json.RawMessage(tt.events))
			if got != tt.want || ok != tt.ok {
				t.Fatalf("BuildProgress() = %q, %v; want %q, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}
