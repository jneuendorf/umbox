package tui

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	// Bubble Tea is the TUI framework. "tea" is the conventional alias.
	tea "github.com/charmbracelet/bubbletea"

	// bubbles are pre-built Bubble Tea components.
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"

	// lipgloss is for terminal styling (colors, borders, layout).
	"github.com/charmbracelet/lipgloss"

	// Our own packages — the TUI is just a frontend over these.
	"github.com/jneuendorf/umbox/formatter"
	"github.com/jneuendorf/umbox/mbox"
)

// --------------------------------------------------------------------------
// Constants
// --------------------------------------------------------------------------

// pane identifies which panel is focused.
const (
	paneList    = 0 // left panel — email list
	panePreview = 1 // right panel — email preview
)

// listWidthFraction controls how much of the terminal the list pane uses.
const listWidthFraction = 0.35

// --------------------------------------------------------------------------
// Model — the entire state of the TUI
// --------------------------------------------------------------------------

// Model holds all state for the TUI. In Bubble Tea's Elm architecture,
// this is the single source of truth. The View function renders it,
// and the Update function produces a new Model in response to events.
type Model struct {
	// --- Data ---
	filepath string           // path to the mbox file
	messages []*mbox.Message  // all parsed emails
	filtered []int            // indices into messages that match the search

	// --- List pane ---
	cursor int  // position within the filtered list
	offset int  // scroll offset (first visible item index)
	selected map[int]bool // checked emails (key = index in messages)

	// --- Preview pane ---
	viewport viewport.Model // scrollable content viewer

	// --- Search ---
	searchInput textinput.Model // the search text field
	searching   bool            // true when search input is focused
	searchQuery string          // current applied search filter

	// --- Export dialog ---
	exporting    bool            // true when export dialog is open
	exportFormat int             // selected format index
	exportDir    textinput.Model // output directory field
	statusMsg    string          // success/error message after export

	// --- Layout ---
	focusedPane  int  // which pane has focus (paneList or panePreview)
	width        int  // terminal width in columns
	height       int  // terminal height in rows
	chromeHeight int  // measured height of title + search + help bars (may be >3 if text wraps)
	ready        bool // true after we've received the first WindowSizeMsg
}

// --------------------------------------------------------------------------
// Run — entry point called from cmd/browse.go
// --------------------------------------------------------------------------

// Run parses the mbox file and starts the interactive TUI.
// This is the function that cmd/browse.go calls — it bridges the CLI and TUI.
func Run(mboxPath string) error {
	// Parse emails using our existing mbox package.
	messages, err := mbox.Parse(mboxPath)
	if err != nil {
		return err
	}
	if len(messages) == 0 {
		return fmt.Errorf("no emails found in %s", mboxPath)
	}

	// Create the initial model.
	m := newModel(mboxPath, messages)

	// tea.NewProgram creates and runs the Bubble Tea application.
	// WithAltScreen puts the TUI in an alternate screen buffer so it
	// doesn't pollute the user's scrollback when they quit.
	// WithMouseCellMotion enables mouse support.
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	// Run blocks until the user quits. The final model is returned.
	_, err = p.Run()
	return err
}

// newModel creates the initial Model with sensible defaults.
func newModel(path string, messages []*mbox.Message) Model {
	// Set up the search input field.
	si := textinput.New()
	si.Placeholder = "type to filter..."
	si.CharLimit = 100

	// Set up the export directory input field.
	ed := textinput.New()
	ed.Placeholder = "./export"
	ed.SetValue("./export")
	ed.CharLimit = 200

	// Build the initial filtered list (all emails).
	filtered := make([]int, len(messages))
	for i := range messages {
		filtered[i] = i
	}

	return Model{
		filepath:    path,
		messages:    messages,
		filtered:    filtered,
		selected:    make(map[int]bool),
		searchInput: si,
		exportDir:   ed,
		viewport:    viewport.New(0, 0), // sized later in WindowSizeMsg
	}
}

// --------------------------------------------------------------------------
// Bubble Tea interface: Init, Update, View
// --------------------------------------------------------------------------

// Init is called once when the program starts. It can return a Cmd (an async
// side effect) but we don't need one — our data is already loaded.
func (m Model) Init() tea.Cmd {
	// Return nil means "no initial command to run."
	return nil
}

// Update is the heart of the Elm architecture. It receives a message (user
// input, window resize, timer tick, etc.) and returns an updated Model plus
// an optional Cmd for side effects.
//
// IMPORTANT: Update must return a NEW model, not modify the existing one.
// In practice, Go copies the struct on assignment, so modifying `m` here
// is fine — it's already a copy because Model is a value receiver.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// tea.Msg is an interface — we use a type switch to handle each kind.
	switch msg := msg.(type) {

	// WindowSizeMsg is sent when the terminal is resized (and once at startup).
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m = m.recalcLayout()
		return m, nil

	// KeyMsg is sent when the user presses a key.
	case tea.KeyMsg:
		return m.handleKey(msg)

	// MouseMsg handles mouse wheel scrolling in both panes.
	case tea.MouseMsg:
		if m.focusedPane == panePreview {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		// Scroll the email list with the mouse wheel.
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			m = m.moveCursor(-3)
			m = m.updatePreview()
		case tea.MouseButtonWheelDown:
			m = m.moveCursor(3)
			m = m.updatePreview()
		}
		return m, nil
	}

	return m, nil
}

// View renders the entire TUI as a string. Bubble Tea calls this after every
// Update to redraw the screen. The string can contain ANSI escape codes
// (which lipgloss generates for colors/borders).
func (m Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	// If the export dialog is open, render it as an overlay.
	if m.exporting {
		return m.viewExportDialog()
	}

	// Build the UI from top to bottom:
	// 1. Title bar
	// 2. Search bar
	// 3. Main content (list + preview side by side)
	// 4. Help bar

	titleBar := titleBarStyle.Width(m.width).Render(
		fmt.Sprintf(" umbox — %s — %d emails (%d shown)",
			m.filepath, len(m.messages), len(m.filtered)))

	searchBar := m.viewSearchBar()

	mainContent := m.viewMainContent()

	// Build the help/status bar.
	var bottomBar string
	if m.statusMsg != "" {
		bottomBar = statusBarStyle.Width(m.width).Render(m.statusMsg)
	} else {
		bottomBar = helpBarStyle.Width(m.width).Render(helpText())
	}

	// Measure chrome height for next frame's layout calculations.
	chromeHeight := lipgloss.Height(titleBar) + lipgloss.Height(searchBar) + lipgloss.Height(bottomBar)
	m.chromeHeight = chromeHeight

	// Stack everything vertically.
	return lipgloss.JoinVertical(lipgloss.Left,
		titleBar,
		searchBar,
		mainContent,
		bottomBar,
	)
}

// --------------------------------------------------------------------------
// Key handling
// --------------------------------------------------------------------------

// handleKey processes keyboard input. This is where most of the interaction
// logic lives. It dispatches to different handlers depending on the current
// state (searching, exporting, or normal browsing).
func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If the search input is focused, send keys to it.
	if m.searching {
		return m.handleSearchKey(msg)
	}

	// If the export dialog is open, handle export-specific keys.
	if m.exporting {
		return m.handleExportKey(msg)
	}

	// Normal mode — handle browsing keys.
	switch {
	case key.Matches(msg, keys.Quit):
		// tea.Quit is a built-in Cmd that tells Bubble Tea to exit.
		return m, tea.Quit

	case key.Matches(msg, keys.Up):
		m = m.moveCursor(-1)

	case key.Matches(msg, keys.Down):
		m = m.moveCursor(1)

	case key.Matches(msg, keys.Tab):
		// Toggle focus between list and preview.
		if m.focusedPane == paneList {
			m.focusedPane = panePreview
		} else {
			m.focusedPane = paneList
		}

	case key.Matches(msg, keys.Space):
		// Toggle selection on the current email.
		if len(m.filtered) > 0 {
			msgIdx := m.filtered[m.cursor]
			if m.selected[msgIdx] {
				delete(m.selected, msgIdx)
			} else {
				m.selected[msgIdx] = true
			}
		}

	case key.Matches(msg, keys.All):
		// Toggle select/deselect all *currently filtered* emails.
		// Check if every filtered email is already selected.
		allFilteredSelected := len(m.filtered) > 0
		for _, idx := range m.filtered {
			if !m.selected[idx] {
				allFilteredSelected = false
				break
			}
		}

		if allFilteredSelected {
			// Deselect only the filtered emails (keep others selected).
			for _, idx := range m.filtered {
				delete(m.selected, idx)
			}
		} else {
			// Select all filtered emails (keep existing selections too).
			for _, idx := range m.filtered {
				m.selected[idx] = true
			}
		}

	case key.Matches(msg, keys.Search):
		// Enter search mode — focus the search input.
		m.searching = true
		m.searchInput.Focus()
		m.statusMsg = "" // clear any status message
		return m, textinput.Blink // start cursor blinking

	case key.Matches(msg, keys.Export):
		// Open the export dialog (only if emails are selected).
		if len(m.selected) > 0 {
			m.exporting = true
			m.exportDir.Focus()
			return m, textinput.Blink
		} else {
			m.statusMsg = "Select emails first (space to toggle, a for all)"
		}

	case key.Matches(msg, keys.Escape):
		m.statusMsg = "" // clear status message
	}

	// Update the preview pane to show the currently highlighted email.
	m = m.updatePreview()

	return m, nil
}

// handleSearchKey processes keys while the search input is focused.
func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Enter):
		// Apply the search and return to normal mode.
		m.searching = false
		m.searchInput.Blur() // remove focus from the input
		m.searchQuery = m.searchInput.Value()
		m = m.applyFilter()
		m = m.updatePreview()
		return m, nil

	case key.Matches(msg, keys.Escape):
		// Cancel search — clear the input and show all emails.
		m.searching = false
		m.searchInput.Blur()
		m.searchInput.SetValue("")
		m.searchQuery = ""
		m = m.applyFilter()
		m = m.updatePreview()
		return m, nil

	default:
		// Forward all other keys to the text input component.
		// This handles typing, backspace, cursor movement, etc.
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		// Live-filter as the user types.
		m.searchQuery = m.searchInput.Value()
		m = m.applyFilter()
		m = m.updatePreview()
		return m, cmd
	}
}

// handleExportKey processes keys while the export dialog is open.
func (m Model) handleExportKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, keys.Escape):
		m.exporting = false
		return m, nil

	case key.Matches(msg, keys.Tab):
		// Cycle through format options.
		formats := formatter.List()
		m.exportFormat = (m.exportFormat + 1) % len(formats)
		return m, nil

	case key.Matches(msg, keys.Enter):
		// Perform the export!
		m.exporting = false
		m = m.doExport()
		return m, nil

	default:
		// Forward to the directory text input.
		var cmd tea.Cmd
		m.exportDir, cmd = m.exportDir.Update(msg)
		return m, cmd
	}
}

// --------------------------------------------------------------------------
// List logic
// --------------------------------------------------------------------------

// moveCursor moves the list cursor by delta (-1 for up, +1 for down).
// It handles bounds checking and scrolling.
func (m Model) moveCursor(delta int) Model {
	if len(m.filtered) == 0 {
		return m
	}

	// Move cursor within bounds.
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}

	// Adjust scroll offset to keep cursor visible.
	visibleRows := m.listHeight()
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visibleRows {
		m.offset = m.cursor - visibleRows + 1
	}

	return m
}

// applyFilter updates the filtered list based on the current search query.
// It searches in the From, To, Subject, and body fields (case-insensitive).
func (m Model) applyFilter() Model {
	query := strings.ToLower(m.searchQuery)

	if query == "" {
		// No filter — show all emails.
		m.filtered = make([]int, len(m.messages))
		for i := range m.messages {
			m.filtered[i] = i
		}
	} else {
		// Filter: check if any field contains the query string.
		m.filtered = nil // reset to empty slice
		for i, msg := range m.messages {
			if strings.Contains(strings.ToLower(msg.From), query) ||
				strings.Contains(strings.ToLower(msg.Subject), query) ||
				strings.Contains(strings.ToLower(msg.TextBody), query) ||
				strings.Contains(strings.ToLower(strings.Join(msg.To, " ")), query) {
				m.filtered = append(m.filtered, i)
			}
		}
	}

	// Reset cursor to stay within the new filtered list.
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
	m.offset = 0

	return m
}

// --------------------------------------------------------------------------
// Preview logic
// --------------------------------------------------------------------------

// updatePreview regenerates the preview pane content for the currently
// highlighted email.
func (m Model) updatePreview() Model {
	if len(m.filtered) == 0 {
		m.viewport.SetContent("No emails to display")
		return m
	}

	msgIdx := m.filtered[m.cursor]
	msg := m.messages[msgIdx]

	// Build a nicely formatted preview string.
	var b strings.Builder

	// Header section.
	b.WriteString(previewLabelStyle.Render("From:"))
	b.WriteString("  " + msg.From + "\n")

	b.WriteString(previewLabelStyle.Render("To:"))
	b.WriteString("  " + strings.Join(msg.To, ", ") + "\n")

	b.WriteString(previewLabelStyle.Render("Date:"))
	b.WriteString("  " + msg.Date.Format("Mon, 02 Jan 2006 15:04:05 -0700") + "\n")

	b.WriteString(previewLabelStyle.Render("Subject:"))
	b.WriteString("  " + msg.Subject + "\n")

	// Separator.
	previewWidth := m.previewWidth() - 4 // account for border + padding
	if previewWidth < 10 {
		previewWidth = 10
	}
	b.WriteString(strings.Repeat("─", previewWidth) + "\n\n")

	// Body.
	body := msg.TextBody
	if body == "" {
		body = msg.HTMLBody
	}
	if body == "" {
		body = "(no body)"
	}
	b.WriteString(body)

	// Attachments section.
	if msg.HasAttachments() {
		b.WriteString("\n\n" + strings.Repeat("─", previewWidth) + "\n")
		b.WriteString(fmt.Sprintf("Attachments (%d):\n", len(msg.Attachments)))
		for i, att := range msg.Attachments {
			name := att.Filename
			if name == "" {
				name = "(unnamed)"
			}
			b.WriteString(fmt.Sprintf("  %d. %s (%s, %d bytes)\n",
				i+1, name, att.ContentType, len(att.Data)))
		}
	}

	m.viewport.SetContent(b.String())
	m.viewport.GotoTop()

	return m
}

// --------------------------------------------------------------------------
// Export logic
// --------------------------------------------------------------------------

// doExport writes the selected emails using the chosen formatter.
// This reuses the same formatter package that the CLI extract command uses.
func (m Model) doExport() Model {
	formats := formatter.List()
	if m.exportFormat >= len(formats) {
		m.statusMsg = "Error: invalid format"
		return m
	}

	formatName := formats[m.exportFormat]
	fmtr, err := formatter.Get(formatName)
	if err != nil {
		m.statusMsg = fmt.Sprintf("Error: %v", err)
		return m
	}

	outputDir := m.exportDir.Value()
	if outputDir == "" {
		outputDir = "./export"
	}

	// Create the output directory.
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		m.statusMsg = fmt.Sprintf("Error creating directory: %v", err)
		return m
	}

	exported := 0
	for msgIdx := range m.selected {
		msg := m.messages[msgIdx]

		// Write the formatted email.
		filename := fmt.Sprintf("%03d%s", msgIdx+1, fmtr.Extension())
		filePath := filepath.Join(outputDir, filename)

		var buf bytes.Buffer
		if err := fmtr.Format(msg, &buf); err != nil {
			m.statusMsg = fmt.Sprintf("Error formatting email %d: %v", msgIdx+1, err)
			return m
		}
		if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
			m.statusMsg = fmt.Sprintf("Error writing %s: %v", filePath, err)
			return m
		}

		// Save attachments to a subfolder (skip for raw format since .eml
		// already contains attachments inline).
		if fmtr.Name() != "raw" && msg.HasAttachments() {
			attDir := filepath.Join(outputDir, fmt.Sprintf("%03d_attachments", msgIdx+1))
			if err := os.MkdirAll(attDir, 0755); err != nil {
				m.statusMsg = fmt.Sprintf("Error creating attachments dir: %v", err)
				return m
			}
			for j, att := range msg.Attachments {
				attName := att.Filename
				if attName == "" {
					attName = fmt.Sprintf("attachment_%d", j+1)
				}
				if err := os.WriteFile(filepath.Join(attDir, attName), att.Data, 0644); err != nil {
					m.statusMsg = fmt.Sprintf("Error writing attachment: %v", err)
					return m
				}
			}
		}

		exported++
	}

	m.statusMsg = fmt.Sprintf("Exported %d emails as %s to %s", exported, formatName, outputDir)
	return m
}

// --------------------------------------------------------------------------
// Layout helpers
// --------------------------------------------------------------------------

// recalcLayout adjusts component sizes after a terminal resize.
func (m Model) recalcLayout() Model {
	// Viewport (preview pane) dimensions use the inner height (excluding border).
	m.viewport.Width = m.previewWidth() - 4 // subtract border + padding
	m.viewport.Height = m.paneInnerHeight()

	m = m.updatePreview()
	return m
}

// paneOuterHeight returns the total height available for each pane including
// its border. We subtract the measured chrome height (title + search + help
// bars) which may be more than 3 lines if any bar text wraps.
func (m Model) paneOuterHeight() int {
	chrome := m.chromeHeight
	if chrome < 3 {
		chrome = 3 // minimum estimate before first render measures it
	}
	h := m.height - chrome
	if h < 3 { // need at least room for top border + 1 content line + bottom border
		h = 3
	}
	return h
}

// paneInnerHeight returns the number of content lines visible inside a pane
// after subtracting the border (2 lines: top + bottom).
func (m Model) paneInnerHeight() int {
	h := m.paneOuterHeight() - 2
	if h < 1 {
		h = 1
	}
	return h
}

// listWidth returns the width of the list pane.
func (m Model) listWidth() int {
	return int(float64(m.width) * listWidthFraction)
}

// previewWidth returns the width of the preview pane.
func (m Model) previewWidth() int {
	return m.width - m.listWidth()
}

// listHeight returns how many email rows are visible in the list pane.
// This must match the actual visible area (inner height), NOT the lipgloss
// pane height, otherwise the scroll offset drifts.
func (m Model) listHeight() int {
	return m.paneInnerHeight()
}

// --------------------------------------------------------------------------
// View helpers
// --------------------------------------------------------------------------

// viewSearchBar renders the search input.
func (m Model) viewSearchBar() string {
	prefix := "  Search: "
	if m.searching {
		return searchBarStyle.Render(prefix + m.searchInput.View())
	}
	if m.searchQuery != "" {
		return searchBarStyle.Render(prefix + m.searchQuery + "  (/ to edit, esc to clear)")
	}
	return searchBarStyle.Render(prefix + "(press / to search)")
}

// viewMainContent renders the list and preview panes side by side.
func (m Model) viewMainContent() string {
	listContent := m.viewList()
	previewContent := m.viewport.View()

	// Choose active/inactive border styles based on focus.
	var listStyle, previewStyle lipgloss.Style
	if m.focusedPane == paneList {
		listStyle = listPaneActiveStyle
		previewStyle = previewPaneStyle
	} else {
		listStyle = listPaneStyle
		previewStyle = previewPaneActiveStyle
	}

	// lipgloss Width() includes padding but NOT border. We subtract 2 for
	// the left+right border characters only. The padding (1 cell each side)
	// is part of the Width value, so lipgloss handles it internally.
	// This means the actual content area (where text wraps) is:
	//   Width - padding = (listWidth-2) - 2 = listWidth-4 = paneInnerW
	listW := m.listWidth() - 2
	previewW := m.previewWidth() - 2

	// innerH is how many lines of content fit inside the pane (excluding border).
	// outerH is the total pane height including border — used for MaxHeight so
	// lipgloss never renders taller than the available space.
	innerH := m.paneInnerHeight()
	outerH := m.paneOuterHeight()

	if listW < 1 {
		listW = 1
	}
	if previewW < 1 {
		previewW = 1
	}

	// Height(innerH) pads short content to fill the pane.
	// MaxHeight(outerH) caps the total rendered height (content + border).
	leftPane := listStyle.Width(listW).Height(innerH).MaxHeight(outerH).Render(listContent)
	rightPane := previewStyle.Width(previewW).Height(innerH).MaxHeight(outerH).Render(previewContent)

	// Join horizontally — this places the two panes side by side.
	return lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
}

// viewList renders the email list with cursor, selection checkboxes, and scrolling.
func (m Model) viewList() string {
	if len(m.filtered) == 0 {
		if m.searchQuery != "" {
			return "No emails match the search."
		}
		return "No emails found."
	}

	var b strings.Builder
	visibleRows := m.listHeight()

	// The pane inner width is what lipgloss gives us for content inside
	// the border and padding. Every list item must fit within this width.
	paneInnerW := m.listWidth() - 4 // border(2) + padding(2)

	// Render only the visible slice of the filtered list.
	end := m.offset + visibleRows
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	// clipStyle hard-clips any line that exceeds the pane width. This is a
	// safety net: even if our width arithmetic is off by a cell (e.g., due to
	// emoji width discrepancies between lipgloss and the actual terminal), the
	// line will be clipped rather than wrapping to a second terminal line.
	clipStyle := lipgloss.NewStyle().MaxWidth(paneInnerW)

	for i := m.offset; i < end; i++ {
		msgIdx := m.filtered[i]
		msg := m.messages[msgIdx]

		// Build the fixed-width prefix parts.
		prefix := "  " // 2 cells: indent for non-cursor items
		if i == m.cursor {
			prefix = "> " // 2 cells: cursor indicator
		}

		checkbox := "[ ] " // 4 cells
		if m.selected[msgIdx] {
			checkbox = "[x] " // 4 cells
		}

		// 📎 takes 2 display cells in the terminal, same as "  ".
		attachIcon := "  " // 2 cells
		if msg.HasAttachments() {
			attachIcon = "📎" // 2 cells
		}

		// Measure the fixed parts using lipgloss to be safe with emoji widths.
		fixedPart := prefix + checkbox + attachIcon + " "
		fixedWidth := lipgloss.Width(fixedPart)

		// Truncate the summary by display width, not byte length.
		// This correctly handles multi-byte UTF-8 characters (ü, ö, etc.)
		// which are 2+ bytes but only 1 display cell.
		summary := msg.Summary()
		maxSummaryW := paneInnerW - fixedWidth
		summary = truncateToWidth(summary, maxSummaryW)

		line := fixedPart + summary

		// Apply styling (color/bold) — this adds ANSI codes but no extra width.
		if i == m.cursor {
			line = listItemCursorStyle.Render(line)
		} else if m.selected[msgIdx] {
			line = listItemCheckedStyle.Render(line)
		} else {
			line = listItemStyle.Render(line)
		}

		// Hard-clip: ensures no line exceeds paneInnerW display cells, even if
		// our measurement was slightly off (e.g., emoji rendered wider than
		// lipgloss expected).
		line = clipStyle.Render(line)

		b.WriteString(line)
		if i < end-1 {
			b.WriteByte('\n')
		}
	}

	return b.String()
}

// viewExportDialog renders the export options as a centered modal overlay.
func (m Model) viewExportDialog() string {
	formats := formatter.List()
	selectedCount := len(m.selected)

	var b strings.Builder

	b.WriteString(exportDialogTitleStyle.Render(
		fmt.Sprintf("Export %d selected email(s)", selectedCount)))
	b.WriteString("\n\n")

	// Format selector.
	b.WriteString("Format (tab to cycle):\n")
	for i, name := range formats {
		if i == m.exportFormat {
			b.WriteString(exportOptionActiveStyle.Render("→ " + name))
		} else {
			b.WriteString(exportOptionStyle.Render("  " + name))
		}
		b.WriteString("\n")
	}

	b.WriteString("\nOutput directory:\n")
	b.WriteString("  " + m.exportDir.View())
	b.WriteString("\n\n")
	b.WriteString(helpBarStyle.Render("enter confirm  tab cycle format  esc cancel"))

	dialog := exportDialogStyle.Render(b.String())

	// Center the dialog in the terminal.
	return lipgloss.Place(m.width, m.height,
		lipgloss.Center, lipgloss.Center,
		dialog)
}

// --------------------------------------------------------------------------
// Utility
// --------------------------------------------------------------------------

// truncateToWidth truncates a string to fit within maxWidth display cells.
// Unlike simple byte slicing, this correctly handles multi-byte UTF-8 characters
// (e.g., "ü" is 2 bytes but 1 display cell) and wide characters (e.g., CJK
// characters that are 1 rune but 2 display cells).
//
// If the string needs truncating, it appends "..." (which takes 3 cells).
func truncateToWidth(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	// Fast path: if it already fits, return as-is.
	w := lipgloss.Width(s)
	if w <= maxWidth {
		return s
	}

	// We need to truncate. Reserve 3 cells for the ellipsis.
	targetWidth := maxWidth - 3
	if targetWidth <= 0 {
		// Not enough room for even "..." — just return what fits.
		return truncateExact(s, maxWidth)
	}

	return truncateExact(s, targetWidth) + "..."
}

// truncateExact returns the longest prefix of s that fits within maxWidth
// display cells, measured by lipgloss.Width (which understands Unicode widths).
func truncateExact(s string, maxWidth int) string {
	result := ""
	currentWidth := 0

	for _, r := range s {
		// Measure what adding this rune would cost in display cells.
		charWidth := lipgloss.Width(string(r))
		if currentWidth+charWidth > maxWidth {
			break
		}
		result += string(r)
		currentWidth += charWidth
	}

	return result
}
