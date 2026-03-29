package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jneuendorf/umbox/formatter"
	"github.com/jneuendorf/umbox/mbox"
	"github.com/spf13/cobra"

	// Blank import to trigger formatter registration via init() functions.
	_ "github.com/jneuendorf/umbox/formatter"
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
	extractCmd.Flags().StringP("output", "o", "./output", "destination directory for extracted files")
	extractCmd.Flags().StringP("format", "f", "raw",
		"output format: "+strings.Join(formatter.List(), ", "))
}

// extractCmd defines the "extract" subcommand.
var extractCmd = &cobra.Command{
	Use:   "extract <mbox-file>",
	Short: "Extract emails from an mbox file",
	Long: `Extract reads an mbox file and saves each email as an individual file.

By default, emails are exported as raw .eml files (RFC 5322 format), which
can be opened by most email clients (Thunderbird, Outlook, Apple Mail, etc.).

Use the --format flag to convert to a human-readable format instead:
  raw        - Standard .eml files (default)
  plaintext  - Simple .txt files
  markdown   - Markdown .md files (renders nicely on GitHub, in VS Code, etc.)

Attachments are saved alongside each email in a subfolder when using
plaintext or markdown formats.

Examples:
  umbox extract inbox.mbox -o ./my-emails
  umbox extract inbox.mbox -f markdown -o ./readable
  umbox extract inbox.mbox -f plaintext`,

	// Args tells cobra to require exactly one positional argument (the mbox file path).
	Args: cobra.ExactArgs(1),

	// RunE is the function that executes when the user runs this command.
	// The "E" suffix means it returns an error (cobra also has "Run" which doesn't).
	RunE: func(cmd *cobra.Command, args []string) error {
		// args[0] is the first positional argument — the mbox file path.
		mboxPath := args[0]

		// GetString retrieves the value of a flag by its name.
		outputDir, _ := cmd.Flags().GetString("output")
		formatName, _ := cmd.Flags().GetString("format")

		return RunExtract(mboxPath, outputDir, formatName)
	},
}

// RunExtract contains the extraction/conversion logic. It's exported (uppercase)
// so the TUI package can call it directly for exporting selected emails.
func RunExtract(mboxPath, outputDir, formatName string) error {
	// Look up the requested formatter in the registry.
	fmtr, err := formatter.Get(formatName)
	if err != nil {
		return err
	}

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

	// Write each message using the chosen formatter.
	for i, msg := range messages {
		if err := writeMessage(fmtr, msg, i, len(messages), outputDir); err != nil {
			return err
		}
	}

	fmt.Printf("\nDone! Extracted %d emails as %s to %s\n", len(messages), formatName, outputDir)
	return nil
}

// writeMessage writes a single email to disk using the given formatter.
// This is also used by the TUI's export function.
func writeMessage(fmtr formatter.Formatter, msg *mbox.Message, index, total int, outputDir string) error {
	// %03d pads the number with zeros (001, 002, ..., 999) so files sort correctly.
	filename := fmt.Sprintf("%03d%s", index+1, fmtr.Extension())
	filePath := filepath.Join(outputDir, filename)

	// Format the message into a buffer first, then write to disk.
	var buf bytes.Buffer
	if err := fmtr.Format(msg, &buf); err != nil {
		return fmt.Errorf("failed to format message %d: %w", index+1, err)
	}

	// os.WriteFile writes bytes to a file, creating it if it doesn't exist.
	// 0644 means owner can read/write, others can only read.
	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", filePath, err)
	}

	// Save attachments to a subfolder (only for non-raw formats, since .eml
	// already contains attachments inline).
	if fmtr.Name() != "raw" && msg.HasAttachments() {
		attDir := filepath.Join(outputDir, fmt.Sprintf("%03d_attachments", index+1))
		if err := os.MkdirAll(attDir, 0755); err != nil {
			return fmt.Errorf("failed to create attachments directory: %w", err)
		}

		for j, att := range msg.Attachments {
			attFilename := att.Filename
			if attFilename == "" {
				// Some attachments don't have filenames — give them a generic one.
				attFilename = fmt.Sprintf("attachment_%d", j+1)
			}
			attPath := filepath.Join(attDir, attFilename)
			if err := os.WriteFile(attPath, att.Data, 0644); err != nil {
				return fmt.Errorf("failed to write attachment %s: %w", attPath, err)
			}
		}

		fmt.Printf("  [%d/%d] %s → %s (+ %d attachments)\n",
			index+1, total, msg.Summary(), filename, len(msg.Attachments))
	} else {
		fmt.Printf("  [%d/%d] %s → %s\n", index+1, total, msg.Summary(), filename)
	}

	return nil
}
