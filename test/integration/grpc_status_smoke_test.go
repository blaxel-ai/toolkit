package integration

import (
	"errors"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// google.golang.org/grpc enters this module only transitively, through
// github.com/google/goexpect (used by TestCreateCommands_TTYAndNoTTY, which needs
// BL_API_KEY and is skipped in CI). goexpect itself only calls status.Errorf and
// carries a codes.Code value (see expect.go), and go list -deps confirms neither
// the bl binary nor any test binary in this repo links grpc's server/transport or
// xds packages, which is where CVE-2026-84304 and GHSA-2v4p-qf9q-27wj actually
// live. This test round-trips the one surface that is reachable, on the upgraded
// v1.83.2, so a future regression there is caught even though the vulnerable
// surface itself is not.
func TestGRPCStatusRoundTrip(t *testing.T) {
	err := status.Errorf(codes.Unimplemented, "only process Expecters supported")

	got, ok := status.FromError(err)
	if !ok {
		t.Fatalf("status.FromError: expected ok=true for a status error")
	}
	if got.Code() != codes.Unimplemented {
		t.Errorf("Code() = %v, want %v", got.Code(), codes.Unimplemented)
	}
	if got.Message() != "only process Expecters supported" {
		t.Errorf("Message() = %q, want %q", got.Message(), "only process Expecters supported")
	}

	if status.Code(errors.New("plain error")) != codes.Unknown {
		t.Errorf("status.Code() on a non-status error should be Unknown")
	}
}
