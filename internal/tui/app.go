package tui

import (
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

	if cfg.HasLastOpenedDir() && len(cfg.Repos) > 0 {
		view = ViewDashboard
		mods = buildModules(cfg)
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

func buildModules(cfg *config.Config) []modules.Module {
	var mods []modules.Module

	// Add modules for each repo
	// Layout: Row 1: CI, TODO, Notes | Row 2: Git Status, placeholder, Tests
	for _, r := range cfg.Repos {
		// Row 1: CI status module (top left)
		ciMod := modules.Create("ci_status")
		if ciMod == nil {
			ciMod = modules.Create("placeholder")
		}
		if ciMod != nil {
			if setter, ok := ciMod.(interface {
				SetRepo(config.Repo) modules.Module
			}); ok {
				ciMod = setter.SetRepo(r)
			}
			mods = append(mods, ciMod)
		}

		// Row 1: TODO module (top middle)
		todoMod := modules.Create("todo")
		if todoMod == nil {
			todoMod = modules.Create("placeholder")
		}
		if todoMod != nil {
			if setter, ok := todoMod.(interface {
				SetRepo(config.Repo) modules.Module
			}); ok {
				todoMod = setter.SetRepo(r)
			}
			mods = append(mods, todoMod)
		}

		// Row 1: Notes module (top right)
		notesMod := modules.Create("notes")
		if notesMod == nil {
			notesMod = modules.Create("placeholder")
		}
		if notesMod != nil {
			if setter, ok := notesMod.(interface {
				SetRepo(config.Repo) modules.Module
			}); ok {
				notesMod = setter.SetRepo(r)
			}
			mods = append(mods, notesMod)
		}

		// Row 2: Git status module (lower left)
		gitMod := modules.Create("git_status")
		if gitMod == nil {
			gitMod = modules.Create("placeholder")
		}
		if gitMod != nil {
			if setter, ok := gitMod.(interface {
				SetRepo(config.Repo) modules.Module
			}); ok {
				gitMod = setter.SetRepo(r)
			}
			mods = append(mods, gitMod)
		}

		// Row 2: Placeholder (lower middle)
		if placeholder := modules.Create("placeholder"); placeholder != nil {
			mods = append(mods, placeholder)
		}

		// Row 2: Test runner module (lower right)
		testMod := modules.Create("test_runner")
		if testMod == nil {
			testMod = modules.Create("placeholder")
		}
		if testMod != nil {
			if setter, ok := testMod.(interface {
				SetRepo(config.Repo) modules.Module
			}); ok {
				testMod = setter.SetRepo(r)
			}
			mods = append(mods, testMod)
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

	case components.CmdAdd:
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
			a.config.AddRepo(newRepo)
			a.config.LastOpenedDir = info.Path
			a.config.Save()

			// Now switch to dashboard view
			a.view = ViewDashboard
			mods := buildModules(a.config)
			a.dashboard = views.NewDashboardModel(a.config, mods)
			a.dashboard = a.dashboard.SetSize(a.width, a.height)
			return a, tea.Batch(a.dashboard.Init(), tickCmd())
		}
		return a, nil

	case components.CmdLoad:
		if cmd.Arg != "" {
			loadedCfg, err := config.LoadProfile(cmd.Arg)
			if err == nil && loadedCfg != nil && len(loadedCfg.Repos) > 0 {
				a.config.Repos = loadedCfg.Repos
				a.config.ProfileName = cmd.Arg
				a.config.LastOpenedDir = loadedCfg.Repos[0].Path
				a.config.Save()

				// Switch to dashboard
				a.view = ViewDashboard
				mods := buildModules(a.config)
				a.dashboard = views.NewDashboardModel(a.config, mods)
				a.dashboard = a.dashboard.SetSize(a.width, a.height)
				return a, tea.Batch(a.dashboard.Init(), tickCmd())
			}
		}
		return a, nil

	default:
		// Other commands don't cause view switch
		return a, nil
	}
}

func (a App) View() string {
	if a.view == ViewGreeting {
		return a.greeting.View()
	}
	return a.dashboard.View()
}
