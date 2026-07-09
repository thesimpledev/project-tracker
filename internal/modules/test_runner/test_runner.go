package test_runner

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thesimpledev/project-tracker/internal/config"
	"github.com/thesimpledev/project-tracker/internal/modules"
)

func init() {
	modules.Register("test_runner", func() modules.Module {
		return New()
	})
}

type State int

const (
	StateIdle State = iota
	StateRunning
	StatePassed
	StateFailed
	StateFocused
	StateDetail // Viewing failure details
)

type TestResult struct {
	Name   string
	Passed bool
	Output string
}

type Module struct {
	repo         config.Repo
	state        State
	prevState    State // For returning from detail view
	selected     bool
	focused      bool
	width        int
	height       int
	err          error
	results      []TestResult
	passed       int
	failed       int
	total        int
	duration     time.Duration
	lastRun      time.Time
	output       string
	scrollPos    int
	cursor       int
	showFailed   bool
	detailScroll int
}

type TestStartedMsg struct {
	Path string
}

type TestFinishedMsg struct {
	Path     string
	Output   string
	Duration time.Duration
	Error    error
}

// RunTestsMsg triggers test run from outside (global hotkey)
type RunTestsMsg struct{}

// Exported function to create the trigger command
func TriggerTestRun() tea.Msg {
	return RunTestsMsg{}
}

func New() *Module {
	return &Module{
		state:   StateIdle,
		results: []TestResult{},
	}
}

func (m *Module) ID() string {
	return "test_runner"
}

func (m *Module) Name() string {
	return "Tests"
}

func (m *Module) SetRepo(repo config.Repo) modules.Module {
	m.repo = repo
	return m
}

func (m *Module) Init() tea.Cmd {
	if m.repo.Path == "" {
		return nil
	}
	return m.ensureJustfile()
}

func (m *Module) ensureJustfile() tea.Cmd {
	path := m.repo.Path
	return func() tea.Msg {
		justfilePath := filepath.Join(path, "justfile")

		// Check if justfile exists
		content, err := os.ReadFile(justfilePath) // #nosec G304 -- repo path validated at config load
		if os.IsNotExist(err) {
			// Create new justfile with test command
			newContent := `# Project commands

# Run tests
test:
    go test ./...
`
			// #nosec G306 -- project file, intentionally world-readable
			if err := os.WriteFile(justfilePath, []byte(newContent), 0644); err != nil {
				return TestFinishedMsg{Path: path, Error: err}
			}
			return nil
		}
		if err != nil {
			return nil
		}

		// Check if test command already exists
		if strings.Contains(string(content), "\ntest:") || strings.HasPrefix(string(content), "test:") {
			return nil
		}

		// Append test command
		appendContent := `
# Run tests
test:
    go test ./...
`
		newContent := string(content) + appendContent
		// #nosec G306 G703 -- project file, intentionally world-readable; repo path validated at config load
		if err := os.WriteFile(justfilePath, []byte(newContent), 0644); err != nil {
			return TestFinishedMsg{Path: path, Error: err}
		}
		return nil
	}
}

func (m *Module) runTests() tea.Cmd {
	path := m.repo.Path
	return func() tea.Msg {
		start := time.Now()

		cmd := exec.Command("just", "test")
		cmd.Dir = path

		output, err := cmd.CombinedOutput()
		duration := time.Since(start)

		return TestFinishedMsg{
			Path:     path,
			Output:   string(output),
			Duration: duration,
			Error:    err,
		}
	}
}

func (m *Module) parseTestOutput(output string) {
	m.results = []TestResult{}
	m.passed = 0
	m.failed = 0
	m.total = 0
	m.output = output

	lines := strings.Split(output, "\n")

	// Patterns for Go test output
	failPattern := regexp.MustCompile(`^--- FAIL: (\S+)`)
	passPattern := regexp.MustCompile(`^--- PASS: (\S+)`)
	// Package summary: ok/FAIL package_name duration
	pkgOkPattern := regexp.MustCompile(`^ok\s+\S+`)
	pkgFailPattern := regexp.MustCompile(`^FAIL\s+\S+`)

	// Track individual test results
	var currentTest *TestResult
	var currentOutput strings.Builder

	for _, line := range lines {
		// Check for test start/end markers
		if matches := failPattern.FindStringSubmatch(line); len(matches) > 1 {
			if currentTest != nil {
				currentTest.Output = currentOutput.String()
				m.results = append(m.results, *currentTest)
			}
			currentTest = &TestResult{Name: matches[1], Passed: false}
			currentOutput.Reset()
			m.failed++
			m.total++
		} else if matches := passPattern.FindStringSubmatch(line); len(matches) > 1 {
			if currentTest != nil {
				currentTest.Output = currentOutput.String()
				m.results = append(m.results, *currentTest)
			}
			currentTest = &TestResult{Name: matches[1], Passed: true}
			currentOutput.Reset()
			m.passed++
			m.total++
		} else if currentTest != nil {
			currentOutput.WriteString(line + "\n")
		}

		// Also check for package-level pass/fail (for when individual tests aren't shown)
		if pkgOkPattern.MatchString(line) && m.total == 0 {
			// Package passed but no individual tests shown
			m.passed++
			m.total++
		} else if pkgFailPattern.MatchString(line) && m.failed == 0 {
			// Package failed
			m.failed++
			m.total++
		}
	}

	// Save last test if any
	if currentTest != nil {
		currentTest.Output = currentOutput.String()
		m.results = append(m.results, *currentTest)
	}

	// If we couldn't parse individual tests, try to extract counts from summary
	if m.total == 0 {
		// Look for patterns like "Tests: X passed, Y failed"
		countPattern := regexp.MustCompile(`(\d+)\s+passed`)
		if matches := countPattern.FindStringSubmatch(output); len(matches) > 1 {
			if n, err := strconv.Atoi(matches[1]); err == nil {
				m.passed = n
				m.total += n
			}
		}
		failCountPattern := regexp.MustCompile(`(\d+)\s+failed`)
		if matches := failCountPattern.FindStringSubmatch(output); len(matches) > 1 {
			if n, err := strconv.Atoi(matches[1]); err == nil {
				m.failed = n
				m.total += n
			}
		}
	}
}

func (m *Module) Update(msg tea.Msg) (modules.Module, tea.Cmd) {
	switch msg := msg.(type) {
	case RunTestsMsg:
		// Global trigger to run tests
		if m.state != StateRunning && m.repo.Path != "" {
			m.state = StateRunning
			m.results = []TestResult{}
			return m, m.runTests()
		}
		return m, nil

	case TestFinishedMsg:
		if msg.Path == m.repo.Path {
			m.parseTestOutput(msg.Output)
			m.duration = msg.Duration
			m.lastRun = time.Now()
			m.err = msg.Error

			if m.failed > 0 || msg.Error != nil {
				m.state = StateFailed
			} else {
				m.state = StatePassed
			}
		}
		return m, nil

	case tea.KeyMsg:
		// Handle detail view navigation
		if m.state == StateDetail {
			switch msg.String() {
			case "esc":
				m.state = m.prevState
				m.detailScroll = 0
			case "j", "down":
				m.detailScroll++
			case "k", "up":
				if m.detailScroll > 0 {
					m.detailScroll--
				}
			case "t":
				m.state = StateRunning
				m.results = []TestResult{}
				m.detailScroll = 0
				return m, m.runTests()
			}
			return m, nil
		}

		if !m.focused {
			return m, nil
		}

		switch msg.String() {
		case "t":
			if m.state != StateRunning {
				m.state = StateRunning
				m.results = []TestResult{}
				return m, m.runTests()
			}
		case "enter":
			// Show detail view for selected failed test or raw output
			failedResults := m.getFailedResults()
			if len(failedResults) > 0 && m.cursor < len(failedResults) {
				m.prevState = m.state
				m.state = StateDetail
				m.detailScroll = 0
			} else if m.state == StateFailed && m.output != "" {
				// Show full output when no parsed test failures
				m.prevState = m.state
				m.state = StateDetail
				m.detailScroll = 0
			} else if m.state != StateRunning {
				// If no failed tests and not failed state, run tests
				m.state = StateRunning
				m.results = []TestResult{}
				return m, m.runTests()
			}
		case "j", "down":
			failedResults := m.getFailedResults()
			if m.cursor < len(failedResults)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "f":
			m.showFailed = !m.showFailed
		}
		return m, nil
	}

	return m, nil
}

func (m *Module) getFailedResults() []TestResult {
	var failed []TestResult
	for _, r := range m.results {
		if !r.Passed {
			failed = append(failed, r)
		}
	}
	return failed
}

func (m *Module) View() string {
	var borderColor lipgloss.Color
	var borderStyle lipgloss.Border

	if m.focused {
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
	b.WriteString(headerStyle.Render("Tests") + "\n")

	// Divider
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	dividerWidth := width - 4
	if dividerWidth < 1 {
		dividerWidth = 1
	}
	b.WriteString(dividerStyle.Render(strings.Repeat("─", dividerWidth)) + "\n")

	switch m.state {
	case StateIdle:
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		b.WriteString(dimStyle.Render("Press 't' or Enter to run tests") + "\n")
		if m.repo.Path != "" {
			pathStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
			b.WriteString(pathStyle.Render("Using: just test") + "\n")
		}

	case StateRunning:
		runningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
		b.WriteString(runningStyle.Render("⟳ Running tests...") + "\n")

	case StatePassed, StateFailed:
		m.renderResults(&b, dividerWidth)

	case StateDetail:
		m.renderDetailView(&b, dividerWidth)
	}

	return b.String()
}

func (m *Module) renderDetailView(b *strings.Builder, dividerWidth int) {
	failedResults := m.getFailedResults()
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)

	var output string

	if len(failedResults) > 0 && m.cursor < len(failedResults) {
		// Show specific test failure
		result := failedResults[m.cursor]
		b.WriteString(failStyle.Render("✗ "+result.Name) + "\n")
		b.WriteString(dividerStyle.Render(strings.Repeat("─", dividerWidth)) + "\n")
		output = result.Output
		if output == "" {
			output = m.output
		}
	} else {
		// Show raw output (linter errors, etc.)
		b.WriteString(failStyle.Render("✗ Test Output") + "\n")
		b.WriteString(dividerStyle.Render(strings.Repeat("─", dividerWidth)) + "\n")
		output = m.output
	}

	if output == "" {
		b.WriteString(dividerStyle.Render("No output available") + "\n")
		return
	}

	lines := strings.Split(output, "\n")
	visibleLines := m.height - 8
	if visibleLines < 3 {
		visibleLines = 3
	}

	startLine := m.detailScroll
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
		// Truncate long lines
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

	// Hint
	hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	b.WriteString(hintStyle.Render("esc: back  j/k: scroll  t: rerun"))
}

func (m *Module) renderResults(b *strings.Builder, dividerWidth int) {
	// Status icon and summary
	var statusLine string
	if m.state == StatePassed {
		checkStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
		statusLine = checkStyle.Render("✓ All tests passed")
	} else {
		failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
		statusLine = failStyle.Render("✗ Tests failed")
	}
	b.WriteString(statusLine + "\n")

	// Counts
	var countParts []string
	if m.passed > 0 {
		passStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
		countParts = append(countParts, passStyle.Render(fmt.Sprintf("%d passed", m.passed)))
	}
	if m.failed > 0 {
		failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
		countParts = append(countParts, failStyle.Render(fmt.Sprintf("%d failed", m.failed)))
	}
	if len(countParts) > 0 {
		b.WriteString(strings.Join(countParts, " · ") + "\n")
	}

	// Duration
	durationStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	b.WriteString(durationStyle.Render(fmt.Sprintf("Duration: %s", m.duration.Round(time.Millisecond))) + "\n")

	// Failed tests list or raw output
	failedResults := m.getFailedResults()
	dividerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	if len(failedResults) > 0 {
		b.WriteString(dividerStyle.Render(strings.Repeat("─", dividerWidth)) + "\n")

		failHeader := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
		b.WriteString(failHeader.Render("Failed:") + "\n")

		visibleCount := m.height - 10
		if visibleCount < 1 {
			visibleCount = 1
		}

		for i, result := range failedResults {
			if i >= visibleCount {
				moreStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
				b.WriteString(moreStyle.Render(fmt.Sprintf("  ... and %d more", len(failedResults)-i)) + "\n")
				break
			}

			prefix := "  "
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
			if m.focused && i == m.cursor {
				prefix = "> "
				style = style.Bold(true).Reverse(true)
			}

			name := result.Name
			maxLen := m.width - 8
			if maxLen < 10 {
				maxLen = 10
			}
			if len(name) > maxLen {
				name = name[:maxLen-3] + "..."
			}

			b.WriteString(style.Render(prefix+"✗ "+name) + "\n")
		}
	} else if m.state == StateFailed && m.output != "" {
		// Show raw output when we couldn't parse individual test failures
		b.WriteString(dividerStyle.Render(strings.Repeat("─", dividerWidth)) + "\n")

		outputStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("247"))
		lines := strings.Split(m.output, "\n")

		// Find the first non-empty error line
		visibleLines := m.height - 8
		if visibleLines < 3 {
			visibleLines = 3
		}

		lineCount := 0
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if lineCount >= visibleLines {
				b.WriteString(dividerStyle.Render(fmt.Sprintf("... %d more lines (enter: view all)", len(lines)-lineCount)) + "\n")
				break
			}
			// Truncate long lines
			maxLen := m.width - 6
			if maxLen < 20 {
				maxLen = 20
			}
			if len(line) > maxLen {
				line = line[:maxLen-3] + "..."
			}
			b.WriteString(outputStyle.Render(line) + "\n")
			lineCount++
		}
	}

	// Hint
	if m.focused {
		hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		if len(failedResults) > 0 {
			b.WriteString(hintStyle.Render("t: rerun  enter: details") + "\n")
		} else if m.state == StateFailed && m.output != "" {
			b.WriteString(hintStyle.Render("t: rerun  enter: full output") + "\n")
		} else {
			b.WriteString(hintStyle.Render("t: rerun") + "\n")
		}
	}
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
	return m
}

func (m *Module) IsFocused() bool {
	return m.focused
}

// GetCopyContent returns the test output for clipboard copying
func (m *Module) GetCopyContent() string {
	return m.output
}
