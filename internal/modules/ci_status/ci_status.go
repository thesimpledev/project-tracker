package ci_status

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thesimpledev/project-tracker/internal/config"
	"github.com/thesimpledev/project-tracker/internal/github"
	"github.com/thesimpledev/project-tracker/internal/modules"
)

func init() {
	modules.Register("ci_status", func() modules.Module {
		return New()
	})
}

type State int

const (
	StateNormal State = iota
	StateFocused
	StateRunDetail
)

type Module struct {
	repo        config.Repo
	runs        []github.WorkflowRun
	status      github.RunStatus
	err         error
	state       State
	width       int
	height      int
	scrollPos   int
	runCursor   int
	detailRun   *github.WorkflowRun
	detailJobs  []github.Job
	jobCursor   int
	loadingJobs bool
	selected    bool
	focused     bool
}

type StatusFetchedMsg struct {
	Owner  string
	Name   string
	Status github.RunStatus
	Runs   []github.WorkflowRun
	Error  error
}

type JobsFetchedMsg struct {
	Owner string
	Name  string
	Jobs  []github.Job
	Error error
}

func New() *Module {
	return &Module{
		status: github.StatusUnknown,
		runs:   []github.WorkflowRun{},
	}
}

func (m *Module) ID() string {
	return "ci_status"
}

func (m *Module) Name() string {
	if m.repo.Name != "" {
		return fmt.Sprintf("CI: %s/%s", m.repo.Owner, m.repo.Name)
	}
	return "CI Status"
}

func (m *Module) SetRepo(repo config.Repo) modules.Module {
	m.repo = repo
	return m
}

func (m *Module) Init() tea.Cmd {
	if m.repo.Owner == "" || m.repo.Name == "" {
		return nil
	}
	return m.fetchStatus()
}

func (m *Module) fetchStatus() tea.Cmd {
	owner := m.repo.Owner
	name := m.repo.Name
	return func() tea.Msg {
		runs, err := github.FetchWorkflowRuns(owner, name, 5)
		status := github.StatusUnknown
		if len(runs) > 0 {
			status = runs[0].RunStatus()
		}
		return StatusFetchedMsg{
			Owner:  owner,
			Name:   name,
			Status: status,
			Runs:   runs,
			Error:  err,
		}
	}
}

func (m *Module) fetchJobs(runID int64) tea.Cmd {
	owner := m.repo.Owner
	name := m.repo.Name
	return func() tea.Msg {
		jobs, err := github.FetchRunJobs(owner, name, runID)
		return JobsFetchedMsg{
			Owner: owner,
			Name:  name,
			Jobs:  jobs,
			Error: err,
		}
	}
}

func (m *Module) Update(msg tea.Msg) (modules.Module, tea.Cmd) {
	switch msg := msg.(type) {
	case StatusFetchedMsg:
		if msg.Owner == m.repo.Owner && msg.Name == m.repo.Name {
			m.status = msg.Status
			m.runs = msg.Runs
			m.err = msg.Error
		}
		return m, nil

	case JobsFetchedMsg:
		if msg.Owner == m.repo.Owner && msg.Name == m.repo.Name {
			m.loadingJobs = false
			m.detailJobs = msg.Jobs
		}
		return m, nil

	case tea.KeyMsg:
		if m.state == StateRunDetail {
			switch msg.String() {
			case "j", "down":
				if m.jobCursor < len(m.detailJobs)-1 {
					m.jobCursor++
				}
			case "k", "up":
				if m.jobCursor > 0 {
					m.jobCursor--
				}
			case "esc":
				m.state = StateFocused
				m.detailRun = nil
				m.detailJobs = nil
				m.jobCursor = 0
			}
			return m, nil
		}

		if m.state == StateFocused {
			switch msg.String() {
			case "j", "down":
				if m.runCursor < len(m.runs)-1 {
					m.runCursor++
					visibleRuns := m.visibleRunCount()
					if m.runCursor >= m.scrollPos+visibleRuns {
						m.scrollPos++
					}
				}
			case "k", "up":
				if m.runCursor > 0 {
					m.runCursor--
					if m.runCursor < m.scrollPos {
						m.scrollPos--
					}
				}
			case "enter":
				if m.runCursor < len(m.runs) {
					run := m.runs[m.runCursor]
					m.state = StateRunDetail
					m.detailRun = &run
					m.loadingJobs = true
					m.jobCursor = 0
					return m, m.fetchJobs(run.ID)
				}
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *Module) View() string {
	var borderColor lipgloss.Color
	var borderStyle lipgloss.Border

	switch m.state {
	case StateFocused, StateRunDetail:
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

	var content string
	if m.state == StateRunDetail {
		content = m.renderRunDetail()
	} else {
		content = m.renderContent()
	}
	return style.Render(content)
}

func (m *Module) renderContent() string {
	var b strings.Builder

	width := m.width
	if width < 20 {
		width = 20
	}

	// Repo name
	repoName := fmt.Sprintf("%s/%s", m.repo.Owner, m.repo.Name)
	maxNameLen := width - 4
	if maxNameLen < 10 {
		maxNameLen = 10
	}
	if len(repoName) > maxNameLen {
		truncLen := maxNameLen - 3
		if truncLen < 1 {
			truncLen = 1
		}
		repoName = repoName[:truncLen] + "..."
	}
	nameStyle := lipgloss.NewStyle().Bold(true)
	b.WriteString(nameStyle.Render(repoName) + "\n")

	// Status line
	statusDot := m.statusDot()
	branch := ""
	if len(m.runs) > 0 {
		branch = m.runs[0].HeadBranch
		if len(branch) > 15 {
			branch = branch[:12] + "..."
		}
	}
	branchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	statusLine := fmt.Sprintf("%s %s", statusDot, branchStyle.Render(branch))
	b.WriteString(statusLine + "\n")

	// Divider
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	dividerWidth := width - 4
	if dividerWidth < 1 {
		dividerWidth = 1
	}
	b.WriteString(dividerStyle.Render(strings.Repeat("─", dividerWidth)) + "\n")

	// Runs list
	if m.err != nil {
		errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		b.WriteString(errStyle.Render("Error loading") + "\n")
	} else if len(m.runs) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		b.WriteString(dimStyle.Render("No runs") + "\n")
	} else {
		visibleCount := m.visibleRunCount()
		endIdx := m.scrollPos + visibleCount
		if endIdx > len(m.runs) {
			endIdx = len(m.runs)
		}

		for i := m.scrollPos; i < endIdx; i++ {
			run := m.runs[i]
			line := m.renderRunLine(run, i == m.runCursor && m.state == StateFocused)
			b.WriteString(line + "\n")
		}

		if len(m.runs) > visibleCount {
			indicator := fmt.Sprintf("(%d/%d)", m.runCursor+1, len(m.runs))
			indStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
			b.WriteString(indStyle.Render(indicator))
		}
	}

	return b.String()
}

func (m *Module) renderRunLine(run github.WorkflowRun, selected bool) string {
	icon := runStatusIcon(run.RunStatus())

	name := run.WorkflowName
	if name == "" {
		name = run.Name
	}

	branch := run.HeadBranch
	if len(branch) > 12 {
		branch = branch[:9] + "..."
	}

	maxNameLen := m.width - 30 - len(branch)
	if maxNameLen < 6 {
		maxNameLen = 6
	}
	if len(name) > maxNameLen {
		truncLen := maxNameLen - 3
		if truncLen < 1 {
			truncLen = 1
		}
		name = name[:truncLen] + "..."
	}

	timeAgo := formatTimeAgo(run.CreatedAt)

	branchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	branchStr := branchStyle.Render("(" + branch + ")")

	line := fmt.Sprintf("%s #%d %s %s %s", icon, run.RunNumber, name, branchStr, timeAgo)

	if selected {
		return lipgloss.NewStyle().Bold(true).Reverse(true).Render(line)
	}
	return line
}

func (m *Module) statusDot() string {
	switch m.status {
	case github.StatusSuccess:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("●")
	case github.StatusFailure:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("●")
	case github.StatusInProgress:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("●")
	case github.StatusPending:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("247")).Render("●")
	case github.StatusCancelled:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("●")
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("○")
	}
}

func runStatusIcon(status github.RunStatus) string {
	switch status {
	case github.StatusSuccess:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("[ok]")
	case github.StatusFailure:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render("[X]")
	case github.StatusInProgress:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render("[~]")
	case github.StatusPending:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("247")).Render("[?]")
	case github.StatusCancelled:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("[-]")
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("[.]")
	}
}

func formatTimeAgo(t time.Time) string {
	diff := time.Since(t)

	if diff < time.Minute {
		return "now"
	}
	if diff < time.Hour {
		mins := int(diff.Minutes())
		return fmt.Sprintf("%dm", mins)
	}
	if diff < 24*time.Hour {
		hours := int(diff.Hours())
		return fmt.Sprintf("%dh", hours)
	}
	days := int(diff.Hours() / 24)
	return fmt.Sprintf("%dd", days)
}

func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func (m *Module) visibleRunCount() int {
	available := m.height - 5
	if available < 1 {
		return 1
	}
	return available
}

func (m *Module) renderRunDetail() string {
	var b strings.Builder

	width := m.width
	if width < 20 {
		width = 20
	}

	if m.detailRun == nil {
		return "No run selected"
	}

	run := m.detailRun

	headerStyle := lipgloss.NewStyle().Bold(true)
	workflowName := run.WorkflowName
	if workflowName == "" {
		workflowName = run.Name
	}
	b.WriteString(headerStyle.Render(fmt.Sprintf("#%d %s", run.RunNumber, workflowName)) + "\n")

	statusIcon := runStatusIcon(run.RunStatus())
	branchStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
	b.WriteString(fmt.Sprintf("%s %s %s\n", statusIcon, branchStyle.Render(run.HeadBranch), formatTimeAgo(run.CreatedAt)))

	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	dividerWidth := width - 4
	if dividerWidth < 1 {
		dividerWidth = 1
	}
	b.WriteString(dividerStyle.Render(strings.Repeat("─", dividerWidth)) + "\n")

	jobsHeader := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("Jobs:")
	b.WriteString(jobsHeader + "\n")

	if m.loadingJobs {
		loadStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		b.WriteString(loadStyle.Render("Loading...") + "\n")
	} else if len(m.detailJobs) == 0 {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		b.WriteString(dimStyle.Render("No jobs") + "\n")
	} else {
		for i, job := range m.detailJobs {
			line := m.renderJobLine(job, i == m.jobCursor)
			b.WriteString(line + "\n")
		}
	}

	b.WriteString("\n")
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	b.WriteString(helpStyle.Render("esc: back"))

	return b.String()
}

func (m *Module) renderJobLine(job github.Job, selected bool) string {
	icon := runStatusIcon(job.JobStatus())

	name := job.Name
	maxNameLen := m.width - 20
	if maxNameLen < 10 {
		maxNameLen = 10
	}
	if len(name) > maxNameLen {
		name = name[:maxNameLen-3] + "..."
	}

	duration := ""
	if d := job.Duration(); d > 0 {
		duration = formatDuration(d)
	}

	line := fmt.Sprintf("%s %s %s", icon, name, duration)

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
		m.runCursor = 0
		m.scrollPos = 0
		m.detailRun = nil
		m.detailJobs = nil
		m.jobCursor = 0
	}
	return m
}

func (m *Module) IsFocused() bool {
	return m.focused
}

// GetCopyContent returns CI status info for clipboard copying
func (m *Module) GetCopyContent() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Repository: %s/%s\n", m.repo.Owner, m.repo.Name))
	b.WriteString(fmt.Sprintf("Status: %s\n\n", m.status))
	b.WriteString("Recent Runs:\n")
	for _, run := range m.runs {
		status := "?"
		switch run.Conclusion {
		case "success":
			status = "✓"
		case "failure":
			status = "✗"
		}
		if run.Status == "in_progress" {
			status = "⟳"
		}
		b.WriteString(fmt.Sprintf("  %s %s (%s) - %s\n", status, run.Name, run.HeadBranch, run.UpdatedAt.Format("Jan 2 15:04")))
	}
	return b.String()
}
