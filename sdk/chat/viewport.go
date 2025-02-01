package chat

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

func (m *ChatModel) createWelcomeMessage(width int) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("130")).
		Bold(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(lipgloss.Color("130")).
		Width(width).
		Align(lipgloss.Center).
		PaddingBottom(1)

	return headerStyle.Render(fmt.Sprintf("Welcome to the chat with the %s named %s! Press Ctrl+C to quit.", m.ResType, m.ResName))
}

func (m *ChatModel) initializeViewport(width, height int) viewport.Model {
	welcomeMsg := m.createWelcomeMessage(width)
	vp := viewport.New(width, height)
	vp.SetContent(welcomeMsg)
	vp.YPosition = 0
	return vp
}

// Add helper method to update viewport content
func (m *ChatModel) updateViewportContent() {
	renderedMessages := m.renderMessages()
	welcomeMsg := m.createWelcomeMessage(m.viewport.Width)
	m.viewport.SetContent(welcomeMsg + "\n" + strings.Join(renderedMessages, "\n"))
}
