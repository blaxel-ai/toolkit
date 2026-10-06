package cli

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
)

// Older event records can contain raw infrastructure errors. Never promote
// those details into a terminal error when reading an existing build history.
var privateFailureDetail = regexp.MustCompile(`(?i)\b(aws|amazon|dynamodb|eventbridge|lambda|stripe|cloudflare|workos|authkit|depot|smithy|grpc|requestid|request-id|stacktrace|secretsmanager|cloudfront)\b|(?:^|[\s:])(s3|sqs|kms|ses|sfn)(?:[\s:]|$)|step[ -]?functions|arn:|https?://|\.internal\b|\.(?:amazonaws\.com|bl\.run)\b|operation error|api error|\b(?:\d{1,3}\.){3}\d{1,3}\b`)

func failureError(summary, message string) error {
	if message == "" {
		return errors.New(summary)
	}
	return errors.New(summary + ": " + message)
}

type failureEvent struct {
	Type     string `json:"type"`
	Status   string `json:"status"`
	Time     string `json:"time"`
	Message  string `json:"message"`
	Revision string `json:"revision,omitempty"`
}

// latestFailureEvent selects the newest event (including a newer non-failure),
// never an older convenient failure. Malformed history cannot establish order.
func latestFailureEvent(raw json.RawMessage, typePrefix string) (failureEvent, bool) {
	var events []failureEvent
	if json.Unmarshal(raw, &events) != nil {
		return failureEvent{}, false
	}
	latest := -1
	var latestTime time.Time
	for i, event := range events {
		if typePrefix != "" && !strings.HasPrefix(event.Type, typePrefix) {
			continue
		}
		timestamp, err := time.Parse(time.RFC3339Nano, event.Time)
		if err != nil {
			return failureEvent{}, false
		}
		if latest == -1 || !timestamp.Before(latestTime) {
			latest, latestTime = i, timestamp
		}
	}
	if latest == -1 {
		return failureEvent{}, false
	}
	return events[latest], true
}

// latestFailureMessage uses timestamps, not response ordering. It deliberately
// does not fall back to an older failure when a newer event has no useful detail.
func latestFailureMessage(raw json.RawMessage, typePrefix string) string {
	event, ok := latestFailureEvent(raw, typePrefix)
	if !ok {
		return ""
	}
	if !strings.EqualFold(event.Status, "failed") && !strings.HasSuffix(event.Type, ".failed") {
		return ""
	}
	message := strings.TrimSpace(strings.ReplaceAll(event.Message, "\r\n", "\n"))
	if privateFailureDetail.MatchString(message) {
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
