package core

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// creatorServer serves GET /users to anyone and answers GET /service_accounts
// with serviceAccountsStatus, counting the requests it receives.
func creatorServer(t *testing.T, serviceAccountsStatus int) (users, serviceAccounts *int32) {
	t.Helper()
	users, serviceAccounts = new(int32), new(int32)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/users"):
			atomic.AddInt32(users, 1)
			_, _ = w.Write([]byte(`[
				{"sub":"u-1","email":"ada@example.com","given_name":"Ada","family_name":"L"},
				{"sub":"u-2","email":"","given_name":"Grace","family_name":"H"}
			]`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/service_accounts"):
			atomic.AddInt32(serviceAccounts, 1)
			w.WriteHeader(serviceAccountsStatus)
			if serviceAccountsStatus == http.StatusOK {
				_, _ = w.Write([]byte(`[{"client_id":"sa-1","name":"ci-runner"}]`))
			} else {
				_, _ = w.Write([]byte(`{"error":"forbidden"}`))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)

	previous := GetClient()
	c := blaxel.NewClient(option.WithBaseURL(server.URL), option.WithAPIKey("test-api-key"), option.WithMaxRetries(0))
	SetClient(&c)
	t.Cleanup(func() { SetClient(previous) })
	return users, serviceAccounts
}

func TestResolveCreators(t *testing.T) {
	users, serviceAccounts := creatorServer(t, http.StatusOK)

	got := resolveCreators([]string{
		"user:u-1", "user:u-1", "user:u-2", "user:gone",
		"service_account:sa-1", "serviceaccount:sa-1", "u-1", "-",
	})

	assert.Equal(t, map[string]string{
		"user:u-1":             "ada@example.com",
		"user:u-2":             "Grace H",
		"user:gone":            "user:gone",
		"service_account:sa-1": "ci-runner",
		"serviceaccount:sa-1":  "ci-runner",
		"u-1":                  "ada@example.com",
	}, got)
	assert.EqualValues(t, 1, *users)
	assert.EqualValues(t, 1, *serviceAccounts)
}

func TestResolveCreatorsServiceAccountsForbidden(t *testing.T) {
	_, serviceAccounts := creatorServer(t, http.StatusForbidden)

	got := resolveCreators([]string{"user:u-1", "service_account:sa-1"})

	assert.Equal(t, "ada@example.com", got["user:u-1"])
	assert.Equal(t, "service_account:sa-1", got["service_account:sa-1"])
	assert.EqualValues(t, 1, *serviceAccounts)
}

func TestResolveCreatorsSkipsServiceAccountsWhenOnlyUsers(t *testing.T) {
	users, serviceAccounts := creatorServer(t, http.StatusOK)

	resolveCreators([]string{"user:u-1"})

	assert.EqualValues(t, 1, *users)
	assert.EqualValues(t, 0, *serviceAccounts)
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = original
	data, err := io.ReadAll(r)
	require.NoError(t, err)
	return string(data)
}

func TestSandboxTableShowsCreatorEmail(t *testing.T) {
	users, _ := creatorServer(t, http.StatusForbidden)
	var sandbox Resource
	for _, r := range GetResources() {
		if r.Kind == "Sandbox" {
			sandbox = *r
		}
	}
	items := []any{
		map[string]any{"metadata": map[string]any{"name": "a", "createdBy": "user:u-1", "createdAt": "2026-01-01T00:00:00Z"}},
		map[string]any{"metadata": map[string]any{"name": "b", "createdBy": "user:u-1", "createdAt": "2026-01-02T00:00:00Z"}},
		map[string]any{"metadata": map[string]any{"name": "c", "createdBy": "user:unknown", "createdAt": "2026-01-03T00:00:00Z"}},
	}

	table := captureStdout(t, func() { OutputPreservingOrder(sandbox, items, "table") })
	assert.Contains(t, table, "CREATED_BY")
	assert.Equal(t, 2, strings.Count(table, "ada@example.com"))
	assert.Contains(t, table, "user:unknown")
	assert.EqualValues(t, 1, *users, "one users request per command, not per row")

	jsonOut := captureStdout(t, func() { OutputPreservingOrder(sandbox, items, "json") })
	assert.Contains(t, jsonOut, "user:u-1")
	assert.EqualValues(t, 1, *users, "no lookup for -o json")
}
