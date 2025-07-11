package chat

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

type streamState struct {
	active  bool
	content strings.Builder
	mu      sync.Mutex
}

type ChatModel struct {
	textarea textarea.Model
	viewport viewport.Model
	spinner  spinner.Model

	Messages              []Message
	Err                   error
	Workspace             string
	ResType               string
	ResName               string
	Loading               bool
	Debug                 bool
	Local                 bool
	Headers               []string
	SendMessage           func(ctx context.Context, workspace string, resType string, resName string, message string, debug bool, local bool, headers []string) (string, error)
	SendMessageStream     func(ctx context.Context, workspace string, resType string, resName string, message string, debug bool, local bool, headers []string, onChunk func(string)) error
	lastUserMessage       string
	textareaFocused       bool
	streamingMessageIndex int
	streamState           *streamState
}

type responseMsg struct {
	content string
}

//nolint:unused
type streamChunkMsg struct {
	chunk string
}

type streamCompleteMsg struct{}

type streamTickMsg struct{}

type errMsg struct {
	err              error
	isStreamingError bool
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
	m.textareaFocused = true // Start with textarea focused
	m.streamState = &streamState{}

	// Only start blinking if textarea is focused
	var blinkCmd tea.Cmd
	if m.textareaFocused {
		blinkCmd = textarea.Blink
	}

	return tea.Batch(blinkCmd, m.spinner.Tick)
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
		case tea.KeyUp:
			if m.lastUserMessage != "" {
				m.textarea.SetValue(m.lastUserMessage)
			}
			return m, nil
		case tea.KeyEnter:
			if msg.Alt {
				m.textarea.InsertString("\n")
				return m, nil
			}
			userInput := m.textarea.Value()
			if userInput == "" {
				return m, nil
			}

			// Store the last user message
			m.lastUserMessage = userInput

			now := time.Now()
			m.Messages = append(m.Messages, Message{
				Content:   userInput,
				Timestamp: &now,
				IsUser:    true,
			})

			// Add empty message for streaming response
			m.Messages = append(m.Messages, Message{
				Content:   "",
				Timestamp: nil,
				IsUser:    false,
			})
			m.streamingMessageIndex = len(m.Messages) - 1
			m.streamState = &streamState{active: true}
			m.updateViewportContent()
			m.textarea.Reset()
			m.viewport.GotoBottom()

			// Start loading
			m.Loading = true
			m.textareaFocused = false // Lose focus while loading

			// Use streaming if available, otherwise fall back to regular SendMessage
			if m.SendMessageStream != nil {
				return m, tea.Batch(
					m.spinner.Tick,
					m.startStreamingCommand(userInput),
					m.streamTickCommand(),
				)
			} else {
				return m, tea.Batch(
					m.spinner.Tick,
					func() tea.Msg {
						response, err := m.SendMessage(context.Background(), m.Workspace, m.ResType, m.ResName, userInput, m.Debug, m.Local, m.Headers)
						if err != nil {
							return errMsg{err, false}
						}
						return responseMsg{response}
					},
				)
			}
		default:
			// Handle other key events to manage textarea focus
			if !m.Loading {
				m.textareaFocused = true
			}
		}
	case streamTickMsg:
		if m.streamState != nil && m.streamState.active {
			m.streamState.mu.Lock()
			content := m.streamState.content.String()
			m.streamState.mu.Unlock()

			if m.streamingMessageIndex < len(m.Messages) {
				m.Messages[m.streamingMessageIndex].Content = content
				m.updateViewportContent()
				m.viewport.GotoBottom()
			}

			// Continue ticking while streaming is active
			return m, tea.Batch(m.streamTickCommand())
		}
		return m, nil
	case streamCompleteMsg:
		m.Loading = false
		m.textareaFocused = true // Regain focus after streaming

		if m.streamState != nil {
			m.streamState.mu.Lock()
			m.streamState.active = false
			finalContent := m.streamState.content.String()
			m.streamState.mu.Unlock()

			// Set timestamp for completed message
			now := time.Now()
			if m.streamingMessageIndex < len(m.Messages) {
				// Only set content if we actually received some, otherwise show a default message
				if finalContent != "" {
					formattedContent := FormatMarkdown(finalContent)
					m.Messages[m.streamingMessageIndex].Content = formattedContent
				} else {
					// Fallback for empty streaming response
					m.Messages[m.streamingMessageIndex].Content = "No response received"
				}
				m.Messages[m.streamingMessageIndex].Timestamp = &now
			}
			m.updateViewportContent()
			m.viewport.GotoBottom()
		}

		// Resume blinking when focused
		return m, textarea.Blink
	case responseMsg:
		m.Loading = false
		m.textareaFocused = true // Regain focus

		// Remove the loader message
		m.Messages = m.Messages[:len(m.Messages)-1]

		now := time.Now()
		formattedContent := FormatMarkdown(msg.content)
		m.Messages = append(m.Messages, Message{
			Content:   formattedContent,
			Timestamp: &now,
			IsUser:    false,
		})
		m.updateViewportContent()
		m.viewport.GotoBottom()

		// Resume blinking when focused
		return m, textarea.Blink
	case errMsg:
		m.Loading = false
		m.textareaFocused = true // Regain focus

		now := time.Now()

		// Handle streaming errors differently - preserve partial content
		if msg.isStreamingError && m.streamState != nil {
			// Stop streaming and get partial content
			m.streamState.mu.Lock()
			m.streamState.active = false
			partialContent := m.streamState.content.String()
			m.streamState.mu.Unlock()

			// If we have partial content, append error to it
			if partialContent != "" && m.streamingMessageIndex < len(m.Messages) {
				errorContent := partialContent + "\n\n**Error:** " + msg.err.Error()
				formattedContent := FormatMarkdown(errorContent)
				m.Messages[m.streamingMessageIndex].Content = formattedContent
				m.Messages[m.streamingMessageIndex].Timestamp = &now
			} else {
				// No partial content, replace with error message
				if m.streamingMessageIndex < len(m.Messages) {
					m.Messages[m.streamingMessageIndex].Content = "Error: " + msg.err.Error()
					m.Messages[m.streamingMessageIndex].Timestamp = &now
				}
			}
		} else {
			// Handle non-streaming errors (original behavior)
			// Remove the loader message
			m.Messages = m.Messages[:len(m.Messages)-1]

			// Display error in red
			m.Messages = append(m.Messages, Message{
				Content:   "Error: " + msg.err.Error(),
				Timestamp: &now,
				IsUser:    false,
			})
		}

		m.updateViewportContent()
		m.viewport.GotoBottom()

		// Resume blinking when focused
		return m, textarea.Blink
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width - 2
		m.viewport.Height = msg.Height - 6
		m.textarea.SetWidth(msg.Width - 2)
	case spinner.TickMsg:
		if m.Loading {
			m.spinner, spCmd = m.spinner.Update(msg)
			// Style spinner with dark orange color
			m.spinner.Style = m.getSpinnerStyle()
			return m, spCmd
		}
	}

	// Only update textarea if focused and not loading
	if m.textareaFocused && !m.Loading {
		m.textarea, tiCmd = m.textarea.Update(msg)
	}
	m.viewport, vpCmd = m.viewport.Update(msg)

	return m, tea.Batch(tiCmd, vpCmd, spCmd)
}

// streamTickCommand creates a command that periodically updates the streaming content
func (m *ChatModel) streamTickCommand() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(t time.Time) tea.Msg {
		return streamTickMsg{}
	})
}

// startStreamingCommand creates a command that will handle streaming responses
func (m *ChatModel) startStreamingCommand(userInput string) tea.Cmd {
	return func() tea.Msg {
		if m.SendMessageStream == nil {
			// Fall back to regular message if streaming not available
			response, err := m.SendMessage(context.Background(), m.Workspace, m.ResType, m.ResName, userInput, m.Debug, m.Local, m.Headers)
			if err != nil {
				return errMsg{err, false}
			}
			return responseMsg{response}
		}

		// Track if we received any streaming content
		receivedContent := false

		err := m.SendMessageStream(context.Background(), m.Workspace, m.ResType, m.ResName, userInput, m.Debug, m.Local, m.Headers, func(chunk string) {
			if m.streamState != nil {
				m.streamState.mu.Lock()
				m.streamState.content.WriteString(chunk)
				receivedContent = true
				m.streamState.mu.Unlock()
			}
		})

		if err != nil {
			return errMsg{err, true}
		}

		// If no streaming content was received, try regular message as fallback
		if !receivedContent {
			response, err := m.SendMessage(context.Background(), m.Workspace, m.ResType, m.ResName, userInput, m.Debug, m.Local, m.Headers)
			if err != nil {
				return errMsg{err, false}
			}
			return responseMsg{response}
		}

		return streamCompleteMsg{}
	}
}

func (m *ChatModel) View() string {
	s := "\n" + m.viewport.View()

	// Slow down the scroll speed by setting mouse wheel delta to 1 line per scroll
	m.viewport.MouseWheelEnabled = true
	m.viewport.MouseWheelDelta = 1

	// Show textarea with spinner when loading, normal textarea when not loading
	if m.Loading {
		// Create a disabled textarea appearance with spinner
		spinnerView := m.spinner.View()
		s += "\n\n" + spinnerView
	} else {
		s += "\n\n" + m.textarea.View()
	}

	style := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("130")). // Changed from 202 to match
		Padding(0)

	return style.Render(s)
}
