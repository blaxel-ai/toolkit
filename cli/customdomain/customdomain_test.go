package customdomain

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testClient(t *testing.T, handler http.HandlerFunc) *blaxel.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		handler(w, r)
	}))
	t.Cleanup(server.Close)
	client := blaxel.NewClient(option.WithBaseURL(server.URL+"/"), option.WithMaxRetries(0))
	return &client
}

func TestBuildCreateRequestOmitsValidationMethodByDefault(t *testing.T) {
	req, err := BuildCreateRequest(CreateOptions{Name: "app.example.com", DomainType: "applications"})
	require.NoError(t, err)
	body, err := json.Marshal(req)
	require.NoError(t, err)
	assert.JSONEq(t, `{"metadata":{"name":"app.example.com"},"spec":{"domainType":"applications"}}`, string(body))
}

func TestBuildCreateRequestSendsValidationMethodAndSANs(t *testing.T) {
	req, err := BuildCreateRequest(CreateOptions{
		Name:                    "example.com",
		DomainType:              "applications",
		SubjectAlternativeNames: []string{"www.example.com", " "},
		ValidationMethod:        "http",
	})
	require.NoError(t, err)
	body, err := json.Marshal(req)
	require.NoError(t, err)
	assert.JSONEq(t, `{"metadata":{"name":"example.com"},"spec":{"domainType":"applications","subjectAlternativeNames":["www.example.com"],"validationMethod":"http"}}`, string(body))
}

func TestBuildCreateRequestPassesUnknownValidationMethodToAPI(t *testing.T) {
	req, err := BuildCreateRequest(CreateOptions{Name: "a.example.com", DomainType: "applications", ValidationMethod: "bogus"})
	require.NoError(t, err)
	assert.Equal(t, "bogus", req.Spec.ValidationMethod)
}

func TestBuildCreateRequestRejectsBadInput(t *testing.T) {
	_, err := BuildCreateRequest(CreateOptions{Name: " ", DomainType: "applications"})
	assert.Error(t, err)
	_, err = BuildCreateRequest(CreateOptions{Name: "a.example.com", DomainType: "apps"})
	assert.ErrorContains(t, err, "--type must be one of previews, applications")
}

func TestCreatePostsRequest(t *testing.T) {
	var got map[string]any
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/customdomains", r.URL.Path)
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &got))
		_, _ = w.Write([]byte(`{"metadata":{"name":"app.example.com"},"spec":{"domainType":"applications","validationMethod":"dns","status":"pending"}}`))
	})
	req, err := BuildCreateRequest(CreateOptions{Name: "app.example.com", DomainType: "applications"})
	require.NoError(t, err)
	domain, err := Create(context.Background(), c, req)
	require.NoError(t, err)
	spec := got["spec"].(map[string]any)
	_, sent := spec["validationMethod"]
	assert.False(t, sent, "validationMethod must not be sent when the flag is omitted")
	assert.Equal(t, "dns", domain["spec"].(map[string]any)["validationMethod"])
}

func TestPathsEscapeWildcardNames(t *testing.T) {
	var paths []string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.Method+" "+r.URL.EscapedPath())
		_, _ = w.Write([]byte(`{"metadata":{"name":"*.p.example.com"},"spec":{}}`))
	})
	ctx := context.Background()
	_, err := Get(ctx, c, "*.p.example.com")
	require.NoError(t, err)
	_, err = Verify(ctx, c, "*.p.example.com")
	require.NoError(t, err)
	require.NoError(t, Delete(ctx, c, "*.p.example.com"))
	assert.Equal(t, []string{
		"GET /customdomains/%2A.p.example.com",
		"POST /customdomains/%2A.p.example.com/verify",
		"DELETE /customdomains/%2A.p.example.com",
	}, paths)
}

func TestDescribeErrorPassesAPIMessage(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"code":403,"error":"HTTP validation is not enabled for this workspace, use dns validation"}`))
	})
	_, err := Create(context.Background(), c, CreateRequest{})
	require.Error(t, err)
	err = DescribeError(err)
	assert.EqualError(t, err, "HTTP validation is not enabled for this workspace, use dns validation (HTTP 403)")

	plain := errors.New("boom")
	assert.Equal(t, plain, DescribeError(plain))
}

func TestRecordsLabelsApexRoutingAsAlias(t *testing.T) {
	domain := map[string]any{"spec": map[string]any{
		"cnameRecords": map[string]any{
			"example.com":           "apps.example.net",
			"www.example.com":       "apps.example.net",
			"_abc.example.com.":     "_def.validations.example.net.",
			"shop.example.co.uk":    "apps.example.net",
			"example.co.uk":         "apps.example.net",
			"*.preview.example.com": "previews.example.net",
		},
	}}
	records := Records(domain)
	byName := map[string]Record{}
	for _, r := range records {
		byName[r.Name] = r
	}
	assert.Equal(t, RecordTypeAlias, byName["example.com"].Type)
	assert.Equal(t, RecordTypeAlias, byName["example.co.uk"].Type)
	assert.Equal(t, RecordTypeCNAME, byName["www.example.com"].Type)
	assert.Equal(t, RecordTypeCNAME, byName["shop.example.co.uk"].Type)
	assert.Equal(t, RecordTypeCNAME, byName["*.preview.example.com"].Type)
	assert.Equal(t, PurposeCertificate, byName["_abc.example.com."].Purpose)
	assert.Equal(t, PurposeRouting, byName["example.com"].Purpose)
	assert.Equal(t, PurposeCertificate, records[len(records)-1].Purpose, "certificate validation records come last")
}

func TestRecordsHTTPValidationApex(t *testing.T) {
	domain := map[string]any{"spec": map[string]any{
		"validationMethod": "http",
		"txtRecords":       map[string]any{"_cf-challenge.my.custom.host": "d1.edge.example.net"},
		"cnameRecords":     map[string]any{"my.custom.host": "apps.example.net"},
	}}
	records := Records(domain)
	require.Len(t, records, 2)
	assert.Equal(t, Record{Type: RecordTypeTXT, Name: "_cf-challenge.my.custom.host", Value: "d1.edge.example.net", Purpose: PurposeApexHTTP}, records[0])
	assert.Equal(t, RecordTypeAlias, records[1].Type, "a _cf-challenge TXT marks the name as an apex")
	for _, r := range records {
		assert.NotEqual(t, PurposeCertificate, r.Purpose)
	}
}

func TestRecordsEmptySpec(t *testing.T) {
	assert.Empty(t, Records(map[string]any{}))
}
