package test

import (
	"os"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
)

func TestDefaultEnvironment(t *testing.T) {
	// Reset to default state
	blaxel.SetEnvironment(blaxel.EnvProduction)

	if blaxel.GetEnvironment() != blaxel.EnvProduction {
		t.Errorf("expected default environment to be production, got %s", blaxel.GetEnvironment())
	}

	if blaxel.GetBaseURL() != "https://api.blaxel.ai/v0" {
		t.Errorf("expected production base URL, got %s", blaxel.GetBaseURL())
	}

	if blaxel.GetRunURL() != "https://run.blaxel.ai" {
		t.Errorf("expected production run URL, got %s", blaxel.GetRunURL())
	}

	if blaxel.GetAppURL() != "https://app.blaxel.ai" {
		t.Errorf("expected production app URL, got %s", blaxel.GetAppURL())
	}

	if blaxel.GetRegistryURL() != "https://us.registry.blaxel.ai" {
		t.Errorf("expected production registry URL, got %s", blaxel.GetRegistryURL())
	}
}

func TestSetEnvironmentDev(t *testing.T) {
	blaxel.SetEnvironment(blaxel.EnvDevelopment)
	defer blaxel.SetEnvironment(blaxel.EnvProduction) // cleanup

	if blaxel.GetEnvironment() != blaxel.EnvDevelopment {
		t.Errorf("expected dev environment, got %s", blaxel.GetEnvironment())
	}

	if blaxel.GetBaseURL() != "https://api.blaxel.dev/v0" {
		t.Errorf("expected dev base URL, got %s", blaxel.GetBaseURL())
	}

	if blaxel.GetRunURL() != "https://run.blaxel.dev" {
		t.Errorf("expected dev run URL, got %s", blaxel.GetRunURL())
	}

	if blaxel.GetAppURL() != "https://app.blaxel.dev" {
		t.Errorf("expected dev app URL, got %s", blaxel.GetAppURL())
	}

	if blaxel.GetRegistryURL() != "https://eu.registry.blaxel.dev" {
		t.Errorf("expected dev registry URL, got %s", blaxel.GetRegistryURL())
	}
}

func TestSetEnvironmentLocal(t *testing.T) {
	blaxel.SetEnvironment(blaxel.EnvLocal)
	defer blaxel.SetEnvironment(blaxel.EnvProduction) // cleanup

	if blaxel.GetEnvironment() != blaxel.EnvLocal {
		t.Errorf("expected local environment, got %s", blaxel.GetEnvironment())
	}

	if blaxel.GetBaseURL() != "http://localhost:8080/v0" {
		t.Errorf("expected local base URL, got %s", blaxel.GetBaseURL())
	}

	if blaxel.GetRunURL() != "https://run.blaxel.dev" {
		t.Errorf("expected local run URL (uses dev), got %s", blaxel.GetRunURL())
	}

	if blaxel.GetAppURL() != "http://localhost:3000" {
		t.Errorf("expected local app URL, got %s", blaxel.GetAppURL())
	}
}

func TestSetEnvironmentInvalid(t *testing.T) {
	blaxel.SetEnvironment(blaxel.EnvProduction) // start fresh

	// Setting invalid environment should not change current config
	blaxel.SetEnvironment(blaxel.Environment("invalid"))

	if blaxel.GetEnvironment() != blaxel.EnvProduction {
		t.Errorf("expected environment to remain production after invalid set, got %s", blaxel.GetEnvironment())
	}
}

func TestCustomURLSetters(t *testing.T) {
	blaxel.SetEnvironment(blaxel.EnvProduction)
	defer blaxel.SetEnvironment(blaxel.EnvProduction) // reset at end

	customBaseURL := "https://custom.api.example.com/v0"
	customRunURL := "https://custom.run.example.com"
	customAppURL := "https://custom.app.example.com"
	customRegistryURL := "https://custom.registry.example.com"

	blaxel.SetBaseURL(customBaseURL)
	blaxel.SetRunURL(customRunURL)
	blaxel.SetAppURL(customAppURL)
	blaxel.SetRegistryURL(customRegistryURL)

	if blaxel.GetBaseURL() != customBaseURL {
		t.Errorf("expected custom base URL %s, got %s", customBaseURL, blaxel.GetBaseURL())
	}

	if blaxel.GetRunURL() != customRunURL {
		t.Errorf("expected custom run URL %s, got %s", customRunURL, blaxel.GetRunURL())
	}

	if blaxel.GetAppURL() != customAppURL {
		t.Errorf("expected custom app URL %s, got %s", customAppURL, blaxel.GetAppURL())
	}

	if blaxel.GetRegistryURL() != customRegistryURL {
		t.Errorf("expected custom registry URL %s, got %s", customRegistryURL, blaxel.GetRegistryURL())
	}
}

func TestEnvironmentOverridesFromEnvVars(t *testing.T) {
	blaxel.SetEnvironment(blaxel.EnvProduction)
	defer func() {
		// Cleanup env vars
		os.Unsetenv("BL_API_URL")
		os.Unsetenv("BL_RUN_URL")
		os.Unsetenv("BL_APP_URL")
		blaxel.SetEnvironment(blaxel.EnvProduction)
	}()

	// Set environment variables
	os.Setenv("BL_API_URL", "https://override.api.example.com/v0")
	os.Setenv("BL_RUN_URL", "https://override.run.example.com")
	os.Setenv("BL_APP_URL", "https://override.app.example.com")

	blaxel.ApplyEnvironmentOverrides()

	if blaxel.GetBaseURL() != "https://override.api.example.com/v0" {
		t.Errorf("expected overridden base URL, got %s", blaxel.GetBaseURL())
	}

	if blaxel.GetRunURL() != "https://override.run.example.com" {
		t.Errorf("expected overridden run URL, got %s", blaxel.GetRunURL())
	}

	if blaxel.GetAppURL() != "https://override.app.example.com" {
		t.Errorf("expected overridden app URL, got %s", blaxel.GetAppURL())
	}
}

func TestPartialEnvironmentOverrides(t *testing.T) {
	blaxel.SetEnvironment(blaxel.EnvProduction)
	defer func() {
		os.Unsetenv("BL_API_URL")
		blaxel.SetEnvironment(blaxel.EnvProduction)
	}()

	// Only override base URL
	os.Setenv("BL_API_URL", "https://override.api.example.com/v0")

	blaxel.ApplyEnvironmentOverrides()

	// Base URL should be overridden
	if blaxel.GetBaseURL() != "https://override.api.example.com/v0" {
		t.Errorf("expected overridden base URL, got %s", blaxel.GetBaseURL())
	}

	// Other URLs should remain production defaults
	if blaxel.GetRunURL() != "https://run.blaxel.ai" {
		t.Errorf("expected production run URL, got %s", blaxel.GetRunURL())
	}
}

func TestBuildSandboxURL(t *testing.T) {
	blaxel.SetEnvironment(blaxel.EnvProduction)

	url := blaxel.BuildSandboxURL("my-workspace", "my-sandbox")
	expected := "https://run.blaxel.ai/my-workspace/sandboxes/my-sandbox"

	if url != expected {
		t.Errorf("expected %s, got %s", expected, url)
	}
}

func TestBuildSandboxURLDev(t *testing.T) {
	blaxel.SetEnvironment(blaxel.EnvDevelopment)
	defer blaxel.SetEnvironment(blaxel.EnvProduction)

	url := blaxel.BuildSandboxURL("my-workspace", "my-sandbox")
	expected := "https://run.blaxel.dev/my-workspace/sandboxes/my-sandbox"

	if url != expected {
		t.Errorf("expected %s, got %s", expected, url)
	}
}

func TestBuildResourceURL(t *testing.T) {
	blaxel.SetEnvironment(blaxel.EnvProduction)

	testCases := []struct {
		workspace    string
		resourceType string
		resourceName string
		expected     string
	}{
		{"ws1", "agents", "agent1", "https://run.blaxel.ai/ws1/agents/agent1"},
		{"ws1", "functions", "fn1", "https://run.blaxel.ai/ws1/functions/fn1"},
		{"ws1", "jobs", "job1", "https://run.blaxel.ai/ws1/jobs/job1"},
	}

	for _, tc := range testCases {
		url := blaxel.BuildResourceURL(tc.workspace, tc.resourceType, tc.resourceName)
		if url != tc.expected {
			t.Errorf("BuildResourceURL(%s, %s, %s) = %s, expected %s",
				tc.workspace, tc.resourceType, tc.resourceName, url, tc.expected)
		}
	}
}

func TestBuildOAuthURLs(t *testing.T) {
	blaxel.SetEnvironment(blaxel.EnvProduction)

	deviceURL := blaxel.BuildOAuthDeviceURL()
	if deviceURL != "https://api.blaxel.ai/v0/login/device" {
		t.Errorf("expected production device URL, got %s", deviceURL)
	}

	tokenURL := blaxel.BuildOAuthTokenURL()
	if tokenURL != "https://api.blaxel.ai/v0/oauth/token" {
		t.Errorf("expected production token URL, got %s", tokenURL)
	}
}

func TestBuildOAuthURLsDev(t *testing.T) {
	blaxel.SetEnvironment(blaxel.EnvDevelopment)
	defer blaxel.SetEnvironment(blaxel.EnvProduction)

	deviceURL := blaxel.BuildOAuthDeviceURL()
	if deviceURL != "https://api.blaxel.dev/v0/login/device" {
		t.Errorf("expected dev device URL, got %s", deviceURL)
	}

	tokenURL := blaxel.BuildOAuthTokenURL()
	if tokenURL != "https://api.blaxel.dev/v0/oauth/token" {
		t.Errorf("expected dev token URL, got %s", tokenURL)
	}
}

func TestBuildObservabilityLogsURL(t *testing.T) {
	blaxel.SetEnvironment(blaxel.EnvProduction)

	url := blaxel.BuildObservabilityLogsURL()
	expected := "https://api.blaxel.ai/v0/observability/logs"

	if url != expected {
		t.Errorf("expected %s, got %s", expected, url)
	}
}

func TestEnvironmentSwitchResetsURLs(t *testing.T) {
	// Start with production
	blaxel.SetEnvironment(blaxel.EnvProduction)

	// Override a URL
	blaxel.SetBaseURL("https://custom.example.com")

	// Switch to dev - should use dev URLs, not the custom one
	blaxel.SetEnvironment(blaxel.EnvDevelopment)

	if blaxel.GetBaseURL() != "https://api.blaxel.dev/v0" {
		t.Errorf("expected dev base URL after environment switch, got %s", blaxel.GetBaseURL())
	}

	// Cleanup
	blaxel.SetEnvironment(blaxel.EnvProduction)
}

func TestInitializeEnvironment(t *testing.T) {
	// This test requires a mock config or temp config file
	// For now, test with empty workspace which should default to production

	defer func() {
		os.Unsetenv("BL_API_URL")
		blaxel.SetEnvironment(blaxel.EnvProduction)
	}()

	// Set an env var override
	os.Setenv("BL_API_URL", "https://test.override.com/v0")

	// Initialize with empty workspace (should use production + env overrides)
	blaxel.InitializeEnvironment("")

	// Should have the override applied
	if blaxel.GetBaseURL() != "https://test.override.com/v0" {
		t.Errorf("expected overridden base URL after InitializeEnvironment, got %s", blaxel.GetBaseURL())
	}
}

// Benchmark tests
func BenchmarkGetBaseURL(b *testing.B) {
	blaxel.SetEnvironment(blaxel.EnvProduction)
	for i := 0; i < b.N; i++ {
		_ = blaxel.GetBaseURL()
	}
}

func BenchmarkBuildSandboxURL(b *testing.B) {
	blaxel.SetEnvironment(blaxel.EnvProduction)
	for i := 0; i < b.N; i++ {
		_ = blaxel.BuildSandboxURL("workspace", "sandbox-name")
	}
}

func BenchmarkSetEnvironment(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			blaxel.SetEnvironment(blaxel.EnvProduction)
		} else {
			blaxel.SetEnvironment(blaxel.EnvDevelopment)
		}
	}
}
