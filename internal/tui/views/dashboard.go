package views

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thesimpledev/project-tracker/internal/config"
	"github.com/thesimpledev/project-tracker/internal/modules"
	"github.com/thesimpledev/project-tracker/internal/repo"
	"github.com/thesimpledev/project-tracker/internal/tui/components"
)

type InputMode int

const (
	ModeGrid InputMode = iota
	ModeCommand
)

type DashboardModel struct {
	config        *config.Config
	grid          components.Grid
	commandInput  components.CommandInput
	mode          InputMode
	width         int
	height        int
	err           error
	profileName   string
	statusMessage string
	statusTime    time.Time
}

type RefreshMsg struct{}
type RepoAddedMsg struct {
	Repo config.Repo
}
type RepoRemovedMsg struct {
	Owner string
	Name  string
}

func NewDashboardModel(cfg *config.Config, mods []modules.Module) DashboardModel {
	ci := components.NewCommandInput()
	if cfg.LastOpenedDir != "" {
		ci = ci.SetLastPath(cfg.LastOpenedDir)
	}
	return DashboardModel{
		config:       cfg,
		grid:         components.NewGrid(mods),
		commandInput: ci,
		mode:         ModeGrid,
		profileName:  cfg.ProfileName,
	}
}

func (m DashboardModel) SetSize(width, height int) DashboardModel {
	m.width = width
	m.height = height
	m.commandInput = m.commandInput.SetSize(width)
	// Grid size will be calculated dynamically in View() based on command area height
	return m
}

func (m DashboardModel) Init() tea.Cmd {
	return tea.Batch(m.grid.Init(), m.setWindowTitle())
}

func (m DashboardModel) setWindowTitle() tea.Cmd {
	title := "project-tracker"
	if m.profileName != "" {
		title = "project-tracker: " + m.profileName
	}
	return tea.SetWindowTitle(title)
}

func (m DashboardModel) Update(msg tea.Msg) (DashboardModel, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m = m.SetSize(msg.Width, msg.Height)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.mode == ModeGrid && m.grid.State == components.GridNavigating {
				return m, tea.Quit
			}
		case ":":
			if m.mode == ModeGrid && m.grid.State == components.GridNavigating {
				m.mode = ModeCommand
				m.commandInput = m.commandInput.SetFocused(true)
				return m, nil
			}
		case "esc":
			if m.mode == ModeCommand {
				m.mode = ModeGrid
				m.commandInput = m.commandInput.SetFocused(false)
				return m, nil
			}
		}

	case components.ExecuteCommandMsg:
		return m.handleCommand(msg.Cmd)

	case RefreshMsg:
		cmds = append(cmds, m.grid.RefreshAll())
		return m, tea.Batch(cmds...)

	case components.ClipboardCopyMsg:
		if msg.Success {
			m.statusMessage = "Copied to clipboard!"
		} else {
			m.statusMessage = "Failed to copy"
		}
		m.statusTime = time.Now()
		return m, nil
	}

	switch m.mode {
	case ModeCommand:
		var cmd tea.Cmd
		m.commandInput, cmd = m.commandInput.Update(msg)
		cmds = append(cmds, cmd)
		if !m.commandInput.Focused {
			m.mode = ModeGrid
		}
	case ModeGrid:
		var cmd tea.Cmd
		m.grid, cmd = m.grid.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m DashboardModel) handleCommand(cmd components.Command) (DashboardModel, tea.Cmd) {
	switch cmd.Type {
	case components.CmdQuit:
		return m, tea.Quit

	case components.CmdChange:
		if cmd.Arg != "" {
			info, err := repo.GetRepoInfo(cmd.Arg)
			if err != nil || info == nil {
				m.err = err
				m.mode = ModeGrid
				return m, nil
			}

			newRepo := config.Repo{
				Path:  info.Path,
				Owner: info.Owner,
				Name:  info.Name,
			}
			// Replace with the new repo
			m.config.Repos = []config.Repo{newRepo}
			m.config.LastOpenedDir = info.Path
			if err := m.config.Save(); err != nil {
				m.err = err
			}

			// Rebuild grid with new module
			mods := BuildModules(m.config)
			m.grid = components.NewGrid(mods)
			m.grid = m.grid.SetSize(m.width, m.height-6)
			m.commandInput = m.commandInput.SetLastPath(info.Path)
			m.mode = ModeGrid
			return m, m.grid.Init()
		}
		m.mode = ModeGrid
		return m, nil

	default:
		m.mode = ModeGrid
		return m, nil
	}
}

// BuildModules constructs the dashboard's module set for the given config.
// Layout: Row 1: CI, TODO, Notes | Row 2: Git Status, Just Commands (spans 2 cols).
// Total of 5 modules per repo; the grid auto-expands the last module in a row
// into any trailing empty cells (see components/grid.go).
func BuildModules(cfg *config.Config) []modules.Module {
	var mods []modules.Module

	ids := []string{"ci_status", "todo", "notes", "git_status", "just_commands"}
	for _, r := range cfg.Repos {
		for _, id := range ids {
			if mod := createWithRepo(id, r); mod != nil {
				mods = append(mods, mod)
			}
		}
	}

	for len(mods) < 5 {
		if placeholder := modules.Create("placeholder"); placeholder != nil {
			mods = append(mods, placeholder)
		} else {
			break
		}
	}

	return mods
}

func createWithRepo(id string, r config.Repo) modules.Module {
	mod := modules.Create(id)
	if mod == nil {
		mod = modules.Create("placeholder")
	}
	if mod == nil {
		return nil
	}
	if setter, ok := mod.(interface {
		SetRepo(config.Repo) modules.Module
	}); ok {
		mod = setter.SetRepo(r)
	}
	return mod
}

func (m DashboardModel) View() string {
	// Get command view first to calculate its height
	cmdView := m.commandInput.View()
	cmdLines := strings.Count(cmdView, "\n") + 1

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("62"))

	titleText := "project-tracker"
	if m.profileName != "" {
		profileStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("212")).
			Bold(true)
		titleText = "project-tracker - " + profileStyle.Render(m.profileName)
	}

	// Show status message if recent (within 2 seconds)
	if m.statusMessage != "" && time.Since(m.statusTime) < 2*time.Second {
		statusStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("42")).
			Bold(true)
		titleText += " " + statusStyle.Render("["+m.statusMessage+"]")
	}
	title := titleStyle.Render(titleText)

	// Calculate grid height dynamically based on command area
	titleHeight := 2
	gridHeight := m.height - titleHeight - cmdLines - 1
	if gridHeight < 6 {
		gridHeight = 6
	}

	// Update grid size and render
	grid := m.grid.SetSize(m.width, gridHeight)
	gridView := grid.View()

	// Build: title + grid + command (at bottom)
	return title + "\n\n" + gridView + "\n" + cmdView
}

func TickCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return RefreshMsg{}
	})
}
