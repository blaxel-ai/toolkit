package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	blaxel "github.com/blaxel-ai/sdk-go"
	requestoption "github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/chat"
	"github.com/blaxel-ai/toolkit/cli/core"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("chat", func() *cobra.Command {
		return ChatCmd()
	})
}

func ChatCmd() *cobra.Command {
	var debug bool
	var local bool
	var port int
	var headerFlags []string

	cmd := &cobra.Command{
		Use:               "chat [agent-name]",
		Args:              cobra.ExactArgs(1),
		Short:             "Chat with an agent",
		ValidArgsFunction: GetChatValidArgsFunction(),
		Long: `Start an interactive chat session with a deployed agent.

This command opens a terminal-based chat interface where you can send messages
to your agent and see responses in real-time. Perfect for testing agent behavior,
exploring capabilities, or debugging conversational flows.

The agent must be deployed and in DEPLOYED status. Use 'bl get agent NAME'
to check deployment status before chatting.

Local Testing:
Use --local flag to chat with a locally running agent (requires 'bl serve'
to be running in another terminal). This is useful during development.

Debug Mode:
Enable --debug to see detailed API calls, responses, and timing information.
Helpful for troubleshooting issues or understanding agent behavior.

Keyboard Controls:
- Type your message and press Enter to send
- Ctrl+C to exit chat session
- Ctrl+L to clear screen (if supported)`,
		Example: `  # Chat with deployed agent
  bl chat my-agent

  # Chat with local development agent (requires 'bl serve')
  bl chat my-agent --local

  # Debug mode (shows API calls and responses)
  bl chat my-agent --debug

  # Add custom headers (for authentication, metadata, etc.)
  bl chat my-agent --header "X-User-ID: 123" --header "X-Session: abc"

  # Development workflow
  bl serve --hotreload         # Terminal 1: Run locally
  bl chat my-agent --local     # Terminal 2: Test chat`,
		Run: func(cmd *cobra.Command, args []string) {
			resourceName := ""
			if len(args) == 0 {
				if !local {
					err := fmt.Errorf("agent name is required")
					core.PrintError("Chat", err)
					core.ExitWithError(err)
				} else {
					resourceName = "local-agent"
				}
			} else {
				resourceName = args[0]
			}

			resourceType := "agent"

			err := Chat(context.Background(), core.GetWorkspace(), resourceType, resourceName, debug, local, port, headerFlags)
			if err != nil {
				core.PrintError("Chat", err)
				core.ExitWithError(err)
			}
		},
	}

	cmd.Flags().BoolVar(&debug, "debug", false, "Debug mode")
	cmd.Flags().BoolVar(&local, "local", false, "Run locally")
	cmd.Flags().IntVarP(&port, "port", "p", 1338, "Port to connect to when using --local")
	cmd.Flags().StringSliceVar(&headerFlags, "header", []string{}, "Request headers in 'Key: Value' format. Can be specified multiple times")
	return cmd
}

func Chat(
	ctx context.Context,
	workspace string,
	resourceType string,
	resourceName string,
	debug bool,
	local bool,
	port int,
	headerFlags []string,
) error {
	if !local {
		err := CheckResource(ctx, workspace, resourceType, resourceName)
		if err != nil {
			return err
		}
	}

	return BootChat(ctx, workspace, resourceType, resourceName, debug, local, port, headerFlags)
}

func BootChat(
	ctx context.Context,
	workspace string,
	resourceType string,
	resourceName string,
	debug bool,
	local bool,
	port int,
	headerFlags []string,
) error {
	m := &chat.ChatModel{
		Messages:          []chat.Message{},
		Workspace:         workspace,
		ResType:           resourceType,
		ResName:           resourceName,
		SendMessage:       SendMessage,
		SendMessageStream: SendMessageStream,
		Debug:             debug,
		Local:             local,
		Port:              port,
		Headers:           headerFlags,
	}

	p := tea.NewProgram(
		m,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}

func CheckResource(
	ctx context.Context,
	workspace string,
	resourceType string,
	resourceName string,
) error {
	// Verify only for agent type
	if resourceType != "agent" {
		return nil
	}

	// Call Agents.Get with the required parameters
	client := core.GetClient()
	_, err := client.Agents.Get(ctx, resourceName, blaxel.AgentGetParams{})
	if err != nil {
		return fmt.Errorf("agent %s not found: %w", resourceName, err)
	}

	return nil
}

func SendMessage(
	ctx context.Context,
	workspace string,
	resourceType string,
	resourceName string,
	message string,
	debug bool,
	local bool,
	port int,
	headers []string,
) (string, error) {
	type Input struct {
		Inputs string `json:"inputs"`
	}
	inputBody, err := json.Marshal(Input{Inputs: message})
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %w", err)
	}
	opts := []requestoption.RequestOption{}
	for _, header := range headers {
		parts := strings.Split(header, ": ")
		if len(parts) == 2 {
			opts = append(opts, requestoption.WithHeader(parts[0], parts[1]))
		}
	}
	if debug {
		opts = append(opts, requestoption.WithDebugLog(nil))
	}
	client := core.GetClient()
	var response *http.Response
	if local {
		baseURL := fmt.Sprintf("http://localhost:%d", port)
		opts = append(opts, requestoption.WithBaseURL(baseURL))
		var res *http.Response
		opts = append(opts, requestoption.WithResponseBodyInto(&res))
		err = client.Execute(ctx, "POST", "/", inputBody, nil, opts...)
		response = res
	} else {
		response, err = client.Run(
			ctx,
			workspace,
			resourceType,
			resourceName,
			"POST",
			"/",
			inputBody,
			opts...,
		)
	}
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	return string(body), nil
}

func SendMessageStream(
	ctx context.Context,
	workspace string,
	resourceType string,
	resourceName string,
	message string,
	debug bool,
	local bool,
	port int,
	headers []string,
	onChunk func(string),
) error {
	type Input struct {
		Inputs string `json:"inputs"`
	}
	inputBody, err := json.Marshal(Input{Inputs: message})
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}
	headersMap := make(map[string]string)
	for _, header := range headers {
		parts := strings.Split(header, ": ")
		if len(parts) == 2 {
			headersMap[parts[0]] = parts[1]
		}
	}

	// Add streaming headers
	headersMap["Accept"] = "text/event-stream"
	headersMap["Cache-Control"] = "no-cache"

	opts := []requestoption.RequestOption{}
	for _, header := range headers {
		parts := strings.Split(header, ": ")
		if len(parts) == 2 {
			opts = append(opts, requestoption.WithHeader(parts[0], parts[1]))
		}
	}
	if debug {
		opts = append(opts, requestoption.WithDebugLog(nil))
	}
	opts = append(opts, requestoption.WithQueryAdd("stream", "true"))
	client := core.GetClient()
	var response *http.Response
	if local {
		baseURL := fmt.Sprintf("http://localhost:%d", port)
		opts = append(opts, requestoption.WithBaseURL(baseURL))
		var res *http.Response
		opts = append(opts, requestoption.WithResponseBodyInto(&res))
		err = client.Execute(ctx, "POST", "/", inputBody, nil, opts...)
		response = res
	} else {
		response, err = client.Run(
			ctx,
			workspace,
			resourceType,
			resourceName,
			"POST",
			"/",
			inputBody,
			opts...,
		)
	}
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	// Check if response is actually streaming
	contentType := response.Header.Get("Content-Type")
	connection := response.Header.Get("Connection")

	if !core.IsStreamingResponse(contentType, connection) &&
		!strings.Contains(contentType, "application/json") {
		// Fall back to reading entire response
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		onChunk(string(body))
		return nil
	}

	if err := core.ReadSSEStream(response.Body, onChunk); err != nil {
		return fmt.Errorf("error reading stream: %w", err)
	}

	return nil
}
