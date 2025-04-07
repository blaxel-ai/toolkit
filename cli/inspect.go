package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/beamlit/toolkit/sdk/mcp"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var (
	docStyle = lipgloss.NewStyle().Margin(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	statusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
				Render
)

// Message types
type toolsLoadedMsg struct {
	tools []list.Item
	err   error
}

type toolExecutedMsg struct {
	result string
	err    error
}

type keyMap struct {
	Up      key.Binding
	Down    key.Binding
	Select  key.Binding
	Back    key.Binding
	Quit    key.Binding
	Execute key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "move up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "move down"),
		),
		Select: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("enter", "select"),
		),
		Back: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q/ctrl+c", "quit"),
		),
		Execute: key.NewBinding(
			key.WithKeys("e"),
			key.WithHelp("e", "execute"),
		),
	}
}

// Item represents a tool in the list
type Item struct {
	name        string
	description string
	toolData    map[string]interface{}
}

func (i Item) Title() string       { return i.name }
func (i Item) Description() string { return i.description }
func (i Item) FilterValue() string { return i.name }

// Selected tool state
type selectedToolState struct {
	tool       Item
	params     map[string]string
	paramIndex int
	executing  bool
	result     string
}

// Model for the Bubble Tea application
type inspectorModel struct {
	keys           keyMap
	toolList       list.Model
	mcpClient      *mcp.MCPClient
	state          string // "list", "params", "result"
	selectedTool   selectedToolState
	requiredParams []string
	ctx            context.Context
	quitting       bool
	err            error
}

func newModel(ctx context.Context, mcpClient *mcp.MCPClient) (inspectorModel, error) {
	keys := newKeyMap()

	// Initialize empty list
	delegate := list.NewDefaultDelegate()
	toolList := list.New([]list.Item{}, delegate, 0, 0)
	toolList.Title = "MCP Tools"
	toolList.SetShowHelp(true)

	// Set custom key bindings
	toolList.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			keys.Quit,
		}
	}

	m := inspectorModel{
		keys:      keys,
		toolList:  toolList,
		mcpClient: mcpClient,
		state:     "list",
		ctx:       ctx,
	}

	// Load the initial tools
	return m, nil
}

func (m inspectorModel) Init() tea.Cmd {
	return m.loadTools
}

func (m *inspectorModel) loadTools() tea.Msg {
	toolsData, err := m.mcpClient.ListTools(m.ctx)
	if err != nil {
		return toolsLoadedMsg{
			err: fmt.Errorf("error listing tools: %v", err),
		}
	}

	// Parse tools
	var toolsRaw []map[string]interface{}
	if err := json.Unmarshal(toolsData, &toolsRaw); err != nil {
		return toolsLoadedMsg{
			err: fmt.Errorf("error parsing tools: %v", err),
		}
	}

	// Convert to list items
	var tools []list.Item
	for _, tool := range toolsRaw {
		name, ok := tool["name"].(string)
		if !ok {
			continue
		}

		desc, _ := tool["description"].(string)
		if desc == "" {
			desc = "No description available"
		}

		item := Item{
			name:        name,
			description: desc,
			toolData:    tool,
		}
		tools = append(tools, item)
	}

	return toolsLoadedMsg{tools: tools}
}

func (m inspectorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Global key handlers
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.quitting = true
			return m, tea.Quit
		}

		// State-specific key handlers
		switch m.state {
		case "list":
			switch {
			case key.Matches(msg, m.keys.Select):
				if i, ok := m.toolList.SelectedItem().(Item); ok {
					m.selectTool(i)
					return m, nil
				}
			}
		case "params":
			switch {
			case key.Matches(msg, m.keys.Back):
				m.state = "list"
				return m, nil
			case key.Matches(msg, m.keys.Execute):
				if m.allParamsEntered() {
					m.selectedTool.executing = true
					return m, m.executeSelectedTool
				}
			}

			// Handle character input for parameter value
			if m.selectedTool.paramIndex < len(m.requiredParams) {
				paramName := m.requiredParams[m.selectedTool.paramIndex]

				// Handle special keys
				switch msg.String() {
				case "enter":
					// Move to next parameter or finish if all params are entered
					m.selectedTool.paramIndex++
					if m.selectedTool.paramIndex >= len(m.requiredParams) {
						// All parameters entered, ready to execute
						m.selectedTool.executing = true
						return m, m.executeSelectedTool
					}
					return m, nil
				case "esc":
					// Go back to tool list
					m.state = "list"
					return m, nil
				case "backspace":
					// Handle backspace for parameter input
					currentValue := m.selectedTool.params[paramName]
					if len(currentValue) > 0 {
						m.selectedTool.params[paramName] = currentValue[:len(currentValue)-1]
					}
					return m, nil
				default:
					// Add character to current parameter value if not a control key
					if len(msg.String()) == 1 {
						current := m.selectedTool.params[paramName]
						m.selectedTool.params[paramName] = current + msg.String()
					}
				}
			}
		case "result":
			switch {
			case key.Matches(msg, m.keys.Back):
				m.state = "list"
				m.selectedTool = selectedToolState{}
				return m, nil
			}
		}

	case toolsLoadedMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.toolList.SetItems(msg.tools)

	case toolExecutedMsg:
		m.state = "result"
		m.selectedTool.executing = false
		m.selectedTool.result = msg.result
		if msg.err != nil {
			m.selectedTool.result = fmt.Sprintf("Error: %v", msg.err)
		}
	}

	// Handle list updates
	if m.state == "list" {
		var cmd tea.Cmd
		m.toolList, cmd = m.toolList.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m inspectorModel) executeSelectedTool() tea.Msg {
	// Convert params from map[string]string to map[string]interface{}
	params := make(map[string]interface{})
	for k, v := range m.selectedTool.params {
		params[k] = v
	}

	// Call the tool
	result, err := m.mcpClient.CallTool(m.ctx, m.selectedTool.tool.name, params)
	if err != nil {
		return toolExecutedMsg{
			err: fmt.Errorf("error calling tool: %v", err),
		}
	}

	// Format the result JSON
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, result, "", "  "); err == nil {
		return toolExecutedMsg{result: prettyJSON.String()}
	}

	return toolExecutedMsg{result: string(result)}
}

func (m *inspectorModel) selectTool(tool Item) {
	m.state = "params"
	m.selectedTool = selectedToolState{
		tool:       tool,
		params:     make(map[string]string),
		paramIndex: 0,
	}

	// Extract required parameters
	m.requiredParams = []string{}
	if parameters, ok := tool.toolData["parameters"].(map[string]interface{}); ok {
		if required, ok := parameters["required"].([]interface{}); ok {
			for _, req := range required {
				if reqStr, ok := req.(string); ok {
					m.requiredParams = append(m.requiredParams, reqStr)
				}
			}
		}
	}
}

func (m inspectorModel) allParamsEntered() bool {
	if len(m.requiredParams) == 0 {
		return true
	}

	for _, param := range m.requiredParams {
		if _, exists := m.selectedTool.params[param]; !exists || m.selectedTool.params[param] == "" {
			return false
		}
	}

	return true
}

func (m inspectorModel) View() string {
	if m.quitting {
		return "Exiting inspector...\n"
	}

	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}

	switch m.state {
	case "list":
		return docStyle.Render(m.toolList.View())

	case "params":
		var sb strings.Builder

		sb.WriteString(titleStyle.Render(fmt.Sprintf(" Tool: %s ", m.selectedTool.tool.name)))
		sb.WriteString("\n\n")
		sb.WriteString(m.selectedTool.tool.description)
		sb.WriteString("\n\n")

		sb.WriteString("Parameters:\n")

		for i, param := range m.requiredParams {
			// Find parameter description if available
			paramDesc := "No description available"
			if parameters, ok := m.selectedTool.tool.toolData["parameters"].(map[string]interface{}); ok {
				if properties, ok := parameters["properties"].(map[string]interface{}); ok {
					if schema, ok := properties[param].(map[string]interface{}); ok {
						if desc, ok := schema["description"].(string); ok && desc != "" {
							paramDesc = desc
						}
					}
				}
			}

			value := m.selectedTool.params[param]
			if i == m.selectedTool.paramIndex {
				sb.WriteString(fmt.Sprintf("> %s: %s | (Enter value)\n", param, value))
				sb.WriteString(fmt.Sprintf("  %s\n", paramDesc))
			} else {
				sb.WriteString(fmt.Sprintf("  %s: %s\n", param, value))
				sb.WriteString(fmt.Sprintf("  %s\n", paramDesc))
			}
		}

		sb.WriteString("\n")
		if m.allParamsEntered() {
			sb.WriteString("Press 'e' to execute, 'esc' to go back\n")
		}

		return docStyle.Render(sb.String())

	case "result":
		var sb strings.Builder

		sb.WriteString(titleStyle.Render(fmt.Sprintf(" Result: %s ", m.selectedTool.tool.name)))
		sb.WriteString("\n\n")

		sb.WriteString("Parameters used:\n")
		for param, value := range m.selectedTool.params {
			sb.WriteString(fmt.Sprintf("  %s: %s\n", param, value))
		}

		sb.WriteString("\nResult:\n")
		sb.WriteString(m.selectedTool.result)
		sb.WriteString("\n\n")
		sb.WriteString("Press 'esc' to go back to tool list\n")

		return docStyle.Render(sb.String())
	}

	return "Unknown state"
}

func (r *Operations) InspectCmd() *cobra.Command {
	var local bool
	var debug bool
	var serverURL string
	var headers []string
	var command string

	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect a MCP server",
		Long: `Inspect a MCP server, discover tools, and interact with them.
		
Examples:
  # Inspect remote MCP server
  bl inspect --server-url https://example.com/mcp
  
  # Inspect local MCP server
  bl inspect --local`,
		Run: func(cmd *cobra.Command, args []string) {
			r.Inspect(context.Background(), local, serverURL, headers, debug, command)
		},
	}

	cmd.Flags().BoolVar(&local, "local", false, "Use local MCP server")
	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug mode")
	cmd.Flags().StringVar(&serverURL, "server-url", "", "MCP server URL (required if not using local mode)")
	cmd.Flags().StringSliceVar(&headers, "header", []string{}, "HTTP headers in the format 'key:value'")
	cmd.Flags().StringVar(&command, "command", "", "Command to execute")
	return cmd
}

func (r *Operations) Inspect(ctx context.Context, local bool, serverURL string, headersSlice []string, debug bool, command string) {
	if !local && serverURL == "" {
		fmt.Println("Error: server-url is required when not using local mode")
		os.Exit(1)
	}

	// Convert headers slice to map
	headersMap := make(map[string]string)
	for _, header := range headersSlice {
		parts := strings.SplitN(header, ":", 2)
		if len(parts) == 2 {
			headersMap[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}

	// Create MCP client based on mode
	var mcpClient *mcp.MCPClient
	if local {
		mcpClient = mcp.NewMCPClient(mcp.TransportStdio, command, "", nil)
	} else {
		mcpClient = mcp.NewMCPClient(mcp.TransportWebSocket, "", serverURL, headersMap)
	}

	defer mcpClient.Close()

	if mcpClient == nil {
		fmt.Println("Error: Failed to create MCP client")
		os.Exit(1)
	}

	// Initialize and start the Bubble Tea application
	m, err := newModel(ctx, mcpClient)
	if err != nil {
		fmt.Println("Error initializing inspector:", err)
		os.Exit(1)
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error running inspector:", err)
		os.Exit(1)
	}
}
