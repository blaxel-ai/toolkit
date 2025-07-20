# Interactive Deployment Mode

The interactive deployment mode provides a real-time Terminal User Interface (TUI) for monitoring and tracking the deployment process of Blaxel resources.

## Features

### 1. Real-time Status Tracking
- **Visual Progress**: Each resource displays its current deployment status with appropriate icons
- **Status Phases**:
  - `○` Pending - Resource is queued for deployment
  - `⣾` Uploading - Code is being uploaded
  - `⣾` Building - Container image is being built
  - `⣾` Deploying - Resource is being deployed
  - `✓` Complete - Deployment successful
  - `✗` Failed - Deployment failed

### 2. Live Build Logs
- Resources with `x-blaxel-auto-generated` label show real-time build logs
- Logs are displayed in a scrollable viewport
- Only the last 100 log lines are kept in memory to prevent issues

### 3. Interactive Navigation
- **↑/↓ or k/j**: Navigate between resources
- **l**: Toggle build log visibility
- **q or Ctrl+C**: Quit the interactive mode

### 4. Concurrent Deployments
- Multiple resources are deployed in parallel for faster overall deployment
- Each resource's progress is tracked independently

## Usage

### Basic Interactive Deployment
```bash
bl deploy --interactive
```

### Testing with Mock Mode
Mock mode simulates the deployment process without making actual API calls:
```bash
bl deploy --interactive --mock
```

### Combined with Other Flags
```bash
# Interactive deployment with custom name
bl deploy --interactive --name my-custom-app

# Interactive deployment from subdirectory
bl deploy --interactive --directory ./my-app

# Interactive deployment with environment file
bl deploy --interactive --env-file .env.production
```

## Architecture

### Components

1. **InteractiveModel** (`interactive.go`)
   - Main TUI model using Bubble Tea framework
   - Manages UI state and user interactions
   - Thread-safe updates for concurrent deployments

2. **BuildLogWatcher** (`logs.go`)
   - Polls for build logs from the API
   - Streams logs to the UI in real-time
   - Includes mock implementation for testing

3. **Deployment Integration** (`deploy.go`)
   - `ApplyInteractive()` method orchestrates the interactive deployment
   - Manages concurrent resource deployments
   - Handles both real and mock deployments

### Message Flow
```
User Input → InteractiveModel → Update UI
                ↑
                |
Deployment Process → Status Updates → Model Updates
                |
                ↓
           Build Logs → Log Watcher → Model Updates
```

## Testing

### Using the Test Script
A comprehensive test script is provided at `test/test_interactive_deploy.sh`:

```bash
# Make it executable
chmod +x test/test_interactive_deploy.sh

# Run the test menu
./test/test_interactive_deploy.sh
```

The test script provides:
1. Mock mode testing
2. Dry run testing
3. Multiple resource testing
4. Real deployment testing

### Manual Testing
1. Create a test project with a `blaxel.toml` file
2. Run `bl deploy --interactive --mock` to test without deployment
3. Run `bl deploy --interactive` for real deployment (requires credentials)

## Implementation Notes

### Thread Safety
- All resource updates are thread-safe using mutex locks
- UI updates are sent through channels to maintain consistency

### Build Log Streaming
- Build logs are fetched periodically (every 2 seconds)
- Only new logs are retrieved using offset tracking
- Supports Server-Sent Events (SSE) format for real-time streaming

### Error Handling
- Deployment failures are clearly displayed with error messages
- Build timeouts are set to 10 minutes by default
- Failed resources show detailed error information

## Future Enhancements

1. **Build Status API**: Implement actual build status checking instead of timeout-based completion
2. **Log Streaming**: Direct WebSocket/SSE connection for real-time logs
3. **Resource Filtering**: Filter resources by type or status
4. **Deployment Metrics**: Show deployment time, resource usage, etc.
5. **Retry Mechanism**: Allow retrying failed deployments
6. **Export Logs**: Save deployment logs to a file