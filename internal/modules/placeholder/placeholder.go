package placeholder

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/thesimpledev/project-tracker/internal/modules"
)

func init() {
	modules.Register("placeholder", func() modules.Module {
		return New()
	})
}

type Module struct {
	id       string
	name     string
	width    int
	height   int
	selected bool
	focused  bool
}

func New() *Module {
	return &Module{
		id:   "placeholder",
		name: "Empty Slot",
	}
}

func (m *Module) ID() string {
	return m.id
}

func (m *Module) Name() string {
	return m.name
}

func (m *Module) Init() tea.Cmd {
	return nil
}

func (m *Module) Update(msg tea.Msg) (modules.Module, tea.Cmd) {
	return m, nil
}

func (m *Module) View() string {
	var borderColor lipgloss.Color
	if m.focused {
		borderColor = lipgloss.Color("62") // Purple for focused
	} else if m.selected {
		borderColor = lipgloss.Color("212") // Pink for selected
	} else {
		borderColor = lipgloss.Color("238") // Gray for normal
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(lipgloss.Color("241"))

	content := "Empty Module\n\nFuture expansion slot"
	return style.Render(content)
}

func (m *Module) SetSize(width, height int) modules.Module {
	m.width = width - 2
	m.height = height - 2
	return m
}

func (m *Module) SetSelected(selected bool) modules.Module {
	m.selected = selected
	return m
}

func (m *Module) SetFocused(focused bool) modules.Module {
	m.focused = focused
	return m
}

func (m *Module) IsFocused() bool {
	return m.focused
}

// GetCopyContent returns empty string for placeholder
func (m *Module) GetCopyContent() string {
	return ""
}
