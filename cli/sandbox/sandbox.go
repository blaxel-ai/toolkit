package sandbox

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	outputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(1, 2).
			Height(20)

	inputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("69")).
			Padding(0, 1)

	promptStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("69")).
			Bold(true)

	completionItemStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Margin(0, 1)

	completionSelectedStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("15")). // White background
				Foreground(lipgloss.Color("0")).  // Black text
				Padding(0, 1).
				Margin(0, 1)

	completionContainerStyle = lipgloss.NewStyle().
					Padding(0, 2)
)

// Key bindings
type keyMap struct {
	Up     key.Binding
	Down   key.Binding
	Left   key.Binding
	Right  key.Binding
	Enter  key.Binding
	Quit   key.Binding
	Clear  key.Binding
	Tab    key.Binding
	Escape key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Enter, k.Quit, k.Clear, k.Tab}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Left, k.Right},
		{k.Enter, k.Quit, k.Clear, k.Tab},
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up"),
		key.WithHelp("↑", "history up / completion up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down"),
		key.WithHelp("↓", "history down / completion down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←", "completion left"),
	),
	Right: key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("→", "completion right"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "execute"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c", "ctrl+d"),
		key.WithHelp("ctrl+c", "exit"),
	),
	Clear: key.NewBinding(
		key.WithKeys("ctrl+l"),
		key.WithHelp("ctrl+l", "clear"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "autocomplete / next"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "cancel completion"),
	),
}

// Messages
type commandExecutedMsg struct {
	output      string
	err         error
	clearScreen bool
}

type completionLoadedMsg struct {
	completions []string
	err         error
}

// Completion item for the list
type CompletionItem struct {
	name string
}

func (i CompletionItem) Title() string       { return i.name }
func (i CompletionItem) Description() string { return "" }
func (i CompletionItem) FilterValue() string { return i.name }

// Model
type SandboxShell struct {
	client         *SandboxClient
	ctx            context.Context
	input          textinput.Model
	output         viewport.Model
	currentDir     string
	commandHistory []string
	historyIndex   int
	executing      bool
	ready          bool
	width          int
	height         int

	// Completion state
	showingCompletion bool
	completions       []string
	completionIndex   int
	completionPrefix  string
	originalInput     string
	completionCols    int // Number of columns in completion grid
	completionRows    int // Number of rows in completion grid
}

func NewSandboxShellWithURL(ctx context.Context, workspace, sandboxName, url string, authHeaders map[string]string) (*SandboxShell, error) {
	client, err := NewSandboxClientWithURL(workspace, sandboxName, url, authHeaders)
	if err != nil {
		return nil, err
	}
	return createSandboxShell(ctx, client), nil
}

func createSandboxShell(ctx context.Context, client *SandboxClient) *SandboxShell {
	input := textinput.New()
	input.Focus()
	input.CharLimit = 1000
	input.Width = 80

	output := viewport.New(100, 25) // Larger default size
	output.SetContent("")

	// Slow down the scroll speed by setting mouse wheel delta to 1 line per scroll
	output.MouseWheelEnabled = true
	output.MouseWheelDelta = 1

	return &SandboxShell{
		client:            client,
		ctx:               ctx,
		input:             input,
		output:            output,
		currentDir:        "/",
		commandHistory:    []string{},
		historyIndex:      -1,
		ready:             false,
		showingCompletion: false,
		completions:       []string{},
		completionIndex:   0,
	}
}

func (m *SandboxShell) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.initializeShell)
}

func (m *SandboxShell) initializeShell() tea.Msg {
	// Display welcome message
	welcome := fmt.Sprintf("Welcome to Sandbox Shell!\nConnected to: %s/%s\nType 'help' for available commands.\n\n", m.client.Workspace, m.client.SandboxName)
	welcome += fmt.Sprintf("Current directory: %s\n", m.currentDir)

	return commandExecutedMsg{
		output: welcome,
		err:    nil,
	}
}

func (m *SandboxShell) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
	)

	// Always update input for typing
	m.input, tiCmd = m.input.Update(msg)

	// Only update viewport for non-key messages to prevent scrolling while typing
	switch msg.(type) {
	case tea.KeyMsg:
		// Don't update viewport on key messages - prevents scrolling while typing
	default:
		m.output, vpCmd = m.output.Update(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update component sizes with validation
		outputHeight := m.height - 6 // Reduced reserved space for more output area
		if outputHeight < 10 {
			outputHeight = 10
		}

		inputWidth := m.width - 15 // Reduced reserved space for prompt
		if inputWidth < 30 {
			inputWidth = 30
		}

		m.output.Width = m.width - 4
		m.output.Height = outputHeight
		m.input.Width = inputWidth

		// Recalculate completion grid if showing completions
		if m.showingCompletion {
			m.calculateCompletionGrid()
		}

		m.ready = true

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, keys.Quit):
			_ = m.client.Close()
			return m, tea.Quit

		case key.Matches(msg, keys.Clear):
			m.output.SetContent("")
			return m, nil

		case key.Matches(msg, keys.Up):
			if m.showingCompletion && len(m.completions) > 0 {
				// Move up in the grid
				newIndex := m.completionIndex - m.completionCols
				if newIndex >= 0 {
					m.completionIndex = newIndex
				} else {
					// Wrap to bottom row of same column
					col := m.completionIndex % m.completionCols
					m.completionIndex = ((m.completionRows - 1) * m.completionCols) + col
					if m.completionIndex >= len(m.completions) {
						m.completionIndex = ((m.completionRows - 2) * m.completionCols) + col
					}
				}
				m.input.SetValue(m.completionPrefix + m.completions[m.completionIndex])
				m.input.CursorEnd()
				return m, nil
			}
			if len(m.commandHistory) > 0 {
				if m.historyIndex == -1 {
					m.historyIndex = len(m.commandHistory) - 1
				} else if m.historyIndex > 0 {
					m.historyIndex--
				}
				if m.historyIndex >= 0 && m.historyIndex < len(m.commandHistory) {
					m.input.SetValue(m.commandHistory[m.historyIndex])
					m.input.CursorEnd()
				}
			}
			return m, nil

		case key.Matches(msg, keys.Down):
			if m.showingCompletion && len(m.completions) > 0 {
				// Move down in the grid
				newIndex := m.completionIndex + m.completionCols
				if newIndex < len(m.completions) {
					m.completionIndex = newIndex
				} else {
					// Wrap to top row of same column
					col := m.completionIndex % m.completionCols
					m.completionIndex = col
				}
				m.input.SetValue(m.completionPrefix + m.completions[m.completionIndex])
				m.input.CursorEnd()
				return m, nil
			}
			if len(m.commandHistory) > 0 {
				if m.historyIndex < len(m.commandHistory)-1 {
					m.historyIndex++
					m.input.SetValue(m.commandHistory[m.historyIndex])
					m.input.CursorEnd()
				} else {
					m.historyIndex = -1
					m.input.SetValue("")
				}
			}
			return m, nil

		case key.Matches(msg, keys.Left):
			if m.showingCompletion && len(m.completions) > 0 {
				// Move left in the grid
				if m.completionIndex > 0 {
					m.completionIndex--
				} else {
					// Wrap to end
					m.completionIndex = len(m.completions) - 1
				}
				m.input.SetValue(m.completionPrefix + m.completions[m.completionIndex])
				m.input.CursorEnd()
				return m, nil
			}
			return m, nil

		case key.Matches(msg, keys.Right):
			if m.showingCompletion && len(m.completions) > 0 {
				// Move right in the grid
				if m.completionIndex < len(m.completions)-1 {
					m.completionIndex++
				} else {
					// Wrap to beginning
					m.completionIndex = 0
				}
				m.input.SetValue(m.completionPrefix + m.completions[m.completionIndex])
				m.input.CursorEnd()
				return m, nil
			}
			return m, nil

		case key.Matches(msg, keys.Tab):
			if !m.showingCompletion {
				m.showingCompletion = true
				m.originalInput = m.input.Value()
				return m, m.startCompletion
			} else {
				// Move right in the grid (same as right arrow)
				if len(m.completions) > 0 {
					if m.completionIndex < len(m.completions)-1 {
						m.completionIndex++
					} else {
						// Wrap to beginning
						m.completionIndex = 0
					}
					m.input.SetValue(m.completionPrefix + m.completions[m.completionIndex])
					m.input.CursorEnd()
				}
				return m, nil
			}

		case key.Matches(msg, keys.Enter):
			if m.showingCompletion {
				// Select current completion item
				if len(m.completions) > 0 {
					m.input.SetValue(m.completionPrefix + m.completions[m.completionIndex])
					m.input.CursorEnd()
				}
				m.showingCompletion = false
				return m, nil
			}

			if m.executing {
				return m, nil
			}

			command := strings.TrimSpace(m.input.Value())
			if command == "" {
				return m, nil
			}

			// Add to history
			m.commandHistory = append(m.commandHistory, command)
			m.historyIndex = -1

			// Clear input
			m.input.SetValue("")

			// Execute command
			m.executing = true
			return m, m.executeCommand(command)

		case key.Matches(msg, keys.Escape):
			if m.showingCompletion {
				m.showingCompletion = false
				m.input.SetValue(m.originalInput)
				m.input.CursorEnd()
			}
			return m, nil
		}

	case commandExecutedMsg:
		m.executing = false

		// Handle clear screen command
		if msg.clearScreen {
			m.output.SetContent("")
			return m, nil
		}

		currentContent := m.output.View()
		if currentContent != "" {
			currentContent += "\n"
		}

		newContent := currentContent + msg.output

		// Limit the total content length to prevent memory issues
		lines := strings.Split(newContent, "\n")
		maxLines := 1000 // Keep last 1000 lines
		if len(lines) > maxLines {
			lines = lines[len(lines)-maxLines:]
			newContent = strings.Join(lines, "\n")
		}

		m.output.SetContent(newContent)
		m.output.GotoBottom()

	case completionLoadedMsg:
		if msg.err != nil {
			m.showingCompletion = false
			return m, nil
		}

		if len(msg.completions) == 0 {
			m.showingCompletion = false
		} else {
			m.completions = msg.completions
			m.completionIndex = 0

			// Calculate grid dimensions
			m.calculateCompletionGrid()

			if len(m.completions) > 0 {
				m.input.SetValue(m.completionPrefix + m.completions[m.completionIndex])
				m.input.CursorEnd()
			}
		}
		return m, nil
	}

	return m, tea.Batch(tiCmd, vpCmd)
}

func (m *SandboxShell) executeCommand(command string) tea.Cmd {
	return func() tea.Msg {
		parts := strings.Fields(command)
		if len(parts) == 0 {
			return commandExecutedMsg{output: "", err: nil}
		}

		cmd := parts[0]

		// Handle built-in commands
		switch cmd {
		case "help":
			help := `Available commands:
  help          - Show this help message
  clear, cls    - Clear screen
  exit, quit    - Exit shell

  Any other command will be executed in the sandbox environment.`
			return commandExecutedMsg{output: help, err: nil}

		case "clear", "cls":
			// Clear the output viewport directly (same as Ctrl+L)
			return commandExecutedMsg{output: "", err: nil, clearScreen: true}

		case "exit", "quit":
			return tea.Quit()

		default:
			// Execute command in sandbox
			return m.runCommand(command)
		}
	}
}

func (m *SandboxShell) runCommand(command string) tea.Msg {
	var output strings.Builder
	output.WriteString(fmt.Sprintf("$ %s", command))
	if strings.HasPrefix(command, "cd") {
		// Extract the target directory from the command
		parts := strings.Fields(command)
		if len(parts) > 1 {
			targetPath := parts[1]
			// Update the current directory
			if filepath.IsAbs(targetPath) {
				m.currentDir = filepath.Clean(targetPath)
			} else {
				m.currentDir = filepath.Clean(filepath.Join(m.currentDir, targetPath))
			}
		}
	} else {
		// Execute the command with current directory as working directory
		processWithLogs, err := m.client.ExecuteCommand(m.ctx, command, "", m.currentDir)
		if err != nil {
			return commandExecutedMsg{
				output: fmt.Sprintf("Error executing command: %v", err),
				err:    err,
			}
		}
		if processWithLogs.Logs != "" {
			output.WriteString("\n" + processWithLogs.Logs)
		}
	}
	return commandExecutedMsg{output: output.String(), err: nil}
}

func (m *SandboxShell) startCompletion() tea.Msg {
	currentInput := strings.TrimSpace(m.originalInput)

	// Parse the command to find the path to complete
	parts := strings.Fields(currentInput)
	if len(parts) == 0 {
		return completionLoadedMsg{completions: []string{}, err: nil}
	}

	// Early check for flags - if the last argument is a flag and there's no trailing space,
	// don't do any completion processing at all
	if len(parts) > 1 {
		lastArg := parts[len(parts)-1]
		if strings.HasPrefix(lastArg, "-") && !strings.HasSuffix(m.originalInput, " ") {
			return completionLoadedMsg{completions: []string{}, err: nil}
		}
	}

	// Commands that might need path completion
	pathCommands := map[string]bool{
		"cd": true, "ls": true, "ll": true, "la": true, "l": true, "cat": true, "less": true, "more": true,
		"mkdir": true, "rmdir": true, "rm": true, "cp": true, "mv": true, "find": true,
		"grep": true, "touch": true, "chmod": true, "chown": true, "file": true, "zip": true, "unzip": true,
		"tar": true, "gzip": true, "gunzip": true, "bz2": true, "bunzip2": true, "xz": true, "unxz": true,
		"lzma": true, "unlzma": true, "lzop": true, "unlzop": true, "rar": true, "unrar": true, "7z": true, "un7z": true, "bzip2": true,
	}

	// Commands that should only complete directories (not files)
	directoryOnlyCommands := map[string]bool{
		"cd": true, "ls": true, "ll": true, "la": true, "l": true, "mkdir": true, "rmdir": true,
	}

	var pathToComplete string
	var prefix string

	if len(parts) == 0 {
		return completionLoadedMsg{completions: []string{}, err: nil}
	}

	command := parts[0]
	isDirectoryOnly := directoryOnlyCommands[command]

	if len(parts) == 1 {
		// Check if this is a path command AND there's a trailing space
		// If no trailing space, don't complete (user is still typing the command)
		if !pathCommands[command] || !strings.HasSuffix(m.originalInput, " ") {
			return completionLoadedMsg{completions: []string{}, err: nil}
		}
		// Only command typed with trailing space, complete current directory
		pathToComplete = m.currentDir
		prefix = ""
	} else if pathCommands[parts[0]] {
		// Command that might need path completion
		lastArg := parts[len(parts)-1]

		// If there's a trailing space in the original input, we're completing a new argument
		// not the last parsed argument
		if strings.HasSuffix(m.originalInput, " ") {
			pathToComplete = m.currentDir
			prefix = ""
		} else if strings.HasPrefix(lastArg, "/") {
			// Absolute path
			if lastArg == "/" {
				// Just root slash, complete from root with no prefix
				pathToComplete = "/"
				prefix = ""
			} else if strings.HasSuffix(lastArg, "/") {
				// Ends with slash (e.g., "/Users/"), complete from that directory with no prefix
				pathToComplete = lastArg
				prefix = ""
			} else if strings.Count(lastArg, "/") == 1 {
				// Single slash at start (e.g., "/doc"), complete from root
				pathToComplete = "/"
				prefix = lastArg[1:] // Remove the leading slash
			} else {
				// Multiple slashes (e.g., "/home/user/doc"), use directory
				pathToComplete = filepath.Dir(lastArg)
				prefix = filepath.Base(lastArg)
			}
		} else if strings.Contains(lastArg, "/") {
			// Relative path with directories
			if strings.HasSuffix(lastArg, "/") {
				// Ends with slash, complete from that directory with no prefix
				pathToComplete = filepath.Join(m.currentDir, lastArg)
				prefix = ""
			} else {
				// Normal relative path
				pathToComplete = filepath.Join(m.currentDir, filepath.Dir(lastArg))
				prefix = filepath.Base(lastArg)
			}
		} else {
			// Just a filename in current directory
			pathToComplete = m.currentDir
			prefix = lastArg
		}
	} else {
		// Command doesn't need path completion
		return completionLoadedMsg{completions: []string{}, err: nil}
	}

	// Get directory listing
	dir, err := m.client.ListDirectory(m.ctx, pathToComplete)
	if err != nil {
		return completionLoadedMsg{completions: []string{}, err: err}
	}

	// Collect matching entries
	var completions []string

	// Convert prefix to lowercase for case-insensitive matching
	lowerPrefix := strings.ToLower(prefix)

	// Add subdirectories first (they're more commonly used)
	for _, subdir := range dir.Subdirectories {
		if prefix == "" || strings.HasPrefix(strings.ToLower(subdir.Name), lowerPrefix) {
			completions = append(completions, subdir.Name+"/")
		}
	}

	// Add files only for commands that need file completion
	if !isDirectoryOnly {
		for _, file := range dir.Files {
			if prefix == "" || strings.HasPrefix(strings.ToLower(file.Name), lowerPrefix) {
				completions = append(completions, file.Name)
			}
		}
	}

	// Sort completions
	sort.Strings(completions)

	// Set the completion prefix for later use
	// This should be everything up to the part we're completing
	if len(parts) == 1 {
		// Only command typed, prefix is just the command + space
		m.completionPrefix = parts[0] + " "
	} else if strings.HasSuffix(m.originalInput, " ") {
		// There's a trailing space, so we're completing a new argument
		// Keep everything including the space
		m.completionPrefix = strings.Join(parts, " ") + " "
	} else {
		// Multiple parts, need to preserve everything except the last part being completed
		baseCommand := strings.Join(parts[:len(parts)-1], " ") + " "
		lastArg := parts[len(parts)-1]

		if strings.HasPrefix(lastArg, "/") {
			// Absolute path
			if strings.HasSuffix(lastArg, "/") {
				// Ends with slash (e.g., "/Users/"), keep the full path
				m.completionPrefix = baseCommand + lastArg
			} else if strings.Count(lastArg, "/") == 1 {
				// Single slash at start (e.g., "/doc"), keep the slash
				m.completionPrefix = baseCommand + "/"
			} else {
				// Multiple slashes, keep the directory part
				dirPrefix := filepath.Dir(lastArg)
				if dirPrefix != "/" {
					m.completionPrefix = baseCommand + dirPrefix + "/"
				} else {
					m.completionPrefix = baseCommand + "/"
				}
			}
		} else if strings.Contains(lastArg, "/") {
			// Relative path with directories
			if strings.HasSuffix(lastArg, "/") {
				// Ends with slash, keep the full path
				m.completionPrefix = baseCommand + lastArg
			} else {
				dirPrefix := filepath.Dir(lastArg)
				if dirPrefix != "." {
					m.completionPrefix = baseCommand + dirPrefix + "/"
				} else {
					m.completionPrefix = baseCommand
				}
			}
		} else {
			// Just a filename in current directory
			m.completionPrefix = baseCommand
		}
	}

	return completionLoadedMsg{completions: completions, err: nil}
}

func (m *SandboxShell) calculateCompletionGrid() {
	if len(m.completions) == 0 {
		m.completionCols = 0
		m.completionRows = 0
		return
	}

	// Calculate the maximum width needed for any completion item
	maxItemWidth := 0
	for _, completion := range m.completions {
		itemWidth := len(completion) + 2 // Add padding
		if itemWidth > maxItemWidth {
			maxItemWidth = itemWidth
		}
	}

	// Calculate how many columns can fit in the available width
	availableWidth := m.width - 4 // Account for borders and margins
	m.completionCols = availableWidth / maxItemWidth
	if m.completionCols < 1 {
		m.completionCols = 1
	}

	// Calculate the number of rows needed
	m.completionRows = (len(m.completions) + m.completionCols - 1) / m.completionCols
}

func (m *SandboxShell) renderCompletions() string {
	if !m.showingCompletion || len(m.completions) == 0 {
		return ""
	}

	var rows []string

	for row := 0; row < m.completionRows; row++ {
		var rowItems []string

		for col := 0; col < m.completionCols; col++ {
			index := row*m.completionCols + col
			if index >= len(m.completions) {
				break
			}

			completion := m.completions[index]
			if index == m.completionIndex {
				// Selected item with white background
				rowItems = append(rowItems, completionSelectedStyle.Render(completion))
			} else {
				// Regular item
				rowItems = append(rowItems, completionItemStyle.Render(completion))
			}
		}

		if len(rowItems) > 0 {
			rows = append(rows, strings.Join(rowItems, ""))
		}
	}

	return strings.Join(rows, "\n")
}

func (m *SandboxShell) View() string {
	if !m.ready {
		return "\n  Initializing sandbox shell..."
	}

	// Create the prompt
	prompt := promptStyle.Render(fmt.Sprintf("[%s@sandbox:%s]$ ", m.client.Workspace, m.currentDir))

	// Build the input section
	inputSection := inputStyle.Render(prompt + m.input.View())

	// Build the output section
	outputSection := outputStyle.Render(m.output.View())

	// Build completion section if showing
	var completionSection string
	if m.showingCompletion {
		completionSection = completionContainerStyle.Render(m.renderCompletions())
	}

	// Combine everything
	if m.showingCompletion {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			outputSection,
			inputSection,
			completionSection,
			"\nPress Ctrl+C to exit, Ctrl+L to clear, ↑/↓ for history, Tab for completion",
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		outputSection,
		inputSection,
		"\nPress Ctrl+C to exit, Ctrl+L to clear, ↑/↓ for history, Tab for completion",
	)
}
