package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListAllAgentsFetchesEveryPage(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/agents", r.URL.Path)
		require.Equal(t, "200", r.URL.Query().Get("limit"))

		requests++
		w.Header().Set("Content-Type", "application/json")

		switch requests {
		case 1:
			assert.Empty(t, r.URL.Query().Get("cursor"))
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{"metadata": map[string]interface{}{"name": "agent-1"}},
				},
				"meta": map[string]interface{}{
					"hasMore":    true,
					"nextCursor": "cursor-2",
					"total":      2,
				},
			})
		case 2:
			assert.Equal(t, "cursor-2", r.URL.Query().Get("cursor"))
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{"metadata": map[string]interface{}{"name": "agent-2"}},
				},
				"meta": map[string]interface{}{
					"hasMore":    false,
					"nextCursor": "",
					"total":      2,
				},
			})
		default:
			t.Fatalf("unexpected request %d", requests)
		}
	}))
	defer server.Close()

	client, err := blaxel.NewDefaultClient(
		option.WithBaseURL(server.URL),
		option.WithWorkspace("test-workspace"),
		option.WithAPIKey("test-api-key"),
	)
	require.NoError(t, err)

	agents, err := ListAllAgents(context.Background(), &client)
	require.NoError(t, err)
	require.Len(t, *agents, 2)
	assert.Equal(t, "agent-1", (*agents)[0].Metadata.Name)
	assert.Equal(t, "agent-2", (*agents)[1].Metadata.Name)
	assert.Equal(t, 2, requests)
}
