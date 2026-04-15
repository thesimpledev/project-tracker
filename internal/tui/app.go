package tui

import (
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thesimpledev/project-tracker/internal/config"
	"github.com/thesimpledev/project-tracker/internal/modules"
	"github.com/thesimpledev/project-tracker/internal/repo"
	"github.com/thesimpledev/project-tracker/internal/tui/components"
	"github.com/thesimpledev/project-tracker/internal/tui/views"
)

type AppView int

const (
	ViewGreeting AppView = iota
	ViewDashboard
)

type App struct {
	config    *config.Config
	view      AppView
	greeting  views.GreetingModel
	dashboard views.DashboardModel
	width     int
	height    int
}

type TickMsg time.Time

func NewApp(cfg *config.Config) App {
	var view AppView
	var mods []modules.Module

	// If launched from inside a git repo, prefer it over the last-opened
	// project from config.
	if cwd, err := os.Getwd(); err == nil {
		if root := repo.FindRepoRoot(cwd); root != "" {
			if info, _ := repo.GetRepoInfo(root); info != nil {
				cfg.Repos = []config.Repo{{
					Path:  info.Path,
					Owner: info.Owner,
					Name:  info.Name,
				}}
				cfg.LastOpenedDir = info.Path
				cfg.Save()
			}
		}
	}

	if cfg.HasLastOpenedDir() && len(cfg.Repos) > 0 {
		view = ViewDashboard
		mods = views.BuildModules(cfg)
	} else {
		view = ViewGreeting
		mods = make([]modules.Module, 0)
	}

	return App{
		config:    cfg,
		view:      view,
		greeting:  views.NewGreeting(cfg),
		dashboard: views.NewDashboardModel(cfg, mods),
	}
}

func (a App) Init() tea.Cmd {
	if a.view == ViewDashboard {
		return tea.Batch(
			a.dashboard.Init(),
			tickCmd(),
		)
	}
	return tea.SetWindowTitle("project-tracker")
}

func tickCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.greeting = a.greeting.SetSize(msg.Width, msg.Height)
		a.dashboard = a.dashboard.SetSize(msg.Width, msg.Height)

	case TickMsg:
		if a.view == ViewDashboard {
			cmds = append(cmds, tickCmd())
			var cmd tea.Cmd
			a.dashboard, cmd = a.dashboard.Update(views.RefreshMsg{})
			cmds = append(cmds, cmd)
			return a, tea.Batch(cmds...)
		}

	case tea.KeyMsg:
		// Global quit
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
		// Quit from greeting when not in command mode
		if a.view == ViewGreeting && msg.String() == "q" && !a.greeting.CommandInput.Focused {
			return a, tea.Quit
		}

	case components.ExecuteCommandMsg:
		// Handle commands from greeting view
		if a.view == ViewGreeting {
			return a.handleGreetingCommand(msg.Cmd)
		}
	}

	// Route to current view
	if a.view == ViewGreeting {
		var cmd tea.Cmd
		a.greeting, cmd = a.greeting.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		var cmd tea.Cmd
		a.dashboard, cmd = a.dashboard.Update(msg)
		cmds = append(cmds, cmd)
	}

	return a, tea.Batch(cmds...)
}

func (a App) handleGreetingCommand(cmd components.Command) (App, tea.Cmd) {
	switch cmd.Type {
	case components.CmdQuit:
		return a, tea.Quit

	case components.CmdChange:
		if cmd.Arg != "" {
			info, err := repo.GetRepoInfo(cmd.Arg)
			if err != nil || info == nil {
				// Stay on greeting, just reset command mode
				return a, nil
			}

			newRepo := config.Repo{
				Path:  info.Path,
				Owner: info.Owner,
				Name:  info.Name,
			}
			a.config.Repos = []config.Repo{newRepo}
			a.config.LastOpenedDir = info.Path
			a.config.Save()

			// Switch to dashboard view
			a.view = ViewDashboard
			mods := views.BuildModules(a.config)
			a.dashboard = views.NewDashboardModel(a.config, mods)
			a.dashboard = a.dashboard.SetSize(a.width, a.height)
			return a, tea.Batch(a.dashboard.Init(), tickCmd())
		}
		return a, nil

	default:
		return a, nil
	}
}

func (a App) View() string {
	if a.view == ViewGreeting {
		return a.greeting.View()
	}
	return a.dashboard.View()
}
