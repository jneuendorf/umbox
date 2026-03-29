// Package cmd defines all CLI commands for umbox.
// Each file in this package adds one subcommand (extract, convert, etc.).
// We use the "cobra" library which is the standard way to build CLIs in Go
// (used by kubectl, docker, hugo, and many other popular tools).
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// rootCmd is the base command — what runs when the user just types "umbox"
// without any subcommand. It shows help text and usage information.
//
// cobra.Command is a struct with many optional fields. We only set the ones
// we need. The "&" creates a pointer to the struct.
var rootCmd = &cobra.Command{
	// Use is the one-line usage pattern shown in help text.
	Use: "umbox",

	// Short is a brief description shown alongside the command name.
	Short: "Extract and convert emails from mbox files",

	// Long is the full description shown in the command's help page.
	Long: `umbox is a CLI tool for working with mbox email archive files.

It can extract individual emails as .eml files and convert them to
human-readable formats like plain text or Markdown.

Use "umbox [command] --help" for more information about a command.`,
}

// Execute is called from main.go. It runs the root command which will
// dispatch to the appropriate subcommand based on the user's input.
func Execute() {
	// If any command returns an error, print it and exit with status code 1.
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
