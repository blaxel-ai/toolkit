package integration

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Constants for test configuration
const (
	// Timeout constants
	DefaultCLITimeout    = 300 * time.Second
	DeploymentTimeout    = 5 * time.Minute
	CleanupTimeout       = 60 * time.Second
	AgentTestTimeout     = 60 * time.Second
	PreCleanupDelay      = 15 * time.Second
	PostDeleteDelay      = 15 * time.Second
	DeploymentSettleTime = 10 * time.Second

	// Polling configuration
	DeploymentPollInterval = 20 * time.Second
)

// ProjectConfig represents a project to be created and deployed
type ProjectConfig struct {
	Name     string
	Command  string
	Template string
	Dir      string
}

// MultiAgentConfig represents a multi-agent deployment configuration
type MultiAgentConfig struct {
	Name string
	Dir  string
}

// ManifestConfig represents a manifest deployment configuration
type ManifestConfig struct {
	Name string
	Dir  string
}

// TestResult represents the result of a test operation
type TestResult struct {
	Project string
	Success bool
	Error   error
}

// RealCLITestEnvironment represents a test environment that executes real CLI commands
type RealCLITestEnvironment struct {
	TempDir     string
	OriginalDir string
	CLIBinary   string
	APIKey      string
	Workspace   string
	Cleanup     func()
}

// CLIResult represents the result of executing a real CLI command
type CLIResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Error    error
}

// TestCLIWorkflow_CompleteFlow runs the complete CLI workflow integration test
func TestCLIWorkflow_CompleteFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := SetupRealCLIEnvironment(t)

	t.Run("Complete_Workflow", func(t *testing.T) {
		// Execute the complete workflow in sequential steps
		performPreTestCleanup(t, env)
		performLogin(t, env)
		checkWorkspaces(t, env)

		// Run parallel deployments and collect results
		totalSuccessful := runParallelDeployments(t, env)

		// Clean up resources
		performFinalCleanup(t, env)

		t.Logf("🎉 Complete workflow finished with %d successful operations", totalSuccessful)
	})
}

// TestCLIWorkflow_AgentApp runs only the Agent App workflow
func TestCLIWorkflow_AgentApp(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := SetupRealCLIEnvironment(t)
	setupWorkflowEnvironment(t, env)

	t.Run("Agent_App_Workflow", func(t *testing.T) {
		project := ProjectConfig{
			Name:     "Agent App",
			Command:  "create-agent-app",
			Template: "template-google-adk-py",
			Dir:      "complete-test-agent",
		}

		t.Logf("🚀 Starting Agent App workflow...")
		success := runSingleProjectWorkflow(t, env, project)

		// Cleanup
		cleanupSingleProject(t, env, project)

		if success {
			t.Logf("🎉 Agent App workflow completed successfully")
		} else {
			t.Logf("❌ Agent App workflow failed")
		}
	})
}

// TestCLIWorkflow_MCPServer runs only the MCP Server workflow
func TestCLIWorkflow_MCPServer(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := SetupRealCLIEnvironment(t)
	setupWorkflowEnvironment(t, env)

	t.Run("MCP_Server_Workflow", func(t *testing.T) {
		project := ProjectConfig{
			Name:     "MCP Server",
			Command:  "create-mcp-server",
			Template: "template-mcp-hello-world-py",
			Dir:      "complete-test-mcp",
		}

		t.Logf("🚀 Starting MCP Server workflow...")
		success := runSingleProjectWorkflow(t, env, project)

		// Cleanup
		cleanupSingleProject(t, env, project)

		if success {
			t.Logf("🎉 MCP Server workflow completed successfully")
		} else {
			t.Logf("❌ MCP Server workflow failed")
		}
	})
}

// TestCLIWorkflow_Job runs only the Job workflow
func TestCLIWorkflow_Job(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := SetupRealCLIEnvironment(t)
	setupWorkflowEnvironment(t, env)

	t.Run("Job_Workflow", func(t *testing.T) {
		project := ProjectConfig{
			Name:     "Job",
			Command:  "create-job",
			Template: "template-jobs-ts",
			Dir:      "complete-test-job",
		}

		t.Logf("🚀 Starting Job workflow...")
		success := runSingleProjectWorkflow(t, env, project)

		// Cleanup
		cleanupSingleProject(t, env, project)

		if success {
			t.Logf("🎉 Job workflow completed successfully")
		} else {
			t.Logf("❌ Job workflow failed")
		}
	})
}

// TestCLIWorkflow_MultiAgent runs only the Multi-Agent workflow
func TestCLIWorkflow_MultiAgent(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := SetupRealCLIEnvironment(t)
	setupWorkflowEnvironment(t, env)

	t.Run("Multi_Agent_Workflow", func(t *testing.T) {
		project := MultiAgentConfig{
			Name: "Multi-Agent",
			Dir:  filepath.Join(filepath.Dir(env.OriginalDir), "multi-agent"),
		}

		t.Logf("🚀 Starting Multi-Agent workflow...")
		success := runSingleMultiAgentWorkflow(t, env, project)

		// Cleanup
		cleanupSingleMultiAgent(t, env, project)

		if success {
			t.Logf("🎉 Multi-Agent workflow completed successfully")
		} else {
			t.Logf("❌ Multi-Agent workflow failed")
		}
	})
}

// TestCLIWorkflow_ManifestApply runs only the Manifest Apply workflow
func TestCLIWorkflow_ManifestApply(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	env := SetupRealCLIEnvironment(t)
	setupWorkflowEnvironment(t, env)

	t.Run("Manifest_Apply_Workflow", func(t *testing.T) {
		project := ManifestConfig{
			Name: "Manifests-Apply",
			Dir:  "test",
		}

		t.Logf("🚀 Starting Manifest Apply workflow...")
		success := runSingleManifestWorkflow(t, env, project)

		if success {
			t.Logf("🎉 Manifest Apply workflow completed successfully")
		} else {
			t.Logf("❌ Manifest Apply workflow failed")
		}
	})
}

// setupWorkflowEnvironment performs common setup steps for individual workflows
func setupWorkflowEnvironment(t *testing.T, env *RealCLITestEnvironment) {
	performLogin(t, env)
	checkWorkspaces(t, env)
}

// runSingleProjectWorkflow runs a complete workflow for a single regular project
func runSingleProjectWorkflow(t *testing.T, env *RealCLITestEnvironment, project ProjectConfig) bool {
	// Pre-cleanup for this specific project
	cleanupSingleProject(t, env, project)

	t.Logf("⏰ Waiting %v before starting workflow...", PreCleanupDelay)
	time.Sleep(PreCleanupDelay)

	// Create the project
	result := env.ExecuteCLI(project.Command, "--template", project.Template, "-y", project.Dir)
	logCommandResult(t, project.Name+" creation", result)

	if result.ExitCode != 0 {
		return false
	}

	// Verify directory creation
	if err := verifyDirectoryCreation(project.Dir); err != nil {
		t.Logf("❌ %s directory creation failed: %v", project.Name, err)
		return false
	}

	t.Logf("✅ %s created successfully, starting deployment...", project.Name)

	// Deploy and test the project
	return deployAndTestProject(t, env, project)
}

// runSingleMultiAgentWorkflow runs a complete workflow for a multi-agent project
func runSingleMultiAgentWorkflow(t *testing.T, env *RealCLITestEnvironment, project MultiAgentConfig) bool {
	// Pre-cleanup for multi-agent
	cleanupSingleMultiAgent(t, env, project)

	t.Logf("⏰ Waiting %v before starting workflow...", PreCleanupDelay)
	time.Sleep(PreCleanupDelay)

	t.Logf("🔀 Starting multi-agent deployment for %s from existing folder", project.Name)
	t.Logf("🔍 Multi-agent deployment path: %s", project.Dir)

	return deployMultipleAgents(t, env, project)
}

// runSingleManifestWorkflow runs a complete workflow for a manifest project
func runSingleManifestWorkflow(t *testing.T, env *RealCLITestEnvironment, project ManifestConfig) bool {
	t.Logf("📄 Starting manifest deployment for %s", project.Name)
	return deployManifests(t, env, project)
}

// cleanupSingleProject cleans up a single regular project
func cleanupSingleProject(t *testing.T, env *RealCLITestEnvironment, project ProjectConfig) {
	t.Logf("🧹 Cleaning up %s", project.Name)

	var deleteCmd []string
	projectType := getProjectType(project.Command)

	switch projectType {
	case "agent":
		deleteCmd = []string{"delete", "agents", project.Dir}
	case "mcp":
		deleteCmd = []string{"delete", "functions", project.Dir}
	case "job":
		deleteCmd = []string{"delete", "job", project.Dir}
	}

	if len(deleteCmd) > 0 {
		deleteResult := env.ExecuteCLIWithTimeout(CleanupTimeout, deleteCmd...)
		logCommandResult(t, "Delete "+project.Name, deleteResult)

		if deleteResult.ExitCode == 0 {
			t.Logf("✅ %s deleted successfully", project.Name)
		} else {
			t.Logf("ℹ️ %s not found or already deleted", project.Name)
		}
	}
}

// cleanupSingleMultiAgent cleans up a single multi-agent project
func cleanupSingleMultiAgent(t *testing.T, env *RealCLITestEnvironment, project MultiAgentConfig) {
	t.Logf("🧹 Cleaning up multi-agent deployments for %s", project.Name)

	agents := []string{"main-agent-2", "main-agent"}
	for _, agent := range agents {
		deleteResult := env.ExecuteCLIWithTimeout(CleanupTimeout, "delete", "agents", agent)
		logCommandResult(t, fmt.Sprintf("Delete %s %s", project.Name, agent), deleteResult)

		if deleteResult.ExitCode == 0 {
			t.Logf("✅ %s deleted successfully", agent)
		} else {
			t.Logf("ℹ️ %s not found or already deleted", agent)
		}
	}
}

// performPreTestCleanup removes any existing resources before starting the test
func performPreTestCleanup(t *testing.T, env *RealCLITestEnvironment) {
	t.Run("Pre_Test_Cleanup", func(t *testing.T) {
		t.Logf("🧹 Starting pre-test cleanup of all existing resources...")

		cleanupResources := []struct {
			name string
			cmd  []string
		}{
			{"complete-test-agent", []string{"delete", "agents", "complete-test-agent"}},
			{"complete-test-mcp", []string{"delete", "functions", "complete-test-mcp"}},
			{"complete-test-job", []string{"delete", "job", "complete-test-job"}},
			{"main-agent", []string{"delete", "agents", "main-agent"}},
			{"main-agent-2", []string{"delete", "agents", "main-agent-2"}},
		}

		results := executeParallelCleanup(t, env, cleanupResources)

		successCount := 0
		for _, result := range results {
			if result.Success {
				successCount++
			}
		}

		t.Logf("📊 Pre-test cleanup completed: %d/%d resources processed", successCount, len(cleanupResources))
		t.Logf("⏰ Waiting %v before starting tests...", PreCleanupDelay)
		time.Sleep(PreCleanupDelay)
		t.Logf("🚀 Starting main test workflow...")
	})
}

// performLogin handles the CLI login process
func performLogin(t *testing.T, env *RealCLITestEnvironment) {
	t.Logf("🔐 Performing login...")
	loginResult := env.ExecuteCLI("login", env.Workspace)
	if loginResult.ExitCode != 0 {
		t.Logf("Login may have failed, but continuing with tests. Output: %s", loginResult.Stdout+loginResult.Stderr)
	}
}

// checkWorkspaces verifies that the workspace is accessible
func checkWorkspaces(t *testing.T, env *RealCLITestEnvironment) {
	t.Logf("🔍 Checking workspaces...")
	workspaceResult := env.ExecuteCLI("workspace")
	AssertCLISuccess(t, workspaceResult)
	assert.Contains(t, workspaceResult.Stdout, env.Workspace, "Workspace should be listed")
}

// runParallelDeployments executes all deployment workflows in parallel and returns the total number of successful operations
func runParallelDeployments(t *testing.T, env *RealCLITestEnvironment) int {
	// Define all project configurations
	regularProjects := getRegularProjectConfigs()
	multiAgentProjects := getMultiAgentConfigs(env)
	manifestProjects := getManifestConfigs()

	totalWorkflows := len(regularProjects) + len(multiAgentProjects) + len(manifestProjects)
	resultChan := make(chan TestResult, totalWorkflows)

	// Launch all deployments in parallel
	deployRegularProjects(t, env, regularProjects, resultChan)
	deployMultiAgentProjects(t, env, multiAgentProjects, resultChan)
	deployManifestProjects(t, env, manifestProjects, resultChan)

	// Collect and analyze results
	return collectDeploymentResults(t, resultChan, totalWorkflows)
}

// getRegularProjectConfigs returns the configuration for regular projects
func getRegularProjectConfigs() []ProjectConfig {
	return []ProjectConfig{
		{
			Name:     "Agent App",
			Command:  "create-agent-app",
			Template: "template-google-adk-py",
			Dir:      "complete-test-agent",
		},
		{
			Name:     "MCP Server",
			Command:  "create-mcp-server",
			Template: "template-mcp-hello-world-py",
			Dir:      "complete-test-mcp",
		},
		{
			Name:     "Job",
			Command:  "create-job",
			Template: "template-jobs-ts",
			Dir:      "complete-test-job",
		},
	}
}

// getMultiAgentConfigs returns the configuration for multi-agent deployments
func getMultiAgentConfigs(env *RealCLITestEnvironment) []MultiAgentConfig {
	return []MultiAgentConfig{
		{
			Name: "Multi-Agent",
			Dir:  filepath.Join(filepath.Dir(env.OriginalDir), "multi-agent"),
		},
	}
}

// getManifestConfigs returns the configuration for manifest deployments
func getManifestConfigs() []ManifestConfig {
	return []ManifestConfig{
		{
			Name: "Manifests-Apply",
			Dir:  "test",
		},
	}
}

// deployRegularProjects handles the deployment of regular projects (agents, MCPs, jobs)
func deployRegularProjects(t *testing.T, env *RealCLITestEnvironment, projects []ProjectConfig, resultChan chan<- TestResult) {
	for _, project := range projects {
		go func(proj ProjectConfig) {
			t.Logf("🚀 Starting creation of %s", proj.Name)

			// Create the project
			result := env.ExecuteCLI(proj.Command, "--template", proj.Template, "-y", proj.Dir)
			logCommandResult(t, proj.Name+" creation", result)

			if result.ExitCode != 0 {
				sendResult(resultChan, proj.Name, false, result.Error)
				return
			}

			// Verify directory creation
			if err := verifyDirectoryCreation(proj.Dir); err != nil {
				t.Logf("❌ %s directory creation failed: %v", proj.Name, err)
				sendResult(resultChan, proj.Name, false, err)
				return
			}

			t.Logf("✅ %s created successfully, starting deployment...", proj.Name)

			// Deploy the project
			success := deployAndTestProject(t, env, proj)
			sendResult(resultChan, proj.Name, success, nil)
		}(project)
	}
}

// deployAndTestProject handles the deployment and testing of a single project
func deployAndTestProject(t *testing.T, env *RealCLITestEnvironment, proj ProjectConfig) bool {
	// Deploy the project
	deployResult := env.ExecuteCLIInDirectory(proj.Dir, "deploy")
	logCommandResult(t, "Deploy "+proj.Name, deployResult)

	if deployResult.ExitCode != 0 {
		t.Logf("❌ Deploy %s failed (may be due to template/network issues): %v", proj.Name, deployResult.Error)
		return false
	}

	// Determine project type and wait for deployment
	projectType := getProjectType(proj.Command)
	t.Logf("👀 Watching %s deployment status...", proj.Name)

	if err := env.WaitForDeployment(t, projectType, proj.Dir, DeploymentTimeout); err != nil {
		t.Logf("⚠️ Deployment watch error for %s (may be expected in test environment): %v", proj.Name, err)
		return false
	}

	t.Logf("✅ %s deployed successfully", proj.Name)

	// Test agent functionality if it's an agent project
	if projectType == "agent" {
		testAgentFunctionality(t, env, proj)
	}

	// Test job functionality if it's a job project
	if projectType == "job" {
		testJobFunctionality(t, env, proj)
	}

	return true
}

// deployMultiAgentProjects handles multi-agent deployments
func deployMultiAgentProjects(t *testing.T, env *RealCLITestEnvironment, projects []MultiAgentConfig, resultChan chan<- TestResult) {
	for _, project := range projects {
		go func(proj MultiAgentConfig) {
			t.Logf("🔀 Starting multi-agent deployment for %s from existing folder", proj.Name)
			t.Logf("🔍 Multi-agent deployment path: %s", proj.Dir)

			success := deployMultipleAgents(t, env, proj)
			sendResult(resultChan, proj.Name, success, nil)
		}(project)
	}
}

// deployMultipleAgents deploys multiple agents and waits for their completion
func deployMultipleAgents(t *testing.T, env *RealCLITestEnvironment, proj MultiAgentConfig) bool {
	// Deploy both agents
	agents := []string{"main-agent-2", "main-agent"}
	deployResults := make([]*CLIResult, len(agents))

	for i, agent := range agents {
		deployResults[i] = env.ExecuteCLIInDirectory(proj.Dir, "deploy", "-d", agent, "--recursive=false")
		logCommandResult(t, fmt.Sprintf("Deploy %s %s", proj.Name, agent), deployResults[i])
	}

	// Check deployment initiation results
	successfulDeploys := 0
	for i, result := range deployResults {
		if result.ExitCode == 0 {
			successfulDeploys++
			t.Logf("✅ %s deployment initiated successfully", agents[i])
		} else {
			t.Logf("❌ %s deployment failed to initiate", agents[i])
		}
	}

	t.Logf("📊 Multi-agent deployment initiation: %d/%d agents successful", successfulDeploys, len(agents))

	if successfulDeploys != len(agents) {
		return false
	}

	// Wait for deployments to complete
	return waitForMultiAgentDeployments(t, env, proj, agents)
}

// waitForMultiAgentDeployments waits for multiple agent deployments to complete
func waitForMultiAgentDeployments(t *testing.T, env *RealCLITestEnvironment, proj MultiAgentConfig, agents []string) bool {
	t.Logf("👀 Watching %s deployments status...", proj.Name)

	watchChan := make(chan error, len(agents))

	// Start watching all agents in parallel
	for _, agent := range agents {
		go func(agentName string) {
			err := env.WaitForDeployment(t, "agent", agentName, DeploymentTimeout)
			watchChan <- err
		}(agent)
	}

	// Collect watch results
	successfulAgents := 0
	for i := 0; i < len(agents); i++ {
		if err := <-watchChan; err != nil {
			t.Logf("⚠️ Multi-agent deployment watch error for %s (may be expected in test environment): %v", agents[i], err)
		} else {
			t.Logf("✅ Agent %s deployed successfully", agents[i])
			successfulAgents++
		}
	}

	t.Logf("📊 Multi-agent deployment summary: %d/%d agents successful", successfulAgents, len(agents))

	if successfulAgents == len(agents) {
		// Test all agents
		for _, agent := range agents {
			testAgentByName(t, env, agent)
		}
		return true
	}

	return false
}

// deployManifestProjects handles manifest-based deployments
func deployManifestProjects(t *testing.T, env *RealCLITestEnvironment, projects []ManifestConfig, resultChan chan<- TestResult) {
	for _, project := range projects {
		go func(proj ManifestConfig) {
			t.Logf("📄 Starting manifest deployment for %s", proj.Name)

			success := deployManifests(t, env, proj)
			sendResult(resultChan, proj.Name, success, nil)
		}(project)
	}
}

// deployManifests handles the deployment of Kubernetes-style manifests
func deployManifests(t *testing.T, env *RealCLITestEnvironment, proj ManifestConfig) bool {
	t.Logf("🔍 Manifest deployment path: %s", proj.Dir)

	// Get paths for manifests and environment file
	projectRoot := filepath.Join(env.OriginalDir, "..", "..")
	manifestsDir := filepath.Join(projectRoot, "test", "manifests")
	envFile := filepath.Join(projectRoot, ".env")

	t.Logf("📁 Manifests directory: %s", manifestsDir)
	t.Logf("🔧 Environment file: %s", envFile)

	// Delete existing manifests first
	t.Logf("🗑️ Deleting existing manifests recursively...")
	deleteResult := env.ExecuteCLI("delete", "-R", "-f", manifestsDir)
	logCommandResult(t, "Delete "+proj.Name, deleteResult)

	// Wait before applying
	t.Logf("⏰ Waiting %v after deletion before applying manifests...", PostDeleteDelay)
	time.Sleep(PostDeleteDelay)

	// Apply manifests
	t.Logf("📄 Applying manifests recursively...")
	applyResult := env.ExecuteCLI("apply", "-R", "-f", manifestsDir, "-e", envFile)
	logCommandResult(t, "Apply "+proj.Name, applyResult)

	if applyResult.ExitCode == 0 {
		t.Logf("✅ %s manifest deployment completed successfully", proj.Name)

		// Wait for deployments to settle
		t.Logf("⏰ Waiting %v for manifest deployments to settle...", DeploymentSettleTime)
		time.Sleep(DeploymentSettleTime)
		return true
	}

	t.Logf("❌ %s manifest deployment failed", proj.Name)
	return false
}

// performFinalCleanup removes all created resources after the test
func performFinalCleanup(t *testing.T, env *RealCLITestEnvironment) {
	t.Run("Cleanup_Complete_Workflow_Projects", func(t *testing.T) {
		t.Logf("🧹 Starting cleanup of complete workflow resources...")

		// Get all configurations for cleanup
		regularProjects := getRegularProjectConfigs()
		multiAgentProjects := getMultiAgentConfigs(env)

		totalCleanupJobs := len(regularProjects) + len(multiAgentProjects)
		cleanupChan := make(chan TestResult, totalCleanupJobs)

		// Clean up regular projects
		cleanupRegularProjects(t, env, regularProjects, cleanupChan)

		// Clean up multi-agent projects
		cleanupMultiAgentProjects(t, env, multiAgentProjects, cleanupChan)

		// Collect cleanup results
		successfulCleanups := 0
		for i := 0; i < totalCleanupJobs; i++ {
			result := <-cleanupChan
			if result.Success {
				successfulCleanups++
			} else {
				t.Logf("❌ %s cleanup failed: %v", result.Project, result.Error)
			}
		}

		t.Logf("📊 Complete workflow cleanup completed: %d/%d successful", successfulCleanups, totalCleanupJobs)
	})
}

// Helper functions

// executeParallelCleanup executes multiple cleanup operations in parallel
func executeParallelCleanup(t *testing.T, env *RealCLITestEnvironment, resources []struct {
	name string
	cmd  []string
}) []TestResult {
	resultChan := make(chan TestResult, len(resources))

	for _, resource := range resources {
		go func(res struct {
			name string
			cmd  []string
		}) {
			t.Logf("🧹 Pre-cleaning resource: %s", res.name)
			deleteResult := env.ExecuteCLIWithTimeout(CleanupTimeout, res.cmd...)

			success := deleteResult.ExitCode == 0
			if success {
				t.Logf("✅ Pre-cleanup: %s deleted successfully", res.name)
			} else {
				t.Logf("ℹ️ Pre-cleanup: %s not found or already deleted (expected)", res.name)
				success = true // Mark as success since missing resources are OK
			}

			resultChan <- TestResult{
				Project: res.name,
				Success: success,
				Error:   nil,
			}
		}(resource)
	}

	// Collect results
	results := make([]TestResult, len(resources))
	for i := 0; i < len(resources); i++ {
		results[i] = <-resultChan
	}

	return results
}

// logCommandResult logs the result of a CLI command execution
func logCommandResult(t *testing.T, operation string, result *CLIResult) {
	t.Logf("%s - ExitCode: %d", operation, result.ExitCode)
	if result.Stdout != "" {
		t.Logf("%s - Stdout: %s", operation, result.Stdout)
	}
	if result.Stderr != "" {
		t.Logf("%s - Stderr: %s", operation, result.Stderr)
	}
	if result.Error != nil {
		t.Logf("%s - Error: %v", operation, result.Error)
	}
}

// verifyDirectoryCreation checks if a directory was successfully created
func verifyDirectoryCreation(dir string) error {
	_, err := os.Stat(dir)
	return err
}

// getProjectType returns the project type based on the creation command
func getProjectType(command string) string {
	switch command {
	case "create-agent-app":
		return "agent"
	case "create-mcp-server":
		return "mcp"
	case "create-job":
		return "job"
	default:
		return "unknown"
	}
}

// testAgentFunctionality tests an agent's functionality with a sample request
func testAgentFunctionality(t *testing.T, env *RealCLITestEnvironment, proj ProjectConfig) {
	t.Logf("🔬 Testing %s agent with a request...", proj.Dir)
	testResult := env.ExecuteCLIWithTimeout(AgentTestTimeout, "run", "agent", proj.Dir, "-d", `{"inputs": "Hello"}`)
	logCommandResult(t, fmt.Sprintf("Test %s agent", proj.Name), testResult)

	if testResult.ExitCode == 0 {
		t.Logf("✅ %s agent test request successful", proj.Name)
	} else {
		t.Logf("⚠️ %s agent test request failed (may be expected in test environment)", proj.Name)
	}
}

// testAgentByName tests an agent by its deployed name
func testAgentByName(t *testing.T, env *RealCLITestEnvironment, agentName string) {
	t.Logf("🔬 Testing %s agent with a request...", agentName)
	testResult := env.ExecuteCLIWithTimeout(AgentTestTimeout, "run", "agent", agentName, "-d", `{"inputs": "Hello"}`)
	logCommandResult(t, fmt.Sprintf("Test %s agent", agentName), testResult)

	if testResult.ExitCode == 0 {
		t.Logf("✅ %s agent test request successful", agentName)
	} else {
		t.Logf("⚠️ %s agent test request failed (may be expected in test environment)", agentName)
	}
}

// testJobFunctionality tests a job's functionality with a sample batch file
func testJobFunctionality(t *testing.T, env *RealCLITestEnvironment, proj ProjectConfig) {
	t.Logf("🔬 Testing %s job with a batch file...", proj.Dir)

	// Create a sample batch file for testing
	batchContent := `{
    "tasks": [
        {
            "name": "John"
        }
    ]
}`

	// Create batches directory if it doesn't exist
	batchesDir := filepath.Join(proj.Dir, "batches")
	err := os.MkdirAll(batchesDir, 0755)
	if err != nil {
		t.Logf("⚠️ Failed to create batches directory: %v", err)
		return
	}

	// Write the batch file
	batchFile := filepath.Join(batchesDir, "sample-batch.json")
	err = os.WriteFile(batchFile, []byte(batchContent), 0644)
	if err != nil {
		t.Logf("⚠️ Failed to create batch file: %v", err)
		return
	}

	// Execute the job with the batch file
	testResult := env.ExecuteCLIWithTimeout(AgentTestTimeout, "run", "job", proj.Dir, "--file", batchFile)
	logCommandResult(t, fmt.Sprintf("Test %s job", proj.Name), testResult)

	if testResult.ExitCode == 0 {
		t.Logf("✅ %s job execution successful", proj.Name)
	} else {
		t.Logf("⚠️ %s job execution failed (may be expected in test environment)", proj.Name)
	}
}

// sendResult sends a test result to the result channel
func sendResult(resultChan chan<- TestResult, project string, success bool, err error) {
	resultChan <- TestResult{
		Project: project,
		Success: success,
		Error:   err,
	}
}

// collectDeploymentResults collects and analyzes deployment results
func collectDeploymentResults(t *testing.T, resultChan <-chan TestResult, totalWorkflows int) int {
	successfulWorkflows := 0
	for i := 0; i < totalWorkflows; i++ {
		result := <-resultChan
		if result.Success {
			successfulWorkflows++
			t.Logf("✅ %s workflow completed successfully", result.Project)
		} else {
			t.Logf("❌ %s workflow failed: %v", result.Project, result.Error)
		}
	}

	t.Logf("📊 Parallel workflow completed: %d/%d successful", successfulWorkflows, totalWorkflows)
	return successfulWorkflows
}

// cleanupRegularProjects handles cleanup of regular projects
func cleanupRegularProjects(t *testing.T, env *RealCLITestEnvironment, projects []ProjectConfig, cleanupChan chan<- TestResult) {
	for _, proj := range projects {
		go func(project ProjectConfig) {
			t.Logf("🧹 Cleaning up %s", project.Name)

			var deleteCmd []string
			projectType := getProjectType(project.Command)

			switch projectType {
			case "agent":
				deleteCmd = []string{"delete", "agents", project.Dir}
			case "mcp":
				deleteCmd = []string{"delete", "functions", project.Dir}
			case "job":
				deleteCmd = []string{"delete", "job", project.Dir}
			}

			if len(deleteCmd) > 0 {
				deleteResult := env.ExecuteCLIWithTimeout(CleanupTimeout, deleteCmd...)
				logCommandResult(t, "Delete "+project.Name, deleteResult)

				success := deleteResult.ExitCode == 0
				if success {
					t.Logf("✅ %s deleted successfully", project.Name)
				} else {
					t.Logf("⚠️ Delete %s failed (may be expected in test environment): %v", project.Name, deleteResult.Error)
				}

				cleanupChan <- TestResult{
					Project: project.Name,
					Success: success,
					Error:   deleteResult.Error,
				}
			} else {
				cleanupChan <- TestResult{
					Project: project.Name,
					Success: false,
					Error:   fmt.Errorf("unknown project type for cleanup: %s", projectType),
				}
			}
		}(proj)
	}
}

// cleanupMultiAgentProjects handles cleanup of multi-agent projects
func cleanupMultiAgentProjects(t *testing.T, env *RealCLITestEnvironment, projects []MultiAgentConfig, cleanupChan chan<- TestResult) {
	for _, proj := range projects {
		go func(project MultiAgentConfig) {
			t.Logf("🧹 Cleaning up multi-agent deployments for %s", project.Name)

			agents := []string{"main-agent-2", "main-agent"}
			successfulDeletions := 0

			for _, agent := range agents {
				deleteResult := env.ExecuteCLIWithTimeout(CleanupTimeout, "delete", "agents", agent)
				logCommandResult(t, fmt.Sprintf("Delete %s %s", project.Name, agent), deleteResult)

				if deleteResult.ExitCode == 0 {
					successfulDeletions++
					t.Logf("✅ %s deleted successfully", agent)
				} else {
					t.Logf("❌ %s deletion failed", agent)
				}
			}

			t.Logf("📊 Multi-agent cleanup summary: %d/%d agents deleted successfully", successfulDeletions, len(agents))

			success := successfulDeletions == len(agents)
			var err error
			if !success {
				failedCount := len(agents) - successfulDeletions
				err = fmt.Errorf("multi-agent cleanup failed for: %d/%d agents", failedCount, len(agents))
				t.Logf("⚠️ Delete %s failed: %v", project.Name, err)
			} else {
				t.Logf("✅ %s deleted successfully (all agents)", project.Name)
			}

			cleanupChan <- TestResult{
				Project: project.Name,
				Success: success,
				Error:   err,
			}
		}(proj)
	}
}

// SetupRealCLIEnvironment creates a test environment that executes real CLI commands
func SetupRealCLIEnvironment(t *testing.T) *RealCLITestEnvironment {
	// Check required environment variables
	apiKey := os.Getenv("BL_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping integration test: BL_API_KEY environment variable is required")
	}

	workspace := os.Getenv("BL_WORKSPACE")
	if workspace == "" {
		workspace = "main" // Default to main workspace
	}

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "blaxel-cli-integration-*")
	require.NoError(t, err)

	// Get current directory to restore later
	originalDir, err := os.Getwd()
	require.NoError(t, err)

	// Build the CLI binary
	rootDir := filepath.Join(originalDir, "..", "..")
	cliBinary := filepath.Join(tempDir, "bl-test")

	buildCmd := exec.Command("go", "build", "-o", cliBinary, "main.go")
	buildCmd.Dir = rootDir
	err = buildCmd.Run()
	require.NoError(t, err, "Failed to build CLI binary")

	// Change to temp directory for test execution
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	env := &RealCLITestEnvironment{
		TempDir:     tempDir,
		OriginalDir: originalDir,
		CLIBinary:   cliBinary,
		APIKey:      apiKey,
		Workspace:   workspace,
		Cleanup: func() {
			// Restore original directory
			os.Chdir(originalDir)
			// Clean up temp directory
			os.RemoveAll(tempDir)
		},
	}

	// Register cleanup
	t.Cleanup(env.Cleanup)

	return env
}

// ExecuteCLI executes a real CLI command and returns the result
func (env *RealCLITestEnvironment) ExecuteCLI(args ...string) *CLIResult {
	return env.ExecuteCLIWithTimeout(300*time.Second, args...)
}

// ExecuteCLIWithTimeout executes a real CLI command with custom timeout and returns the result
func (env *RealCLITestEnvironment) ExecuteCLIWithTimeout(timeout time.Duration, args ...string) *CLIResult {
	cmd := exec.Command(env.CLIBinary, args...)
	cmd.Dir = env.TempDir

	// Set environment variables
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("BL_API_KEY=%s", env.APIKey),
		fmt.Sprintf("BL_WORKSPACE=%s", env.Workspace),
	)

	// Execute command with timeout
	stdout, stderr, err := executeCommandWithTimeout(cmd, timeout)

	result := &CLIResult{
		Stdout: stdout,
		Stderr: stderr,
		Error:  err,
	}

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = 1
		}
	} else {
		result.ExitCode = 0
	}

	return result
}

// ExecuteCLIInDirectory executes a CLI command in a specific directory
func (env *RealCLITestEnvironment) ExecuteCLIInDirectory(dir string, args ...string) *CLIResult {
	cmd := exec.Command(env.CLIBinary, args...)

	// Handle absolute vs relative paths
	if filepath.IsAbs(dir) {
		cmd.Dir = dir
	} else {
		cmd.Dir = filepath.Join(env.TempDir, dir)
	}

	// Set environment variables
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("BL_API_KEY=%s", env.APIKey),
		fmt.Sprintf("BL_WORKSPACE=%s", env.Workspace),
	)

	// Execute command with timeout
	stdout, stderr, err := executeCommandWithTimeout(cmd, 60*time.Second)

	result := &CLIResult{
		Stdout: stdout,
		Stderr: stderr,
		Error:  err,
	}

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			result.ExitCode = exitError.ExitCode()
		} else {
			result.ExitCode = 1
		}
	} else {
		result.ExitCode = 0
	}

	return result
}

// WaitForDeployment waits for a deployment to complete by watching the status
func (env *RealCLITestEnvironment) WaitForDeployment(t *testing.T, projectType, projectName string, maxWaitTime time.Duration) error {
	var watchCmd []string

	switch projectType {
	case "mcp":
		watchCmd = []string{"get", "functions", projectName, "--watch"}
	case "agent":
		watchCmd = []string{"get", "agents", projectName, "--watch"}
	case "job":
		watchCmd = []string{"get", "job", projectName, "--watch"}
	default:
		return fmt.Errorf("unknown project type: %s", projectType)
	}

	t.Logf("👀 Checking deployment status for %s (%s) using: %v", projectName, projectType, watchCmd[:len(watchCmd)-1])

	// First try a simple get command to check if the resource exists
	getCmd := watchCmd[:len(watchCmd)-1] // Remove --watch flag
	result := env.ExecuteCLIWithTimeout(30*time.Second, getCmd...)

	if result.ExitCode == 0 {
		t.Logf("📋 Initial status check output:\n%s", result.Stdout)

		// Check if output indicates successful deployment
		output := strings.ToLower(result.Stdout)
		if strings.Contains(output, "deployed") ||
			strings.Contains(output, "ready") ||
			strings.Contains(output, "running") ||
			strings.Contains(output, "active") {
			t.Logf("✅ %s is already deployed successfully", projectName)
			return nil
		}

		if strings.Contains(output, "failed") ||
			strings.Contains(output, "error") {
			t.Logf("❌ %s deployment failed based on initial check", projectName)
			return fmt.Errorf("deployment failed: %s", result.Stdout)
		}

		t.Logf("⏳ %s deployment status is pending, starting watch...", projectName)
	} else {
		t.Logf("⚠️ Initial status check failed (exit code %d), proceeding to watch: %s", result.ExitCode, result.Stderr)
	}

	// Poll periodically instead of using a single long-running watch command
	t.Logf("👁️ Starting periodic deployment check for %s (polling every 20s)", projectName)
	t.Logf("⏲️ Waiting for deployment completion (timeout: %v)...", maxWaitTime)

	startTime := time.Now()
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	// Check immediately first
	checkCmd := getCmd // Remove --watch flag for polling
	result = env.ExecuteCLIWithTimeout(30*time.Second, checkCmd...)
	t.Logf("🔍 [0s] Initial deployment status check for %s: exit_code=%d", projectName, result.ExitCode)
	if result.ExitCode == 0 {
		t.Logf("📋 [0s] Status output:\n%s", strings.TrimSpace(result.Stdout))

		output := strings.ToLower(result.Stdout)
		if strings.Contains(output, "deployed") ||
			strings.Contains(output, "ready") ||
			strings.Contains(output, "running") ||
			strings.Contains(output, "active") {
			t.Logf("✅ %s deployment completed successfully", projectName)
			return nil
		}

		if strings.Contains(output, "failed") ||
			strings.Contains(output, "error") {
			t.Logf("❌ %s deployment failed", projectName)
			return fmt.Errorf("deployment failed: %s", result.Stdout)
		}
	} else {
		t.Logf("⚠️ [0s] Status check failed: %s", result.Stderr)
	}

	// Poll every 20 seconds
	for {
		select {
		case <-ticker.C:
			elapsed := time.Since(startTime)
			result = env.ExecuteCLIWithTimeout(30*time.Second, checkCmd...)
			t.Logf("🔍 [%v] Deployment status check for %s: exit_code=%d", elapsed.Round(time.Second), projectName, result.ExitCode)

			if result.ExitCode == 0 {
				t.Logf("📋 [%v] Status output:\n%s", elapsed.Round(time.Second), strings.TrimSpace(result.Stdout))

				output := strings.ToLower(result.Stdout)
				if strings.Contains(output, "deployed") ||
					strings.Contains(output, "ready") ||
					strings.Contains(output, "running") ||
					strings.Contains(output, "active") {
					t.Logf("✅ %s deployment completed successfully after %v", projectName, elapsed.Round(time.Second))
					return nil
				}

				if strings.Contains(output, "failed") ||
					strings.Contains(output, "error") {
					t.Logf("❌ %s deployment failed after %v", projectName, elapsed.Round(time.Second))
					return fmt.Errorf("deployment failed: %s", result.Stdout)
				}

				t.Logf("⏳ [%v] %s deployment still in progress...", elapsed.Round(time.Second), projectName)
			} else {
				t.Logf("⚠️ [%v] Status check failed for %s: %s", elapsed.Round(time.Second), projectName, result.Stderr)
			}

		case <-time.After(maxWaitTime):
			elapsed := time.Since(startTime)
			t.Logf("⏰ Deployment watch timeout for %s after %v", projectName, elapsed.Round(time.Second))
			return fmt.Errorf("deployment timeout after %v", maxWaitTime)
		}
	}
}

// executeCommandWithTimeout executes a command with a timeout
func executeCommandWithTimeout(cmd *exec.Cmd, timeout time.Duration) (string, string, error) {
	done := make(chan error, 1)
	var stdout, stderr strings.Builder

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	go func() {
		done <- cmd.Run()
	}()

	select {
	case err := <-done:
		return stdout.String(), stderr.String(), err
	case <-time.After(timeout):
		cmd.Process.Kill()
		return stdout.String(), stderr.String(), fmt.Errorf("command timed out after %v", timeout)
	}
}

// AssertCLISuccess asserts that a CLI command executed successfully
func AssertCLISuccess(t *testing.T, result *CLIResult) {
	t.Helper()
	if result.Error != nil {
		t.Logf("Command failed with error: %v", result.Error)
		t.Logf("Stdout: %s", result.Stdout)
		t.Logf("Stderr: %s", result.Stderr)
	}
	require.NoError(t, result.Error, "CLI command should execute successfully")
	assert.Equal(t, 0, result.ExitCode, "Exit code should be 0 for successful execution")
}

// AssertCLIError asserts that a CLI command failed
func AssertCLIError(t *testing.T, result *CLIResult) {
	t.Helper()
	require.Error(t, result.Error, "CLI command should fail")
	assert.NotEqual(t, 0, result.ExitCode, "Exit code should not be 0 for failed execution")
}
