package git_status

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thesimpledev/project-tracker/internal/config"
	"github.com/thesimpledev/project-tracker/internal/modules"
)

func init() {
	modules.Register("git_status", func() modules.Module {
		return New()
	})
}

type State int

const (
	StateNormal State = iota
	StateFocused
)

type FileChange struct {
	Status string // M, A, D, ?, etc.
	Path   string
}

type Module struct {
	repo      config.Repo
	branch    string
	changes   []FileChange
	staged    int
	modified  int
	untracked int
	state     State
	selected  bool
	focused   bool
	width     int
	height    int
	err       error
	scrollPos int
	cursor    int
	fetching  bool // Prevents overlapping git status fetches
}

type GitStatusMsg struct {
	Path      string
	Branch    string
	Changes   []FileChange
	Staged    int
	Modified  int
	Untracked int
	Error     error
}

type GitTickMsg struct {
	Path string
}

func New() *Module {
	return &Module{
		changes: []FileChange{},
	}
}

func (m *Module) ID() string {
	return "git_status"
}

func (m *Module) Name() string {
	if m.repo.Name != "" {
		return fmt.Sprintf("Git: %s", m.repo.Name)
	}
	return "Git Status"
}

func (m *Module) SetRepo(repo config.Repo) modules.Module {
	m.repo = repo
	return m
}

func (m *Module) Init() tea.Cmd {
	if m.repo.Path == "" {
		return nil
	}
	m.fetching = true
	return tea.Batch(m.fetchStatus(), m.tick())
}

func (m *Module) tick() tea.Cmd {
	path := m.repo.Path
	return tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
		return GitTickMsg{Path: path}
	})
}

func (m *Module) fetchStatus() tea.Cmd {
	path := m.repo.Path
	return func() tea.Msg {
		// Get current branch (use --no-optional-locks to avoid blocking during heavy writes)
		// #nosec G204 -- fixed binary; repo path validated at config load
		branchCmd := exec.Command("git", "-C", path, "--no-optional-locks", "branch", "--show-current")
		branchOut, err := branchCmd.Output()
		if err != nil {
			return GitStatusMsg{Path: path, Error: err}
		}
		branch := strings.TrimSpace(string(branchOut))

		// Get status (use --no-optional-locks to avoid blocking during heavy writes)
		// #nosec G204 -- fixed binary; repo path validated at config load
		statusCmd := exec.Command("git", "-C", path, "--no-optional-locks", "status", "--porcelain")
		statusOut, err := statusCmd.Output()
		if err != nil {
			return GitStatusMsg{Path: path, Branch: branch, Error: err}
		}

		var changes []FileChange
		var staged, modified, untracked int

		lines := strings.Split(string(statusOut), "\n")
		for _, line := range lines {
			if len(line) < 3 {
				continue
			}
			x := line[0] // Index status (staged)
			y := line[1] // Worktree status (modified)
			filePath := strings.TrimSpace(line[3:])

			change := FileChange{Path: filePath}

			// Determine status
			if x == '?' && y == '?' {
				change.Status = "?"
				untracked++
			} else {
				if x != ' ' && x != '?' {
					staged++
					change.Status = string(x)
				}
				if y != ' ' && y != '?' {
					modified++
					if change.Status == "" {
						change.Status = string(y)
					}
				}
			}

			changes = append(changes, change)
		}

		return GitStatusMsg{
			Path:      path,
			Branch:    branch,
			Changes:   changes,
			Staged:    staged,
			Modified:  modified,
			Untracked: untracked,
		}
	}
}

func (m *Module) Update(msg tea.Msg) (modules.Module, tea.Cmd) {
	switch msg := msg.(type) {
	case GitStatusMsg:
		if msg.Path == m.repo.Path {
			m.fetching = false
			m.branch = msg.Branch
			m.changes = msg.Changes
			m.staged = msg.Staged
			m.modified = msg.Modified
			m.untracked = msg.Untracked
			m.err = msg.Error
		}
		return m, nil

	case GitTickMsg:
		if msg.Path == m.repo.Path {
			// Skip if already fetching to prevent overlapping requests
			if m.fetching {
				return m, m.tick()
			}
			m.fetching = true
			return m, tea.Batch(m.fetchStatus(), m.tick())
		}
		return m, nil

	case tea.KeyMsg:
		if m.state != StateFocused {
			return m, nil
		}

		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.changes)-1 {
				m.cursor++
				m.ensureCursorVisible()
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.ensureCursorVisible()
			}
		case "r": // Manual refresh
			return m, m.fetchStatus()
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
	available := m.height - 6 // Header, branch, summary, divider, padding
	if available < 1 {
		return 1
	}
	return available
}

func (m *Module) View() string {
	var borderColor lipgloss.Color
	var borderStyle lipgloss.Border

	switch m.state {
	case StateFocused:
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
	b.WriteString(headerStyle.Render("Git Status") + "\n")

	// Branch
	branchIcon := lipgloss.NewStyle().Foreground(lipgloss.Color("212")).Render("")
	branchName := m.branch
	if branchName == "" {
		branchName = "unknown"
	}
	branchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	b.WriteString(branchIcon + " " + branchStyle.Render(branchName) + "\n")

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

	// Summary line
	if len(m.changes) == 0 {
		cleanStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
		b.WriteString(cleanStyle.Render("✓ Clean working tree") + "\n")
		return b.String()
	}

	// Status counts
	var parts []string
	if m.staged > 0 {
		stagedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
		parts = append(parts, stagedStyle.Render(fmt.Sprintf("+%d staged", m.staged)))
	}
	if m.modified > 0 {
		modStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		parts = append(parts, modStyle.Render(fmt.Sprintf("~%d modified", m.modified)))
	}
	if m.untracked > 0 {
		untrackedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		parts = append(parts, untrackedStyle.Render(fmt.Sprintf("?%d untracked", m.untracked)))
	}
	b.WriteString(strings.Join(parts, " ") + "\n")

	// File list
	visibleCount := m.visibleCount()
	endIdx := m.scrollPos + visibleCount
	if endIdx > len(m.changes) {
		endIdx = len(m.changes)
	}

	for i := m.scrollPos; i < endIdx; i++ {
		change := m.changes[i]
		line := m.renderChangeLine(change, i == m.cursor && m.state == StateFocused)
		b.WriteString(line + "\n")
	}

	// Scroll indicator
	if len(m.changes) > visibleCount {
		indicator := fmt.Sprintf("(%d/%d)", m.cursor+1, len(m.changes))
		indStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		b.WriteString(indStyle.Render(indicator))
	}

	return b.String()
}

func (m *Module) renderChangeLine(change FileChange, selected bool) string {
	var statusStyle lipgloss.Style

	switch change.Status {
	case "M":
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	case "A":
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	case "D":
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	case "?":
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	default:
		statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("247"))
	}

	// Truncate path if needed
	path := change.Path
	maxLen := m.width - 8
	if maxLen < 10 {
		maxLen = 10
	}
	if len(path) > maxLen {
		path = "..." + path[len(path)-maxLen+3:]
	}

	prefix := "  "
	if selected {
		prefix = "> "
	}

	line := prefix + statusStyle.Render(change.Status) + " " + path

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
		m.cursor = 0
		m.scrollPos = 0
	}
	return m
}

func (m *Module) IsFocused() bool {
	return m.focused
}

// GetCopyContent returns git status info for clipboard copying
func (m *Module) GetCopyContent() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Branch: %s\n", m.branch))
	b.WriteString(fmt.Sprintf("Staged: %d, Modified: %d, Untracked: %d\n\n", m.staged, m.modified, m.untracked))
	for _, change := range m.changes {
		b.WriteString(fmt.Sprintf("%s %s\n", change.Status, change.Path))
	}
	return b.String()
}
