package chat

import "github.com/charmbracelet/bubbles/textarea"

func (m *ChatModel) initializeTextarea(width int) textarea.Model {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.Focus()
	ta.ShowLineNumbers = false
	ta.SetHeight(1)
	ta.SetWidth(width)
	return ta
}
