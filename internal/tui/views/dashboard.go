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
	config       *config.Config
	grid         components.Grid
	commandInput components.CommandInput
	mode         InputMode
	width        int
	height       int
	err          error
	profileName  string
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
	return DashboardModel{
		config:       cfg,
		grid:         components.NewGrid(mods),
		commandInput: components.NewCommandInput(cfg.Repos),
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

	case components.CmdRefresh:
		m.mode = ModeGrid
		return m, m.grid.RefreshAll()

	case components.CmdAdd:
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
			m.config.AddRepo(newRepo)
			m.config.LastOpenedDir = info.Path
			m.config.Save()

			// Rebuild grid with new module
			mods := buildModulesFromConfig(m.config)
			m.grid = components.NewGrid(mods)
			m.grid = m.grid.SetSize(m.width, m.height-6)
			m.commandInput = m.commandInput.SetRepos(m.config.Repos)
			m.commandInput = m.commandInput.SetLastPath(info.Path)
			m.mode = ModeGrid
			return m, m.grid.Init()
		}
		m.mode = ModeGrid
		return m, nil

	case components.CmdRemove:
		if cmd.Arg != "" {
			parts := splitOwnerName(cmd.Arg)
			if len(parts) == 2 {
				m.config.RemoveRepo(parts[0], parts[1])
				m.config.Save()

				mods := buildModulesFromConfig(m.config)
				m.grid = components.NewGrid(mods)
				m.grid = m.grid.SetSize(m.width, m.height-6)
				m.commandInput = m.commandInput.SetRepos(m.config.Repos)
			}
		}
		m.mode = ModeGrid
		return m, nil

	case components.CmdSave:
		if cmd.Arg != "" {
			m.config.SaveProfile(cmd.Arg)
			m.profileName = cmd.Arg
			m.config.ProfileName = cmd.Arg
			m.config.Save()
		}
		m.mode = ModeGrid
		return m, m.setWindowTitle()

	case components.CmdLoad:
		if cmd.Arg != "" {
			loadedCfg, err := config.LoadProfile(cmd.Arg)
			if err == nil && loadedCfg != nil {
				m.config.Repos = loadedCfg.Repos
				m.config.ProfileName = cmd.Arg
				m.config.Save()
				m.profileName = cmd.Arg

				mods := buildModulesFromConfig(m.config)
				m.grid = components.NewGrid(mods)
				m.grid = m.grid.SetSize(m.width, m.height-6)
				m.commandInput = m.commandInput.SetRepos(m.config.Repos)
				m.mode = ModeGrid
				return m, tea.Batch(m.grid.Init(), m.setWindowTitle())
			}
		}
		m.mode = ModeGrid
		return m, nil

	case components.CmdNew:
		m.config.Repos = []config.Repo{}
		m.config.ProfileName = ""
		m.config.Save()
		m.profileName = ""

		mods := buildModulesFromConfig(m.config)
		m.grid = components.NewGrid(mods)
		m.grid = m.grid.SetSize(m.width, m.height-6)
		m.commandInput = m.commandInput.SetRepos(m.config.Repos)
		m.mode = ModeGrid
		return m, m.setWindowTitle()

	default:
		m.mode = ModeGrid
		return m, nil
	}
}

func splitOwnerName(s string) []string {
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			return []string{s[:i], s[i+1:]}
		}
	}
	return nil
}

func buildModulesFromConfig(cfg *config.Config) []modules.Module {
	var mods []modules.Module

	// Add CI status module for each repo
	for _, r := range cfg.Repos {
		mod := modules.Create("ci_status")
		if mod == nil {
			mod = modules.Create("placeholder")
		}
		if mod != nil {
			// Set repo info on the module if it supports it
			if setter, ok := mod.(interface{ SetRepo(config.Repo) modules.Module }); ok {
				mod = setter.SetRepo(r)
			}
			mods = append(mods, mod)
		}
	}

	// Fill remaining slots with placeholders (up to 6)
	for len(mods) < 6 {
		if placeholder := modules.Create("placeholder"); placeholder != nil {
			mods = append(mods, placeholder)
		} else {
			break
		}
	}

	return mods
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
