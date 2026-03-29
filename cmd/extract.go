package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jneuendorf/umbox/mbox"
	"github.com/spf13/cobra"
)

// init is called automatically when this package is loaded.
// It registers the "extract" subcommand with the root command.
func init() {
	// AddCommand tells cobra that "extract" is a subcommand of the root command.
	// So the user runs: umbox extract <file> -o <dir>
	rootCmd.AddCommand(extractCmd)

	// Flags define the command-line options. "StringP" creates a flag that takes
	// a string value. The "P" suffix means it has both a long form (--output)
	// and a short form (-o).
	//
	// Parameters: long name, short name, default value, description
	extractCmd.Flags().StringP("output", "o", "./output", "destination directory for extracted .eml files")
}

// extractCmd defines the "extract" subcommand.
var extractCmd = &cobra.Command{
	Use:   "extract <mbox-file>",
	Short: "Extract emails from an mbox file as individual .eml files",
	Long: `Extract reads an mbox file and saves each email as a separate .eml file.

EML is the standard RFC 5322 email format. These files can be opened by most
email clients (Thunderbird, Outlook, Apple Mail, etc.).

Example:
  umbox extract inbox.mbox -o ./my-emails`,

	// Args tells cobra to require exactly one positional argument (the mbox file path).
	Args: cobra.ExactArgs(1),

	// RunE is the function that executes when the user runs this command.
	// The "E" suffix means it returns an error (cobra also has "Run" which doesn't).
	RunE: func(cmd *cobra.Command, args []string) error {
		// args[0] is the first positional argument — the mbox file path.
		mboxPath := args[0]

		// GetString retrieves the value of a flag by its name.
		outputDir, _ := cmd.Flags().GetString("output")

		return runExtract(mboxPath, outputDir)
	},
}

// runExtract contains the actual extraction logic, separated from the cobra
// command definition so it can be reused by other code (e.g., a future TUI).
func runExtract(mboxPath, outputDir string) error {
	// Parse the mbox file into individual messages.
	fmt.Printf("Parsing %s...\n", mboxPath)
	messages, err := mbox.Parse(mboxPath)
	if err != nil {
		return err
	}
	fmt.Printf("Found %d emails\n", len(messages))

	// Create the output directory. os.MkdirAll creates the directory and any
	// missing parent directories (like "mkdir -p" in the shell).
	// 0755 is the Unix permission mode: owner can read/write/execute,
	// others can read/execute.
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Write each message as a .eml file.
	for i, msg := range messages {
		// %03d pads the number with zeros (001, 002, ..., 999) so files sort correctly.
		filename := fmt.Sprintf("%03d.eml", i+1)
		filePath := filepath.Join(outputDir, filename)

		// os.WriteFile writes bytes to a file, creating it if it doesn't exist.
		// 0644 means owner can read/write, others can only read.
		if err := os.WriteFile(filePath, msg.RawBytes, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", filePath, err)
		}

		fmt.Printf("  [%d/%d] %s → %s\n", i+1, len(messages), msg.Summary(), filename)
	}

	fmt.Printf("\nDone! Extracted %d emails to %s\n", len(messages), outputDir)
	return nil
}
