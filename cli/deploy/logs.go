package deploy

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/blaxel-ai/toolkit/sdk"
)

// BuildLogWatcher watches build logs for a resource
type BuildLogWatcher struct {
	client       *sdk.ClientWithResponses
	workspace    string
	resourceType string
	resourceName string
	onLog        func(string)
	ctx          context.Context
	cancel       context.CancelFunc
	seenLogs     map[string]bool // Track logs we've already shown
	mu           sync.Mutex
}

// NewBuildLogWatcher creates a new build log watcher
func NewBuildLogWatcher(client *sdk.ClientWithResponses, workspace, resourceType, resourceName string, onLog func(string)) *BuildLogWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &BuildLogWatcher{
		client:       client,
		workspace:    workspace,
		resourceType: resourceType,
		resourceName: resourceName,
		onLog:        onLog,
		ctx:          ctx,
		cancel:       cancel,
		seenLogs:     make(map[string]bool),
	}
}

// Start begins watching build logs
func (w *BuildLogWatcher) Start() {
	go w.watchLogs()
}

// Stop stops watching build logs
func (w *BuildLogWatcher) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
}

func (w *BuildLogWatcher) watchLogs() {
	// Initial delay to allow build to start
	time.Sleep(200 * time.Millisecond)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	failureCount := 0
	maxFailures := 5

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			// Fetch all logs each time - we'll deduplicate locally
			logs, err := w.fetchBuildLogs(0)
			if err != nil {
				failureCount++
				if failureCount >= maxFailures {
					w.onLog(fmt.Sprintf("Error: Failed to fetch logs after %d attempts: %v", maxFailures, err))
					return
				}
				// Log error but continue trying
				if failureCount == 1 {
					w.onLog(fmt.Sprintf("Warning: Error fetching logs: %v", err))
				}
				continue
			}

			// Reset failure count on success
			failureCount = 0

			// Process new log lines
			for _, log := range logs {
				w.onLog(log)
			}
		}
	}
}

func (w *BuildLogWatcher) fetchBuildLogs(offset int) ([]string, error) {
	// Calculate time window - look back 15 minutes for build logs
	now := time.Now().UTC()
	start := now.Add(-2 * time.Minute).Format("2006-01-02T15:04:05")
	end := now.Add(15 * time.Minute).Format("2006-01-02T15:04:05")

	workspace := w.workspace

	// Build URL with proper encoding
	baseURL := strings.TrimSuffix(os.Getenv("BL_API_URL"), "/")
	if baseURL == "" {
		baseURL = "https://api.blaxel.ai"
	}

	// Create URL with query parameters
	u, err := url.Parse(baseURL + "/v0/observability/logs")
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Add query parameters
	q := u.Query()
	q.Set("start", start)
	q.Set("end", end)
	q.Set("resourceType", fmt.Sprintf("%ss", w.resourceType)) // Add 's' to make it plural
	q.Set("workloadIds", w.resourceName)
	q.Set("type", "build")
	q.Set("limit", "1000")
	q.Set("offset", fmt.Sprintf("%d", offset))
	q.Set("severity", "FATAL,ERROR,WARNING,INFO,UNKNOWN")
	q.Set("interval", "3600")
	q.Set("workspace", workspace)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	credentials := sdk.LoadCredentials(workspace)
	if credentials.IsValid() {
		if credentials.AccessToken != "" {
			req.Header.Set("X-Blaxel-Authorization", fmt.Sprintf("Bearer %s", credentials.AccessToken))
		}
		if credentials.APIKey != "" {
			req.Header.Set("X-Blaxel-Authorization", fmt.Sprintf("Bearer %s", credentials.APIKey))
		}
		if workspace != "" {
			req.Header.Set("X-Blaxel-Workspace", workspace)
		}
	}

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var response map[string]struct {
		Logs []struct {
			Timestamp string `json:"timestamp"`
			Message   string `json:"message"`
			Severity  int    `json:"severity"`
			TraceID   string `json:"trace_id"`
		} `json:"logs"`
		TotalCount int `json:"totalCount"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	_, ok := response[w.resourceName]
	if !ok {
		keys := make([]string, 0, len(response))
		for k := range response {
			keys = append(keys, k)
		}
		return nil, fmt.Errorf("resource %s not found, keys: %v", w.resourceName, keys)
	}
	// Extract messages from the logs
	var messages []string
	if resourceData, ok := response[w.resourceName]; ok {
		// Return logs with deduplication check
		w.mu.Lock()
		defer w.mu.Unlock()

		for _, log := range resourceData.Logs {
			// Use timestamp + message as unique key
			key := fmt.Sprintf("%s:%s", log.Timestamp, log.Message)
			if !w.seenLogs[key] {
				w.seenLogs[key] = true
				messages = append(messages, log.Message)
			}
		}
	}

	return messages, nil
}

// MockBuildLogWatcher provides mock build logs for testing
type MockBuildLogWatcher struct {
	onLog        func(string)
	ctx          context.Context
	cancel       context.CancelFunc
	resourceType string
	language     string
}

// NewMockBuildLogWatcher creates a mock build log watcher for testing
func NewMockBuildLogWatcher(onLog func(string)) *MockBuildLogWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &MockBuildLogWatcher{
		onLog:        onLog,
		ctx:          ctx,
		cancel:       cancel,
		resourceType: "agent",
		language:     "typescript",
	}
}

// NewMockBuildLogWatcherWithType creates a mock build log watcher with specific resource type
func NewMockBuildLogWatcherWithType(onLog func(string), resourceType, language string) *MockBuildLogWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &MockBuildLogWatcher{
		onLog:        onLog,
		ctx:          ctx,
		cancel:       cancel,
		resourceType: resourceType,
		language:     language,
	}
}

// Start begins generating mock build logs
func (m *MockBuildLogWatcher) Start() {
	go m.generateMockLogs()
}

// Stop stops generating mock logs
func (m *MockBuildLogWatcher) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

func (m *MockBuildLogWatcher) generateMockLogs() {
	var mockLogs []string

	switch m.resourceType {
	case "function":
		mockLogs = m.generatePythonFunctionLogs()
	case "sandbox":
		mockLogs = m.generateSandboxLogs()
	default:
		mockLogs = m.generateNodeAgentLogs()
	}

	for i, log := range mockLogs {
		select {
		case <-m.ctx.Done():
			return
		default:
			// More realistic delays - slower for actual build steps
			delay := 300 * time.Millisecond
			if strings.Contains(log, "Running in") || strings.Contains(log, "npm") || strings.Contains(log, "pip") {
				delay = 800 * time.Millisecond
			} else if strings.Contains(log, "Pushed") || strings.Contains(log, "built") {
				delay = 500 * time.Millisecond
			} else if log == "" {
				delay = 100 * time.Millisecond
			}

			time.Sleep(delay)
			m.onLog(log)
			_ = i // unused variable
		}
	}
}

func (m *MockBuildLogWatcher) generateNodeAgentLogs() []string {
	return []string{
		"Starting build process...",
		"Fetching base image...",
		"Step 1/8 : FROM node:18-alpine",
		" ---> 9c7a6a2b8b5c",
		"Step 2/8 : WORKDIR /app",
		" ---> Using cache",
		" ---> 4f5e6d7c8a9b",
		"Step 3/8 : COPY package*.json ./",
		" ---> Using cache",
		" ---> 2a3b4c5d6e7f",
		"Step 4/8 : RUN npm ci --only=production",
		" ---> Running in 8a9b0c1d2e3f",
		"",
		"added 234 packages, and audited 567 packages in 12s",
		"",
		"87 packages are looking for funding",
		"  run `npm fund` for details",
		"",
		"found 0 vulnerabilities",
		" ---> 5f6e7d8c9a0b",
		"Removing intermediate container 8a9b0c1d2e3f",
		"Step 5/8 : COPY . .",
		" ---> 1a2b3c4d5e6f",
		"Step 6/8 : RUN npm run build",
		" ---> Running in 7f8e9d0c1b2a",
		"",
		"> test-agent@1.0.0 build",
		"> tsc",
		"",
		"✓ Compiled successfully",
		"  src/index.ts -> dist/index.js",
		"  src/types.ts -> dist/types.js",
		"  src/utils.ts -> dist/utils.js",
		"",
		"Build completed in 4.2s",
		" ---> 9a8b7c6d5e4f",
		"Removing intermediate container 7f8e9d0c1b2a",
		"Step 7/8 : EXPOSE 8080",
		" ---> Running in 3c2b1a0f9e8d",
		" ---> 6d5e4f3c2b1a",
		"Removing intermediate container 3c2b1a0f9e8d",
		"Step 8/8 : CMD [\"node\", \"dist/index.js\"]",
		" ---> Running in 0f9e8d7c6b5a",
		" ---> 4c3b2a1f0e9d",
		"Removing intermediate container 0f9e8d7c6b5a",
		"Successfully built 4c3b2a1f0e9d",
		"Successfully tagged test-agent:latest",
		"",
		"Pushing image to registry...",
		"The push refers to repository [registry.blaxel.ai/test-agent]",
		"2a1b0c9d8e7f: Preparing",
		"5c4d3e2f1a0b: Preparing",
		"8b7c6d5e4f3a: Preparing",
		"2a1b0c9d8e7f: Pushed",
		"5c4d3e2f1a0b: Pushed",
		"8b7c6d5e4f3a: Pushed",
		"latest: digest: sha256:1234567890abcdef size: 2048",
		"",
		"✓ Image pushed successfully!",
		"✓ Build completed successfully!",
	}
}

func (m *MockBuildLogWatcher) generatePythonFunctionLogs() []string {
	return []string{
		"Starting build process for Python function...",
		"Fetching base image...",
		"Step 1/7 : FROM python:3.11-slim",
		" ---> a1b2c3d4e5f6",
		"Step 2/7 : WORKDIR /function",
		" ---> Using cache",
		" ---> 7f8e9d0c1b2a",
		"Step 3/7 : COPY requirements.txt .",
		" ---> Using cache",
		" ---> 3c4d5e6f7a8b",
		"Step 4/7 : RUN pip install --no-cache-dir -r requirements.txt",
		" ---> Running in 9e8d7c6b5a4f",
		"Collecting requests==2.31.0",
		"  Downloading requests-2.31.0-py3-none-any.whl (62 kB)",
		"Collecting urllib3<3,>=1.21.1",
		"  Downloading urllib3-2.0.7-py3-none-any.whl (124 kB)",
		"Collecting certifi>=2017.4.17",
		"  Downloading certifi-2023.7.22-py3-none-any.whl (158 kB)",
		"Installing collected packages: urllib3, certifi, requests",
		"Successfully installed certifi-2023.7.22 requests-2.31.0 urllib3-2.0.7",
		" ---> 8b9c0d1e2f3a",
		"Removing intermediate container 9e8d7c6b5a4f",
		"Step 5/7 : COPY . .",
		" ---> 4d5e6f7a8b9c",
		"Step 6/7 : ENV PYTHONUNBUFFERED=1",
		" ---> Running in 2f3a4b5c6d7e",
		" ---> 9c0d1e2f3a4b",
		"Removing intermediate container 2f3a4b5c6d7e",
		"Step 7/7 : CMD [\"python\", \"weather.py\"]",
		" ---> Running in 5a6b7c8d9e0f",
		" ---> 1e2f3a4b5c6d",
		"Removing intermediate container 5a6b7c8d9e0f",
		"Successfully built 1e2f3a4b5c6d",
		"Successfully tagged weather-function:latest",
		"",
		"Running function tests...",
		"test_get_weather ... ok",
		"test_invalid_location ... ok",
		"",
		"Ran 2 tests in 0.003s",
		"OK",
		"",
		"Pushing image to registry...",
		"The push refers to repository [registry.blaxel.ai/weather-function]",
		"9e8d7c6b5a4f: Pushed",
		"3c4d5e6f7a8b: Pushed",
		"1a2b3c4d5e6f: Pushed",
		"latest: digest: sha256:abcdef1234567890 size: 1580",
		"",
		"✓ Function image pushed successfully!",
		"✓ Build completed successfully!",
	}
}

func (m *MockBuildLogWatcher) generateSandboxLogs() []string {
	return []string{
		"Starting sandbox environment build...",
		"Fetching base image...",
		"Step 1/10 : FROM ubuntu:22.04",
		" ---> 3b418d7b466a",
		"Step 2/10 : RUN apt-get update && apt-get install -y python3 nodejs npm curl git",
		" ---> Running in 7a8b9c0d1e2f",
		"Get:1 http://archive.ubuntu.com/ubuntu jammy InRelease [270 kB]",
		"Get:2 http://archive.ubuntu.com/ubuntu jammy-updates InRelease [119 kB]",
		"Reading package lists...",
		"Building dependency tree...",
		"The following packages will be installed:",
		"  curl git nodejs npm python3 python3-pip",
		"Installing packages...",
		"Setting up python3 (3.10.12-1~22.04.2) ...",
		"Setting up nodejs (18.17.1-1nodesource1) ...",
		"Setting up npm (9.6.7+ds-1) ...",
		"Setting up git (1:2.34.1-1ubuntu1.10) ...",
		" ---> 5c6d7e8f9a0b",
		"Removing intermediate container 7a8b9c0d1e2f",
		"Step 3/10 : WORKDIR /sandbox",
		" ---> Running in 2b3c4d5e6f7a",
		" ---> 8d9e0f1a2b3c",
		"Step 4/10 : COPY requirements.txt package.json ./",
		" ---> 4e5f6a7b8c9d",
		"Step 5/10 : RUN pip3 install -r requirements.txt && npm install",
		" ---> Running in 0a1b2c3d4e5f",
		"Installing Python dependencies...",
		"Collecting numpy pandas matplotlib",
		"Installing collected packages...",
		"Successfully installed numpy-1.24.3 pandas-2.0.3 matplotlib-3.7.2",
		"",
		"Installing Node.js dependencies...",
		"added 156 packages in 8s",
		" ---> 6f7a8b9c0d1e",
		"Step 6/10 : COPY . .",
		" ---> 2f3a4b5c6d7e",
		"Step 7/10 : RUN chmod +x /sandbox/entrypoint.sh",
		" ---> Running in 8a9b0c1d2e3f",
		" ---> 4b5c6d7e8f9a",
		"Step 8/10 : EXPOSE 8080 8081 8082",
		" ---> Running in 0c1d2e3f4a5b",
		" ---> 6d7e8f9a0b1c",
		"Step 9/10 : ENV SANDBOX_MODE=development",
		" ---> Running in 2e3f4a5b6c7d",
		" ---> 8f9a0b1c2d3e",
		"Step 10/10 : ENTRYPOINT [\"/sandbox/entrypoint.sh\"]",
		" ---> Running in 4a5b6c7d8e9f",
		" ---> 0b1c2d3e4f5a",
		"Successfully built 0b1c2d3e4f5a",
		"Successfully tagged dev-sandbox:latest",
		"",
		"Validating sandbox environment...",
		"✓ Python 3.10.12 available",
		"✓ Node.js v18.17.1 available",
		"✓ Git version 2.34.1 available",
		"✓ All required tools installed",
		"",
		"Pushing image to registry...",
		"The push refers to repository [registry.blaxel.ai/dev-sandbox]",
		"8f9a0b1c2d3e: Pushed",
		"6d7e8f9a0b1c: Pushed",
		"4b5c6d7e8f9a: Pushed",
		"latest: digest: sha256:fedcba9876543210 size: 3072",
		"",
		"✓ Sandbox image pushed successfully!",
		"✓ Build completed successfully!",
	}
}

// StreamBuildLogs streams build logs from an HTTP response
func StreamBuildLogs(resp *http.Response, onLog func(string)) error {
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Handle Server-Sent Events format
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return nil
			}
			onLog(data)
		} else {
			onLog(line)
		}
	}
}
