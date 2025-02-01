package chat

import (
	"context"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

type ChatModel struct {
	textarea textarea.Model
	viewport viewport.Model
	spinner  spinner.Model

	Messages    []Message
	Err         error
	Workspace   string
	Environment string
	ResType     string
	ResName     string
	Loading     bool
	SendMessage func(ctx context.Context, workspace string, environment string, resType string, resName string, message string) (string, error)
}

type responseMsg struct {
	content string
}

type errMsg struct {
	err error
}

func (m *ChatModel) Init() tea.Cmd {
	physicalWidth, physicalHeight, _ := term.GetSize(int(os.Stdout.Fd()))

	// Account for borders and padding
	width := physicalWidth - 2
	height := physicalHeight - 6

	ta := m.initializeTextarea(width)
	sp := m.initializeSpinner()
	vp := m.initializeViewport(width, height)

	m.textarea = ta
	m.viewport = vp
	m.spinner = sp

	return tea.Batch(textarea.Blink, m.spinner.Tick)
}

func (m *ChatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		tiCmd tea.Cmd
		vpCmd tea.Cmd
		spCmd tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			if msg.Alt {
				m.textarea.InsertString("\n")
				return m, nil
			}
			userInput := m.textarea.Value()
			if userInput == "" {
				return m, nil
			}

			now := time.Now()
			m.Messages = append(m.Messages, Message{
				Content:   userInput,
				Timestamp: &now,
				IsUser:    true,
			})

			// Add empty message that will be updated by spinner
			m.Messages = append(m.Messages, Message{})
			m.updateViewportContent()
			m.textarea.Reset()
			m.viewport.GotoBottom()

			// Start loading
			m.Loading = true
			return m, tea.Batch(
				m.spinner.Tick,
				func() tea.Msg {
					response, err := m.SendMessage(context.Background(), m.Workspace, m.Environment, m.ResType, m.ResName, userInput)
					if err != nil {
						return errMsg{err}
					}
					return responseMsg{response}
				},
			)
		}
	case responseMsg:
		m.Loading = false
		// Remove the loader message
		m.Messages = m.Messages[:len(m.Messages)-1]

		now := time.Now()
		formattedContent := FormatMarkdownImage(msg.content)
		m.Messages = append(m.Messages, Message{
			Content:   formattedContent,
			Timestamp: &now,
			IsUser:    false,
		})
		m.updateViewportContent()
		m.viewport.GotoBottom()
	case errMsg:
		m.Loading = false
		now := time.Now()
		// Remove the loader message
		m.Messages = m.Messages[:len(m.Messages)-1]

		// Display error in red
		m.Messages = append(m.Messages, Message{
			Content:   "Error: " + msg.err.Error(),
			Timestamp: &now,
			IsUser:    false,
		})
		m.updateViewportContent()
		m.viewport.GotoBottom()
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width - 2
		m.viewport.Height = msg.Height - 6
		m.textarea.SetWidth(msg.Width - 2)
	case spinner.TickMsg:
		if m.Loading {
			m.spinner, spCmd = m.spinner.Update(msg)
			// Style spinner with dark orange color
			m.spinner.Style = m.getSpinnerStyle()
			m.Messages[len(m.Messages)-1] = Message{
				Content:   m.spinner.View(),
				Timestamp: nil,
				IsUser:    false,
			}
			m.updateViewportContent()
			return m, spCmd
		}
	}

	m.textarea, tiCmd = m.textarea.Update(msg)
	m.viewport, vpCmd = m.viewport.Update(msg)

	return m, tea.Batch(tiCmd, vpCmd, spCmd)
}

func (m *ChatModel) View() string {
	s := "\n" + m.viewport.View()

	// Only show textarea if not loading
	if !m.Loading {
		s += "\n\n" + m.textarea.View()
	}

	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("130")). // Changed from 202 to match
		Padding(0)

	return style.Render(s)
}
