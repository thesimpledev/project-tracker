package modules

import tea "github.com/charmbracelet/bubbletea"

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
