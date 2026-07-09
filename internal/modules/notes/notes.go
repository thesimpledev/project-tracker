package notes

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thesimpledev/project-tracker/internal/config"
	"github.com/thesimpledev/project-tracker/internal/modules"
)

func init() {
	modules.Register("notes", func() modules.Module {
		return New()
	})
}

type State int

const (
	StateNormal State = iota
	StateFocused
	StateEditing
)

type Module struct {
	repo        config.Repo
	lines       []string
	cursor      int
	state       State
	selected    bool
	focused     bool
	width       int
	height      int
	err         error
	scrollPos   int
	inputBuffer string
	editingLine int
}

type NotesLoadedMsg struct {
	Path  string
	Lines []string
	Error error
}

func New() *Module {
	return &Module{
		lines: []string{},
	}
}

func (m *Module) ID() string {
	return "notes"
}

func (m *Module) Name() string {
	if m.repo.Name != "" {
		return fmt.Sprintf("Notes: %s", m.repo.Name)
	}
	return "Notes"
}

func (m *Module) SetRepo(repo config.Repo) modules.Module {
	m.repo = repo
	return m
}

func (m *Module) Init() tea.Cmd {
	if m.repo.Path == "" {
		return nil
	}
	return m.loadNotes()
}

func (m *Module) loadNotes() tea.Cmd {
	path := m.repo.Path
	return func() tea.Msg {
		notesPath := filepath.Join(path, "NOTES.md")

		// Create if doesn't exist
		if _, err := os.Stat(notesPath); os.IsNotExist(err) {
			if err := createDefaultNotes(notesPath); err != nil {
				return NotesLoadedMsg{Path: path, Error: err}
			}
		}

		content, err := os.ReadFile(notesPath) // #nosec G304 -- repo path validated at config load
		if err != nil {
			return NotesLoadedMsg{Path: path, Error: err}
		}

		lines := strings.Split(string(content), "\n")
		return NotesLoadedMsg{Path: path, Lines: lines}
	}
}

func createDefaultNotes(path string) error {
	content := `# Notes

`
	return os.WriteFile(path, []byte(content), 0600)
}

func (m *Module) saveNotes() error {
	if m.repo.Path == "" {
		return nil
	}

	notesPath := filepath.Join(m.repo.Path, "NOTES.md")
	content := strings.Join(m.lines, "\n")
	return os.WriteFile(notesPath, []byte(content), 0600)
}

// persist saves the notes and surfaces any failure in the module's error
// display instead of silently losing data.
func (m *Module) persist() {
	if err := m.saveNotes(); err != nil {
		m.err = err
	}
}

func (m *Module) Update(msg tea.Msg) (modules.Module, tea.Cmd) {
	switch msg := msg.(type) {
	case NotesLoadedMsg:
		if msg.Path == m.repo.Path {
			m.lines = msg.Lines
			m.err = msg.Error
		}
		return m, nil

	case tea.KeyMsg:
		// Handle editing state
		if m.state == StateEditing {
			switch msg.String() {
			case "esc":
				// Save current edit and exit editing
				if m.editingLine < len(m.lines) {
					m.lines[m.editingLine] = m.inputBuffer
				}
				m.persist()
				m.state = StateFocused
				m.inputBuffer = ""
			case "enter":
				// Save current line and move to next/create new
				if m.editingLine < len(m.lines) {
					m.lines[m.editingLine] = m.inputBuffer
				}
				// Insert new line after current
				m.editingLine++
				newLines := make([]string, len(m.lines)+1)
				copy(newLines[:m.editingLine], m.lines[:m.editingLine])
				newLines[m.editingLine] = ""
				copy(newLines[m.editingLine+1:], m.lines[m.editingLine:])
				m.lines = newLines
				m.inputBuffer = ""
				m.cursor = m.editingLine
				m.ensureCursorVisible()
				m.persist()
			case "backspace":
				if len(m.inputBuffer) > 0 {
					m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
				} else if m.editingLine > 0 {
					// Merge with previous line
					prevLine := m.lines[m.editingLine-1]
					m.lines = append(m.lines[:m.editingLine], m.lines[m.editingLine+1:]...)
					m.editingLine--
					m.cursor = m.editingLine
					m.inputBuffer = prevLine
					m.ensureCursorVisible()
				}
			default:
				if len(msg.String()) == 1 {
					m.inputBuffer += msg.String()
				}
			}
			return m, nil
		}

		if m.state != StateFocused {
			return m, nil
		}

		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.lines)-1 {
				m.cursor++
				m.ensureCursorVisible()
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.ensureCursorVisible()
			}
		case "i", "enter": // Edit current line
			if m.cursor < len(m.lines) {
				m.state = StateEditing
				m.editingLine = m.cursor
				m.inputBuffer = m.lines[m.cursor]
			}
		case "a": // Append new line
			newLine := ""
			insertPos := m.cursor + 1
			if insertPos > len(m.lines) {
				insertPos = len(m.lines)
			}
			m.lines = append(m.lines[:insertPos], append([]string{newLine}, m.lines[insertPos:]...)...)
			m.cursor = insertPos
			m.state = StateEditing
			m.editingLine = insertPos
			m.inputBuffer = ""
			m.ensureCursorVisible()
		case "o": // Open new line below
			insertPos := m.cursor + 1
			m.lines = append(m.lines[:insertPos], append([]string{""}, m.lines[insertPos:]...)...)
			m.cursor = insertPos
			m.state = StateEditing
			m.editingLine = insertPos
			m.inputBuffer = ""
			m.ensureCursorVisible()
		case "O": // Open new line above
			insertPos := m.cursor
			m.lines = append(m.lines[:insertPos], append([]string{""}, m.lines[insertPos:]...)...)
			m.state = StateEditing
			m.editingLine = insertPos
			m.inputBuffer = ""
			m.ensureCursorVisible()
		case "d", "x": // Delete line
			if len(m.lines) > 1 && m.cursor < len(m.lines) {
				m.lines = append(m.lines[:m.cursor], m.lines[m.cursor+1:]...)
				if m.cursor >= len(m.lines) {
					m.cursor = len(m.lines) - 1
				}
				m.persist()
			}
		}
		return m, nil
	}

	return m, nil
}

func (m *Module) ensureCursorVisible() {
	visibleCount := m.visibleCount()
	if m.cursor < m.scrollPos {
		m.scrollPos = m.cursor
	} else if m.cursor >= m.scrollPos+visibleCount {
		m.scrollPos = m.cursor - visibleCount + 1
	}
}

func (m *Module) visibleCount() int {
	available := m.height - 4
	if available < 1 {
		return 1
	}
	return available
}

func (m *Module) View() string {
	var borderColor lipgloss.Color
	var borderStyle lipgloss.Border

	switch m.state {
	case StateFocused, StateEditing:
		borderColor = lipgloss.Color("62")
		borderStyle = lipgloss.ThickBorder()
	default:
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
	b.WriteString(headerStyle.Render("Notes") + "\n")

	// Divider
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	dividerWidth := width - 4
	if dividerWidth < 1 {
		dividerWidth = 1
	}
	b.WriteString(dividerStyle.Render(strings.Repeat("─", dividerWidth)) + "\n")

	// Error state
	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		b.WriteString(errStyle.Render("Error: "+m.err.Error()) + "\n")
		return b.String()
	}

	// Empty state
	if len(m.lines) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		b.WriteString(dimStyle.Render("(press 'a' to add notes)") + "\n")
		return b.String()
	}

	// Render lines
	visibleCount := m.visibleCount()
	endIdx := m.scrollPos + visibleCount
	if endIdx > len(m.lines) {
		endIdx = len(m.lines)
	}

	for i := m.scrollPos; i < endIdx; i++ {
		line := m.renderLine(i)
		b.WriteString(line + "\n")
	}

	// Scroll indicator
	if len(m.lines) > visibleCount {
		indicator := fmt.Sprintf("(%d/%d)", m.cursor+1, len(m.lines))
		indStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		b.WriteString(indStyle.Render(indicator))
	}

	return b.String()
}

func (m *Module) renderLine(idx int) string {
	isCurrentLine := idx == m.cursor && (m.state == StateFocused || m.state == StateEditing)
	isEditing := m.state == StateEditing && idx == m.editingLine

	var text string
	if isEditing {
		text = m.inputBuffer + "█"
	} else if idx < len(m.lines) {
		text = m.lines[idx]
	}

	// Truncate if needed
	maxLen := m.width - 6
	if maxLen < 10 {
		maxLen = 10
	}
	if len(text) > maxLen {
		text = text[:maxLen-3] + "..."
	}

	prefix := "  "
	if isCurrentLine {
		prefix = "> "
	}

	line := prefix + text

	if isCurrentLine && !isEditing {
		return lipgloss.NewStyle().Bold(true).Reverse(true).Render(line)
	}
	if isEditing {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(line)
	}
	return line
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
	if focused {
		m.state = StateFocused
	} else {
		// Save when unfocusing
		if m.state == StateEditing {
			if m.editingLine < len(m.lines) {
				m.lines[m.editingLine] = m.inputBuffer
			}
			m.persist()
		}
		m.state = StateNormal
		m.cursor = 0
		m.scrollPos = 0
		m.inputBuffer = ""
	}
	return m
}

func (m *Module) IsFocused() bool {
	return m.focused
}

// GetCopyContent returns the notes content for clipboard copying
func (m *Module) GetCopyContent() string {
	return strings.Join(m.lines, "\n")
}
