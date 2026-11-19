package deploy

import (
	"encoding/json"
	"strings"
	"unicode"
)

// buildProgressType is the event a build in progress reports itself with:
// queued, waiting for capacity, started.
const buildProgressType = "ai.blaxel.controlplane.buildimage.created"

// BuildProgress is what an image build that is not over says of itself (its
// newest build event, when that is a progress event), or false when it says
// nothing printable. One line only; raw infrastructure details and control
// characters are never shown.
func BuildProgress(raw json.RawMessage) (string, bool) {
	event, ok := LatestEvent(raw, "ai.blaxel.controlplane.buildimage.")
	if !ok || event.Type != buildProgressType {
		return "", false
	}
	message, _, _ := strings.Cut(strings.TrimSpace(event.Message), "\n")
	message = strings.TrimSpace(message)
	if message == "" || PrivateFailureDetail.MatchString(message) || strings.ContainsFunc(message, unicode.IsControl) {
		return "", false
	}
	return message, true
}
