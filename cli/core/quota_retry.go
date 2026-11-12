package core

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/blaxel-ai/sdk-go/option"
)

// QuotaExceededCode is the API error code for a workspace quota that is used up.
// Retrying does not help until resources are freed or the quota is raised.
const QuotaExceededCode = "QUOTA_EXCEEDED"

// maxErrorBodyPeek bounds how much of a 429 body is read to find its error code.
const maxErrorBodyPeek = 64 << 10

// NoRetryOnQuotaExceeded stops the SDK from retrying a 429 that IsQuotaExceeded
// recognizes. The SDK retries every 429, which turns a quota error into
// extra create calls and a slower failure. A server that already sends
// x-should-retry keeps the final say.
func NoRetryOnQuotaExceeded(req *http.Request, next option.MiddlewareNext) (*http.Response, error) {
	res, err := next(req)
	if err != nil || res == nil || res.StatusCode != http.StatusTooManyRequests || res.Body == nil {
		return res, err
	}
	if res.Header.Get("x-should-retry") != "" {
		return res, err
	}

	body, readErr := io.ReadAll(io.LimitReader(res.Body, maxErrorBodyPeek))
	rest := res.Body
	res.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(body), rest), rest}
	if readErr != nil {
		return res, err
	}

	if IsQuotaExceeded(res.StatusCode, body) {
		if res.Header == nil {
			res.Header = http.Header{}
		}
		res.Header.Set("x-should-retry", "false")
	}
	return res, err
}

// APIErrorCode returns the string error code of an API error body. It reads
// both the flat shape {"code": "..."} and the gateway envelope
// {"error": {"code": "..."}}. It returns "" when there is no string code.
func APIErrorCode(body []byte) string {
	var parsed struct {
		Code  interface{}     `json:"code"`
		Error json.RawMessage `json:"error"`
	}
	if json.Unmarshal(body, &parsed) != nil {
		return ""
	}
	if code, ok := parsed.Code.(string); ok && code != "" {
		return code
	}
	var inner struct {
		Code interface{} `json:"code"`
	}
	if len(parsed.Error) > 0 && json.Unmarshal(parsed.Error, &inner) == nil {
		if code, ok := inner.Code.(string); ok {
			return code
		}
	}
	return ""
}

// plansQuotaPage is linked by every control-plane quota error that still uses
// the generic {"error": "<message>", "code": 429} body (image storage on
// builds and deploys, job and volume quotas). The build-backlog refusal links
// it too, but clears once builds finish.
const plansQuotaPage = "/account/plan-quotas"

// buildBacklogText marks the build-backlog 429 from control planes that send
// it without a code ("Too many image builds are waiting ...").
const buildBacklogText = "builds are waiting"

// BuildQueueFullCode names the build-backlog 429 on newer control planes.
const BuildQueueFullCode = "BUILD_QUEUE_FULL"

// AuthenticationUnavailableCode is the 503 the API answers when it cannot check
// credentials for a moment (a throttled backend). It is retryable, with
// Retry-After; it is not a credential or permission error.
const AuthenticationUnavailableCode = "AUTHENTICATION_UNAVAILABLE"

type legacyErrorBody struct {
	Code      interface{} `json:"code"`
	Error     interface{} `json:"error"`
	Retryable *bool       `json:"retryable"`
}

// IsBuildBacklog reports whether a 429 is the account's build queue being
// full: BUILD_QUEUE_FULL, or an older numeric-code body that says builds are
// waiting. It clears once builds finish, so it is retryable.
func IsBuildBacklog(status int, body []byte) bool {
	if status != http.StatusTooManyRequests {
		return false
	}
	if code := APIErrorCode(body); code != "" {
		return code == BuildQueueFullCode
	}
	var parsed legacyErrorBody
	if json.Unmarshal(body, &parsed) != nil {
		return false
	}
	code, _ := parsed.Code.(float64)
	msg, _ := parsed.Error.(string)
	return code == http.StatusTooManyRequests && strings.Contains(msg, buildBacklogText)
}

// IsQuotaExceeded reports whether an error response is a quota error, which no
// retry fixes until usage drops or the quota is raised. The server's signal
// wins: a body "retryable" flag, then a string code (QUOTA_EXCEEDED or not).
// Only an older numeric-code 429 body with neither falls back to its text: a
// message that links the plan quotas page is a quota, unless it says builds
// are waiting. Callers that see an x-should-retry header use that instead.
func IsQuotaExceeded(status int, body []byte) bool {
	if code := APIErrorCode(body); code != "" {
		return code == QuotaExceededCode
	}
	if status != http.StatusTooManyRequests {
		return false
	}
	var parsed legacyErrorBody
	if json.Unmarshal(body, &parsed) != nil {
		return false
	}
	if parsed.Retryable != nil {
		return !*parsed.Retryable
	}
	code, _ := parsed.Code.(float64)
	msg, _ := parsed.Error.(string)
	return code == http.StatusTooManyRequests && strings.Contains(msg, plansQuotaPage) &&
		!strings.Contains(msg, buildBacklogText)
}
