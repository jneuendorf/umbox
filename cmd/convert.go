package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jneuendorf/umbox/formatter"
	"github.com/jneuendorf/umbox/mbox"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(convertCmd)

	convertCmd.Flags().StringP("output", "o", "./output", "destination directory for converted files")
	convertCmd.Flags().StringP("format", "f", "plaintext", "output format: "+strings.Join(formatter.List(), ", "))
}

var convertCmd = &cobra.Command{
	Use:   "convert <mbox-file>",
	Short: "Convert emails from an mbox file to a human-readable format",
	Long: `Convert reads an mbox file and saves each email in a human-readable format.

Available formats:
  plaintext  - Simple .txt files
  markdown   - Markdown .md files (renders nicely on GitHub, in VS Code, etc.)

Attachments are saved alongside each email in a subfolder.

Examples:
  umbox convert inbox.mbox -f markdown -o ./readable
  umbox convert inbox.mbox -f plaintext`,

	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		mboxPath := args[0]
		outputDir, _ := cmd.Flags().GetString("output")
		formatName, _ := cmd.Flags().GetString("format")

		return runConvert(mboxPath, outputDir, formatName)
	},
}

// runConvert contains the conversion logic, separated for reuse by a future TUI.
func runConvert(mboxPath, outputDir, formatName string) error {
	// Look up the requested formatter in the registry.
	// We name this "fmtr" (not "fmt") to avoid shadowing the "fmt" package import.
	// In Go, if you name a local variable the same as an import, the import becomes
	// inaccessible in that scope — a common gotcha!
	fmtr, err := formatter.Get(formatName)
	if err != nil {
		return err
	}

	// Parse the mbox file.
	fmt.Printf("Parsing %s...\n", mboxPath)
	messages, err := mbox.Parse(mboxPath)
	if err != nil {
		return err
	}
	fmt.Printf("Found %d emails\n", len(messages))

	// Create the output directory.
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Convert each message.
	for i, msg := range messages {
		// Create a numbered filename with the formatter's extension.
		filename := fmt.Sprintf("%03d%s", i+1, fmtr.Extension())
		filePath := filepath.Join(outputDir, filename)

		// Create the output file.
		file, err := os.Create(filePath)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", filePath, err)
		}

		// Run the formatter, writing to the file.
		if err := fmtr.Format(msg, file); err != nil {
			file.Close()
			return fmt.Errorf("failed to format message %d: %w", i+1, err)
		}
		file.Close()

		// Save attachments to a subfolder alongside the email.
		if msg.HasAttachments() {
			attDir := filepath.Join(outputDir, fmt.Sprintf("%03d_attachments", i+1))
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
				i+1, len(messages), msg.Summary(), filename, len(msg.Attachments))
		} else {
			fmt.Printf("  [%d/%d] %s → %s\n", i+1, len(messages), msg.Summary(), filename)
		}
	}

	fmt.Printf("\nDone! Converted %d emails to %s format in %s\n", len(messages), formatName, outputDir)
	return nil
}
