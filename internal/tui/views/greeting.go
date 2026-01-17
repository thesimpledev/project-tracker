package views

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thesimpledev/project-tracker/internal/config"
	"github.com/thesimpledev/project-tracker/internal/tui/components"
)

const logo = `
 ____            _           _     _____               _
|  _ \ _ __ ___ (_) ___  ___| |_  |_   _| __ __ _  ___| | _____ _ __
| |_) | '__/ _ \| |/ _ \/ __| __|   | || '__/ _` + "`" + ` |/ __| |/ / _ \ '__|
|  __/| | | (_) | |  __/ (__| |_    | || | | (_| | (__|   <  __/ |
|_|   |_|  \___// |\___|\___|\__|   |_||_|  \__,_|\___|_|\_\___|_|
              |__/
`

type GreetingModel struct {
	CommandInput components.CommandInput
	commandMode  bool
	width        int
	height       int
}

func NewGreeting(cfg *config.Config) GreetingModel {
	return GreetingModel{
		CommandInput: components.NewCommandInput(),
	}
}

func (m GreetingModel) Init() tea.Cmd {
	return nil
}

func (m GreetingModel) Update(msg tea.Msg) (GreetingModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.CommandInput = m.CommandInput.SetSize(msg.Width)

	case tea.KeyMsg:
		if !m.commandMode {
			switch msg.String() {
			case ":":
				m.commandMode = true
				m.CommandInput = m.CommandInput.SetFocused(true)
				return m, nil
			}
		} else {
			if msg.String() == "esc" {
				m.commandMode = false
				m.CommandInput = m.CommandInput.SetFocused(false)
				return m, nil
			}
		}
	}

	if m.commandMode {
		var cmd tea.Cmd
		m.CommandInput, cmd = m.CommandInput.Update(msg)
		cmds = append(cmds, cmd)
		if !m.CommandInput.Focused {
			m.commandMode = false
		}
	}

	return m, tea.Batch(cmds...)
}

func (m GreetingModel) SetSize(width, height int) GreetingModel {
	m.width = width
	m.height = height
	m.CommandInput = m.CommandInput.SetSize(width)
	return m
}

func (m GreetingModel) View() string {
	// Logo styling
	logoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("62")).
		Bold(true)

	// Title
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("247")).
		MarginTop(1)

	// Keybinds section
	keybindHeaderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("62")).
		Bold(true).
		MarginTop(2)

	keybindStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("247"))

	// Build content (logo + help)
	var content strings.Builder
	content.WriteString(logoStyle.Render(logo))
	content.WriteString("\n")
	content.WriteString(titleStyle.Render("A modular project status dashboard"))
	content.WriteString("\n\n")

	content.WriteString(keybindHeaderStyle.Render("Quick Start"))
	content.WriteString("\n\n")

	keybinds := []struct {
		key  string
		desc string
	}{
		{":add <path>", "Add a project directory"},
		{":remove <repo>", "Remove a project"},
		{":save <name>", "Save current layout as profile"},
		{":load <profile>", "Load a saved profile"},
		{":q", "Quit"},
	}

	for _, kb := range keybinds {
		keyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Width(20)
		content.WriteString("  ")
		content.WriteString(keyStyle.Render(kb.key))
		content.WriteString(keybindStyle.Render(kb.desc))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(keybindHeaderStyle.Render("Navigation"))
	content.WriteString("\n\n")

	navKeys := []struct {
		key  string
		desc string
	}{
		{"h j k l", "Move between modules"},
		{"Enter", "Focus selected module"},
		{"Esc", "Unfocus / cancel command"},
		{":", "Enter command mode"},
	}

	for _, kb := range navKeys {
		keyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Width(20)
		content.WriteString("  ")
		content.WriteString(keyStyle.Render(kb.key))
		content.WriteString(keybindStyle.Render(kb.desc))
		content.WriteString("\n")
	}

	// Get the command view to calculate its height
	cmdView := m.CommandInput.View()
	cmdLines := strings.Count(cmdView, "\n") + 1

	// Content goes at top, centered
	// Then flexible padding pushes command area to very bottom
	contentHeight := m.height - cmdLines - 1

	contentStyle := lipgloss.NewStyle().
		Width(m.width).
		Height(contentHeight).
		Align(lipgloss.Center, lipgloss.Center)

	// Build: content (with padding to push to center), then command at bottom
	var view strings.Builder
	view.WriteString(contentStyle.Render(content.String()))
	view.WriteString("\n")
	view.WriteString(cmdView)

	return view.String()
}
