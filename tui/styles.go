// Package tui implements an interactive terminal UI for browsing and exporting
// emails from mbox files. It uses the Bubble Tea framework (Elm architecture).
package tui

import "github.com/charmbracelet/lipgloss"

// These are lipgloss styles — think of them as CSS classes for the terminal.
// lipgloss lets you define colors, borders, padding, alignment, and more.
// Each style is immutable; methods like .Border() return a NEW style.

// Color palette — we define colors once and reuse them across styles.
// AdaptiveColor picks the first color on dark terminals, second on light.
var (
	// accentColor is used for highlights, active borders, and the cursor.
	accentColor = lipgloss.Color("#7D56F4") // purple

	// dimColor is used for less important text (help bar, deselected items).
	dimColor = lipgloss.Color("#626262") // grey

	// selectedColor highlights emails the user has marked for export.
	selectedColor = lipgloss.Color("#04B575") // green
)

// Layout styles — these control the overall structure of the TUI.
var (
	// titleBarStyle is the top bar showing the filename and email count.
	titleBarStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(accentColor).
			Padding(0, 1)

	// searchBarStyle wraps the search input field.
	searchBarStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// helpBarStyle is the bottom bar showing available keybindings.
	helpBarStyle = lipgloss.NewStyle().
			Foreground(dimColor).
			Padding(0, 1)

	// statusBarStyle shows status messages (e.g., "Exported 3 emails").
	statusBarStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(selectedColor).
			Padding(0, 1)
)

// List pane styles — the left panel showing the email list.
var (
	// listPaneStyle is the border/frame around the email list.
	listPaneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(dimColor).
			Padding(0, 1)

	// listPaneActiveStyle is used when the list pane is focused.
	listPaneActiveStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(accentColor).
				Padding(0, 1)

	// listItemStyle is the default style for an email in the list.
	listItemStyle = lipgloss.NewStyle()

	// listItemSelectedStyle highlights the email under the cursor.
	listItemCursorStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(accentColor)

	// listItemCheckedStyle shows emails that are marked for export.
	listItemCheckedStyle = lipgloss.NewStyle().
				Foreground(selectedColor)
)

// Preview pane styles — the right panel showing the email content.
var (
	// previewPaneStyle is the border/frame around the preview.
	previewPaneStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(dimColor).
				Padding(0, 1)

	// previewPaneActiveStyle is used when the preview pane is focused.
	previewPaneActiveStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(accentColor).
				Padding(0, 1)

	// previewHeaderStyle styles the email header section (From, To, etc.).
	previewHeaderStyle = lipgloss.NewStyle().
				Bold(true)

	// previewLabelStyle styles the field labels (From:, To:, etc.).
	previewLabelStyle = lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true).
				Width(10)
)

// Export dialog styles — the modal overlay for choosing export options.
var (
	exportDialogStyle = lipgloss.NewStyle().
				Border(lipgloss.DoubleBorder()).
				BorderForeground(accentColor).
				Padding(1, 2).
				Width(50)

	exportDialogTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(accentColor).
				MarginBottom(1)

	exportOptionStyle = lipgloss.NewStyle().
				PaddingLeft(2)

	exportOptionActiveStyle = lipgloss.NewStyle().
				PaddingLeft(2).
				Bold(true).
				Foreground(accentColor)
)
