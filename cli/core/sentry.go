package core

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/charmbracelet/huh"
	"github.com/getsentry/sentry-go"
	"github.com/gorilla/websocket"
	"github.com/spf13/pflag"
)

// SentryDSN is the default Sentry DSN for the CLI.
var SentryDSN = ""

// SentryConfig holds the configuration for Sentry initialization.
type SentryConfig struct {
	DSN     string
	Release string
}

// CLIErrorCategory is a stable, non-sensitive telemetry classification.
type CLIErrorCategory string

const (
	CLIErrorUsage          CLIErrorCategory = "usage"
	CLIErrorValidation     CLIErrorCategory = "validation"
	CLIErrorAuthentication CLIErrorCategory = "authentication"
	CLIErrorNotFound       CLIErrorCategory = "not_found"
	CLIErrorConflict       CLIErrorCategory = "conflict"
	CLIErrorOperational    CLIErrorCategory = "operational"
	CLIErrorInternal       CLIErrorCategory = "internal"
	CLIErrorPanic          CLIErrorCategory = "panic"
)

type classifiedCLIError struct {
	category CLIErrorCategory
	cause    error
}

func (e *classifiedCLIError) Error() string { return e.cause.Error() }
func (e *classifiedCLIError) Unwrap() error { return e.cause }

func isExpectedCategory(category CLIErrorCategory) bool {
	switch category {
	case CLIErrorUsage,
		CLIErrorValidation,
		CLIErrorAuthentication,
		CLIErrorNotFound,
		CLIErrorConflict,
		CLIErrorOperational:
		return true
	default:
		return false
	}
}

// MarkExpectedError preserves the local error while explicitly excluding a
// handled usage, validation, authentication, or operational failure from
// error-level Sentry reporting.
func MarkExpectedError(err error, category CLIErrorCategory) error {
	if err == nil {
		return nil
	}
	if !isExpectedCategory(category) {
		return err
	}
	return &classifiedCLIError{category: category, cause: err}
}

// MarkExpectedHTTPError classifies a handled HTTP response without changing
// the error text shown to the user.
func MarkExpectedHTTPError(err error, statusCode int) error {
	if err == nil || statusCode < http.StatusBadRequest {
		return err
	}
	return MarkExpectedError(err, categoryForHTTPStatus(statusCode))
}

type sentryCLIError struct {
	category CLIErrorCategory
}

func (e *sentryCLIError) Error() string {
	return fmt.Sprintf("unexpected CLI failure (%s)", e.category)
}

type errorClassification struct {
	category CLIErrorCategory
	expected bool
}

var (
	cobraArgumentError = regexp.MustCompile(
		`^(?:accepts (?:at most )?\d+ arg\(s\)|accepts between \d+ and \d+ arg\(s\)|requires at least \d+ arg\(s\))(?:, .*)?$`,
	)
	cobraRequiredFlagError = regexp.MustCompile(`^required flag\(s\) ".+" not set$`)
	cobraUnknownCommand    = regexp.MustCompile(`^unknown command ".+" for ".+"$`)
)

func categoryForHTTPStatus(statusCode int) CLIErrorCategory {
	switch statusCode {
	case 400, 405, 406, 411, 413, 414, 415, 422:
		return CLIErrorValidation
	case 401, 403:
		return CLIErrorAuthentication
	case 404, 410:
		return CLIErrorNotFound
	case 409, 412:
		return CLIErrorConflict
	default:
		return CLIErrorOperational
	}
}

func classifyCLIError(err error) errorClassification {
	if err == nil {
		return errorClassification{category: CLIErrorInternal, expected: true}
	}

	var classified *classifiedCLIError
	if errors.As(err, &classified) {
		return errorClassification{category: classified.category, expected: true}
	}

	var apiError *blaxel.Error
	if errors.As(err, &apiError) && apiError.StatusCode >= 400 {
		return errorClassification{
			category: categoryForHTTPStatus(apiError.StatusCode),
			expected: true,
		}
	}

	var unknownFlag *pflag.NotExistError
	var invalidFlag *pflag.InvalidValueError
	var missingFlagValue *pflag.ValueRequiredError
	var invalidFlagSyntax *pflag.InvalidSyntaxError
	if errors.As(err, &unknownFlag) ||
		errors.As(err, &invalidFlag) ||
		errors.As(err, &missingFlagValue) ||
		errors.As(err, &invalidFlagSyntax) ||
		errors.Is(err, flag.ErrHelp) {
		return errorClassification{category: CLIErrorUsage, expected: true}
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return errorClassification{category: CLIErrorOperational, expected: true}
	}
	if errors.Is(err, huh.ErrUserAborted) {
		return errorClassification{category: CLIErrorUsage, expected: true}
	}
	if errors.Is(err, websocket.ErrBadHandshake) {
		return errorClassification{category: CLIErrorOperational, expected: true}
	}

	if errors.Is(err, fs.ErrNotExist) {
		return errorClassification{category: CLIErrorNotFound, expected: true}
	}
	if errors.Is(err, fs.ErrExist) {
		return errorClassification{category: CLIErrorConflict, expected: true}
	}
	if errors.Is(err, fs.ErrPermission) {
		return errorClassification{category: CLIErrorOperational, expected: true}
	}

	var pathError *os.PathError
	if errors.As(err, &pathError) {
		category := CLIErrorOperational
		if errors.Is(pathError, fs.ErrNotExist) {
			category = CLIErrorNotFound
		}
		return errorClassification{category: category, expected: true}
	}

	var networkError net.Error
	if errors.As(err, &networkError) {
		return errorClassification{category: CLIErrorOperational, expected: true}
	}

	var processExit *exec.ExitError
	if errors.As(err, &processExit) {
		return errorClassification{category: CLIErrorOperational, expected: true}
	}
	var processStart *exec.Error
	if errors.As(err, &processStart) {
		category := CLIErrorOperational
		if errors.Is(processStart, fs.ErrNotExist) {
			category = CLIErrorNotFound
		}
		return errorClassification{category: category, expected: true}
	}

	message := strings.TrimSpace(err.Error())
	if cobraArgumentError.MatchString(message) ||
		cobraRequiredFlagError.MatchString(message) ||
		cobraUnknownCommand.MatchString(message) {
		return errorClassification{category: CLIErrorUsage, expected: true}
	}

	return errorClassification{category: CLIErrorInternal, expected: false}
}

// IsExpectedCLIError reports whether an error has a typed or explicitly
// marked expected classification. It does not inspect broad message fragments.
func IsExpectedCLIError(err error) bool {
	return err != nil && classifyCLIError(err).expected
}

func sanitizeSentryEvent(event *sentry.Event, _ *sentry.EventHint) *sentry.Event {
	if event == nil {
		return nil
	}

	// Host/user/request context is not needed to diagnose a CLI implementation
	// defect and can contain machine or resource identifiers.
	event.ServerName = ""
	event.User = sentry.User{}
	event.Request = nil
	event.Breadcrumbs = nil
	event.Contexts = nil
	event.Modules = nil
	event.Threads = nil
	event.Message = ""
	event.Transaction = ""
	event.Logger = ""
	event.Dist = ""
	event.Fingerprint = nil
	event.DebugMeta = nil
	event.Attachments = nil
	event.Type = ""
	event.StartTime = time.Time{}
	event.Spans = nil
	event.TransactionInfo = nil
	event.CheckIn = nil
	event.MonitorConfig = nil
	event.Logs = nil
	event.Metrics = nil
	event.Environment = normalizeEnvironment(event.Environment)
	event.Release = sanitizeTagValue(event.Release)
	event.Platform = "go"
	event.Level = sentry.LevelError

	allowedTags := map[string]struct{}{
		"version":        {},
		"commit":         {},
		"command.class":  {},
		"error.category": {},
	}
	safeTags := make(map[string]string, len(event.Tags))
	for key, value := range event.Tags {
		if _, ok := allowedTags[key]; ok {
			safeTags[key] = sanitizeTagValue(value)
		}
	}
	event.Tags = safeTags

	for exceptionIndex := range event.Exception {
		exception := &event.Exception[exceptionIndex]
		exception.Type = "CLIInternalError"
		exception.Value = "Unexpected CLI failure"
		exception.Module = "github.com/blaxel-ai/toolkit/cli/core"
		exception.ThreadID = 0
		exception.Mechanism = nil
		if exception.Stacktrace == nil {
			continue
		}

		safeFrames := make([]sentry.Frame, 0, len(exception.Stacktrace.Frames))
		for _, frame := range exception.Stacktrace.Frames {
			if frame.Module != "github.com/blaxel-ai/toolkit" &&
				!strings.HasPrefix(frame.Module, "github.com/blaxel-ai/toolkit/") {
				continue
			}
			safeFrames = append(safeFrames, sentry.Frame{
				Function: frame.Function,
				Module:   frame.Module,
				Filename: path.Base(strings.ReplaceAll(frame.Filename, "\\", "/")),
				Lineno:   frame.Lineno,
				Colno:    frame.Colno,
				InApp:    true,
			})
		}
		exception.Stacktrace.Frames = safeFrames
		exception.Stacktrace.FramesOmitted = nil
	}

	return event
}

func normalizeEnvironment(environment string) string {
	if environment == "dev" {
		return "dev"
	}
	return "prod"
}

// InitSentry initializes the SDK with a minimal, allowlisted event surface.
func InitSentry(cfg SentryConfig) error {
	if cfg.DSN == "" {
		SentryDSN = ""
		return nil
	}

	if err := sentry.Init(sentry.ClientOptions{
		Dsn:              cfg.DSN,
		Environment:      normalizeEnvironment(os.Getenv("BL_ENV")),
		Release:          cfg.Release,
		AttachStacktrace: true,
		BeforeSend:       sanitizeSentryEvent,
		SendDefaultPII:   false,
		MaxBreadcrumbs:   -1,
		MaxErrorDepth:    1,
	}); err != nil {
		SentryDSN = ""
		return err
	}

	SentryDSN = cfg.DSN
	return nil
}

// FlushSentry flushes buffered events before the program exits.
func FlushSentry(timeout time.Duration) {
	if SentryDSN == "" {
		return
	}
	sentry.Flush(timeout)
}

func captureUnexpectedError(err error) bool {
	classification := classifyCLIError(err)
	if err == nil || classification.expected || SentryDSN == "" {
		return false
	}

	sentry.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("error.category", string(classification.category))
		sentry.CaptureException(&sentryCLIError{category: classification.category})
	})
	return true
}

// CaptureException reports only unexpected CLI defects and never forwards the
// original error string or wrapped error chain.
func CaptureException(err error) {
	captureUnexpectedError(err)
}

var safeTagValue = regexp.MustCompile(`^[A-Za-z0-9._+ -]+$`)

func sanitizeTagValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 80 || !safeTagValue.MatchString(value) {
		return "unknown"
	}
	return value
}

// SetSentryTag accepts only build-controlled or command-taxonomy tags.
func SetSentryTag(key, value string) {
	if SentryDSN == "" {
		return
	}

	switch key {
	case "version", "commit", "command.class":
		sentry.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetTag(key, sanitizeTagValue(value))
		})
	}
}

// RecoverWithSentry reports a sanitized panic category, then preserves Go's
// original panic behavior and value for the local process.
func RecoverWithSentry() {
	if SentryDSN == "" {
		return
	}
	if recovered := recover(); recovered != nil {
		sentry.WithScope(func(scope *sentry.Scope) {
			scope.SetTag("error.category", string(CLIErrorPanic))
			sentry.CaptureException(&sentryCLIError{category: CLIErrorPanic})
		})
		sentry.Flush(2 * time.Second)
		panic(recovered)
	}
}

// ExitWithError preserves local hints and exit code while reporting only
// sanitized, unexpected implementation defects.
func ExitWithError(err error) {
	if IsAuthError(err) {
		PrintAuthSourceHint()
	}
	if captureUnexpectedError(err) {
		sentry.Flush(2 * time.Second)
	}
	os.Exit(1)
}

// ExitWithMessage is a user-facing expected exit and is never error telemetry.
func ExitWithMessage(_ string) {
	os.Exit(1)
}

// Exit exits with the given code after flushing any previously queued event.
func Exit(code int) {
	if code != 0 && SentryDSN != "" {
		sentry.Flush(2 * time.Second)
	}
	os.Exit(code)
}
