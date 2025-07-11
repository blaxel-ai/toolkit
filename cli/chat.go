package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"encoding/json"
	"io"
	"net/http"

	"github.com/blaxel-ai/toolkit/cli/chat"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"

	"bufio"

	tea "github.com/charmbracelet/bubbletea"
)

func init() {
	core.RegisterCommand("chat", func() *cobra.Command {
		return ChatCmd()
	})
}

func ChatCmd() *cobra.Command {
	var debug bool
	var local bool
	var headerFlags []string

	cmd := &cobra.Command{
		Use:     "chat [agent-name]",
		Args:    cobra.ExactArgs(1),
		Short:   "Chat with an agent",
		Example: `bl chat my-agent`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				fmt.Println("Error: Agent name is required")
				os.Exit(1)
			}

			resourceType := "agent"
			resourceName := args[0]

			err := Chat(context.Background(), core.GetWorkspace(), resourceType, resourceName, debug, local, headerFlags)
			if err != nil {
				fmt.Println("Error: Failed to chat", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().BoolVar(&debug, "debug", false, "Debug mode")
	cmd.Flags().BoolVar(&local, "local", false, "Run locally")
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
	headerFlags []string,
) error {
	if !local {
		err := CheckResource(ctx, workspace, resourceType, resourceName)
		if err != nil {
			return err
		}
	}

	return BootChat(ctx, workspace, resourceType, resourceName, debug, local, headerFlags)
}

func BootChat(
	ctx context.Context,
	workspace string,
	resourceType string,
	resourceName string,
	debug bool,
	local bool,
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

	// Call GetAgent with the required parameters
	client := core.GetClient()
	resp, err := client.GetAgent(ctx, resourceName)
	if err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Check response status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent %s not found", resourceName)
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
	headers []string,
) (string, error) {
	type Input struct {
		Inputs string `json:"inputs"`
	}
	inputBody, err := json.Marshal(Input{Inputs: message})
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %w", err)
	}
	headersMap := make(map[string]string)
	for _, header := range headers {
		parts := strings.Split(header, ": ")
		if len(parts) == 2 {
			headersMap[parts[0]] = parts[1]
		}
	}
	client := core.GetClient()
	response, err := client.Run(
		ctx,
		workspace,
		resourceType,
		resourceName,
		"POST",
		"/",
		headersMap,
		[]string{},
		string(inputBody),
		debug,
		local,
	)
	if err != nil {
		return "", fmt.Errorf("failed to send message: %w", err)
	}

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

	client := core.GetClient()
	response, err := client.Run(
		ctx,
		workspace,
		resourceType,
		resourceName,
		"POST",
		"/",
		headersMap,
		[]string{"stream=true"},
		string(inputBody),
		debug,
		local,
	)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	// Check if response is actually streaming
	contentType := response.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/event-stream") &&
		!strings.Contains(contentType, "text/plain") &&
		!strings.Contains(contentType, "application/x-ndjson") &&
		!strings.Contains(contentType, "application/json") {
		// Fall back to reading entire response
		body, err := io.ReadAll(response.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body: %w", err)
		}
		onChunk(string(body))
		return nil
	}

	// Stream the response
	scanner := bufio.NewScanner(response.Body)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		// Handle Server-Sent Events format
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			// Try to parse as JSON to extract content
			if strings.HasPrefix(data, "{") && strings.HasSuffix(data, "}") {
				var streamData map[string]interface{}
				if err := json.Unmarshal([]byte(data), &streamData); err == nil {
					// Handle OpenAI-style streaming format
					if choices, ok := streamData["choices"].([]interface{}); ok && len(choices) > 0 {
						if choice, ok := choices[0].(map[string]interface{}); ok {
							if delta, ok := choice["delta"].(map[string]interface{}); ok {
								if content, ok := delta["content"].(string); ok && content != "" {
									onChunk(content)
									continue
								}
							}
						}
					}

					// Handle other formats - look for common content fields
					if content, ok := streamData["content"].(string); ok && content != "" {
						onChunk(content)
						continue
					}
					if text, ok := streamData["text"].(string); ok && text != "" {
						onChunk(text)
						continue
					}
					if message, ok := streamData["message"].(string); ok && message != "" {
						onChunk(message)
						continue
					}
				}
			}

			// If not JSON or no content field found, use raw data
			onChunk(data)
		} else if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
			// Handle NDJSON format (newline-delimited JSON)
			var streamData map[string]interface{}
			if err := json.Unmarshal([]byte(line), &streamData); err == nil {
				// Similar parsing as above for NDJSON
				if choices, ok := streamData["choices"].([]interface{}); ok && len(choices) > 0 {
					if choice, ok := choices[0].(map[string]interface{}); ok {
						if delta, ok := choice["delta"].(map[string]interface{}); ok {
							if content, ok := delta["content"].(string); ok && content != "" {
								onChunk(content)
								continue
							}
						}
					}
				}

				if content, ok := streamData["content"].(string); ok && content != "" {
					onChunk(content)
					continue
				}
				if text, ok := streamData["text"].(string); ok && text != "" {
					onChunk(text)
					continue
				}
				if message, ok := streamData["message"].(string); ok && message != "" {
					onChunk(message)
					continue
				}
			}

			// If parsing failed, use the whole line
			onChunk(line)
		} else {
			// Handle plain text streaming
			onChunk(line + "\n")
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading stream: %w", err)
	}

	return nil
}
