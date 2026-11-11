package core

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/blaxel-ai/sdk-go/option"
)

// QuotaExceededCode is the API error code for a workspace quota that is used up.
// Retrying does not help until resources are freed or the quota is raised.
const QuotaExceededCode = "QUOTA_EXCEEDED"

// maxErrorBodyPeek bounds how much of a 429 body is read to find its error code.
const maxErrorBodyPeek = 64 << 10

// NoRetryOnQuotaExceeded stops the SDK from retrying a 429 whose body carries
// QUOTA_EXCEEDED. The SDK retries every 429, which turns a quota error into
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

	if APIErrorCode(body) == QuotaExceededCode {
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
