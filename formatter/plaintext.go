package formatter

import (
	"fmt"
	"io"
	"strings"

	"github.com/jneuendorf/umbox/mbox"
)

// init() runs automatically when this package is imported.
// It registers the plaintext formatter so the CLI can find it by name.
func init() {
	Register(&PlainTextFormatter{})
}

// PlainTextFormatter converts emails to simple .txt files.
// The struct is empty because this formatter has no configuration — but it
// still needs to be a struct to implement the Formatter interface methods.
type PlainTextFormatter struct{}

// Name returns "plaintext" — the value users pass to --format.
func (f *PlainTextFormatter) Name() string {
	return "plaintext"
}

// Extension returns ".txt" for output files.
func (f *PlainTextFormatter) Extension() string {
	return ".txt"
}

// Format writes a human-readable plain text representation of the email.
func (f *PlainTextFormatter) Format(msg *mbox.Message, w io.Writer) error {
	// fmt.Fprintf writes formatted text to any io.Writer (file, buffer, etc.).
	// It's like printf in C or fprintf in Python.

	// Write the email header section.
	fmt.Fprintf(w, "From:    %s\n", msg.From)
	fmt.Fprintf(w, "To:      %s\n", strings.Join(msg.To, ", "))
	fmt.Fprintf(w, "Date:    %s\n", msg.Date.Format("Mon, 02 Jan 2006 15:04:05 -0700"))
	fmt.Fprintf(w, "Subject: %s\n", msg.Subject)

	// Print a separator line between headers and body.
	fmt.Fprintln(w, strings.Repeat("-", 60))
	fmt.Fprintln(w)

	// Prefer plain text body; fall back to HTML body if that's all we have.
	body := msg.TextBody
	if body == "" {
		body = msg.HTMLBody
	}
	fmt.Fprintln(w, body)

	// List attachments at the bottom if there are any.
	if msg.HasAttachments() {
		fmt.Fprintln(w)
		fmt.Fprintln(w, strings.Repeat("-", 60))
		fmt.Fprintf(w, "Attachments (%d):\n", len(msg.Attachments))
		for i, att := range msg.Attachments {
			fmt.Fprintf(w, "  %d. %s (%s, %d bytes)\n", i+1, att.Filename, att.ContentType, len(att.Data))
		}
	}

	return nil
}
