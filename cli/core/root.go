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
	"github.com/spf13/cobra"
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
	fmt.Printf("%s⚠️  A new version of Blaxel CLI is available: %s (current: %s)\nTo update follow the instructions at %s\n\n%s",
		colorYellow, latestVersion, currentVersion, UPDATE_CLI_DOC_URL, colorReset)
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
	defer resp.Body.Close()

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

func init() {
	env := os.Getenv("BL_ENV")
	if env == "dev" {
		BASE_URL = "https://api.blaxel.dev/v0"
		APP_URL = "https://app.blaxel.dev"
		RUN_URL = "https://run.blaxel.dev"
		REGISTRY_URL = "https://eu.registry.blaxel.dev"
	} else if env == "local" {
		BASE_URL = "http://localhost:8080/v0"
		APP_URL = "http://localhost:3000"
		RUN_URL = "https://run.blaxel.dev"
		REGISTRY_URL = "https://eu.registry.blaxel.dev"
	}
}

var envFiles []string
var config Config
var workspace string
var outputFormat string
var client *sdk.ClientWithResponses
var verbose bool
var version string
var commit string
var date string
var utc bool
var skipVersionWarning bool
var commandSecrets []string
var rootCmd = &cobra.Command{
	Use:   "bl",
	Short: "Blaxel CLI is a command line tool to interact with Blaxel APIs.",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if !skipVersionWarning && cmd.Name() != "__complete" && cmd.Name() != "completion" {
			checkForUpdates(version)
		}

		setEnvs()
		readConfigToml("")

		credentials := sdk.LoadCredentials(workspace)
		if !credentials.IsValid() && workspace != "" {
			fmt.Printf("Invalid credentials for workspace %s\n", workspace)
			fmt.Printf("Please run `bl login %s` to fix it credentials.\n", workspace)
		}

		os := runtime.GOOS
		arch := runtime.GOARCH
		commitShort := "unknown"
		if commit != "" && len(commit) > 7 {
			commitShort = commit[:7]
		}
		c, err := sdk.NewClientWithCredentials(
			sdk.RunClientWithCredentials{
				ApiURL:      BASE_URL,
				RunURL:      RUN_URL,
				Credentials: credentials,
				Workspace:   workspace,
				Headers: map[string]string{
					"User-Agent": fmt.Sprintf("blaxel/v%s (%s/%s) blaxel/%s", version, os, arch, commitShort),
				},
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
		workspace = sdk.CurrentContext().Workspace
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

func ReadConfigToml(folder string) {
	readConfigToml(folder)
}

func GetConfig() Config {
	return config
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
