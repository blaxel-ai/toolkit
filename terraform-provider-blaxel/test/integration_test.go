package test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	providerBinary  = "../terraform-provider-blaxel"
	terraformrcPath = ".terraformrc"
)

// TestIntegrationSandbox tests the single sandbox example
func TestIntegrationSandbox(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check required environment variables
	checkEnvVars(t)

	exampleDir := "../examples/sandbox"

	t.Log("Running sandbox example integration test")

	// Setup provider override
	defer cleanupTerraformRC(t)
	setupTerraformRC(t)

	// Initialize
	t.Log("Running terraform init...")
	runTerraform(t, exampleDir, "init")

	// Validate
	t.Log("Running terraform validate...")
	runTerraform(t, exampleDir, "validate")

	// Plan
	t.Log("Running terraform plan...")
	runTerraform(t, exampleDir, "plan")

	// Apply
	t.Log("Running terraform apply...")
	runTerraformWithAutoApprove(t, exampleDir, "apply")

	// Wait a bit for resources to stabilize
	t.Log("Waiting for resources to stabilize...")
	time.Sleep(5 * time.Second)

	// Show
	t.Log("Running terraform show...")
	output := runTerraform(t, exampleDir, "show")
	t.Logf("Terraform state:\n%s", output)

	// Verify outputs exist
	t.Log("Checking terraform outputs...")
	outputs := runTerraform(t, exampleDir, "output", "-json")
	if !strings.Contains(outputs, "sandbox_status") {
		t.Error("Expected sandbox_status output not found")
	}
	if !strings.Contains(outputs, "sandbox_id") {
		t.Error("Expected sandbox_id output not found")
	}

	// Destroy
	t.Log("Running terraform destroy...")
	runTerraformWithAutoApprove(t, exampleDir, "destroy")

	t.Log("Sandbox integration test completed successfully")
}

// TestIntegrationSandboxCluster tests the sandbox cluster example
func TestIntegrationSandboxCluster(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check required environment variables
	checkEnvVars(t)

	exampleDir := "../examples/sandbox_cluster"

	t.Log("Running sandbox cluster example integration test")

	// Setup provider override
	defer cleanupTerraformRC(t)
	setupTerraformRC(t)

	// Initialize
	t.Log("Running terraform init...")
	runTerraform(t, exampleDir, "init")

	// Validate
	t.Log("Running terraform validate...")
	runTerraform(t, exampleDir, "validate")

	// Plan
	t.Log("Running terraform plan...")
	runTerraform(t, exampleDir, "plan")

	// Apply
	t.Log("Running terraform apply...")
	runTerraformWithAutoApprove(t, exampleDir, "apply")

	// Wait a bit for resources to stabilize
	t.Log("Waiting for resources to stabilize...")
	time.Sleep(10 * time.Second)

	// Show
	t.Log("Running terraform show...")
	output := runTerraform(t, exampleDir, "show")
	t.Logf("Terraform state:\n%s", output)

	// Verify outputs exist
	t.Log("Checking terraform outputs...")
	outputs := runTerraform(t, exampleDir, "output", "-json")
	if !strings.Contains(outputs, "deployed_sandboxes") {
		t.Error("Expected deployed_sandboxes output not found")
	}
	if !strings.Contains(outputs, "cluster_id") {
		t.Error("Expected cluster_id output not found")
	}

	// Verify the correct number of sandboxes were deployed
	if !strings.Contains(outputs, "worker-0") || !strings.Contains(outputs, "worker-1") || !strings.Contains(outputs, "worker-2") {
		t.Error("Expected worker sandboxes not found in output")
	}

	// Destroy
	t.Log("Running terraform destroy...")
	runTerraformWithAutoApprove(t, exampleDir, "destroy")

	t.Log("Sandbox cluster integration test completed successfully")
}

// Helper functions

func checkEnvVars(t *testing.T) {
	// Check for BLAXEL_* or BL_* environment variables (both are valid)
	apiKey := os.Getenv("BL_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("BL_API_KEY")
	}

	workspace := os.Getenv("BL_WORKSPACE")
	if workspace == "" {
		workspace = os.Getenv("BL_WORKSPACE")
	}

	missing := []string{}
	if apiKey == "" {
		missing = append(missing, "BL_API_KEY or BL_API_KEY")
	}
	if workspace == "" {
		missing = append(missing, "BL_WORKSPACE or BL_WORKSPACE")
	}

	if len(missing) > 0 {
		t.Fatalf("Missing required environment variables: %s", strings.Join(missing, ", "))
	}

	// Set the BLAXEL_* variables if they're not set (for Terraform provider)
	if os.Getenv("BL_API_KEY") == "" && apiKey != "" {
		os.Setenv("BL_API_KEY", apiKey)
	}
	if os.Getenv("BL_WORKSPACE") == "" && workspace != "" {
		os.Setenv("BL_WORKSPACE", workspace)
	}
}

func setupTerraformRC(t *testing.T) {
	// Get absolute path to provider binary
	absPath, err := filepath.Abs(providerBinary)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Check if provider binary exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		t.Fatalf("Provider binary not found at %s. Run 'make build' first.", absPath)
	}

	providerDir := filepath.Dir(absPath)

	content := fmt.Sprintf(`provider_installation {
  dev_overrides {
    "blaxel-ai/blaxel" = "%s"
  }
  direct {}
}
`, providerDir)

	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	rcPath := filepath.Join(homeDir, terraformrcPath)

	// Backup existing .terraformrc if it exists
	if _, err := os.Stat(rcPath); err == nil {
		backupPath := rcPath + ".backup"
		if err := os.Rename(rcPath, backupPath); err != nil {
			t.Fatalf("Failed to backup existing .terraformrc: %v", err)
		}
		t.Logf("Backed up existing .terraformrc to %s", backupPath)
	}

	if err := os.WriteFile(rcPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write .terraformrc: %v", err)
	}

	t.Logf("Created .terraformrc at %s", rcPath)
}

func cleanupTerraformRC(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Logf("Warning: Failed to get home directory for cleanup: %v", err)
		return
	}

	rcPath := filepath.Join(homeDir, terraformrcPath)
	backupPath := rcPath + ".backup"

	// Remove the test .terraformrc
	if err := os.Remove(rcPath); err != nil && !os.IsNotExist(err) {
		t.Logf("Warning: Failed to remove .terraformrc: %v", err)
	}

	// Restore backup if it exists
	if _, err := os.Stat(backupPath); err == nil {
		if err := os.Rename(backupPath, rcPath); err != nil {
			t.Logf("Warning: Failed to restore .terraformrc backup: %v", err)
		} else {
			t.Logf("Restored .terraformrc from backup")
		}
	}
}

func runTerraform(t *testing.T, dir string, args ...string) string {
	// Skip terraform init when using dev overrides
	if len(args) > 0 && args[0] == "init" {
		t.Log("Skipping 'terraform init' (using dev overrides)")
		return "Skipped terraform init (dev overrides active)"
	}

	cmd := exec.Command("terraform", args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Terraform output:\n%s", string(output))
		t.Fatalf("Terraform command failed: %v", err)
	}

	return string(output)
}

func runTerraformWithAutoApprove(t *testing.T, dir string, command string) string {
	cmd := exec.Command("terraform", command, "-auto-approve")
	cmd.Dir = dir
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Terraform output:\n%s", string(output))
		t.Fatalf("Terraform %s failed: %v", command, err)
	}

	return string(output)
}

// Helper test to just validate examples without applying
func TestValidateExamples(t *testing.T) {
	examples := []string{"../examples/sandbox", "../examples/sandbox_cluster"}

	// Setup provider override
	defer cleanupTerraformRC(t)
	setupTerraformRC(t)

	for _, example := range examples {
		t.Run(filepath.Base(example), func(t *testing.T) {
			t.Logf("Validating example: %s", example)

			// Initialize
			runTerraform(t, example, "init")

			// Validate
			runTerraform(t, example, "validate")

			// Format check
			output := runTerraform(t, example, "fmt", "-check", "-diff")
			if output != "" {
				t.Logf("Format check output:\n%s", output)
			}

			t.Logf("Example %s validated successfully", example)
		})
	}
}
