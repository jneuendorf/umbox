package cmd

import (
	"github.com/jneuendorf/umbox/tui"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(browseCmd)
}

// browseCmd launches the interactive TUI for browsing emails.
// This is intentionally a thin wrapper — all logic lives in the tui package.
var browseCmd = &cobra.Command{
	Use:   "browse <mbox-file>",
	Short: "Interactively browse emails in an mbox file",
	Long: `Browse opens a terminal UI for viewing emails in an mbox file.

You can navigate the email list, preview message contents, search/filter,
select individual emails, and export them to various formats.

Key bindings:
  ↑/↓ or j/k   Navigate the email list
  tab           Switch focus between list and preview panes
  space         Toggle selection on the current email
  a             Select/deselect all emails
  /             Search (filters by from, to, subject, body)
  e             Export selected emails
  q             Quit

Example:
  umbox browse inbox.mbox`,

	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Run(args[0])
	},
}
