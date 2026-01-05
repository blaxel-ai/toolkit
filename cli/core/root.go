package core

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/blaxel-ai/toolkit/sdk"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var BASE_URL = "https://api.blaxel.ai/v0"
var APP_URL = "https://app.blaxel.ai"
var RUN_URL = "https://run.blaxel.ai"
var REGISTRY_URL = "https://us.registry.blaxel.ai"
var GITHUB_RELEASES_URL = "https://api.github.com/repos/blaxel-ai/toolkit/releases"
var UPDATE_CLI_DOC_URL = "https://docs.blaxel.ai/cli-reference/introduction#update"

// ANSI color codes
const (
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorGreen  = "\033[32m"
	colorBold   = "\033[1m"
	colorReset  = "\033[0m"
)

// Simple command registry
var commandRegistry = make(map[string]func() *cobra.Command)

// RegisterCommand allows commands to register themselves
func RegisterCommand(name string, cmdFunc func() *cobra.Command) {
	commandRegistry[name] = cmdFunc
}

// GetCommand returns a registered command
func GetCommand(name string) *cobra.Command {
	if cmdFunc, exists := commandRegistry[name]; exists {
		return cmdFunc()
	}
	return &cobra.Command{Use: name, Short: fmt.Sprintf("%s (not implemented)", name)}
}

type versionCache struct {
	Version   string    `json:"version"`
	LastCheck time.Time `json:"last_check"`
}

func getVersionCachePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".blaxel", "version")
}

func readVersionCache() (versionCache, error) {
	var cache versionCache
	path := getVersionCachePath()
	if path == "" {
		return cache, fmt.Errorf("could not determine cache path")
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return cache, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cache, nil
		}
		return cache, err
	}

	if err := json.Unmarshal(data, &cache); err != nil {
		return cache, err
	}

	return cache, nil
}

func writeVersionCache(cache versionCache) error {
	path := getVersionCachePath()
	if path == "" {
		return fmt.Errorf("could not determine cache path")
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func notifyNewVersionAvailable(latestVersion, currentVersion string) {
	fmt.Printf("%s⚠️  A new version of Blaxel CLI is available: %s%s%s%s (current: %s%s%s)\n%sYou can update by running: %sbl upgrade%s\n%sOr follow the instructions at %s%s%s\n\n%s",
		colorYellow, colorBold+colorGreen, latestVersion, colorReset, colorYellow, colorBold, currentVersion, colorReset+colorYellow,
		colorYellow, colorBold+colorGreen, colorReset+colorYellow,
		colorYellow, colorCyan, UPDATE_CLI_DOC_URL, colorReset+colorYellow, colorReset)
}

func checkForUpdates(currentVersion string) {
	if currentVersion == "dev" {
		return
	}
	if strings.Contains(currentVersion, "-SNAPSHOT") {
		return
	}

	// Skip update check for pre-release versions
	cleanCurrentVersion := strings.Split(currentVersion, "-SNAPSHOT")[0]
	if semVer, err := semver.NewVersion(cleanCurrentVersion); err == nil {
		if semVer.Prerelease() != "" {
			return // Skip check for pre-release versions like 1.2.3-rc1, 1.2.3-alpha, etc.
		}
	}

	// Read from cache
	cache, err := readVersionCache()
	if err == nil && cache.Version != "" && time.Since(cache.LastCheck) < 6*time.Hour {
		if isNewerVersion(cache.Version, currentVersion) {
			notifyNewVersionAvailable(cache.Version, currentVersion)
		}
		return
	}

	// If cache is invalid or expired, fetch from GitHub
	resp, err := http.Get(GITHUB_RELEASES_URL)
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var releases []struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &releases); err != nil {
		return
	}

	// Filter releases to only include stable versions (N.N.N format)
	var stableVersions []*semver.Version
	for _, release := range releases {
		version := strings.TrimPrefix(release.TagName, "v")

		// Parse with semver to validate format
		semVer, err := semver.NewVersion(version)
		if err != nil {
			continue // Skip invalid versions
		}

		// Only include versions without pre-release identifiers (N.N.N format only)
		if semVer.Prerelease() == "" {
			stableVersions = append(stableVersions, semVer)
		}
	}

	// Find the highest version among stable versions
	if len(stableVersions) == 0 {
		return // No stable versions found
	}

	var latestVersion *semver.Version
	for _, version := range stableVersions {
		if latestVersion == nil || version.GreaterThan(latestVersion) {
			latestVersion = version
		}
	}

	latestVersionString := latestVersion.String()

	// Update cache
	cache = versionCache{
		Version:   latestVersionString,
		LastCheck: time.Now(),
	}
	_ = writeVersionCache(cache)

	if strings.Contains(currentVersion, "-SNAPSHOT") {
		currentVersion = strings.Split(currentVersion, "-SNAPSHOT")[0]
	}
	if isNewerVersion(latestVersionString, currentVersion) {
		notifyNewVersionAvailable(latestVersionString, currentVersion)
	}
}

// isNewerVersion returns true if latestVersion is newer than currentVersion using semver
func isNewerVersion(latestVersion, currentVersion string) bool {
	latest, err1 := semver.NewVersion(latestVersion)
	current, err2 := semver.NewVersion(currentVersion)
	if err1 != nil || err2 != nil {
		// fallback to string compare if semver parsing fails
		return latestVersion != currentVersion
	}
	return latest.GreaterThan(current)
}

func initEnv(env string) {
	switch env {
	case "dev":
		BASE_URL = "https://api.blaxel.dev/v0"
		APP_URL = "https://app.blaxel.dev"
		RUN_URL = "https://run.blaxel.dev"
		REGISTRY_URL = "https://eu.registry.blaxel.dev"
	case "local":
		BASE_URL = "http://localhost:8080/v0"
		APP_URL = "http://localhost:3000"
		RUN_URL = "https://run.blaxel.dev"
		REGISTRY_URL = "https://eu.registry.blaxel.dev"
	}
}

func init() {
	initEnv(os.Getenv("BL_ENV"))
}

var envFiles []string
var config Config
var workspace string
var outputFormat string
var client *sdk.ClientWithResponses
var verbose bool
var version string

// commit can be set at build time via ldflags: -ldflags "-X github.com/blaxel-ai/toolkit/cli/core.commit=abc1234"
var commit string
var date string
var utc bool
var skipVersionWarning bool
var commandSecrets []string
var rootCmd = &cobra.Command{
	Use:   "bl",
	Short: "Blaxel CLI is a command line tool to interact with Blaxel APIs.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip version warning for specific commands/conditions
		shouldSkipWarning := skipVersionWarning ||
			cmd.Name() == "__complete" ||
			cmd.Name() == "completion" ||
			cmd.Name() == "token" ||
			cmd.Name() == "upgrade" ||
			(cmd.Name() == "workspaces" && cmd.Flag("current") != nil && cmd.Flag("current").Changed) ||
			outputFormat == "json" ||
			outputFormat == "yaml"

		if !shouldSkipWarning {
			checkForUpdates(version)
		}

		// Load .env file for all commands except serve, deploy, run, and apply cause they use envFiles
		excludedCommands := map[string]bool{
			"serve":  true,
			"deploy": true,
			"run":    true,
			"apply":  true,
		}
		if !excludedCommands[cmd.Name()] {
			if err := godotenv.Load(); err != nil {
				// Only log error if .env file exists but can't be loaded
				if !os.IsNotExist(err) {
					fmt.Printf("Warning: Could not load .env file: %v\n", err)
				}
			}
		}

		setEnvs()

		// Skip config reading for deploy command as it handles its own config logic with special type prompting
		if cmd.Name() != "deploy" {
			readConfigToml("", true)
		}

		// Check if workspace is required but not available
		// Commands that don't require a workspace
		workspaceExemptCommands := map[string]bool{
			"login":      true,
			"logout":     true,
			"version":    true,
			"upgrade":    true,
			"workspaces": true,
			"workspace":  true,
			"ws":         true,
			"completion": true,
			"__complete": true,
			"help":       true,
			"new":        true,
			"docs":       true,
		}

		if !workspaceExemptCommands[cmd.Name()] {
			// Check if BL_WORKSPACE is set or if there are workspaces in config
			if workspace == "" {
				workspaces := sdk.ListWorkspaces()
				if len(workspaces) == 0 {
					PrintError("Login required", fmt.Errorf("no workspace configured. Please run 'bl login' first to authenticate"))
					Exit(1)
				}
			}
		}

		credentials := sdk.LoadCredentials(workspace)
		if !credentials.IsValid() && workspace != "" {
			PrintWarning(fmt.Sprintf("Invalid credentials for workspace '%s'\n", workspace))
			PrintWarning(fmt.Sprintf("Please run 'bl login %s' to refresh your credentials.\n", workspace))
		}

		// Get OS/arch and commit info for User-Agent
		osArch := runtime.GOOS + "/" + runtime.GOARCH
		commitHash := "unknown"

		// Check if commit was injected at build time via ldflags
		if commit != "" {
			if len(commit) > 7 {
				commitHash = commit[:7]
			} else {
				commitHash = commit
			}
		}

		headers := map[string]string{
			"User-Agent": fmt.Sprintf("blaxel/cli/golang/%s (%s) blaxel/%s", version, osArch, commitHash),
		}

		c, err := sdk.NewClientWithCredentials(
			sdk.RunClientWithCredentials{
				ApiURL:      BASE_URL,
				RunURL:      RUN_URL,
				Credentials: credentials,
				Workspace:   workspace,
				Headers:     headers,
			},
		)
		if err != nil {
			return err
		}
		client = c

		// Register SDK CLI commands
		ctx := context.Background()
		reg := &Operations{
			BaseURL:     BASE_URL,
			RunURL:      RUN_URL,
			AppURL:      APP_URL,
			RegistryURL: REGISTRY_URL,
		}
		client.RegisterCliCommands(reg, ctx)

		// TODO: Handle SDK command registration if needed
		return nil
	},
}

func setEnvs() {
	if url := os.Getenv("BL_API_URL"); url != "" {
		BASE_URL = url
	}
	if runUrl := os.Getenv("BL_RUN_URL"); runUrl != "" {
		RUN_URL = runUrl
	}
	if appUrl := os.Getenv("BL_APP_URL"); appUrl != "" {
		APP_URL = appUrl
	}
}

func Execute(releaseVersion string, releaseCommit string, releaseDate string) error {
	setEnvs()

	// Prompt for tracking consent if not already configured
	promptForTracking()

	rootCmd.PersistentFlags().StringVarP(&workspace, "workspace", "w", "", "Specify the workspace name")
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "", "Output format. One of: pretty,yaml,json,table")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVarP(&utc, "utc", "u", false, "Enable UTC timezone")
	rootCmd.PersistentFlags().BoolVarP(&skipVersionWarning, "skip-version-warning", "", false, "Skip version warning")

	// Add all registered commands to the root command
	for _, cmdFunc := range commandRegistry {
		cmd := cmdFunc()
		if cmd != nil {
			rootCmd.AddCommand(cmd)
		}
	}

	if workspace == "" {
		// Check for BL_WORKSPACE environment variable first
		if envWorkspace := os.Getenv("BL_WORKSPACE"); envWorkspace != "" {
			workspace = envWorkspace
		} else {
			workspace = sdk.CurrentContext().Workspace
		}
		env := sdk.LoadEnv(workspace)
		initEnv(env)
	}
	if version == "" {
		version = releaseVersion
	}
	if commit == "" {
		commit = releaseCommit
	}
	if date == "" {
		date = releaseDate
	}
	SetSentryTag("version", version)
	SetSentryTag("commit", commit)
	SetSentryTag("workspace", workspace)

	return rootCmd.Execute()
}

func GetBaseURL() string {
	return BASE_URL
}

func GetRunURL() string {
	return RUN_URL
}

func GetAppURL() string {
	return APP_URL
}

func GetRegistryURL() string {
	return REGISTRY_URL
}

func IsVerbose() bool {
	return verbose
}

func SetEnvs() {
	if url := os.Getenv("BL_API_URL"); url != "" {
		BASE_URL = url
	}
	if runUrl := os.Getenv("BL_RUN_URL"); runUrl != "" {
		RUN_URL = runUrl
	}
	if appUrl := os.Getenv("BL_APP_URL"); appUrl != "" {
		APP_URL = appUrl
	}
}

func CheckForUpdates(currentVersion string) {
	checkForUpdates(currentVersion)
}

func LoadCommandSecrets(commandSecrets []string) {
	SetCommandSecrets(commandSecrets)
	loadCommandSecrets()
}

func SetCommandSecrets(secrets []string) {
	commandSecrets = secrets
}

func ReadSecrets(folder string, envFiles []string) {
	setEnvFiles(envFiles)
	readSecrets(folder)
}

func setEnvFiles(files []string) {
	envFiles = files
}

func ReadConfigToml(folder string, setDefaultType bool) {
	readConfigToml(folder, setDefaultType)
}

func GetConfig() Config {
	return config
}

// SetConfigType sets the config type
func SetConfigType(t string) {
	config.Type = t
}

// GetClient returns the current client
func GetClient() *sdk.ClientWithResponses {
	return client
}

// GetWorkspace returns the current workspace
func GetWorkspace() string {
	return workspace
}

// SetWorkspace sets the current workspace
func SetWorkspace(ws string) {
	workspace = ws
}

// GetOutputFormat returns the current output format
func GetOutputFormat() string {
	return outputFormat
}

func GetEnvFiles() []string {
	return envFiles
}

func GetCommandSecrets() []string {
	return commandSecrets
}

func GetVersion() string {
	return version
}

func GetCommit() string {
	return commit
}

func GetDate() string {
	return date
}

func GetVerbose() bool {
	return verbose
}

var interactiveMode bool

func SetInteractiveMode(interactive bool) {
	interactiveMode = interactive
}

func IsInteractiveMode() bool {
	return interactiveMode
}

// IsCIEnvironment returns true when running in a known CI environment.
// It checks common CI environment variables used by providers like GitHub and GitLab.
func IsCIEnvironment() bool {
	if os.Getenv("CI") == "true" || os.Getenv("CI") == "1" {
		return true
	}
	if os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("GITLAB_CI") == "true" {
		return true
	}
	if os.Getenv("BUILDKITE") == "true" || os.Getenv("CIRCLECI") == "true" || os.Getenv("TRAVIS") == "true" {
		return true
	}
	if os.Getenv("JENKINS_URL") != "" || os.Getenv("TEAMCITY_VERSION") != "" {
		return true
	}
	return false
}

// IsTerminalInteractive returns true if both stdin and stdout are terminals (TTY).
func IsTerminalInteractive() bool {
	return term.IsTerminal(int(os.Stdout.Fd())) && term.IsTerminal(int(os.Stdin.Fd()))
}

// promptForTracking prompts the user for tracking consent if not already configured.
// Only prompts in interactive mode and not in CI environments.
func promptForTracking() {
	// Skip if tracking is already configured
	if sdk.IsTrackingConfigured() {
		return
	}

	// Skip in CI environments
	if IsCIEnvironment() {
		return
	}

	// Skip if not in interactive mode
	if !IsTerminalInteractive() {
		return
	}

	// Prompt user for tracking consent
	fmt.Println()
	fmt.Print("Do you want to enable tracking to help improve Blaxel? [Y/n] ")

	var response string
	fmt.Scanln(&response)

	// Default to true (Y) if empty or yes
	enabled := true
	response = strings.ToLower(strings.TrimSpace(response))
	if response == "n" || response == "no" {
		enabled = false
	}

	sdk.SetTracking(enabled)

	if enabled {
		fmt.Println("✓ Tracking enabled. Thank you for helping improve Blaxel!")
	} else {
		fmt.Println("✓ Tracking disabled.")
	}
	fmt.Println()
}
