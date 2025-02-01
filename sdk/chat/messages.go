package chat

import (
	"time"

	"github.com/charmbracelet/lipgloss"
)

// Add new Message type
type Message struct {
	Content   string
	Timestamp *time.Time
	IsUser    bool
}

func (m *ChatModel) getTimestampStyle(isUser bool, content string) lipgloss.Style {
	len := len(content)
	leftMargin := m.viewport.Width - len - 3
	if isUser {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			MarginRight(1).
			MarginLeft(leftMargin).
			PaddingLeft(1).
			PaddingRight(1).
			Width(m.viewport.Width / 4 * 3).
			SetString("\n")
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		MarginRight(1).
		MarginLeft(1).
		PaddingRight(1).
		Width(m.viewport.Width / 4 * 3).
		SetString("\n").
		BorderForeground(lipgloss.Color("240"))
}

func (m *ChatModel) getMessageStyle(isUser bool, content string) lipgloss.Style {
	len := len(content)
	leftMargin := m.viewport.Width - len - 3
	if isUser {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("255")).
			MarginRight(1).
			MarginLeft(leftMargin).
			PaddingLeft(1).
			PaddingRight(1).
			Width(m.viewport.Width / 4 * 3).
			SetString("\n").
			MarginTop(1)
	}

	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("130")).
		MarginRight(1).
		MarginLeft(1).
		PaddingRight(1).
		Width(m.viewport.Width / 4 * 3).
		SetString("\n").
		MarginTop(1).
		BorderForeground(lipgloss.Color("130"))
}

func (m *ChatModel) renderMessages() []string {
	renderedMessages := []string{}
	for _, msg := range m.Messages {
		// Render message content
		style := m.getMessageStyle(msg.IsUser, msg.Content)
		rendered := style.Render(msg.Content)

		toRender := rendered
		// Render timestamp
		if msg.Timestamp != nil {
			timestampStr := msg.Timestamp.Format("15:04:05")
			timestamp := m.getTimestampStyle(msg.IsUser, timestampStr).Render(timestampStr)
			toRender = rendered + timestamp
		}

		// Combine timestamp and message
		renderedMessages = append(renderedMessages, toRender)
	}
	return renderedMessages
}
