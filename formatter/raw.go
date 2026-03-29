package formatter

import (
	"io"

	"github.com/jneuendorf/umbox/mbox"
)

// init registers the raw formatter automatically on package import.
func init() {
	Register(&RawFormatter{})
}

// RawFormatter writes emails in their original RFC 5322 format (.eml).
// This is the "no conversion" option — it just dumps the raw bytes as they
// appeared in the mbox file. The resulting .eml files can be opened by any
// email client (Thunderbird, Outlook, Apple Mail, etc.).
type RawFormatter struct{}

func (f *RawFormatter) Name() string      { return "raw" }
func (f *RawFormatter) Extension() string { return ".eml" }

// Format writes the raw email bytes to the writer — no transformation at all.
func (f *RawFormatter) Format(msg *mbox.Message, w io.Writer) error {
	_, err := w.Write(msg.RawBytes)
	return err
}
