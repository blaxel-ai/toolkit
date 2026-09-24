package cli

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/require"
)

func imageTestClient(t *testing.T, handler http.HandlerFunc) *blaxel.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	client := blaxel.NewClient(option.WithBaseURL(server.URL+"/"), option.WithMaxRetries(0))
	return &client
}

func TestImagePagesDefaultAndEmptyContinuation(t *testing.T) {
	calls := 0
	client := imageTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, imageAPIVersion, r.Header.Get("Blaxel-Version"))
		require.Equal(t, "application/json", r.Header.Get("Accept"))
		require.Equal(t, "/images", r.URL.Path)
		require.Equal(t, "base", r.URL.Query().Get("q"))
		if r.URL.Query().Get("cursor") == "" {
			_, _ = fmt.Fprint(w, `{"data":[],"meta":{"hasMore":true,"nextCursor":"next+/="}}`)
			return
		}
		require.Equal(t, "next+/=", r.URL.Query().Get("cursor"))
		_, _ = fmt.Fprint(w, `{"data":[{"metadata":{"name":"base"},"spec":{"tagCount":10000,"size":123}}],"meta":{"hasMore":false}}`)
	})
	query := imageQuery(imageListOptions{limit: 20, query: "base"})
	page, err := collectImagePages(context.Background(), client, "images", query, false)
	require.NoError(t, err)
	require.Empty(t, page.Data)
	require.True(t, page.Meta.HasMore)
	require.Equal(t, 1, calls)
	page, err = collectImagePages(context.Background(), client, "images", query, true)
	require.NoError(t, err)
	require.Len(t, page.Data, 1)
	require.False(t, page.Meta.HasMore)
	require.Equal(t, 3, calls)
}

func TestImageTagQueriesAndSummary(t *testing.T) {
	client := imageTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, imageAPIVersion, r.Header.Get("Blaxel-Version"))
		require.Equal(t, "owner", r.URL.Query().Get("sourceWorkspace"))
		if r.URL.Path == "/images/sandbox/base" {
			_, _ = fmt.Fprint(w, `{"metadata":{"name":"base","lastDeployedAt":"2026-09-23T01:00:00Z"},"spec":{"size":12,"tagCount":10000}}`)
			return
		}
		require.Equal(t, "/images/sandbox/base/tags", r.URL.Path)
		require.Equal(t, "v1", r.URL.Query().Get("name"))
		_, _ = fmt.Fprint(w, `{"data":[{"name":"v1","size":12}],"meta":{"hasMore":false}}`)
	})
	summary, err := fetchImageSummary(context.Background(), client, "sandbox", "base", "owner")
	require.NoError(t, err)
	require.EqualValues(t, 10000, summary["spec"].(map[string]any)["tagCount"])
	query := imageQuery(imageListOptions{limit: 1, source: "owner"})
	query.Set("name", "v1")
	page, err := collectImagePages(context.Background(), client, imagePath("sandbox", "base")+"/tags", query, true)
	require.NoError(t, err)
	require.Len(t, page.Data, 1)
}

func TestLatestImageTagWalksPagesAndComparesTimestamps(t *testing.T) {
	calls := 0
	client := imageTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "/images/sandbox/base/tags", r.URL.Path)
		require.Equal(t, "name:asc", r.URL.Query().Get("sort"))
		require.Equal(t, "100", r.URL.Query().Get("limit"))
		if r.URL.Query().Get("cursor") == "" {
			_, _ = fmt.Fprint(w, `{"data":[{"name":"a","createdAt":"2026-09-23T04:00:00+02:00"}],"meta":{"hasMore":true,"nextCursor":"next"}}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"data":[{"name":"z","createdAt":"2026-09-23T03:00:00Z"}],"meta":{"hasMore":false}}`)
	})
	tag, err := latestImageTag(context.Background(), client, "sandbox", "base", "")
	require.NoError(t, err)
	require.Equal(t, "z", tag)
	require.Equal(t, 2, calls)
}

func TestImagePaginationInvalidCursorResponses(t *testing.T) {
	for _, body := range []string{`{"data":[],"meta":{"hasMore":true}}`, `{"data":[],"meta":{"hasMore":true,"nextCursor":"same"}}`} {
		t.Run(body, func(t *testing.T) {
			calls := 0
			client := imageTestClient(t, func(w http.ResponseWriter, r *http.Request) { calls++; _, _ = fmt.Fprint(w, body) })
			_, err := collectImagePages(context.Background(), client, "images", url.Values{"limit": {"1"}}, true)
			require.Error(t, err)
			require.LessOrEqual(t, calls, 2)
		})
	}
}

func TestImageLatestCommand(t *testing.T) {
	original := core.GetClient()
	t.Cleanup(func() { core.SetClient(original) })
	core.SetClient(imageTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `{"data":[{"name":"v1","createdAt":"2026-09-23T00:00:00Z"}],"meta":{"hasMore":false}}`)
	}))
	cmd := GetImagesCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"sandbox/base", "--latest"})
	require.NoError(t, cmd.Execute())
	require.Equal(t, "sandbox/base:v1\n", buf.String())
	for _, args := range [][]string{{"--limit=0"}, {"--latest"}, {"sandbox/base:v1", "--latest"}, {"sandbox/base", "--latest", "--cursor=x"}} {
		cmd := GetImagesCmd()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs(args)
		require.Error(t, cmd.Execute())
	}
}

func TestNamedImageTableUsesCommandOutput(t *testing.T) {
	original := core.GetClient()
	t.Cleanup(func() { core.SetClient(original) })
	core.SetClient(imageTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/images/sandbox/base" {
			_, _ = fmt.Fprint(w, `{ "metadata":{"name":"base"},"spec":{"size":12,"tagCount":10000}}`)
		} else {
			_, _ = fmt.Fprint(w, `{"data":[{"name":"v1","size":12}],"meta":{"hasMore":false}}`)
		}
	}))
	cmd := GetImagesCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"sandbox/base"})
	require.NoError(t, cmd.Execute())
	require.Contains(t, buf.String(), "Image: sandbox/base")
	require.Contains(t, buf.String(), "Tags: 10000")
	require.Contains(t, buf.String(), "sandbox/base:v1")
}
