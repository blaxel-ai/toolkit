package integration

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	customDomainTimeout       = 60 * time.Second
	customDomainHTTPForbidden = "HTTP validation is not enabled for this workspace"
	customDomainCertPurpose   = "certificate validation"
)

// TestCustomDomainCommands drives `bl custom-domain` against a real workspace.
// Opt-in: requires BL_API_KEY and RUN_CUSTOM_DOMAIN_TESTS=true. Domains are
// created under CUSTOM_DOMAIN_TEST_BASE (default example.com) with unique names
// and are never verified, so no real DNS is needed.
func TestCustomDomainCommands(t *testing.T) {
	if os.Getenv("RUN_CUSTOM_DOMAIN_TESTS") != "true" {
		t.Skip("Skipping custom domain integration test: set RUN_CUSTOM_DOMAIN_TESTS=true to run it")
	}
	env := SetupRealCLIEnvironment(t)

	base := os.Getenv("CUSTOM_DOMAIN_TEST_BASE")
	if base == "" {
		base = "example.com"
	}
	suffix := randomSuffix(t)
	name := func(label string) string { return fmt.Sprintf("bl-cli-%s-%s.%s", label, suffix, base) }

	var created []string
	t.Cleanup(func() {
		for _, domain := range created {
			res := env.ExecuteCLIWithTimeout(customDomainTimeout, "custom-domain", "delete", domain)
			if res.ExitCode != 0 {
				t.Errorf("cleanup: failed to delete custom domain %s: %s%s", domain, res.Stdout, res.Stderr)
				continue
			}
			t.Logf("cleanup: deleted custom domain %s", domain)
		}
	})
	create := func(domain string, args ...string) *CLIResult {
		res := env.ExecuteCLIWithTimeout(customDomainTimeout, append([]string{"custom-domain", "create", domain}, args...)...)
		if res.ExitCode == 0 {
			created = append(created, domain)
		}
		return res
	}

	t.Run("default validation method is dns", func(t *testing.T) {
		domain := name("dns")
		res := create(domain, "--type", "applications")
		require.Equal(t, 0, res.ExitCode, "create failed: %s%s", res.Stdout, res.Stderr)
		assert.Equal(t, "dns", getValidationMethod(t, env, domain))
	})

	t.Run("http validation on applications", func(t *testing.T) {
		domain := name("http")
		res := create(domain, "--type", "applications", "--validation-method", "http")
		if res.ExitCode != 0 {
			require.Contains(t, res.Stdout+res.Stderr, customDomainHTTPForbidden, "unexpected create failure")
			t.Logf("http validation is disabled for workspace %s (403 as expected)", env.Workspace)
			return
		}
		t.Logf("http validation is enabled for workspace %s", env.Workspace)
		assert.Equal(t, "http", getValidationMethod(t, env, domain))

		recordsRes := env.ExecuteCLIWithTimeout(customDomainTimeout, "custom-domain", "records", domain, "-o", "json")
		require.Equal(t, 0, recordsRes.ExitCode, "records failed: %s%s", recordsRes.Stdout, recordsRes.Stderr)
		var records []struct {
			Type    string `json:"type"`
			Name    string `json:"name"`
			Purpose string `json:"purpose"`
		}
		require.NoError(t, json.Unmarshal([]byte(recordsRes.Stdout), &records), recordsRes.Stdout)
		for _, record := range records {
			assert.NotEqual(t, customDomainCertPurpose, record.Purpose, "http domain must not have a certificate validation record: %+v", record)
		}
	})

	errorCases := []struct {
		name   string
		domain string
		args   []string
		want   string
	}{
		{"http on previews is rejected", "*." + name("prev"), []string{"--type", "previews", "--validation-method", "http"}, "http validation"},
		{"http with a wildcard SAN is rejected", name("wsan"), []string{"--type", "applications", "--validation-method", "http", "--san", "*." + name("wsan")}, "http validation"},
		{"unknown validation method is rejected", name("bogus"), []string{"--type", "applications", "--validation-method", "bogus"}, `validationMethod must be one of dns, http (got "bogus")`},
	}
	for _, tc := range errorCases {
		t.Run(tc.name, func(t *testing.T) {
			res := create(tc.domain, tc.args...)
			require.NotEqual(t, 0, res.ExitCode, "create unexpectedly succeeded: %s", res.Stdout)
			output := res.Stdout + res.Stderr
			assert.Contains(t, output, tc.want)
			assert.Contains(t, output, "HTTP 400")
			t.Logf("rejected as expected: %s", strings.TrimSpace(output))
		})
	}
}

func getValidationMethod(t *testing.T, env *RealCLITestEnvironment, domain string) string {
	t.Helper()
	res := env.ExecuteCLIWithTimeout(customDomainTimeout, "custom-domain", "get", domain, "-o", "json")
	require.Equal(t, 0, res.ExitCode, "get failed: %s%s", res.Stdout, res.Stderr)
	var items []struct {
		Spec struct {
			ValidationMethod string `json:"validationMethod"`
		} `json:"spec"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.Stdout), &items), res.Stdout)
	require.Len(t, items, 1)
	return items[0].Spec.ValidationMethod
}

func randomSuffix(t *testing.T) string {
	t.Helper()
	b := make([]byte, 4)
	_, err := rand.Read(b)
	require.NoError(t, err)
	return hex.EncodeToString(b)
}
