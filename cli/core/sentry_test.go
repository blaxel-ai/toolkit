package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/charmbracelet/huh"
	"github.com/getsentry/sentry-go"
	"github.com/gorilla/websocket"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSentryDSN = "https://public@example.com/123"

func bindMockSentry(t *testing.T) *sentry.MockTransport {
	t.Helper()

	transport := &sentry.MockTransport{}
	client, err := sentry.NewClient(sentry.ClientOptions{
		Dsn:              testSentryDSN,
		Release:          "0.1.106-test",
		Environment:      "prod",
		AttachStacktrace: true,
		BeforeSend:       sanitizeSentryEvent,
		Transport:        transport,
	})
	require.NoError(t, err)

	hub := sentry.CurrentHub()
	originalDSN := SentryDSN
	hub.PushScope()
	hub.BindClient(client)
	SentryDSN = testSentryDSN
	t.Cleanup(func() {
		hub.PopScope()
		SentryDSN = originalDSN
	})

	return transport
}

func unknownFlagError(t *testing.T) error {
	t.Helper()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	return flags.Parse([]string{"--definitely-unknown"})
}

func missingFlagValueError(t *testing.T) error {
	t.Helper()
	flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
	flags.String("name", "", "test flag")
	return flags.Parse([]string{"--name"})
}

func TestSentryConfigStruct(t *testing.T) {
	cfg := SentryConfig{DSN: testSentryDSN, Release: "v1.0.0"}
	assert.Equal(t, testSentryDSN, cfg.DSN)
	assert.Equal(t, "v1.0.0", cfg.Release)
}

func TestInitSentryWithEmptyDSN(t *testing.T) {
	err := InitSentry(SentryConfig{Release: "v1.0.0"})
	require.NoError(t, err)
	assert.Empty(t, SentryDSN)
}

func TestInitSentryDoesNotEnableCaptureWhenInitializationFails(t *testing.T) {
	err := InitSentry(SentryConfig{DSN: "://invalid", Release: "v1.0.0"})
	require.Error(t, err)
	assert.Empty(t, SentryDSN)
}

func TestNormalizeEnvironment(t *testing.T) {
	assert.Equal(t, "dev", normalizeEnvironment("dev"))
	assert.Equal(t, "prod", normalizeEnvironment("prod"))
	assert.Equal(t, "prod", normalizeEnvironment("customer-workspace"))
}

func TestExpectedErrorClassification(t *testing.T) {
	missingFile := &os.PathError{Op: "open", Path: "/private/customer/file", Err: os.ErrNotExist}
	_, missingManifest := GetResults("apply", filepath.Join(t.TempDir(), "missing.yaml"), false)
	require.Error(t, missingManifest)
	apiNotFound := &blaxel.Error{StatusCode: 404}
	positional := cobra.ExactArgs(1)(&cobra.Command{Use: "test"}, nil)

	tests := []struct {
		name     string
		err      error
		category CLIErrorCategory
	}{
		{name: "unknown flag", err: unknownFlagError(t), category: CLIErrorUsage},
		{name: "missing flag value", err: missingFlagValueError(t), category: CLIErrorUsage},
		{name: "missing argument", err: positional, category: CLIErrorUsage},
		{name: "missing file", err: missingFile, category: CLIErrorNotFound},
		{name: "wrapped missing manifest", err: missingManifest, category: CLIErrorNotFound},
		{name: "existing file", err: fmt.Errorf("create file: %w", fs.ErrExist), category: CLIErrorConflict},
		{name: "file permission", err: fmt.Errorf("open file: %w", fs.ErrPermission), category: CLIErrorOperational},
		{name: "process exit", err: &exec.ExitError{}, category: CLIErrorOperational},
		{name: "missing executable", err: &exec.Error{Name: "missing", Err: fs.ErrNotExist}, category: CLIErrorNotFound},
		{name: "API not found", err: fmt.Errorf("request failed: %w", apiNotFound), category: CLIErrorNotFound},
		{name: "authentication", err: fmt.Errorf("request failed: %w", &blaxel.Error{StatusCode: 401}), category: CLIErrorAuthentication},
		{name: "deadline", err: context.DeadlineExceeded, category: CLIErrorOperational},
		{name: "handled HTTP response", err: MarkExpectedHTTPError(errors.New("request failed (HTTP 429): private body"), 429), category: CLIErrorOperational},
		{name: "explicit validation", err: MarkExpectedError(errors.New("bad config"), CLIErrorValidation), category: CLIErrorValidation},
		{name: "cancelled login", err: huh.ErrUserAborted, category: CLIErrorUsage},
		{name: "missing credentials", err: MarkExpectedError(errors.New("no valid credentials found. Please run 'bl login' first"), CLIErrorAuthentication), category: CLIErrorAuthentication},
		{name: "denied device authorization", err: MarkExpectedError(errors.New("authentication failed with status 400: access_denied"), CLIErrorAuthentication), category: CLIErrorAuthentication},
		{name: "image build failure", err: MarkExpectedError(errors.New("image build failed"), CLIErrorOperational), category: CLIErrorOperational},
		{name: "deployment failure", err: MarkExpectedError(errors.New("deployment failed for /snapshot: apply returned no results"), CLIErrorOperational), category: CLIErrorOperational},
		{name: "non-interactive command", err: MarkExpectedError(errors.New("this command requires an interactive terminal"), CLIErrorUsage), category: CLIErrorUsage},
		{name: "websocket handshake", err: fmt.Errorf("failed to connect to terminal: %w", websocket.ErrBadHandshake), category: CLIErrorOperational},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			classification := classifyCLIError(test.err)
			assert.True(t, classification.expected)
			assert.Equal(t, test.category, classification.category)
		})
	}
}

func TestMarkExpectedErrorPreservesLocalMessageAndCause(t *testing.T) {
	cause := errors.New("local detail remains visible")
	marked := MarkExpectedError(cause, CLIErrorValidation)
	require.Error(t, marked)
	assert.Equal(t, cause.Error(), marked.Error())
	assert.ErrorIs(t, marked, cause)
	assert.True(t, IsExpectedCLIError(marked))
	assert.False(t, IsExpectedCLIError(cause))
	assert.False(t, IsExpectedCLIError(nil))
	assert.Nil(t, MarkExpectedError(nil, CLIErrorValidation))
	assert.Same(t, cause, MarkExpectedError(cause, CLIErrorInternal))
	assert.Same(t, cause, MarkExpectedError(cause, CLIErrorPanic))
	assert.Same(t, cause, MarkExpectedHTTPError(cause, 200))
}

func TestExpectedErrorsCreateNoSentryEvents(t *testing.T) {
	transport := bindMockSentry(t)

	expectedErrors := []error{
		unknownFlagError(t),
		cobra.ExactArgs(1)(&cobra.Command{Use: "test"}, nil),
		&os.PathError{Op: "open", Path: "/private/customer/file", Err: os.ErrNotExist},
		fmt.Errorf("API request: %w", &blaxel.Error{StatusCode: 404}),
		MarkExpectedError(errors.New("permission denied: please login"), CLIErrorAuthentication),
		MarkExpectedHTTPError(errors.New("raw response body"), 503),
		huh.ErrUserAborted,
		MarkExpectedError(errors.New("no valid credentials found. Please run 'bl login' first"), CLIErrorAuthentication),
		MarkExpectedError(errors.New("image build failed"), CLIErrorOperational),
		MarkExpectedError(errors.New("this command requires an interactive terminal"), CLIErrorUsage),
		fmt.Errorf("failed to connect to terminal: %w", websocket.ErrBadHandshake),
	}

	for _, err := range expectedErrors {
		assert.False(t, captureUnexpectedError(err))
	}
	assert.Empty(t, transport.Events())
}

func TestUnmarkedLookalikeErrorsRemainReportable(t *testing.T) {
	transport := bindMockSentry(t)

	lookalikes := []error{
		errors.New("internal index not found"),
		errors.New("internal operation timed out"),
		errors.New("invalid internal state"),
		errors.New("permission denied while loading internal state"),
		errors.New("client not initialized"),
		errors.New("user aborted"),
		errors.New("websocket: bad handshake"),
	}

	for _, err := range lookalikes {
		assert.True(t, captureUnexpectedError(err))
	}

	events := transport.Events()
	require.Len(t, events, len(lookalikes))
	for _, event := range events {
		assert.Equal(t, "internal", event.Tags["error.category"])
		assert.Equal(t, "Unexpected CLI failure", event.Exception[0].Value)
	}
}

func TestUnexpectedErrorCreatesOneSanitizedEvent(t *testing.T) {
	transport := bindMockSentry(t)
	SetSentryTag("version", "0.1.106")
	SetSentryTag("commit", "abc1234")
	SetSentryTag("command.class", "bl deploy")
	SetSentryTag("workspace", "private-workspace")
	sentry.ConfigureScope(func(scope *sentry.Scope) {
		scope.SetTag("commit", "/Users/customer/private-project")
		scope.SetUser(sentry.User{ID: "private-user"})
		scope.SetContext("private-context", sentry.Context{"resource": "private-resource"})
		scope.SetFingerprint([]string{"private-fingerprint"})
		scope.SetRequest(httptest.NewRequest("POST", "https://example.com/private-resource", strings.NewReader("private-body")))
		scope.AddAttachment(&sentry.Attachment{Filename: "private.txt", Payload: []byte("private-attachment")})
		scope.AddBreadcrumb(&sentry.Breadcrumb{Message: "private-breadcrumb"}, 10)
	})

	err := errors.New("internal invariant failed for /Users/customer/private-project and token secret-123")
	assert.True(t, captureUnexpectedError(err))

	events := transport.Events()
	require.Len(t, events, 1)
	event := events[0]
	require.Len(t, event.Exception, 1)
	assert.Equal(t, "CLIInternalError", event.Exception[0].Type)
	assert.Equal(t, "Unexpected CLI failure", event.Exception[0].Value)
	assert.Equal(t, "internal", event.Tags["error.category"])
	assert.Equal(t, "bl deploy", event.Tags["command.class"])
	assert.Equal(t, "0.1.106", event.Tags["version"])
	assert.Equal(t, "unknown", event.Tags["commit"])
	assert.NotContains(t, event.Tags, "workspace")
	assert.Empty(t, event.ServerName)
	assert.Nil(t, event.Request)
	assert.Empty(t, event.User)
	assert.Nil(t, event.Attachments)
	assert.Nil(t, event.Fingerprint)
	assert.Nil(t, event.Contexts)
	assert.Nil(t, event.DebugMeta)

	for _, frame := range event.Exception[0].Stacktrace.Frames {
		assert.Empty(t, frame.AbsPath)
		assert.False(t, strings.Contains(frame.Filename, "/"))
		assert.True(t, strings.HasPrefix(frame.Module, "github.com/blaxel-ai/toolkit/"))
		assert.Nil(t, frame.Vars)
		assert.Nil(t, frame.PreContext)
		assert.Empty(t, frame.ContextLine)
		assert.Nil(t, frame.PostContext)
	}

	serialized, marshalErr := json.Marshal(event)
	require.NoError(t, marshalErr)
	assert.NotContains(t, string(serialized), "private-project")
	assert.NotContains(t, string(serialized), "secret-123")
	assert.NotContains(t, string(serialized), "private-workspace")
	assert.NotContains(t, string(serialized), "private-user")
	assert.NotContains(t, string(serialized), "private-resource")
	assert.NotContains(t, string(serialized), "private-fingerprint")
	assert.NotContains(t, string(serialized), "private-body")
	assert.NotContains(t, string(serialized), "private-attachment")
	assert.NotContains(t, string(serialized), "private-breadcrumb")
	assert.NotContains(t, string(serialized), "/Users/customer")
}

func TestCaptureExceptionWithNil(t *testing.T) {
	transport := bindMockSentry(t)
	CaptureException(nil)
	assert.Empty(t, transport.Events())
}

func TestFlushSentryWithEmptyDSN(t *testing.T) {
	SentryDSN = ""
	FlushSentry(time.Second)
}

func TestSetSentryTagWithEmptyDSN(t *testing.T) {
	SentryDSN = ""
	SetSentryTag("version", "v1")
}

func TestRecoverWithSentryEmptyDSN(t *testing.T) {
	SentryDSN = ""
	RecoverWithSentry()
}

func TestRecoverWithSentrySanitizesEventAndPreservesPanic(t *testing.T) {
	transport := bindMockSentry(t)
	const panicValue = "panic with private resource secret-123"

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		func() {
			defer RecoverWithSentry()
			panic(panicValue)
		}()
	}()

	assert.Equal(t, panicValue, recovered)
	events := transport.Events()
	require.Len(t, events, 1)
	require.Len(t, events[0].Exception, 1)
	assert.Equal(t, "panic", events[0].Tags["error.category"])
	assert.Equal(t, "Unexpected CLI failure", events[0].Exception[0].Value)
	serialized, err := json.Marshal(events[0])
	require.NoError(t, err)
	assert.NotContains(t, string(serialized), "secret-123")
	assert.NotContains(t, string(serialized), "private resource")
}
