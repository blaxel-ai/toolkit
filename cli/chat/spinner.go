package chat

import (
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

func (m *ChatModel) getSpinnerStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("99")).
		PaddingLeft(1).
		MarginTop(1)
}

func (m *ChatModel) initializeSpinner() spinner.Model {
	sp := spinner.New()
	sp.Spinner = spinner.Spinner{
		Frames: []string{"🌑", "🌒", "🌓", "🌔", "🌕", "🌖", "🌗", "🌘"},
		FPS:    100 * time.Millisecond,
	}
	sp.Style = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		PaddingLeft(2)
	return sp
}
