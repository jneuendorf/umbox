// Package formatter defines the interface for converting parsed email messages
// into human-readable output formats. It uses a registry pattern so new formats
// can be added easily without modifying existing code.
//
// To add a new format:
//  1. Create a new file in this package (e.g., "html.go").
//  2. Define a struct that implements the Formatter interface.
//  3. In an init() function, call Register() to register your formatter.
//
// That's it! The CLI will automatically pick up the new format.
package formatter

import (
	"io"

	"github.com/jneuendorf/umbox/mbox"
)

// Formatter is an interface that defines how to convert an email message into
// a specific output format.
//
// In Go, an interface is a set of method signatures. Any type that implements
// all the methods automatically satisfies the interface — no "implements" keyword
// needed. This is called "structural typing" or "duck typing."
//
// To create a new output format, you just need a struct with these three methods.
type Formatter interface {
	// Name returns the formatter's name (e.g., "plaintext", "markdown").
	// This is what users pass to the --format flag on the CLI.
	Name() string

	// Extension returns the file extension for output files (e.g., ".txt", ".md").
	Extension() string

	// Format writes the formatted email to the given writer.
	// io.Writer is a Go interface that anything writable implements (files, buffers, etc.).
	// This design lets us write to files, stdout, or network connections equally well.
	Format(msg *mbox.Message, w io.Writer) error
}
