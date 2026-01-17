package todo

import (
	"bufio"
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
	modules.Register("todo", func() modules.Module {
		return New()
	})
}

type TodoItem struct {
	Text    string
	Checked bool
}

type State int

const (
	StateNormal State = iota
	StateFocused
	StateAdding
	StateEditing
)

type Module struct {
	repo        config.Repo
	items       []TodoItem
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

type TodoLoadedMsg struct {
	Path  string
	Items []TodoItem
	Error error
}

func New() *Module {
	return &Module{
		items: []TodoItem{},
	}
}

func (m *Module) ID() string {
	return "todo"
}

func (m *Module) Name() string {
	if m.repo.Name != "" {
		return fmt.Sprintf("TODO: %s/%s", m.repo.Owner, m.repo.Name)
	}
	return "TODO"
}

func (m *Module) SetRepo(repo config.Repo) modules.Module {
	m.repo = repo
	return m
}

func (m *Module) Init() tea.Cmd {
	if m.repo.Path == "" {
		return nil
	}
	return m.loadTodo()
}

func (m *Module) loadTodo() tea.Cmd {
	path := m.repo.Path
	return func() tea.Msg {
		todoPath := filepath.Join(path, "TODO.md")

		// Create if doesn't exist
		if _, err := os.Stat(todoPath); os.IsNotExist(err) {
			if err := createDefaultTodo(todoPath); err != nil {
				return TodoLoadedMsg{Path: path, Error: err}
			}
		}

		items, err := loadTodoFile(todoPath)
		return TodoLoadedMsg{Path: path, Items: items, Error: err}
	}
}

func createDefaultTodo(path string) error {
	content := `# TODO

- [ ] Add your first task here
`
	return os.WriteFile(path, []byte(content), 0644)
}

func loadTodoFile(path string) ([]TodoItem, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var items []TodoItem
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		item, ok := parseTodoLine(line)
		if ok {
			items = append(items, item)
		}
	}

	return items, scanner.Err()
}

func parseTodoLine(line string) (TodoItem, bool) {
	line = strings.TrimSpace(line)

	// Check for unchecked: - [ ] text
	if strings.HasPrefix(line, "- [ ] ") {
		return TodoItem{Text: strings.TrimPrefix(line, "- [ ] "), Checked: false}, true
	}

	// Check for checked: - [x] text or - [X] text
	if strings.HasPrefix(line, "- [x] ") || strings.HasPrefix(line, "- [X] ") {
		text := line[6:] // Skip "- [x] "
		return TodoItem{Text: text, Checked: true}, true
	}

	return TodoItem{}, false
}

func (m *Module) saveTodo() error {
	if m.repo.Path == "" {
		return nil
	}

	todoPath := filepath.Join(m.repo.Path, "TODO.md")

	var b strings.Builder
	b.WriteString("# TODO\n\n")

	// Write unchecked items first
	for _, item := range m.items {
		if !item.Checked {
			b.WriteString(fmt.Sprintf("- [ ] %s\n", item.Text))
		}
	}

	// Write checked items
	for _, item := range m.items {
		if item.Checked {
			b.WriteString(fmt.Sprintf("- [x] %s\n", item.Text))
		}
	}

	return os.WriteFile(todoPath, []byte(b.String()), 0644)
}

func (m *Module) Update(msg tea.Msg) (modules.Module, tea.Cmd) {
	switch msg := msg.(type) {
	case TodoLoadedMsg:
		if msg.Path == m.repo.Path {
			m.items = msg.Items
			m.err = msg.Error
		}
		return m, nil

	case tea.KeyMsg:
		// Handle adding state
		if m.state == StateAdding {
			switch msg.String() {
			case "esc":
				m.state = StateFocused
				m.inputBuffer = ""
			case "enter":
				if m.inputBuffer != "" {
					// Insert new item at cursor position (in unchecked section)
					newItem := TodoItem{Text: m.inputBuffer, Checked: false}
					insertPos := m.cursor
					if insertPos > len(m.items) {
						insertPos = len(m.items)
					}
					m.items = append(m.items[:insertPos], append([]TodoItem{newItem}, m.items[insertPos:]...)...)
					m.cursor = insertPos
					m.saveTodo()
				}
				m.state = StateFocused
				m.inputBuffer = ""
			case "backspace":
				if len(m.inputBuffer) > 0 {
					m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
				}
			default:
				if len(msg.String()) == 1 {
					m.inputBuffer += msg.String()
				}
			}
			return m, nil
		}

		// Handle editing state
		if m.state == StateEditing {
			switch msg.String() {
			case "esc":
				// Save edit and return to focused
				if m.editingLine < len(m.items) {
					m.items[m.editingLine].Text = m.inputBuffer
					m.saveTodo()
				}
				m.state = StateFocused
				m.inputBuffer = ""
			case "enter":
				// Save edit and return to focused
				if m.editingLine < len(m.items) {
					m.items[m.editingLine].Text = m.inputBuffer
					m.saveTodo()
				}
				m.state = StateFocused
				m.inputBuffer = ""
			case "backspace":
				if len(m.inputBuffer) > 0 {
					m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
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
			if m.cursor < len(m.items)-1 {
				m.cursor++
				m.ensureCursorVisible()
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.ensureCursorVisible()
			}
		case " ":
			// Toggle check/uncheck
			if m.cursor < len(m.items) {
				m.toggleItem(m.cursor)
				m.saveTodo()
			}
		case "a": // Add new item
			m.state = StateAdding
			m.inputBuffer = ""
		case "i": // Edit current item
			if m.cursor < len(m.items) {
				m.state = StateEditing
				m.editingLine = m.cursor
				m.inputBuffer = m.items[m.cursor].Text
			}
		case "d", "x": // Delete item
			if m.cursor < len(m.items) {
				m.items = append(m.items[:m.cursor], m.items[m.cursor+1:]...)
				if m.cursor >= len(m.items) && m.cursor > 0 {
					m.cursor--
				}
				m.saveTodo()
			}
		case "J": // Shift+J - move item down
			if m.cursor < len(m.items)-1 {
				m.items[m.cursor], m.items[m.cursor+1] = m.items[m.cursor+1], m.items[m.cursor]
				m.cursor++
				m.ensureCursorVisible()
				m.saveTodo()
			}
		case "K": // Shift+K - move item up
			if m.cursor > 0 {
				m.items[m.cursor], m.items[m.cursor-1] = m.items[m.cursor-1], m.items[m.cursor]
				m.cursor--
				m.ensureCursorVisible()
				m.saveTodo()
			}
		}
		return m, nil
	}

	return m, nil
}

func (m *Module) toggleItem(idx int) {
	if idx >= len(m.items) {
		return
	}

	item := m.items[idx]
	item.Checked = !item.Checked

	// Remove from current position
	m.items = append(m.items[:idx], m.items[idx+1:]...)

	if item.Checked {
		// Move to end (after all items)
		m.items = append(m.items, item)
		m.cursor = len(m.items) - 1
	} else {
		// Move to end of unchecked section
		insertPos := 0
		for i, it := range m.items {
			if it.Checked {
				insertPos = i
				break
			}
			insertPos = i + 1
		}
		// Insert at position
		m.items = append(m.items[:insertPos], append([]TodoItem{item}, m.items[insertPos:]...)...)
		m.cursor = insertPos
	}
	m.ensureCursorVisible()
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
	// Account for header, divider, and padding
	available := m.height - 5
	if available < 1 {
		return 1
	}
	return available
}

func (m *Module) View() string {
	var borderColor lipgloss.Color
	var borderStyle lipgloss.Border

	switch m.state {
	case StateFocused, StateAdding, StateEditing:
		borderColor = lipgloss.Color("62") // Purple for focused
		borderStyle = lipgloss.ThickBorder()
	default:
		if m.selected {
			borderColor = lipgloss.Color("212") // Pink for selected
		} else {
			borderColor = lipgloss.Color("238") // Gray for normal
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
	header := "TODO"
	if m.repo.Name != "" {
		header = fmt.Sprintf("TODO: %s", m.repo.Name)
	}
	headerStyle := lipgloss.NewStyle().Bold(true)
	b.WriteString(headerStyle.Render(header) + "\n")

	// Divider
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	dividerWidth := width - 4
	if dividerWidth < 1 {
		dividerWidth = 1
	}
	b.WriteString(dividerStyle.Render(strings.Repeat("─", dividerWidth)) + "\n")

	// Show input field when adding
	if m.state == StateAdding {
		inputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
		b.WriteString(inputStyle.Render("> [ ] "+m.inputBuffer+"█") + "\n")
		hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		b.WriteString(hintStyle.Render("  enter: save  esc: cancel") + "\n")
	}

	// Show input field when editing
	if m.state == StateEditing && m.editingLine < len(m.items) {
		inputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		checkbox := "[ ]"
		if m.items[m.editingLine].Checked {
			checkbox = "[x]"
		}
		b.WriteString(inputStyle.Render("> "+checkbox+" "+m.inputBuffer+"█") + "\n")
		hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		b.WriteString(hintStyle.Render("  enter/esc: save") + "\n")
	}

	// Error state
	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		b.WriteString(errStyle.Render("Error: "+m.err.Error()) + "\n")
		return b.String()
	}

	// Empty state
	if len(m.items) == 0 && m.state != StateAdding {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		b.WriteString(dimStyle.Render("No tasks (press 'a' to add)") + "\n")
		return b.String()
	}

	// Separate items into unchecked and checked
	var unchecked, checked []int
	for i, item := range m.items {
		if item.Checked {
			checked = append(checked, i)
		} else {
			unchecked = append(unchecked, i)
		}
	}

	visibleCount := m.visibleItemCount()
	totalItems := len(m.items)
	endIdx := m.scrollPos + visibleCount
	if endIdx > totalItems {
		endIdx = totalItems
	}

	// Track if we need to show divider between unchecked and checked
	showedDivider := false
	lineCount := 0

	for i := m.scrollPos; i < endIdx && lineCount < visibleCount; i++ {
		item := m.items[i]

		// Show divider before first checked item
		if item.Checked && !showedDivider && len(unchecked) > 0 {
			divLine := dividerStyle.Render(strings.Repeat("─", dividerWidth))
			b.WriteString(divLine + "\n")
			lineCount++
			showedDivider = true
			if lineCount >= visibleCount {
				break
			}
		}

		line := m.renderItemLine(item, i == m.cursor && m.state == StateFocused)
		b.WriteString(line + "\n")
		lineCount++
	}

	// Scroll indicator
	if totalItems > visibleCount {
		indicator := fmt.Sprintf("(%d/%d)", m.cursor+1, totalItems)
		indStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		b.WriteString(indStyle.Render(indicator))
	}

	return b.String()
}

func (m *Module) renderItemLine(item TodoItem, selected bool) string {
	var checkbox string
	var textStyle lipgloss.Style

	if item.Checked {
		checkbox = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("[x]")
		textStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Strikethrough(true)
	} else {
		checkbox = lipgloss.NewStyle().Foreground(lipgloss.Color("247")).Render("[ ]")
		textStyle = lipgloss.NewStyle()
	}

	// Truncate text if needed
	text := item.Text
	maxLen := m.width - 10
	if maxLen < 10 {
		maxLen = 10
	}
	if len(text) > maxLen {
		text = text[:maxLen-3] + "..."
	}

	prefix := "  "
	if selected {
		prefix = "> "
	}

	line := prefix + checkbox + " " + textStyle.Render(text)

	if selected {
		return lipgloss.NewStyle().Bold(true).Reverse(true).Render(line)
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
		m.state = StateNormal
	}
	return m
}

func (m *Module) IsFocused() bool {
	return m.focused
}

// GetCopyContent returns the TODO list for clipboard copying
func (m *Module) GetCopyContent() string {
	var b strings.Builder
	b.WriteString("# TODO\n\n")
	for _, item := range m.items {
		if item.Checked {
			b.WriteString(fmt.Sprintf("- [x] %s\n", item.Text))
		} else {
			b.WriteString(fmt.Sprintf("- [ ] %s\n", item.Text))
		}
	}
	return b.String()
}
