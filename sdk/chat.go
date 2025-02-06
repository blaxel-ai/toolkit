package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/beamlit/toolkit/sdk/chat"

	tea "github.com/charmbracelet/bubbletea"
)

func (c *Client) Chat(
	ctx context.Context,
	workspace string,
	environment string,
	resourceType string,
	resourceName string,
	debug bool,
	local bool,
) error {
	if !local {
		err := c.CheckResource(ctx, workspace, environment, resourceType, resourceName)
		if err != nil {
			return err
		}
	}

	return c.BootChat(ctx, workspace, environment, resourceType, resourceName, debug, local)
}

func (c *Client) BootChat(
	ctx context.Context,
	workspace string,
	environment string,
	resourceType string,
	resourceName string,
	debug bool,
	local bool,
) error {

	m := &chat.ChatModel{
		Messages:    []chat.Message{},
		Workspace:   workspace,
		Environment: environment,
		ResType:     resourceType,
		ResName:     resourceName,
		SendMessage: c.SendMessage,
		Debug:       debug,
		Local:       local,
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

func (c *Client) CheckResource(
	ctx context.Context,
	workspace string,
	environment string,
	resourceType string,
	resourceName string,
) error {
	// Verify only for agent type
	if resourceType != "agent" {
		return nil
	}

	// Call GetAgent with the required parameters
	resp, err := c.GetAgent(ctx, resourceName, &GetAgentParams{
		Environment: &environment,
	})
	if err != nil {
		return fmt.Errorf("failed to get agent: %w", err)
	}
	defer resp.Body.Close()

	// Check response status code
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("agent %s not found in environment %s", resourceName, environment)
	}

	return nil
}

func (c *Client) SendMessage(
	ctx context.Context,
	workspace string,
	environment string,
	resourceType string,
	resourceName string,
	message string,
	debug bool,
	local bool,
) (string, error) {
	type Input struct {
		Inputs string `json:"inputs"`
	}
	inputBody, err := json.Marshal(Input{Inputs: message})
	if err != nil {
		return "", fmt.Errorf("failed to marshal message: %w", err)
	}
	response, err := c.Run(
		ctx,
		workspace,
		environment,
		resourceType,
		resourceName,
		"POST",
		"/",
		map[string]string{},
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
