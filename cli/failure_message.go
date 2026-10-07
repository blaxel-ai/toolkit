package cli

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode"

	"github.com/blaxel-ai/toolkit/cli/deploy"
)

func failureError(summary, message string) error {
	if message == "" {
		return errors.New(summary)
	}
	return errors.New(summary + ": " + message)
}

// latestFailureMessage uses timestamps, not response ordering. It deliberately
// does not fall back to an older failure when a newer event has no useful detail.
func latestFailureMessage(raw json.RawMessage, typePrefix string) string {
	event, ok := deploy.LatestEvent(raw, typePrefix)
	if !ok {
		return ""
	}
	if !strings.EqualFold(event.Status, "failed") && !strings.HasSuffix(event.Type, ".failed") {
		return ""
	}
	message := strings.TrimSpace(strings.ReplaceAll(event.Message, "\r\n", "\n"))
	if deploy.PrivateFailureDetail.MatchString(message) {
		return ""
	}
	// Preserve multiline diagnostics from the control plane while rejecting
	// terminal control sequences and standalone carriage returns.
	if strings.ContainsFunc(message, func(r rune) bool {
		return unicode.IsControl(r) && r != '\n' && r != '\t'
	}) {
		return ""
	}
	return message
}
