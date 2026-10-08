package core

import (
	"net/http"
	"net/http/httptest"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newPaginationTestClient(t *testing.T, handler http.HandlerFunc) *blaxel.Client {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	c := blaxel.NewClient(
		option.WithBaseURL(server.URL+"/"),
		option.WithAPIKey("test-key"),
		option.WithMaxRetries(0),
	)
	return &c
}

func jsonResponder(status int, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

func TestFetchPageDecodesPaginatedEnvelope(t *testing.T) {
	c := newPaginationTestClient(t, jsonResponder(http.StatusOK,
		`{"data":[{"name":"a"},{"name":"b"}],"meta":{"hasMore":true,"nextCursor":"c2","total":5}}`))

	result, err := fetchPage(c, "drives", DefaultPageLimit, "")
	require.NoError(t, err)
	assert.Len(t, result.Items, 2)
	assert.True(t, result.Meta.HasMore)
	assert.Equal(t, "c2", result.Meta.NextCursor)
	assert.Equal(t, 5, result.Meta.Total)
}

// A workspace with no drives (or a server that omits "data") must list as an
// empty page, not crash with "unexpected end of JSON input" (issue CLI-3G).
func TestFetchPageTreatsAbsentOrNullDataAsEmptyPage(t *testing.T) {
	bodies := map[string]string{
		"absent data field": `{"meta":{"total":0}}`,
		"null data":         `{"data":null,"meta":{"total":0}}`,
		"empty array":       `{"data":[],"meta":{"total":0}}`,
		"empty object body": `{}`,
	}

	for name, body := range bodies {
		t.Run(name, func(t *testing.T) {
			c := newPaginationTestClient(t, jsonResponder(http.StatusOK, body))
			result, err := fetchPage(c, "drives", DefaultPageLimit, "")
			require.NoError(t, err)
			assert.Empty(t, result.Items)
		})
	}
}

// A 2xx response the CLI cannot decode is a server/transport condition, not an
// unexpected CLI defect, so it must be classified as expected and never paged to
// Sentry (issue CLI-3G).
func TestFetchPageMarksUndecodableSuccessBodyAsExpected(t *testing.T) {
	handlers := map[string]http.HandlerFunc{
		"bare array payload": jsonResponder(http.StatusOK, `[{"name":"a"}]`),
		"non-JSON body": func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html>gateway error</html>"))
		},
		"malformed json": jsonResponder(http.StatusOK, `{"data":[}`),
	}

	for name, handler := range handlers {
		t.Run(name, func(t *testing.T) {
			transport := bindMockSentry(t)
			c := newPaginationTestClient(t, handler)

			_, err := fetchPage(c, "drives", DefaultPageLimit, "")
			require.Error(t, err)
			assert.True(t, IsExpectedCLIError(err))
			assert.Equal(t, CLIErrorOperational, classifyCLIError(err).category)
			assert.False(t, captureUnexpectedError(err))
			assert.Empty(t, transport.Events())
		})
	}
}

// Genuine HTTP error responses keep their status-derived classification instead
// of being flattened to a generic operational error.
func TestFetchPagePreservesHTTPStatusClassification(t *testing.T) {
	cases := []struct {
		name     string
		status   int
		category CLIErrorCategory
	}{
		{name: "server error", status: http.StatusInternalServerError, category: CLIErrorOperational},
		{name: "not found", status: http.StatusNotFound, category: CLIErrorNotFound},
		{name: "unauthorized", status: http.StatusUnauthorized, category: CLIErrorAuthentication},
		{name: "forbidden", status: http.StatusForbidden, category: CLIErrorAuthentication},
		{name: "conflict", status: http.StatusConflict, category: CLIErrorConflict},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newPaginationTestClient(t, jsonResponder(tc.status, `{"error":{"message":"boom"}}`))
			_, err := fetchPage(c, "drives", DefaultPageLimit, "")
			require.Error(t, err)
			assert.True(t, IsExpectedCLIError(err))
			assert.Equal(t, tc.category, classifyCLIError(err).category)
		})
	}
}
