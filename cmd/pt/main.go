package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/thesimpledev/project-tracker/internal/config"
	"github.com/thesimpledev/project-tracker/internal/github"
	"github.com/thesimpledev/project-tracker/internal/tui"

	// Register modules
	_ "github.com/thesimpledev/project-tracker/internal/modules/ci_status"
	_ "github.com/thesimpledev/project-tracker/internal/modules/git_status"
	_ "github.com/thesimpledev/project-tracker/internal/modules/just_commands"
	_ "github.com/thesimpledev/project-tracker/internal/modules/notes"
	_ "github.com/thesimpledev/project-tracker/internal/modules/placeholder"
	_ "github.com/thesimpledev/project-tracker/internal/modules/test_runner"
	_ "github.com/thesimpledev/project-tracker/internal/modules/todo"
)

func main() {
	if !github.IsGHInstalled() {
		fmt.Fprintln(os.Stderr, "Error: gh CLI is not installed.")
		fmt.Fprintln(os.Stderr, "Please install it from: https://cli.github.com/")
		os.Exit(1)
	}

	if !github.IsAuthenticated() {
		fmt.Fprintln(os.Stderr, "Error: gh CLI is not authenticated.")
		fmt.Fprintln(os.Stderr, "Please run: gh auth login")
		os.Exit(1)
	}

	// Start pprof server for profiling
	go func() {
		http.ListenAndServe("localhost:6060", nil)
	}()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	app := tui.NewApp(cfg)
	p := tea.NewProgram(app, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
