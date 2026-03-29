package formatter

import (
	"fmt"
	"io"
	"strings"

	"github.com/jneuendorf/umbox/mbox"
)

// init registers the markdown formatter automatically on package import.
func init() {
	Register(&MarkdownFormatter{})
}

// MarkdownFormatter converts emails to Markdown (.md) files.
// Markdown is a lightweight markup language that renders nicely on GitHub,
// in VS Code, and in many other tools.
type MarkdownFormatter struct{}

func (f *MarkdownFormatter) Name() string {
	return "markdown"
}

func (f *MarkdownFormatter) Extension() string {
	return ".md"
}

// Format writes a Markdown-formatted representation of the email.
func (f *MarkdownFormatter) Format(msg *mbox.Message, w io.Writer) error {
	// Use the subject as a Markdown heading.
	fmt.Fprintf(w, "# %s\n\n", msg.Subject)

	// Email metadata in a Markdown table for clean formatting.
	fmt.Fprintln(w, "| Field   | Value |")
	fmt.Fprintln(w, "|---------|-------|")
	fmt.Fprintf(w, "| **From**    | %s |\n", msg.From)
	fmt.Fprintf(w, "| **To**      | %s |\n", strings.Join(msg.To, ", "))
	fmt.Fprintf(w, "| **Date**    | %s |\n", msg.Date.Format("Mon, 02 Jan 2006 15:04:05 -0700"))
	fmt.Fprintln(w)

	// Horizontal rule to separate headers from body.
	fmt.Fprintln(w, "---")
	fmt.Fprintln(w)

	// Write the body. Prefer plain text; fall back to HTML.
	body := msg.TextBody
	if body == "" {
		body = msg.HTMLBody
	}
	fmt.Fprintln(w, body)

	// List attachments as a Markdown section with a bullet list.
	if msg.HasAttachments() {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "---")
		fmt.Fprintf(w, "\n## Attachments (%d)\n\n", len(msg.Attachments))
		for i, att := range msg.Attachments {
			// The backticks render the filename in monospace font.
			fmt.Fprintf(w, "%d. `%s` — %s, %d bytes\n", i+1, att.Filename, att.ContentType, len(att.Data))
		}
	}

	return nil
}
