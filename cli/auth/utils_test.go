package auth

import (
	"testing"

	"github.com/stretchr/testify/assert"
	blaxel "github.com/stainless-sdks/blaxel-go"
)

func TestCredentialsTypes(t *testing.T) {
	// Test empty credentials
	creds := blaxel.Credentials{}

	assert.Empty(t, creds.APIKey)
	assert.Empty(t, creds.AccessToken)
	assert.Empty(t, creds.ClientCredentials)
}

func TestCredentialsWithAPIKey(t *testing.T) {
	creds := blaxel.Credentials{
		APIKey: "test-api-key",
	}

	assert.Equal(t, "test-api-key", creds.APIKey)
	assert.Empty(t, creds.AccessToken)
}

func TestCredentialsWithAccessToken(t *testing.T) {
	creds := blaxel.Credentials{
		AccessToken: "test-access-token",
	}

	assert.Equal(t, "test-access-token", creds.AccessToken)
	assert.Empty(t, creds.APIKey)
}

func TestCredentialsWithClientCredentials(t *testing.T) {
	creds := blaxel.Credentials{
		ClientCredentials: "test-client-credentials",
	}

	assert.Equal(t, "test-client-credentials", creds.ClientCredentials)
	assert.Empty(t, creds.APIKey)
	assert.Empty(t, creds.AccessToken)
}
