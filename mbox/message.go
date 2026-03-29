// Package mbox provides functionality for parsing mbox files and working with
// email messages. This package is designed to be reusable — the CLI commands
// use it, and a future TUI will use the same functions.
package mbox

import (
	"time"
)

// Message represents a single parsed email message.
// In Go, a "struct" is like a class in other languages — it groups related data together.
// The fields use Go's type system: string for text, time.Time for dates, etc.
type Message struct {
	// MessageID is the unique identifier from the email's "Message-ID" header.
	MessageID string

	// From is the sender's email address (e.g., "alice@example.com").
	From string

	// To is the list of recipients. It's a "slice" (Go's dynamic array/list).
	To []string

	// Subject is the email's subject line.
	Subject string

	// Date is when the email was sent, parsed into Go's time.Time type.
	Date time.Time

	// Headers stores ALL email headers as key-value pairs.
	// map[string][]string means: keys are strings, values are slices of strings.
	// Some headers (like "Received") can appear multiple times, hence the slice.
	Headers map[string][]string

	// TextBody is the plain text version of the email body.
	TextBody string

	// HTMLBody is the HTML version of the email body (if present).
	HTMLBody string

	// Attachments holds any files attached to the email.
	Attachments []Attachment

	// RawBytes is the complete, unmodified email content as it appeared in the mbox file.
	// This is useful for exporting to .eml format without any modification.
	RawBytes []byte
}

// Attachment represents a file attached to an email.
type Attachment struct {
	// Filename is the name of the attached file (e.g., "report.pdf").
	Filename string

	// ContentType is the MIME type (e.g., "application/pdf", "image/png").
	ContentType string

	// Data holds the raw bytes of the attachment content.
	Data []byte
}

// HasAttachments returns true if the message has any attachments.
// In Go, methods are defined outside the struct — the "(m *Message)" part
// means this method belongs to the Message type.
// The "*" means we receive a pointer (reference) to the Message, not a copy.
func (m *Message) HasAttachments() bool {
	return len(m.Attachments) > 0
}

// Summary returns a short one-line summary of the message, useful for listings.
func (m *Message) Summary() string {
	// Sprintf is Go's equivalent of Python's f-strings or JS template literals.
	// It formats a string using placeholders like %s (string) and %s (another string).
	date := m.Date.Format("2006-01-02") // Go uses this specific reference date for formatting
	return date + " | " + m.From + " | " + m.Subject
}
