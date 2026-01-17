package just_commands

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thesimpledev/project-tracker/internal/config"
	"github.com/thesimpledev/project-tracker/internal/modules"
)

func init() {
	modules.Register("just_commands", func() modules.Module {
		return New()
	})
}

type State int

const (
	StateNormal State = iota
	StateFocused
	StateRunning
	StateResult
)

type JustCommand struct {
	Name    string
	Comment string
}

type Module struct {
	repo       config.Repo
	commands   []JustCommand
	cursor     int
	scrollPos  int
	state      State
	selected   bool
	focused    bool
	width      int
	height     int
	err        error
	output     string
	outputErr  error
	duration   time.Duration
	lastCmd    string
	viewScroll int
}

type CommandsLoadedMsg struct {
	Path     string
	Commands []JustCommand
	Error    error
}

type CommandFinishedMsg struct {
	Path     string
	Command  string
	Output   string
	Duration time.Duration
	Error    error
}

func New() *Module {
	return &Module{
		commands: []JustCommand{},
	}
}

func (m *Module) ID() string {
	return "just_commands"
}

func (m *Module) Name() string {
	return "Just Commands"
}

func (m *Module) SetRepo(repo config.Repo) modules.Module {
	m.repo = repo
	return m
}

func (m *Module) Init() tea.Cmd {
	if m.repo.Path == "" {
		return nil
	}
	return m.loadCommands()
}

func (m *Module) loadCommands() tea.Cmd {
	path := m.repo.Path
	return func() tea.Msg {
		justfilePath := filepath.Join(path, "justfile")

		file, err := os.Open(justfilePath)
		if err != nil {
			return CommandsLoadedMsg{Path: path, Error: err}
		}
		defer file.Close()

		var commands []JustCommand
		var lastComment string

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()

			// Capture comments (lines starting with #)
			if strings.HasPrefix(line, "#") {
				lastComment = strings.TrimSpace(strings.TrimPrefix(line, "#"))
				continue
			}

			// Check for command definition (name followed by colon, not indented)
			if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") && strings.Contains(line, ":") {
				// Extract command name (everything before the colon)
				parts := strings.SplitN(line, ":", 2)
				name := strings.TrimSpace(parts[0])

				// Skip if it looks like a variable assignment or empty
				if name == "" || strings.Contains(name, "=") || strings.Contains(name, " ") {
					lastComment = ""
					continue
				}

				commands = append(commands, JustCommand{
					Name:    name,
					Comment: lastComment,
				})
				lastComment = ""
			} else if line != "" && !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
				// Non-command line, reset comment
				lastComment = ""
			}
		}

		return CommandsLoadedMsg{Path: path, Commands: commands, Error: scanner.Err()}
	}
}

func (m *Module) runCommand(name string) tea.Cmd {
	path := m.repo.Path
	return func() tea.Msg {
		start := time.Now()

		cmd := exec.Command("just", name)
		cmd.Dir = path

		output, err := cmd.CombinedOutput()
		duration := time.Since(start)

		return CommandFinishedMsg{
			Path:     path,
			Command:  name,
			Output:   string(output),
			Duration: duration,
			Error:    err,
		}
	}
}

func (m *Module) Update(msg tea.Msg) (modules.Module, tea.Cmd) {
	switch msg := msg.(type) {
	case CommandsLoadedMsg:
		if msg.Path == m.repo.Path {
			m.commands = msg.Commands
			m.err = msg.Error
		}
		return m, nil

	case CommandFinishedMsg:
		if msg.Path == m.repo.Path {
			m.state = StateResult
			m.output = msg.Output
			m.outputErr = msg.Error
			m.duration = msg.Duration
			m.lastCmd = msg.Command
			m.viewScroll = 0
		}
		return m, nil

	case tea.KeyMsg:
		// Handle result view
		if m.state == StateResult {
			switch msg.String() {
			case "esc":
				m.state = StateFocused
				m.output = ""
				m.viewScroll = 0
			case "j", "down":
				m.viewScroll++
			case "k", "up":
				if m.viewScroll > 0 {
					m.viewScroll--
				}
			case "enter":
				// Re-run the same command
				if m.lastCmd != "" {
					m.state = StateRunning
					return m, m.runCommand(m.lastCmd)
				}
			}
			return m, nil
		}

		if !m.focused {
			return m, nil
		}

		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.commands)-1 {
				m.cursor++
				m.ensureCursorVisible()
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.ensureCursorVisible()
			}
		case "enter":
			if m.cursor < len(m.commands) && m.state != StateRunning {
				m.state = StateRunning
				return m, m.runCommand(m.commands[m.cursor].Name)
			}
		}
		return m, nil
	}

	return m, nil
}

func (m *Module) ensureCursorVisible() {
	visibleCount := m.visibleItemCount()
	if m.cursor < m.scrollPos {
		m.scrollPos = m.cursor
	} else if m.cursor >= m.scrollPos+visibleCount {
		m.scrollPos = m.cursor - visibleCount + 1
	}
}

func (m *Module) visibleItemCount() int {
	available := m.height - 6
	if available < 1 {
		return 1
	}
	return available
}

func (m *Module) View() string {
	var borderColor lipgloss.Color
	var borderStyle lipgloss.Border

	if m.focused || m.state == StateResult {
		borderColor = lipgloss.Color("62")
		borderStyle = lipgloss.ThickBorder()
	} else {
		if m.selected {
			borderColor = lipgloss.Color("212")
		} else {
			borderColor = lipgloss.Color("238")
		}
		borderStyle = lipgloss.RoundedBorder()
	}

	style := lipgloss.NewStyle().
		Border(borderStyle).
		BorderForeground(borderColor).
		Width(m.width-2).
		Height(m.height-2).
		Padding(0, 1)

	return style.Render(m.renderContent())
}

func (m *Module) renderContent() string {
	var b strings.Builder

	width := m.width
	if width < 20 {
		width = 20
	}

	// Header
	headerStyle := lipgloss.NewStyle().Bold(true)
	b.WriteString(headerStyle.Render("Just Commands") + "\n")

	// Divider
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	dividerWidth := width - 4
	if dividerWidth < 1 {
		dividerWidth = 1
	}
	b.WriteString(dividerStyle.Render(strings.Repeat("─", dividerWidth)) + "\n")

	switch m.state {
	case StateRunning:
		runningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		b.WriteString(runningStyle.Render("⟳ Running: just "+m.commands[m.cursor].Name) + "\n")

	case StateResult:
		m.renderResultView(&b, dividerWidth)

	default:
		m.renderCommandList(&b, dividerWidth)
	}

	return b.String()
}

func (m *Module) renderCommandList(b *strings.Builder, dividerWidth int) {
	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		b.WriteString(errStyle.Render("Error: "+m.err.Error()) + "\n")
		return
	}

	if len(m.commands) == 0 {
		b.WriteString(dimStyle.Render("No justfile found") + "\n")
		return
	}

	visibleCount := m.visibleItemCount()
	endIdx := m.scrollPos + visibleCount
	if endIdx > len(m.commands) {
		endIdx = len(m.commands)
	}

	for i := m.scrollPos; i < endIdx; i++ {
		cmd := m.commands[i]
		prefix := "  "
		if m.focused && i == m.cursor {
			prefix = "> "
		}

		nameStyle := lipgloss.NewStyle()
		if m.focused && i == m.cursor {
			nameStyle = nameStyle.Bold(true).Reverse(true)
		}

		line := prefix + cmd.Name
		if cmd.Comment != "" {
			commentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
			maxNameLen := 15
			if len(cmd.Name) < maxNameLen {
				line += strings.Repeat(" ", maxNameLen-len(cmd.Name))
			}
			// Truncate comment if needed
			comment := cmd.Comment
			maxCommentLen := m.width - maxNameLen - 8
			if maxCommentLen > 0 && len(comment) > maxCommentLen {
				comment = comment[:maxCommentLen-3] + "..."
			}
			if maxCommentLen > 0 {
				line += " " + commentStyle.Render(comment)
			}
		}

		if m.focused && i == m.cursor {
			b.WriteString(nameStyle.Render(line) + "\n")
		} else {
			b.WriteString(line + "\n")
		}
	}

	// Scroll indicator
	if len(m.commands) > visibleCount {
		indicator := fmt.Sprintf("(%d/%d)", m.cursor+1, len(m.commands))
		indStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		b.WriteString(indStyle.Render(indicator) + "\n")
	}

	// Hint
	if m.focused {
		hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		b.WriteString(hintStyle.Render("enter: run  j/k: navigate"))
	}
}

func (m *Module) renderResultView(b *strings.Builder, dividerWidth int) {
	// Command name and status
	var statusStyle lipgloss.Style
	var statusIcon string
	if m.outputErr != nil {
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
		statusIcon = "✗"
	} else {
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
		statusIcon = "✓"
	}
	b.WriteString(statusStyle.Render(statusIcon+" just "+m.lastCmd) + "\n")

	// Duration
	durationStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	b.WriteString(durationStyle.Render(fmt.Sprintf("Duration: %s", m.duration.Round(time.Millisecond))) + "\n")

	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	b.WriteString(dividerStyle.Render(strings.Repeat("─", dividerWidth)) + "\n")

	// Output
	if m.output == "" {
		b.WriteString(durationStyle.Render("(no output)") + "\n")
	} else {
		lines := strings.Split(m.output, "\n")
		visibleLines := m.height - 9
		if visibleLines < 3 {
			visibleLines = 3
		}

		startLine := m.viewScroll
		if startLine > len(lines)-visibleLines {
			startLine = len(lines) - visibleLines
		}
		if startLine < 0 {
			startLine = 0
		}

		endLine := startLine + visibleLines
		if endLine > len(lines) {
			endLine = len(lines)
		}

		outputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("247"))
		for i := startLine; i < endLine; i++ {
			line := lines[i]
			maxLen := m.width - 6
			if maxLen < 20 {
				maxLen = 20
			}
			if len(line) > maxLen {
				line = line[:maxLen-3] + "..."
			}
			b.WriteString(outputStyle.Render(line) + "\n")
		}

		// Scroll indicator
		if len(lines) > visibleLines {
			indicator := fmt.Sprintf("(%d/%d lines)", startLine+1, len(lines))
			indStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
			b.WriteString(indStyle.Render(indicator) + "\n")
		}
	}

	// Hint
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	b.WriteString(hintStyle.Render("esc: back  enter: rerun  j/k: scroll"))
}

func (m *Module) SetSize(width, height int) modules.Module {
	m.width = width
	m.height = height
	return m
}

func (m *Module) SetSelected(selected bool) modules.Module {
	m.selected = selected
	return m
}

func (m *Module) SetFocused(focused bool) modules.Module {
	m.focused = focused
	if focused && m.state == StateNormal {
		m.state = StateFocused
	} else if !focused && m.state == StateFocused {
		m.state = StateNormal
	}
	return m
}

func (m *Module) IsFocused() bool {
	return m.focused
}

// GetCopyContent returns the command output for clipboard copying
func (m *Module) GetCopyContent() string {
	if m.output != "" {
		return m.output
	}
	var b strings.Builder
	for _, cmd := range m.commands {
		if cmd.Comment != "" {
			b.WriteString(fmt.Sprintf("%s - %s\n", cmd.Name, cmd.Comment))
		} else {
			b.WriteString(cmd.Name + "\n")
		}
	}
	return b.String()
}
