package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func deviceTokenServer(t *testing.T, responses ...func(http.ResponseWriter)) (*httptest.Server, *int) {
	t.Helper()
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request DeviceLoginFinalizeRequest
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		assert.Equal(t, "urn:ietf:params:oauth:grant-type:device_code", request.GrantType)
		assert.Equal(t, "device-code", request.DeviceCode)
		index := calls
		if index >= len(responses) {
			index = len(responses) - 1
		}
		calls++
		responses[index](w)
	}))
	t.Cleanup(server.Close)
	return server, &calls
}

func pending(w http.ResponseWriter) { w.WriteHeader(http.StatusAccepted) }

func pendingError(w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte(`{"error":"authorization_pending"}`))
}

func TestPollDeviceTokenWaitsForConfirmation(t *testing.T) {
	server, calls := deviceTokenServer(t, pending, pendingError, func(w http.ResponseWriter) {
		_, _ = w.Write([]byte(`{"access_token":"access","refresh_token":"refresh","expires_in":3600}`))
	})
	token, err := pollDeviceToken(context.Background(), server.URL, "device-code", time.Millisecond, 10)
	require.NoError(t, err)
	assert.Equal(t, DeviceLoginFinalizeResponse{AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 3600}, token)
	assert.Equal(t, 3, *calls)
}

func TestPollDeviceTokenReportsFailures(t *testing.T) {
	server, calls := deviceTokenServer(t, pending)
	_, err := pollDeviceToken(context.Background(), server.URL, "device-code", time.Millisecond, 4)
	assert.ErrorContains(t, err, "timed out waiting for confirmation")
	assert.Equal(t, 4, *calls)

	server, _ = deviceTokenServer(t, func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"access_denied"}`))
	})
	_, err = pollDeviceToken(context.Background(), server.URL, "device-code", time.Millisecond, 4)
	assert.ErrorContains(t, err, "status 400")

	server, _ = deviceTokenServer(t, func(w http.ResponseWriter) { _, _ = w.Write([]byte("not json")) })
	_, err = pollDeviceToken(context.Background(), server.URL, "device-code", time.Millisecond, 4)
	assert.ErrorContains(t, err, "unmarshalling")
}

func TestRequestDeviceLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request DeviceLogin
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		assert.Equal(t, DeviceLogin{ClientID: "blaxel", Scope: "offline_access"}, request)
		if r.URL.Path == "/broken" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{}`))
			return
		}
		_, _ = w.Write([]byte(`{"device_code":"device-code","verification_uri_complete":"https://app.blaxel.ai/device?code=1"}`))
	}))
	defer server.Close()
	response, err := requestDeviceLogin(context.Background(), server.URL+"/login/device")
	require.NoError(t, err)
	assert.Equal(t, "device-code", response.DeviceCode)

	_, err = requestDeviceLogin(context.Background(), server.URL+"/broken")
	assert.ErrorContains(t, err, "status 503", "an unusable answer stops the login instead of polling for nothing")
}

func TestRequestDeviceLoginStopsWhenCancelled(t *testing.T) {
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	defer server.Close()
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(30 * time.Millisecond); cancel() }()
	started := time.Now()
	_, err := requestDeviceLogin(ctx, server.URL)
	assert.ErrorIs(t, err, context.Canceled, "skipping the login stops a request that hangs")
	assert.Less(t, time.Since(started), 2*time.Second)
}

func TestPollDeviceTokenStopsWhenCancelled(t *testing.T) {
	server, calls := deviceTokenServer(t, pending)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { time.Sleep(30 * time.Millisecond); cancel() }()
	started := time.Now()
	_, err := pollDeviceToken(ctx, server.URL, "device-code", 10*time.Millisecond, 1000)
	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, time.Since(started), time.Second)
	assert.Less(t, *calls, 10)
}

func TestRequestDeviceLoginPreservesCLITarget(t *testing.T) {
	for _, workspace := range []string{"", "requested-workspace"} {
		t.Run(workspace, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var payload map[string]string
				require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
				assert.Equal(t, workspace, payload["workspace"])
				if workspace == "" {
					assert.NotContains(t, payload, "workspace")
				}
				_, _ = w.Write([]byte(`{"device_code":"device-code","verification_uri_complete":"https://app.blaxel.dev/device"}`))
			}))
			defer server.Close()
			_, err := requestDeviceLogin(context.Background(), server.URL, workspace)
			require.NoError(t, err)
		})
	}
}
