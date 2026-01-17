package components

import (
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thesimpledev/project-tracker/internal/config"
	"github.com/thesimpledev/project-tracker/internal/repo"
)

type CommandType int

const (
	CmdUnknown CommandType = iota
	CmdAdd
	CmdRemove
	CmdRefresh
	CmdQuit
	CmdSave
	CmdLoad
	CmdNew
)

type Command struct {
	Type CommandType
	Arg  string
}

type CommandInput struct {
	Input           string
	Focused         bool
	Suggestions     []string
	SuggestionIdx   int
	ShowSuggestions bool
	Width           int
	repos           []config.Repo
	LastPath        string
}

type ExecuteCommandMsg struct {
	Cmd Command
}

func NewCommandInput(repos []config.Repo) CommandInput {
	return CommandInput{
		repos: repos,
	}
}

func (c CommandInput) SetSize(width int) CommandInput {
	c.Width = width
	return c
}

func (c CommandInput) SetRepos(repos []config.Repo) CommandInput {
	c.repos = repos
	return c
}

func (c CommandInput) SetLastPath(path string) CommandInput {
	c.LastPath = filepath.Dir(path)
	return c
}

func (c CommandInput) SetFocused(focused bool) CommandInput {
	c.Focused = focused
	if focused {
		c.Input = ":"
		c.updateSuggestions()
	} else {
		c.Input = ""
		c.Suggestions = nil
		c.ShowSuggestions = false
	}
	return c
}

func (c CommandInput) Update(msg tea.Msg) (CommandInput, tea.Cmd) {
	if !c.Focused {
		return c, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			c.Focused = false
			c.Input = ""
			c.Suggestions = nil
			c.ShowSuggestions = false
			return c, nil

		case "enter":
			if c.ShowSuggestions && len(c.Suggestions) > 0 {
				c = c.acceptSuggestion()
				return c, nil
			}
			cmd := c.parseCommand()
			c.Focused = false
			c.Input = ""
			c.Suggestions = nil
			c.ShowSuggestions = false
			return c, func() tea.Msg { return ExecuteCommandMsg{Cmd: cmd} }

		case "tab":
			if len(c.Suggestions) > 0 {
				suggestion := c.Suggestions[c.SuggestionIdx]

				if strings.Contains(suggestion, " <") {
					idx := strings.Index(suggestion, " <")
					c.Input = suggestion[:idx+1]
					c.updateSuggestions()
					return c, nil
				}

				isRepo := strings.HasSuffix(suggestion, " [repo]")
				isAddPath := strings.HasPrefix(suggestion, ":add ")
				suggestion = strings.TrimSuffix(suggestion, " [repo]")

				if isRepo {
					c.Input = suggestion
				} else if isAddPath && !isRepo {
					c.Input = suggestion + "/"
				} else {
					c.Input = suggestion
				}
				c.updateSuggestions()
			}
			return c, nil

		case "shift+tab", "up":
			if len(c.Suggestions) > 0 {
				c.ShowSuggestions = true
				c.SuggestionIdx--
				if c.SuggestionIdx < 0 {
					c.SuggestionIdx = len(c.Suggestions) - 1
				}
			}
			return c, nil

		case "down":
			if len(c.Suggestions) > 0 {
				c.ShowSuggestions = true
				c.SuggestionIdx = (c.SuggestionIdx + 1) % len(c.Suggestions)
			}
			return c, nil

		case "backspace":
			if len(c.Input) > 0 {
				c.Input = c.Input[:len(c.Input)-1]
				c.updateSuggestions()
			}
			return c, nil

		default:
			if len(msg.String()) == 1 {
				c.Input += msg.String()
				c.updateSuggestions()
			}
			return c, nil
		}
	}

	return c, nil
}

func (c *CommandInput) updateSuggestions() {
	c.Suggestions = nil
	c.SuggestionIdx = 0
	c.ShowSuggestions = false

	input := strings.TrimPrefix(c.Input, ":")
	parts := strings.SplitN(input, " ", 2)
	cmd := parts[0]
	arg := ""
	if len(parts) > 1 {
		arg = parts[1]
	}

	type cmdInfo struct {
		name string
		hint string
	}
	commands := []cmdInfo{
		{"add", "<path>"},
		{"remove", "<repo>"},
		{"save", "<name>"},
		{"load", "<profile>"},
		{"new", ""},
		{"refresh", ""},
		{"quit", ""},
		{"q", ""},
	}

	if arg == "" && !strings.Contains(input, " ") {
		for _, command := range commands {
			if strings.HasPrefix(command.name, cmd) {
				suggestion := ":" + command.name
				if command.hint != "" {
					suggestion += " " + command.hint
				}
				c.Suggestions = append(c.Suggestions, suggestion)
			}
		}
	} else {
		switch cmd {
		case "add":
			c.Suggestions = c.completePath(arg)
		case "remove":
			c.Suggestions = c.completeRepo(arg)
		case "load":
			c.Suggestions = c.completeProfile(arg)
		}
	}
}

func (c *CommandInput) completePath(partial string) []string {
	var suggestions []string

	dir := "."
	if c.LastPath != "" {
		dir = c.LastPath
	}
	prefix := ""

	if partial != "" {
		if strings.HasSuffix(partial, "/") {
			dir = partial
			prefix = ""
		} else {
			dir = filepath.Dir(partial)
			prefix = filepath.Base(partial)
		}
	}

	dir = filepath.Clean(dir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return suggestions
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if prefix != "" && !strings.HasPrefix(entry.Name(), prefix) {
			continue
		}

		fullPath := filepath.Join(dir, entry.Name())

		isGitRepo := repo.IsGitRepo(fullPath)
		suggestion := ":add " + fullPath
		if isGitRepo {
			suggestion += " [repo]"
		}
		suggestions = append(suggestions, suggestion)

		if len(suggestions) >= 10 {
			break
		}
	}

	return suggestions
}

func (c *CommandInput) completeRepo(partial string) []string {
	var suggestions []string

	for _, r := range c.repos {
		name := r.Owner + "/" + r.Name
		if partial == "" || strings.Contains(strings.ToLower(name), strings.ToLower(partial)) {
			suggestions = append(suggestions, ":remove "+name)
		}
		if len(suggestions) >= 10 {
			break
		}
	}

	return suggestions
}

func (c *CommandInput) completeProfile(partial string) []string {
	var suggestions []string

	profiles, err := config.ListProfiles()
	if err != nil {
		return suggestions
	}

	for _, profile := range profiles {
		if partial == "" || strings.HasPrefix(strings.ToLower(profile), strings.ToLower(partial)) {
			suggestions = append(suggestions, ":load "+profile)
		}
		if len(suggestions) >= 10 {
			break
		}
	}

	return suggestions
}

func (c CommandInput) acceptSuggestion() CommandInput {
	if c.SuggestionIdx < len(c.Suggestions) {
		suggestion := c.Suggestions[c.SuggestionIdx]
		suggestion = strings.TrimSuffix(suggestion, " [repo]")
		c.Input = suggestion
		c.ShowSuggestions = false
		c.updateSuggestions()
	}
	return c
}

func (c CommandInput) parseCommand() Command {
	input := strings.TrimPrefix(c.Input, ":")
	parts := strings.SplitN(input, " ", 2)
	cmd := parts[0]
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSuffix(parts[1], " [repo]")
	}

	switch cmd {
	case "add":
		return Command{Type: CmdAdd, Arg: arg}
	case "remove":
		return Command{Type: CmdRemove, Arg: arg}
	case "save":
		return Command{Type: CmdSave, Arg: arg}
	case "load":
		return Command{Type: CmdLoad, Arg: arg}
	case "new":
		return Command{Type: CmdNew}
	case "refresh":
		return Command{Type: CmdRefresh}
	case "quit", "q":
		return Command{Type: CmdQuit}
	default:
		return Command{Type: CmdUnknown}
	}
}

func (c CommandInput) View() string {
	var b strings.Builder

	// Input line first
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Width(c.Width-2).
		Padding(0, 1)

	prompt := "> "
	if c.Focused {
		promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("62"))
		inputLine := promptStyle.Render(prompt) + c.Input + "█"
		b.WriteString(inputStyle.Render(inputLine))
	} else {
		dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		inputLine := dimStyle.Render(prompt + "Type : to enter command...")
		b.WriteString(inputStyle.Render(inputLine))
	}

	// Suggestions BELOW the input line
	if c.Focused && len(c.Suggestions) > 0 {
		b.WriteString("\n")

		// Check if these are command suggestions (contain <) or path suggestions
		isCommandHints := len(c.Suggestions) > 0 && strings.Contains(c.Suggestions[0], " <")

		if isCommandHints {
			// Show commands inline on a single line
			hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
			var cmds []string
			for _, sug := range c.Suggestions {
				// Extract just the command name
				parts := strings.SplitN(sug, " ", 2)
				cmds = append(cmds, parts[0])
			}
			b.WriteString(hintStyle.Render("  " + strings.Join(cmds, "  ")))
		} else {
			// Show path/repo suggestions as vertical list
			sugStyle := lipgloss.NewStyle().
				Foreground(lipgloss.Color("241")).
				PaddingLeft(2)

			maxSuggestions := 5
			for i, sug := range c.Suggestions {
				if i >= maxSuggestions {
					b.WriteString(sugStyle.Render("  ..."))
					break
				}

				prefix := "  "
				style := sugStyle
				if c.ShowSuggestions && i == c.SuggestionIdx {
					prefix = "> "
					style = lipgloss.NewStyle().
						Foreground(lipgloss.Color("212")).
						Bold(true).
						PaddingLeft(2)
				}

				display := sug
				repoColor := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

				if strings.HasSuffix(sug, " [repo]") {
					base := strings.TrimSuffix(sug, " [repo]")
					display = base + repoColor.Render(" [repo]")
				}
				b.WriteString(style.Render(prefix+display) + "\n")
			}
		}
	} else if c.Focused {
		// Show inline hint when focused but no suggestions yet
		b.WriteString("\n")
		hintStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
		b.WriteString(hintStyle.Render("  :add  :remove  :refresh  :quit"))
	}

	return b.String()
}
