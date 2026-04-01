package core

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseRegistryCred(t *testing.T) {
	tests := []struct {
		name        string
		cred        string
		wantReg     string
		wantAuth    string
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid basic",
			cred:     "ghcr.io=user:token",
			wantReg:  "ghcr.io",
			wantAuth: base64.StdEncoding.EncodeToString([]byte("user:token")),
		},
		{
			name:     "valid with url registry",
			cred:     "https://index.docker.io/v1/=myuser:mypass",
			wantReg:  "https://index.docker.io/v1/",
			wantAuth: base64.StdEncoding.EncodeToString([]byte("myuser:mypass")),
		},
		{
			name:     "password with special chars",
			cred:     "ghcr.io=user:p@ss=word:123",
			wantReg:  "ghcr.io",
			wantAuth: base64.StdEncoding.EncodeToString([]byte("user:p@ss=word:123")),
		},
		{
			name:        "missing equals",
			cred:        "ghcr.io-user:token",
			wantErr:     true,
			errContains: "expected registry=username:password",
		},
		{
			name:        "empty registry",
			cred:        "=user:token",
			wantErr:     true,
			errContains: "registry cannot be empty",
		},
		{
			name:        "missing colon in userpass",
			cred:        "ghcr.io=usertoken",
			wantErr:     true,
			errContains: "expected username:password",
		},
		{
			name:        "empty username",
			cred:        "ghcr.io=:token",
			wantErr:     true,
			errContains: "username cannot be empty",
		},
		{
			name:        "empty password",
			cred:        "ghcr.io=user:",
			wantErr:     true,
			errContains: "password cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg, auth, err := ParseRegistryCred(tt.cred)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.errContains)
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Fatalf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if reg != tt.wantReg {
				t.Errorf("registry = %q, want %q", reg, tt.wantReg)
			}
			if auth != tt.wantAuth {
				t.Errorf("auth = %q, want %q", auth, tt.wantAuth)
			}
		})
	}
}

func TestLoadDockerConfigFile(t *testing.T) {
	t.Run("valid file", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.json")
		data := `{"auths":{"ghcr.io":{"auth":"dXNlcjp0b2tlbg=="}}}`
		if err := os.WriteFile(configPath, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}

		config, err := LoadDockerConfigFile(configPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(config.Auths) != 1 {
			t.Fatalf("expected 1 auth entry, got %d", len(config.Auths))
		}
		if config.Auths["ghcr.io"].Auth != "dXNlcjp0b2tlbg==" {
			t.Errorf("unexpected auth value: %s", config.Auths["ghcr.io"].Auth)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		_, err := LoadDockerConfigFile("/nonexistent/path/config.json")
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.json")
		if err := os.WriteFile(configPath, []byte("{invalid"), 0600); err != nil {
			t.Fatal(err)
		}

		_, err := LoadDockerConfigFile(configPath)
		if err == nil {
			t.Fatal("expected error for malformed json")
		}
	})

	t.Run("empty auths", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.json")
		if err := os.WriteFile(configPath, []byte(`{}`), 0600); err != nil {
			t.Fatal(err)
		}

		config, err := LoadDockerConfigFile(configPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config.Auths == nil {
			t.Fatal("expected non-nil auths map")
		}
		if len(config.Auths) != 0 {
			t.Fatalf("expected 0 auth entries, got %d", len(config.Auths))
		}
	})
}

func TestBuildDockerConfigFromFlags(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		config, err := BuildDockerConfigFromFlags(nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if config != nil {
			t.Fatal("expected nil config for empty input")
		}
	})

	t.Run("single cred", func(t *testing.T) {
		config, err := BuildDockerConfigFromFlags([]string{"ghcr.io=user:token"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(config.Auths) != 1 {
			t.Fatalf("expected 1 auth entry, got %d", len(config.Auths))
		}
		if _, ok := config.Auths["ghcr.io"]; !ok {
			t.Fatal("expected ghcr.io in auths")
		}
	})

	t.Run("multiple creds", func(t *testing.T) {
		config, err := BuildDockerConfigFromFlags([]string{
			"ghcr.io=user:token",
			"docker.io=other:pass",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(config.Auths) != 2 {
			t.Fatalf("expected 2 auth entries, got %d", len(config.Auths))
		}
	})

	t.Run("invalid cred in list", func(t *testing.T) {
		_, err := BuildDockerConfigFromFlags([]string{"ghcr.io=user:token", "invalid"})
		if err == nil {
			t.Fatal("expected error for invalid credential")
		}
	})
}

func TestMergeDockerConfigs(t *testing.T) {
	t.Run("both nil", func(t *testing.T) {
		result := MergeDockerConfigs(nil, nil)
		if result != nil {
			t.Fatal("expected nil result")
		}
	})

	t.Run("base only", func(t *testing.T) {
		base := &DockerConfig{
			Auths: map[string]DockerConfigAuth{
				"ghcr.io": {Auth: "base-auth"},
			},
		}
		result := MergeDockerConfigs(base, nil)
		if len(result.Auths) != 1 {
			t.Fatalf("expected 1 auth entry, got %d", len(result.Auths))
		}
		if result.Auths["ghcr.io"].Auth != "base-auth" {
			t.Errorf("expected base-auth, got %s", result.Auths["ghcr.io"].Auth)
		}
	})

	t.Run("override only", func(t *testing.T) {
		override := &DockerConfig{
			Auths: map[string]DockerConfigAuth{
				"ghcr.io": {Auth: "override-auth"},
			},
		}
		result := MergeDockerConfigs(nil, override)
		if result.Auths["ghcr.io"].Auth != "override-auth" {
			t.Errorf("expected override-auth, got %s", result.Auths["ghcr.io"].Auth)
		}
	})

	t.Run("override wins on conflict", func(t *testing.T) {
		base := &DockerConfig{
			Auths: map[string]DockerConfigAuth{
				"ghcr.io":   {Auth: "base-auth"},
				"docker.io": {Auth: "docker-base"},
			},
		}
		override := &DockerConfig{
			Auths: map[string]DockerConfigAuth{
				"ghcr.io":  {Auth: "override-auth"},
				"quay.io":  {Auth: "quay-auth"},
			},
		}
		result := MergeDockerConfigs(base, override)
		if len(result.Auths) != 3 {
			t.Fatalf("expected 3 auth entries, got %d", len(result.Auths))
		}
		if result.Auths["ghcr.io"].Auth != "override-auth" {
			t.Errorf("ghcr.io: expected override-auth, got %s", result.Auths["ghcr.io"].Auth)
		}
		if result.Auths["docker.io"].Auth != "docker-base" {
			t.Errorf("docker.io: expected docker-base, got %s", result.Auths["docker.io"].Auth)
		}
		if result.Auths["quay.io"].Auth != "quay-auth" {
			t.Errorf("quay.io: expected quay-auth, got %s", result.Auths["quay.io"].Auth)
		}
	})

	t.Run("both empty auths", func(t *testing.T) {
		base := &DockerConfig{Auths: map[string]DockerConfigAuth{}}
		override := &DockerConfig{Auths: map[string]DockerConfigAuth{}}
		result := MergeDockerConfigs(base, override)
		if result != nil {
			t.Fatal("expected nil for empty merge")
		}
	})
}

func TestResolveDockerConfig(t *testing.T) {
	t.Run("no config anywhere", func(t *testing.T) {
		dir := t.TempDir()
		data, err := ResolveDockerConfig(dir, nil, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if data != nil {
			t.Fatal("expected nil data when no config exists")
		}
	})

	t.Run("project file only", func(t *testing.T) {
		dir := t.TempDir()
		dockerDir := filepath.Join(dir, ".docker")
		if err := os.MkdirAll(dockerDir, 0700); err != nil {
			t.Fatal(err)
		}
		configData := `{"auths":{"ghcr.io":{"auth":"cHJvamVjdA=="}}}`
		if err := os.WriteFile(filepath.Join(dockerDir, "config.json"), []byte(configData), 0600); err != nil {
			t.Fatal(err)
		}

		data, err := ResolveDockerConfig(dir, nil, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var config DockerConfig
		if err := json.Unmarshal(data, &config); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		if config.Auths["ghcr.io"].Auth != "cHJvamVjdA==" {
			t.Errorf("unexpected auth: %s", config.Auths["ghcr.io"].Auth)
		}
	})

	t.Run("flags only", func(t *testing.T) {
		dir := t.TempDir()
		data, err := ResolveDockerConfig(dir, []string{"ghcr.io=user:token"}, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var config DockerConfig
		if err := json.Unmarshal(data, &config); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		expectedAuth := base64.StdEncoding.EncodeToString([]byte("user:token"))
		if config.Auths["ghcr.io"].Auth != expectedAuth {
			t.Errorf("unexpected auth: %s", config.Auths["ghcr.io"].Auth)
		}
	})

	t.Run("flag file only", func(t *testing.T) {
		dir := t.TempDir()
		flagConfigPath := filepath.Join(dir, "my-config.json")
		configData := `{"auths":{"docker.io":{"auth":"ZmxhZ2ZpbGU="}}}`
		if err := os.WriteFile(flagConfigPath, []byte(configData), 0600); err != nil {
			t.Fatal(err)
		}

		data, err := ResolveDockerConfig(dir, nil, flagConfigPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var config DockerConfig
		if err := json.Unmarshal(data, &config); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}
		if config.Auths["docker.io"].Auth != "ZmxhZ2ZpbGU=" {
			t.Errorf("unexpected auth: %s", config.Auths["docker.io"].Auth)
		}
	})

	t.Run("all three sources with merge priority", func(t *testing.T) {
		dir := t.TempDir()

		// Project file: ghcr.io=project, docker.io=project
		dockerDir := filepath.Join(dir, ".docker")
		if err := os.MkdirAll(dockerDir, 0700); err != nil {
			t.Fatal(err)
		}
		projectConfig := `{"auths":{"ghcr.io":{"auth":"project"},"docker.io":{"auth":"project"}}}`
		if err := os.WriteFile(filepath.Join(dockerDir, "config.json"), []byte(projectConfig), 0600); err != nil {
			t.Fatal(err)
		}

		// Flag file: ghcr.io=flagfile, quay.io=flagfile
		flagConfigPath := filepath.Join(dir, "flag-config.json")
		flagFileConfig := `{"auths":{"ghcr.io":{"auth":"flagfile"},"quay.io":{"auth":"flagfile"}}}`
		if err := os.WriteFile(flagConfigPath, []byte(flagFileConfig), 0600); err != nil {
			t.Fatal(err)
		}

		// Flag creds: ghcr.io=flagcred
		flagCreds := []string{"ghcr.io=flaguser:flagpass"}

		data, err := ResolveDockerConfig(dir, flagCreds, flagConfigPath)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var config DockerConfig
		if err := json.Unmarshal(data, &config); err != nil {
			t.Fatalf("failed to unmarshal result: %v", err)
		}

		// ghcr.io should come from flag creds (highest priority)
		expectedFlagAuth := base64.StdEncoding.EncodeToString([]byte("flaguser:flagpass"))
		if config.Auths["ghcr.io"].Auth != expectedFlagAuth {
			t.Errorf("ghcr.io: expected flag cred auth, got %s", config.Auths["ghcr.io"].Auth)
		}

		// docker.io should come from project file (only source)
		if config.Auths["docker.io"].Auth != "project" {
			t.Errorf("docker.io: expected project auth, got %s", config.Auths["docker.io"].Auth)
		}

		// quay.io should come from flag file
		if config.Auths["quay.io"].Auth != "flagfile" {
			t.Errorf("quay.io: expected flagfile auth, got %s", config.Auths["quay.io"].Auth)
		}

		// Should have exactly 3 entries
		if len(config.Auths) != 3 {
			t.Errorf("expected 3 auth entries, got %d", len(config.Auths))
		}
	})
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
