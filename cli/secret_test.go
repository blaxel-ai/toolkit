package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecretCmd_HasListSetDelete(t *testing.T) {
	cmd := SecretCmd()
	names := map[string]bool{}
	for _, sub := range cmd.Commands() {
		names[sub.Name()] = true
	}
	assert.True(t, names["list"] && names["set"] && names["delete"], "got %v", names)
}

func TestParseDestinations_RepeatedAndCommaSeparated(t *testing.T) {
	got := parseDestinations([]string{"api.stripe.com", " api.a.com, api.b.com/v2/* ", "", ","})
	assert.Equal(t, []string{"api.stripe.com", "api.a.com", "api.b.com/v2/*"}, got)
	assert.Empty(t, parseDestinations(nil))
}

func TestReadSecretValue_DropsOneTrailingNewlineOnly(t *testing.T) {
	v, err := readSecretValue(strings.NewReader("s3cr3t\n"))
	assert.NoError(t, err)
	assert.Equal(t, "s3cr3t", v)
	v, _ = readSecretValue(strings.NewReader("with-crlf\r\n"))
	assert.Equal(t, "with-crlf", v)
	v, _ = readSecretValue(strings.NewReader("keeps inner\nnewline\n\n"))
	assert.Equal(t, "keeps inner\nnewline\n", v)
}
