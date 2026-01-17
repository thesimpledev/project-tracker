package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	PrimaryColor   = lipgloss.Color("62")  // Purple
	SecondaryColor = lipgloss.Color("241") // Gray
	SuccessColor   = lipgloss.Color("42")  // Green
	FailureColor   = lipgloss.Color("196") // Red
	WarningColor   = lipgloss.Color("214") // Orange
	PendingColor   = lipgloss.Color("247") // Light gray

	// Status styles
	SuccessStyle = lipgloss.NewStyle().Foreground(SuccessColor)
	FailureStyle = lipgloss.NewStyle().Foreground(FailureColor)
	PendingStyle = lipgloss.NewStyle().Foreground(PendingColor)
	RunningStyle = lipgloss.NewStyle().Foreground(WarningColor)

	// Layout styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor)

	HelpStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor).
			MarginTop(1)

	SelectedStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor)

	NormalStyle = lipgloss.NewStyle()

	// Item styles
	RepoNameStyle = lipgloss.NewStyle().
			Bold(true)

	BranchStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor)

	DimStyle = lipgloss.NewStyle().
			Foreground(SecondaryColor)

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(SecondaryColor).
			Padding(0, 1)

	// Focus styles
	FocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(PrimaryColor)

	UnfocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("238"))
)

// Status indicators
const (
	StatusIconSuccess = "[ok]"
	StatusIconFailure = "[X]"
	StatusIconRunning = "[~]"
	StatusIconPending = "[?]"
	StatusIconUnknown = "[-]"
)
