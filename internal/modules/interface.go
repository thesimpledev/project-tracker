package modules

import tea "github.com/charmbracelet/bubbletea"

// Modules that hold a repo (most do) should expose
//
//	SetRepo(config.Repo) Module
//
// via duck typing — see views.BuildModules. When the path changes, SetRepo is
// expected to clear any cached per-repo state so a future module reuse can't
// leak data across projects. just_commands.SetRepo is the canonical example.
type Module interface {
	ID() string
	Name() string
	Init() tea.Cmd
	Update(msg tea.Msg) (Module, tea.Cmd)
	View() string
	SetSize(width, height int) Module
	SetSelected(selected bool) Module
	SetFocused(focused bool) Module
	IsFocused() bool
}

// Copyable is an optional interface modules can implement to support
// copying their content to clipboard with the 'yy' command
type Copyable interface {
	GetCopyContent() string
}

type ModuleFactory func() Module

var registry = map[string]ModuleFactory{}

func Register(id string, factory ModuleFactory) {
	registry[id] = factory
}

func Create(id string) Module {
	if factory, ok := registry[id]; ok {
		return factory()
	}
	return nil
}

func ListModuleTypes() []string {
	var types []string
	for id := range registry {
		types = append(types, id)
	}
	return types
}
