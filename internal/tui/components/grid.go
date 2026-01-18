package components

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/thesimpledev/project-tracker/internal/clipboard"
	"github.com/thesimpledev/project-tracker/internal/modules"
)

// ClipboardCopyMsg is sent when content is copied to clipboard
type ClipboardCopyMsg struct {
	Success bool
	Error   error
}

const (
	GridCols = 3
	GridRows = 2
)

type GridState int

const (
	GridNavigating GridState = iota
	GridModuleFocused
)

type Grid struct {
	Modules    []modules.Module
	Cursor     int
	State      GridState
	Width      int
	Height     int
	lastYPress time.Time // For detecting 'yy' sequence
}

func NewGrid(mods []modules.Module) Grid {
	return Grid{
		Modules: mods,
		State:   GridNavigating,
		Cursor:  0,
	}
}

func (g Grid) SetSize(width, height int) Grid {
	g.Width = width
	g.Height = height

	// Calculate module dimensions
	moduleWidth := width / GridCols
	moduleHeight := height / GridRows

	for i := range g.Modules {
		g.Modules[i] = g.Modules[i].SetSize(moduleWidth, moduleHeight)
	}

	return g
}

func (g Grid) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, mod := range g.Modules {
		if cmd := mod.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (g Grid) Update(msg tea.Msg) (Grid, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if g.State == GridModuleFocused {
			// Forward to focused module
			if g.Cursor < len(g.Modules) {
				var cmd tea.Cmd
				g.Modules[g.Cursor], cmd = g.Modules[g.Cursor].Update(msg)
				cmds = append(cmds, cmd)
			}

			// Handle escape to unfocus
			if msg.String() == "esc" {
				g.State = GridNavigating
				if g.Cursor < len(g.Modules) {
					g.Modules[g.Cursor] = g.Modules[g.Cursor].SetFocused(false)
				}
			}
			return g, tea.Batch(cmds...)
		}

		// Grid navigation mode
		switch msg.String() {
		case "h", "left":
			g = g.moveCursor(-1, 0)
		case "l", "right":
			g = g.moveCursor(1, 0)
		case "k", "up":
			g = g.moveCursor(0, -1)
		case "j", "down":
			g = g.moveCursor(0, 1)
		case "enter":
			if g.Cursor < len(g.Modules) {
				g.State = GridModuleFocused
				g.Modules[g.Cursor] = g.Modules[g.Cursor].SetFocused(true)
			}
		case "y":
			// Check for 'yy' sequence (two y presses within 500ms)
			now := time.Now()
			if now.Sub(g.lastYPress) < 500*time.Millisecond {
				// yy detected - copy module content to clipboard
				if g.Cursor < len(g.Modules) {
					if copyable, ok := g.Modules[g.Cursor].(modules.Copyable); ok {
						content := copyable.GetCopyContent()
						if content != "" {
							err := clipboard.Copy(content)
							cmds = append(cmds, func() tea.Msg {
								return ClipboardCopyMsg{Success: err == nil, Error: err}
							})
						}
					}
				}
				g.lastYPress = time.Time{} // Reset
			} else {
				g.lastYPress = now
			}
		}

	default:
		// Forward other messages to all modules
		for i := range g.Modules {
			var cmd tea.Cmd
			g.Modules[i], cmd = g.Modules[i].Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	}

	return g, tea.Batch(cmds...)
}

func (g Grid) moveCursor(dx, dy int) Grid {
	if len(g.Modules) == 0 {
		return g
	}

	// Current position
	col := g.Cursor % GridCols
	row := g.Cursor / GridCols

	// Calculate new position
	newCol := col + dx
	newRow := row + dy

	// Clamp to grid bounds
	if newCol < 0 {
		newCol = 0
	}
	if newCol >= GridCols {
		newCol = GridCols - 1
	}
	if newRow < 0 {
		newRow = 0
	}
	maxRow := (len(g.Modules) - 1) / GridCols
	if newRow > maxRow {
		newRow = maxRow
	}

	newCursor := newRow*GridCols + newCol
	if newCursor >= len(g.Modules) {
		newCursor = len(g.Modules) - 1
	}

	g.Cursor = newCursor
	return g
}

func (g Grid) RefreshAll() tea.Cmd {
	var cmds []tea.Cmd
	for _, mod := range g.Modules {
		if cmd := mod.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (g Grid) SelectedModule() modules.Module {
	if g.Cursor < len(g.Modules) {
		return g.Modules[g.Cursor]
	}
	return nil
}

func (g Grid) View() string {
	if len(g.Modules) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("241")).
			Width(g.Width).
			Height(g.Height).
			Align(lipgloss.Center, lipgloss.Center)
		return emptyStyle.Render("No modules loaded.\nType :add to add a project.")
	}

	moduleWidth := g.Width / GridCols
	moduleHeight := g.Height / GridRows

	var rows []string

	for row := 0; row < GridRows; row++ {
		var rowModules []string
		for col := 0; col < GridCols; col++ {
			idx := row*GridCols + col
			if idx < len(g.Modules) {
				// Check if this module should span remaining columns
				nextIdx := idx + 1
				remainingCols := GridCols - col
				isLastInRow := nextIdx >= len(g.Modules) || nextIdx >= (row+1)*GridCols

				// Calculate width: if this is the last module in the row and there are
				// empty slots after it, expand to fill them
				effectiveWidth := moduleWidth
				if isLastInRow && remainingCols > 1 && nextIdx >= len(g.Modules) {
					effectiveWidth = moduleWidth * remainingCols
				}

				mod := g.Modules[idx].SetSize(effectiveWidth, moduleHeight)

				// Set selection state (cursor on module but not focused)
				isSelected := idx == g.Cursor && g.State == GridNavigating
				mod = mod.SetSelected(isSelected)

				rowModules = append(rowModules, mod.View())

				// Skip remaining columns if we expanded
				if effectiveWidth > moduleWidth {
					break
				}
			} else {
				// Empty cell
				emptyStyle := lipgloss.NewStyle().
					Border(lipgloss.RoundedBorder()).
					BorderForeground(lipgloss.Color("236")).
					Width(moduleWidth-2).
					Height(moduleHeight-2).
					Align(lipgloss.Center, lipgloss.Center).
					Foreground(lipgloss.Color("241"))
				rowModules = append(rowModules, emptyStyle.Render(""))
			}
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, rowModules...))
	}

	return strings.Join(rows, "\n")
}
