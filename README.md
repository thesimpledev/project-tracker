# Project Tracker

A modular terminal dashboard for tracking project status. Built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

## Features

- **3x2 grid layout** with 6 module slots
- **Vim-style navigation** (hjkl, :commands)
- **Multiple modules**: CI Status, TODO, Git Status
- **Profile support**: Save and load different project layouts

## Installation

```bash
# Build from source
go build -o project-tracker .

# Run
./project-tracker
```

### Requirements

- Go 1.21+
- [GitHub CLI](https://cli.github.com/) (`gh`) - for CI status module

## Quick Start

1. Run `project-tracker`
2. Press `:` to enter command mode
3. Type `:add /path/to/your/repo` and press Tab to autocomplete
4. Press Enter on a git repository to add it

## Navigation

| Key | Action |
|-----|--------|
| `h` `j` `k` `l` | Move between modules (vim-style) |
| `Arrow keys` | Move between modules |
| `Enter` | Focus selected module |
| `Esc` | Unfocus module / cancel command |
| `:` | Enter command mode |
| `q` | Quit (when not in command mode) |
| `Ctrl+C` | Force quit |

## Commands

Press `:` to enter command mode, then type a command:

| Command | Description |
|---------|-------------|
| `:add <path>` | Add a project directory |
| `:remove <repo>` | Remove a project (owner/name) |
| `:refresh` | Refresh all modules |
| `:save <name>` | Save current layout as profile |
| `:load <name>` | Load a saved profile |
| `:new` | Clear all projects |
| `:q` or `:quit` | Quit |

**Tab completion** is available for paths, repos, and profiles.

---

## Modules

### CI Status (Top Left)

Shows GitHub Actions workflow runs for the repository.

**When focused:**

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate workflow runs |
| `Enter` | View run details and jobs |
| `Esc` | Go back / unfocus |

**Display:**
- Repository name and branch
- Overall CI status indicator (green/red/yellow)
- List of recent workflow runs with status

---

### TODO (Top Middle)

Manages a `TODO.md` file in the project root. Creates the file automatically if it doesn't exist.

**When focused:**

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate items |
| `Space` | Toggle check/uncheck |
| `a` | Add new item |
| `d` or `x` | Delete selected item |
| `Shift+J` | Move item down |
| `Shift+K` | Move item up |
| `Esc` | Unfocus |

**When adding (after pressing `a`):**

| Key | Action |
|-----|--------|
| Type | Enter task text |
| `Enter` | Save new item |
| `Esc` | Cancel |

**Display:**
- Unchecked items at top
- Horizontal divider
- Checked items below (crossed out)

**File format** (`TODO.md`):
```markdown
# TODO

- [ ] Unchecked task
- [x] Completed task
```

---

### Git Status (Bottom Left)

Shows the current branch and file changes. Updates automatically every 3 seconds.

**When focused:**

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate changed files |
| `r` | Manual refresh |
| `Esc` | Unfocus |

**Display:**
- Current branch name
- Summary: staged, modified, untracked counts
- List of changed files with status indicators:
  - `M` (yellow) - Modified
  - `A` (green) - Added/Staged
  - `D` (red) - Deleted
  - `?` (gray) - Untracked

---

## Grid Layout

```
┌─────────────┬─────────────┬─────────────┐
│  CI Status  │    TODO     │ (available) │
├─────────────┼─────────────┼─────────────┤
│ Git Status  │ (available) │ (available) │
└─────────────┴─────────────┴─────────────┘
```

---

## Configuration

Config is stored at `~/.config/project-tracker/config.json`

Profiles are stored at `~/.config/project-tracker/profiles/`

---

## License

MIT
