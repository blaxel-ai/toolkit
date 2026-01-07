package core

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Installation step statuses
type Status int

const (
	StatusPending Status = iota
	StatusRunning
	StatusComplete
	StatusFailed
)

// Step represents an installation step
type Step struct {
	Title       string
	Description string
	Status      Status
}

// Model holds the state of our installation UI
type Model struct {
	steps       []Step
	currentStep int
	currentLogs []string
	spinner     spinner.Model
	width       int
	height      int
	done        bool
	err         error
	showLogs    bool
	maxLogLines int
	logOffset   int      // For scrolling effect
	logBuffer   []string // Full buffer of all logs
}

// Messages for updating the model
type (
	stepCompleteMsg int
	stepFailedMsg   struct {
		step int
		err  error
	}
	logMsg        string
	tickMsg       time.Time
	scrollTickMsg time.Time // New message type for scrolling
	doneMsg       bool
)

// Styles
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("214")) // Changed from 12 to 214 (orange)

	descStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))

	checkMark = lipgloss.NewStyle().
			Foreground(lipgloss.Color("10")).
			SetString("✓")

	xMark = lipgloss.NewStyle().
		Foreground(lipgloss.Color("9")).
		SetString("✗")

	pendingMark = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8")).
			SetString("○")
)

// NewInstallationModel creates a new installation progress model
func NewInstallationModel() Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Changed from 12 to 214 (orange)

	return Model{
		steps: []Step{
			{Title: "Creating project", Description: "Setting up directory"},
			{Title: "Cloning repository", Description: "Downloading template"},
			{Title: "Installing dependencies", Description: "This might take a moment"},
			{Title: "Finalizing", Description: "Cleaning up"},
		},
		currentStep: -1,
		spinner:     s,
		currentLogs: make([]string, 0, 4),
		showLogs:    false,
		maxLogLines: 4, // Show 4 lines at a time
		logOffset:   0,
		logBuffer:   make([]string, 0, 100),
	}
}

// Init is the first function called
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.WindowSize(),
		// Add scroll ticker
		tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
			return scrollTickMsg(t)
		}),
	)
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		}

	case stepMsg:
		m.StartStep(msg.step)
		return m, nil

	case stepCompleteMsg:
		if int(msg) < len(m.steps) {
			m.steps[int(msg)].Status = StatusComplete
			// Clear logs when dependency installation completes
			if int(msg) == 2 {
				m.currentLogs = []string{}
				m.logBuffer = []string{}
				m.logOffset = 0
			}
		}
		return m, nil

	case stepFailedMsg:
		if msg.step < len(m.steps) {
			m.steps[msg.step].Status = StatusFailed
			m.err = msg.err
		}
		return m, tea.Quit

	case enableLogsMsg:
		m.showLogs = bool(msg)
		return m, nil

	case updateDescriptionMsg:
		if msg.step >= 0 && msg.step < len(m.steps) {
			m.steps[msg.step].Description = msg.desc
		}
		return m, nil

	case logMsg:
		// Add to full buffer
		if m.showLogs {
			m.logBuffer = append(m.logBuffer, string(msg))

			// Auto-scroll to show latest logs
			if len(m.logBuffer) > m.maxLogLines {
				m.logOffset = len(m.logBuffer) - m.maxLogLines
			}

			// Update current visible logs
			m.updateVisibleLogs()
		}
		return m, nil

	case tickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case scrollTickMsg:
		// Auto-scroll logs if we're showing them and have more than can fit
		if m.showLogs && len(m.logBuffer) > m.maxLogLines {
			// Always show the latest logs
			m.logOffset = len(m.logBuffer) - m.maxLogLines
			m.updateVisibleLogs()
		}
		// Continue ticking
		return m, tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
			return scrollTickMsg(t)
		})

	case doneMsg:
		m.done = true
		return m, tea.Quit

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	default:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	return m, nil
}

// updateVisibleLogs updates the currently visible log lines based on offset
func (m *Model) updateVisibleLogs() {
	m.currentLogs = []string{}

	start := m.logOffset
	end := m.logOffset + m.maxLogLines

	if end > len(m.logBuffer) {
		end = len(m.logBuffer)
	}

	if start < len(m.logBuffer) {
		m.currentLogs = m.logBuffer[start:end]
	}
}

// View renders the UI
func (m Model) View() string {
	if m.done {
		return ""
	}

	var s strings.Builder

	// Add a title
	s.WriteString("\n")
	s.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("214")). // Changed from 12 to 214 (orange)
		MarginBottom(1).
		Render("Installing your Blaxel project..."))
	s.WriteString("\n\n")

	for i, step := range m.steps {
		// Status icon
		var icon string
		switch step.Status {
		case StatusRunning:
			icon = m.spinner.View()
		case StatusComplete:
			icon = checkMark.String()
		case StatusFailed:
			icon = xMark.String()
		default:
			icon = pendingMark.String()
		}

		// Step info with better formatting
		stepLine := fmt.Sprintf("%s %s",
			titleStyle.Render(step.Title),
			descStyle.Render(fmt.Sprintf("— %s", step.Description)))

		s.WriteString(fmt.Sprintf("  %s %s\n", icon, stepLine))

		// Show logs for current running step if it's dependencies
		if step.Status == StatusRunning && i == 2 && m.showLogs && len(m.currentLogs) > 0 {
			// Show log window with current logs (exactly 4 lines)
			for idx := 0; idx < m.maxLogLines; idx++ {
				if idx < len(m.currentLogs) {
					logLine := m.currentLogs[idx]
					if logLine != "" {
						// Show the log line as-is, no truncation
						s.WriteString(fmt.Sprintf("     %s\n",
							lipgloss.NewStyle().
								Foreground(lipgloss.Color("240")).
								Render(logLine)))
					} else {
						// Empty line to maintain consistent height
						s.WriteString("\n")
					}
				} else {
					// Pad with empty lines to always show 4 lines
					s.WriteString("\n")
				}
			}
		}

		// Add spacing between steps
		if i < len(m.steps)-1 {
			s.WriteString("\n")
		}
	}

	if m.err != nil {
		s.WriteString("\n\n" + lipgloss.NewStyle().
			Foreground(lipgloss.Color("9")).
			Bold(true).
			Render("✗ Error: ") + m.err.Error())
	}

	return s.String()
}

// Helper functions to control the installation flow

func (m *Model) StartStep(step int) {
	if step >= 0 && step < len(m.steps) {
		m.currentStep = step
		m.steps[step].Status = StatusRunning
		m.currentLogs = make([]string, 0, 5)
	}
}

func (m *Model) CompleteStep(step int) {
	if step >= 0 && step < len(m.steps) {
		m.steps[step].Status = StatusComplete
	}
}

func (m *Model) FailStep(step int, err error) {
	if step >= 0 && step < len(m.steps) {
		m.steps[step].Status = StatusFailed
		m.err = err
	}
}

func (m *Model) Done() {
	m.done = true
}

// RunInstallationWithTea runs the installation process with Bubble Tea UI
func RunInstallationWithTea(t Template, opts TemplateOptions) error {
	// Create the model
	model := NewInstallationModel()

	// Create the Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Run installation in a goroutine
	errChan := make(chan error, 1)
	go func() {
		err := runInstallationSteps(p, t, opts)
		errChan <- err
	}()

	// Run the UI
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running UI: %w", err)
	}

	// Check for installation errors
	installErr := <-errChan
	if installErr != nil {
		return installErr
	}

	// Check if the model has an error
	if m, ok := finalModel.(Model); ok && m.err != nil {
		return m.err
	}

	return nil
}

func runInstallationSteps(p *tea.Program, t Template, opts TemplateOptions) error {
	// Step 1: Create project directory
	p.Send(stepMsg{step: 0, status: StatusRunning})
	time.Sleep(100 * time.Millisecond)

	if err := os.MkdirAll(opts.Directory, 0755); err != nil {
		p.Send(stepFailedMsg{step: 0, err: err})
		return err
	}

	p.Send(stepCompleteMsg(0))
	time.Sleep(200 * time.Millisecond)

	// Step 2: Clone repository
	p.Send(stepMsg{step: 1, status: StatusRunning})

	if !isCommandAvailable("git") {
		err := fmt.Errorf("git is not available on your system. Please install git and try again")
		p.Send(stepFailedMsg{step: 1, err: err})
		return err
	}

	env := os.Getenv("BL_ENV")
	branch := "main"
	if env == "dev" || env == "local" {
		branch = "develop"
	}

	cloneCmd := exec.Command("git", "clone", "-b", branch, "--progress", t.URL, opts.Directory)
	if err := cloneCmd.Run(); err != nil {
		p.Send(stepFailedMsg{step: 1, err: fmt.Errorf("failed to clone: %w", err)})
		return err
	}

	// Remove .git folder
	gitDir := opts.Directory + "/.git"
	_ = os.RemoveAll(gitDir)

	p.Send(stepCompleteMsg(1))
	time.Sleep(200 * time.Millisecond)

	// Step 3: Install dependencies
	p.Send(stepMsg{step: 2, status: StatusRunning})
	p.Send(enableLogsMsg(true))

	var depErr error
	switch opts.Language {
	case "python":
		depErr = installPythonDepsWithLogs(opts.Directory, p)
	case "typescript":
		depErr = installTypescriptDepsWithLogs(opts.Directory, p)
	}

	p.Send(enableLogsMsg(false))

	if depErr != nil {
		p.Send(stepFailedMsg{step: 2, err: depErr})
		return depErr
	}

	p.Send(stepCompleteMsg(2))
	time.Sleep(200 * time.Millisecond)

	// Step 4: Finalize
	p.Send(stepMsg{step: 3, status: StatusRunning})
	time.Sleep(200 * time.Millisecond)
	p.Send(stepCompleteMsg(3))
	time.Sleep(100 * time.Millisecond)

	// Done
	p.Send(doneMsg(true))
	return nil
}

// Additional message types
type stepMsg struct {
	step   int
	status Status
}

type enableLogsMsg bool

type updateDescriptionMsg struct {
	step int
	desc string
}

// GetDependencyManager returns the dependency manager and version for a given language
func GetDependencyManager(language, directory string) (string, string) {
	switch language {
	case "python":
		if isCommandAvailable("uv") {
			if output, err := exec.Command("uv", "--version").Output(); err == nil {
				return "uv", strings.TrimSpace(string(output))
			}
		}
		if isCommandAvailable("pip") {
			if output, err := exec.Command("pip", "--version").Output(); err == nil {
				parts := strings.Fields(string(output))
				if len(parts) >= 2 {
					return "pip", parts[1]
				}
			}
		}
	case "typescript":
		if isCommandAvailable("pnpm") {
			if output, err := exec.Command("pnpm", "--version").Output(); err == nil {
				return "pnpm", strings.TrimSpace(string(output))
			}
		}
		if isCommandAvailable("npm") {
			if output, err := exec.Command("npm", "--version").Output(); err == nil {
				return "npm", strings.TrimSpace(string(output))
			}
		}
	}
	return "", ""
}

// sendLogUpdate just sends the log line as-is
func sendLogUpdate(p *tea.Program, line string) {
	line = strings.TrimSpace(line)
	if line != "" {
		p.Send(logMsg(line))
	}
}

func installPythonDepsWithLogs(directory string, p *tea.Program) error {
	// Update description based on package manager
	if isCommandAvailable("uv") {
		p.Send(updateDescriptionMsg{step: 2, desc: "Installing with uv"})
		cmd := exec.Command("uv", "sync")
		cmd.Dir = directory
		return runCommandWithLogs(cmd, p)
	}

	// Handle pip installation with proper streaming
	if isCommandAvailable("python") || isCommandAvailable("python3") {
		p.Send(updateDescriptionMsg{step: 2, desc: "Installing with pip"})

		// Check for requirements.txt or pyproject.toml
		requirementsPath := filepath.Join(directory, "requirements.txt")
		pyprojectPath := filepath.Join(directory, "pyproject.toml")
		venvPath := filepath.Join(directory, ".venv")

		// Create virtual environment if it doesn't exist
		if _, err := os.Stat(venvPath); os.IsNotExist(err) {
			var venvCreateCmd *exec.Cmd
			if isCommandAvailable("python3") {
				venvCreateCmd = exec.Command("python3", "-m", "venv", ".venv")
			} else if isCommandAvailable("python") {
				venvCreateCmd = exec.Command("python", "-m", "venv", ".venv")
			} else {
				return fmt.Errorf("neither python3 nor python command found")
			}

			venvCreateCmd.Dir = directory
			if err := venvCreateCmd.Run(); err != nil {
				return fmt.Errorf("failed to create virtual environment: %w", err)
			}
		}

		// Determine the python executable path in the virtual environment
		var pythonPath string
		possiblePaths := []string{
			filepath.Join(venvPath, "bin", "python"),
			filepath.Join(venvPath, "bin", "python3"),
			filepath.Join(venvPath, "Scripts", "python.exe"),
			filepath.Join(venvPath, "Scripts", "python3.exe"),
		}

		for _, path := range possiblePaths {
			if absPath, err := filepath.Abs(path); err == nil {
				if _, err := os.Stat(absPath); err == nil {
					pythonPath = absPath
					break
				}
			}
		}

		if pythonPath == "" {
			return fmt.Errorf("could not find python executable in virtual environment")
		}

		// Install dependencies with streaming output
		var pipCmd *exec.Cmd
		if _, err := os.Stat(pyprojectPath); err == nil {
			pipCmd = exec.Command(pythonPath, "-u", "-m", "pip", "install", "-e", ".", "--progress-bar=on")
		} else if _, err := os.Stat(requirementsPath); err == nil {
			pipCmd = exec.Command(pythonPath, "-u", "-m", "pip", "install", "-r", "requirements.txt", "--progress-bar=on")
		} else {
			return fmt.Errorf("neither pyproject.toml nor requirements.txt found in %s", directory)
		}

		pipCmd.Dir = directory
		// Set environment to ensure unbuffered output
		pipCmd.Env = append(os.Environ(), "PYTHONUNBUFFERED=1")
		return runCommandWithLogs(pipCmd, p)
	}

	return fmt.Errorf("neither uv nor pip is available on your system")
}

func installTypescriptDepsWithLogs(directory string, p *tea.Program) error {
	// Update description
	p.Send(updateDescriptionMsg{step: 2, desc: "Installing with pnpm"})

	cmd := exec.Command("npx", "pnpm", "install", "--reporter=default")
	cmd.Dir = directory
	return runCommandWithLogs(cmd, p)
}

func runCommandWithLogs(cmd *exec.Cmd, p *tea.Program) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Read output in real-time
	go streamOutput(stdout, p)
	go streamOutput(stderr, p)

	return cmd.Wait()
}

func streamOutput(reader io.Reader, p *tea.Program) {
	scanner := bufio.NewScanner(reader)
	// Set a larger buffer for scanner to handle long lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		sendLogUpdate(p, line)
	}
}
